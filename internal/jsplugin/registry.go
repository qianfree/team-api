package jsplugin

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// 说明：按生成式 dao 惯例应由 `make dao` 生成访问层；该命令需可达数据库，
// 当前以 g.DB().Model 直访 + 本文件内的行结构承担，生成条件具备后可替换。
const tableName = "js_plugin_versions"

// pluginOpCode JS 插件管理操作的用户可修复校验错误码（HTTP 422）。
// 不带码的 gerror 会被 response.Error 当作内部错误脱敏成「服务器内部错误」，
// 管理员在前端看不到真实原因（只能翻服务端日志）——版本冲突、路由冲突、
// 状态限制等校验文案本身就是给上传者看的，必须带业务码透出。
func pluginOpCode() gcode.Code {
	return gcode.New(consts.CodeJSPluginOperation, "", nil)
}

// pluginOpError 创建带业务码的校验错误（Newf 语义）。
func pluginOpError(format string, args ...any) error {
	return gerror.NewCodef(pluginOpCode(), format, args...)
}

// PluginVersionRow js_plugin_versions 行。
type PluginVersionRow struct {
	Id         int64           `json:"id"`
	PluginKey  string          `json:"plugin_key"`
	Version    string          `json:"version"`
	Source     string          `json:"source"` // plugin.js 源码原文
	Origin     string          `json:"origin"` // 来源：builtin | upload（默认 upload）
	Icon       string          `json:"icon"`   // 品牌图标 data URI（上传版本；空 = 用 manifest.icon）
	SourceHash string          `json:"source_hash"`
	Compiled   string          `json:"compiled"`
	Manifest   json.RawMessage `json:"manifest"`
	Enabled    bool            `json:"enabled"`
	Active     bool            `json:"active"`
	Remark     string          `json:"remark"`
	CreatedAt  *time.Time      `json:"created_at"`
	UpdatedAt  *time.Time      `json:"updated_at"`
	// Stale 过期内置版本标记（List 时按当前二进制内置清单计算，非 DB 列）：
	// origin=builtin 且 (key,version) 不在二进制内——这类行可手动删除且不会复活
	Stale bool `json:"stale"`
}

// Registry JS 插件注册中心：版本存储 + 生命周期 + generation 重建。
type Registry struct {
	engine Engine

	// rebuildMu 串行化 generation 重建：materialize 含 esbuild+goja 编译（数百 ms），
	// 并发生命周期操作若都读 DB→编译→发布，后完成者可能带着旧快照覆盖新发布
	rebuildMu sync.Mutex
	// compileCache 源码哈希 → 编译产物。rebuild/Bootstrap 每次装载全部插件都会
	// 走 Compile（esbuild 打包 + probe 执行），源码未变的重复编译纯耗 CPU
	compileCache sync.Map

	// builtinLive 当前二进制内置插件的 "key@version" 集合（过期内置版本的判定
	// 基准）。Bootstrap seed 时缓存；未缓存（seed 失败或单测路径）时惰性补算
	builtinMu   sync.Mutex
	builtinLive map[string]struct{}
}

// cacheBuiltinLive 缓存当前二进制内置清单（seedBuiltins 编译后顺手留存，零额外开销）。
func (r *Registry) cacheBuiltinLive(builtins []*CompiledPlugin) {
	r.builtinMu.Lock()
	defer r.builtinMu.Unlock()
	r.builtinLive = buildBuiltinVersionSet(builtins)
}

// builtinVersionSet 返回当前二进制内置插件的 "key@version" 集合。「过期内置版本」
// 判定的权威来源是二进制 embed 源而非 DB 行；未缓存时（seed 失败或单测路径）惰性
// 编译补算，compileCache 保证补算走缓存、无重复打包开销。
func (r *Registry) builtinVersionSet() (map[string]struct{}, error) {
	r.builtinMu.Lock()
	defer r.builtinMu.Unlock()
	if r.builtinLive != nil {
		return r.builtinLive, nil
	}
	builtins, err := compileBuiltins(r.engine)
	if err != nil {
		return nil, err
	}
	r.builtinLive = buildBuiltinVersionSet(builtins)
	return r.builtinLive, nil
}

// registryAdvisoryLockKey 激活路径 pg_advisory_xact_lock 的锁键（任意常量，
// 与其他业务域的 advisory lock 区分即可）。
const registryAdvisoryLockKey = 843_100_031

var defaultRegistry = &Registry{engine: NewGojaEngine()}

// DefaultRegistry 返回进程级注册中心。
func DefaultRegistry() *Registry { return defaultRegistry }

// ---------------------------------------------------------------------------
// 启动装载
// ---------------------------------------------------------------------------

// Bootstrap 启动装载：读取全部 active 版本，enabled 的进 generation
// （在 cmd.go 的 plugin.Bootstrap 之后调用）。
func (r *Registry) Bootstrap(ctx context.Context) error {
	// 内置插件 seed/refresh（go:embed → js_plugin_versions，幂等；失败不阻断启动）
	if err := r.seedBuiltins(ctx); err != nil {
		glog.Warningf(ctx, "jsplugin builtin seed: %v", err)
	}
	rows, err := r.loadActiveRows(ctx)
	if err != nil {
		return err
	}
	var plugins []*ActivePlugin
	for i := range rows {
		row := rows[i]
		if !row.Enabled {
			continue
		}
		active, err := r.materialize(ctx, &row)
		if err != nil {
			glog.Errorf(ctx, "jsplugin %s@%s 装载失败: %v", row.PluginKey, row.Version, err)
			continue
		}
		plugins = append(plugins, active)
	}
	publishGeneration(buildGeneration(plugins))
	glog.Infof(ctx, "jsplugin bootstrap: %d active plugin(s) in generation", len(plugins))
	warnUnboundJSChannels(ctx)
	return nil
}

// jsTaskChannelType JS 任务插件渠道类型（relay/constant.ProviderJSTask）。
const jsTaskChannelType = 43

// warnUnboundJSChannels 启动核查：JS 任务渠道必须绑定插件（settings.plugin_key）。
// 早期版本允许缺省按模型回落解析，该路径已移除——未绑定的存量渠道此后提交即 400，
// 启动时列出渠道 ID 提示管理员补配。仅告警，不阻断启动。
func warnUnboundJSChannels(ctx context.Context) {
	var ids []int64
	rows, err := g.DB().Ctx(ctx).Raw(`
SELECT id FROM chn_channels
WHERE type = ? AND COALESCE(NULLIF(settings::jsonb ->> 'plugin_key', ''), '') = ''
ORDER BY id`, jsTaskChannelType).All()
	if err != nil {
		glog.Warningf(ctx, "jsplugin: 核查未绑定插件的 JS 渠道失败: %v", err)
		return
	}
	for _, row := range rows {
		ids = append(ids, row["id"].Int64())
	}
	if len(ids) > 0 {
		glog.Warningf(ctx, "jsplugin: %d 个 JS 任务渠道未绑定插件（settings.plugin_key），提交将被拒绝，请在渠道管理补配：%v", len(ids), ids)
	}
}

// seedBuiltins 幂等装载内置插件（origin=builtin）：
//   - 同 key+version 已是 builtin 行且源码变化（二进制升级）→ 刷新内容，保留 enabled/active
//   - 同 key+version 是 upload 行 → 不动（上传优先，内置同版本被遮蔽）
//   - 无行 → 插入（origin=builtin，首版本自动 active，enabled 由管理员开启；
//     升级带来的新版本号 inactive 入库，不改变运行行为）
//   - 历次升级遗留的过期内置版本行不自动清理，由管理员在版本历史中选择性删除
//     （见 DeleteVersion 的过期判定）
func (r *Registry) seedBuiltins(ctx context.Context) error {
	builtins, err := compileBuiltins(r.engine)
	if err != nil {
		return err
	}
	r.cacheBuiltinLive(builtins)
	for _, compiled := range builtins {
		hash := sourceHash(compiled.Source)
		existing, err := r.loadRow(ctx, compiled.Key, compiled.Version)
		if err != nil {
			return err
		}
		manifestJSON, _ := json.Marshal(*compiled.Manifest)
		if existing != nil {
			if existing.Origin != "builtin" || existing.SourceHash == hash {
				continue
			}
			if _, err := g.DB().Model(tableName).Ctx(ctx).
				Where("plugin_key", compiled.Key).Where("version", compiled.Version).
				Data(g.Map{
					"source": compiled.Source, "source_hash": hash,
					"compiled": compiled.Bundle, "manifest": manifestJSON,
					"updated_at": time.Now(),
				}).Update(); err != nil {
				return gerror.Wrapf(err, "刷新内置插件 %s 失败", compiled.Key)
			}
			continue
		}
		hasActive, err := r.hasActiveRow(ctx, compiled.Key)
		if err != nil {
			return err
		}
		if _, err := g.DB().Model(tableName).Ctx(ctx).Data(g.Map{
			"plugin_key": compiled.Key, "version": compiled.Version,
			"source": compiled.Source, "source_hash": hash,
			"compiled": compiled.Bundle, "manifest": manifestJSON,
			"origin": "builtin", "enabled": false, "active": !hasActive,
		}).Insert(); err != nil {
			return gerror.Wrapf(err, "写入内置插件 %s 失败", compiled.Key)
		}
		glog.Infof(ctx, "jsplugin builtin seeded: %s@%s (active=%v)", compiled.Key, compiled.Version, !hasActive)
	}
	// 下线清理：二进制内已不存在的内置 key（如 golden 改为纯模板后移出 builtin/），
	// 删除其存量 builtin 行，部署实例升级后真正下线
	return r.removeRetiredBuiltins(ctx, builtins)
}

// retiredBuiltinKeys 找出 DB 中存在 builtin 行、但二进制内已下线的插件 key
// （纯逻辑，可单测）。live 为当前二进制的内置 key 集。
func retiredBuiltinKeys(rows []PluginVersionRow, live map[string]struct{}) []string {
	seen := make(map[string]struct{})
	var out []string
	for i := range rows {
		key := rows[i].PluginKey
		if rows[i].Origin != "builtin" {
			continue
		}
		if _, isActive := live[key]; isActive {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

// buildBuiltinVersionSet 当前二进制内置插件的 "key@version" 集合（纯逻辑，可单测）。
func buildBuiltinVersionSet(builtins []*CompiledPlugin) map[string]struct{} {
	live := make(map[string]struct{}, len(builtins))
	for _, compiled := range builtins {
		live[compiled.Key+"@"+compiled.Version] = struct{}{}
	}
	return live
}

// isStaleBuiltin 版本行是否为「过期内置版本」（纯逻辑，可单测）：origin=builtin 且
// (key,version) 不在当前二进制内置清单（live 为 "key@version" 集合）内。过期行删除
// 后不会被 seed 复活（seed 只补种二进制内存在的版本），是唯一可手动删除的内置形态；
// 当前二进制版本删除属假删除（重启 seed 复活），始终拒绝。
func isStaleBuiltin(row PluginVersionRow, live map[string]struct{}) bool {
	if row.Origin != "builtin" {
		return false
	}
	_, current := live[row.PluginKey+"@"+row.Version]
	return !current
}

// removeRetiredBuiltins 内置插件下线清理：删除已下线 key 的 builtin 行（同 key 的
// upload 行保留——上传版本不受内置下线影响，继续按上传插件服务）；key 失去全部
// 版本行时连带清理 plugin.{key}.* 配置键（对齐 DeletePlugin 的键清理语义，防同 key
// 重新上传时静默继承旧配置值）。随 Bootstrap 在 seed 之后、generation 发布之前执行，
// 清掉的行不会进入本次发布；插件缺席的在途任务由 30 分钟超时网兜底（与 DeletePlugin
// 对在途任务的语义一致）。
func (r *Registry) removeRetiredBuiltins(ctx context.Context, live []*CompiledPlugin) error {
	liveKeys := make(map[string]struct{}, len(live))
	for _, compiled := range live {
		liveKeys[compiled.Key] = struct{}{}
	}
	var builtinRows []PluginVersionRow
	if err := g.DB().Model(tableName).Ctx(ctx).Where("origin", "builtin").Scan(&builtinRows); err != nil {
		return gerror.Wrap(err, "读取内置插件行失败")
	}
	for _, key := range retiredBuiltinKeys(builtinRows, liveKeys) {
		if _, err := g.DB().Model(tableName).Ctx(ctx).
			Where("plugin_key", key).Where("origin", "builtin").Delete(); err != nil {
			return gerror.Wrapf(err, "清理下线内置插件 %s 失败", key)
		}
		count, err := g.DB().Model(tableName).Ctx(ctx).Where("plugin_key", key).Count()
		if err != nil {
			return gerror.Wrapf(err, "核查插件 %s 残留版本失败", key)
		}
		// key 已无任何版本行（无上传版本接盘）时清理配置键；清理失败仅告警——
		// 残留键惰性无害（无人读取），不应阻断启动
		if count == 0 {
			if err := deletePluginConfigKeys(ctx, key); err != nil {
				glog.Warningf(ctx, "jsplugin 下线内置 %s：配置键清理失败（版本已删除，残留键惰性无害）: %v", key, err)
			}
		}
		glog.Infof(ctx, "jsplugin builtin retired: %s（builtin 行已清理，upload 版本不受影响）", key)
	}
	return nil
}

// Shutdown 关闭全部插件池与跨实例失效订阅。
func (r *Registry) Shutdown() {
	stopInvalidateSubscriber()
	for _, p := range CurrentGeneration().Plugins() {
		p.Pool.Close()
	}
}

// ---------------------------------------------------------------------------
// 上传 / 版本 / 启停
// ---------------------------------------------------------------------------

// Upload 上传插件源码：编译校验 + 幂等入库（enabled=false）。
// iconDataURI 可选：品牌图标（SVG/PNG data URI，经 ValidateIconDataURI 安全校验）；
// remark 可选：运营备注（同源码重复上传时非空且不同才更新）。
func (r *Registry) Upload(ctx context.Context, source string, iconDataURI string, remark string) (*contract.Manifest, error) {
	compiled, err := r.engine.Compile(source)
	if err != nil {
		// 编译/manifest 校验失败是源码问题（上传者可修复），带码透出文案
		return nil, gerror.WrapCode(pluginOpCode(), err)
	}
	// 加密配置字段类型限制：加密仅支持 string/text（写入侧只对这两型加密、读取侧
	// 一律 GetSecret 解密），其余类型声明 encrypted 会明文落库——上传期拒绝。
	// 不放 Manifest.Validate：它在 Compile 内还服务存量行装载（materialize），
	// 会让 DB 里的坏行毒化整个 rebuildGeneration（全有或全无的热更新）。
	if err := validateEncryptedConfigTypes(compiled.Manifest); err != nil {
		return nil, gerror.WrapCode(pluginOpCode(), err)
	}
	hash := sourceHash(source)
	manifestJSON, _ := json.Marshal(*compiled.Manifest)

	// 幂等：key+version 已存在且源码一致 → 直接返回；不一致 → 拒绝
	// 品牌图标校验（失败拒绝上传）
	if iconDataURI != "" {
		if _, err := ValidateIconDataURI(iconDataURI); err != nil {
			return nil, gerror.WrapCode(pluginOpCode(), err, "图标校验失败")
		}
	}
	existing, err := r.loadRow(ctx, compiled.Key, compiled.Version)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.SourceHash != hash {
			return nil, pluginOpError("插件 %s@%s 已存在且源码不同（同版本号不允许改内容，请升版本号）", compiled.Key, compiled.Version)
		}
		// 同源码重复上传：允许顺带更新图标与备注（合并为一次 Update）。
		// 图标的内置行例外——icon 列语义为「上传版本的品牌图」（内置版本用
		// manifest.icon），覆盖内置行会破坏该约定；remark 是运营备注，不限来源。
		updates := g.Map{"updated_at": time.Now()}
		if iconDataURI != "" && iconDataURI != existing.Icon && existing.Origin != "builtin" {
			updates["icon"] = iconDataURI
		}
		if remark != "" && remark != existing.Remark {
			updates["remark"] = remark
		}
		if len(updates) > 1 {
			if _, err := g.DB().Model(tableName).Ctx(ctx).
				Where("plugin_key", compiled.Key).Where("version", compiled.Version).
				Data(updates).Update(); err != nil {
				return nil, gerror.Wrap(err, "更新插件图标/备注失败")
			}
		}
		return compiled.Manifest, nil
	}

	// 该 key 首个版本自动 active（仍需显式 Enable 才进 generation）
	hasActive, err := r.hasActiveRow(ctx, compiled.Key)
	if err != nil {
		return nil, err
	}
	_, err = g.DB().Model(tableName).Ctx(ctx).Data(g.Map{
		"plugin_key":  compiled.Key,
		"version":     compiled.Version,
		"source":      source,
		"source_hash": hash,
		"compiled":    compiled.Bundle,
		"manifest":    manifestJSON,
		"origin":      "upload",
		"icon":        iconDataURI,
		"remark":      remark,
		"enabled":     false,
		"active":      !hasActive,
	}).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "写入插件版本失败")
	}
	// 配置默认值种入 sys_options（已存在的键不动）
	// （含宿主内置项，恒执行）
	if err := seedConfigDefaults(ctx, compiled.Manifest); err != nil {
		return nil, gerror.Wrap(err, "插件配置默认值注册失败")
	}
	return compiled.Manifest, nil
}

// PluginSummary 列表视图：按 key 归组的版本信息。
type PluginSummary struct {
	Key          string             `json:"key"`
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	Author       string             `json:"author"`
	Website      string             `json:"website"`
	Origin       string             `json:"origin"` // 激活版本来源：builtin | upload
	ActiveVer    string             `json:"active_version"`
	Enabled      bool               `json:"enabled"`
	InService    bool               `json:"in_service"` // 当前 generation 内
	Models       []string           `json:"models"`     // 支持的上游模型（meta.models 基线）
	FetchMode    string             `json:"fetch_mode"`
	AllowedHosts []string           `json:"allowed_hosts"`
	Icon         string             `json:"icon"`      // manifest 声明：text/text:label/图标名
	HasImage     bool               `json:"has_image"` // 激活版本是否带品牌图片（GET icon 端点）
	Manifest     json.RawMessage    `json:"manifest"`  // 激活版本 manifest 全量
	Versions     []PluginVersionRow `json:"versions"`
}

// List 全量插件列表（按 key 归组，active 优先）。
func (r *Registry) List(ctx context.Context) ([]PluginSummary, error) {
	var rows []PluginVersionRow
	err := g.DB().Model(tableName).Ctx(ctx).
		Order("plugin_key ASC, active DESC, created_at DESC").Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "读取插件列表失败")
	}
	gen := CurrentGeneration()
	// 过期内置版本标记（尽力而为）：清单读取失败时不标记、不阻断列表——删除入口
	// 的守卫（DeleteVersion）才是权威校验
	live, _ := r.builtinVersionSet()
	byKey := map[string]*PluginSummary{}
	var order []string
	for i := range rows {
		row := rows[i]
		if live != nil && isStaleBuiltin(row, live) {
			row.Stale = true
		}
		s, ok := byKey[row.PluginKey]
		if !ok {
			s = &PluginSummary{Key: row.PluginKey}
			byKey[row.PluginKey] = s
			order = append(order, row.PluginKey)
		}
		s.Versions = append(s.Versions, row)
		if row.Active {
			s.ActiveVer = row.Version
			s.Origin = row.Origin
			s.Enabled = row.Enabled
			var m contract.Manifest
			if json.Unmarshal(row.Manifest, &m) == nil {
				s.Name = m.Name
				s.Description = m.Description.String()
				s.Author = m.Author.String()
				s.Website = m.Website
				s.Models = m.Models
				s.FetchMode = m.EffectiveFetchMode()
				s.AllowedHosts = m.AllowedHosts
				s.Icon = m.Icon
				s.HasImage = row.Icon != ""
				s.Manifest = row.Manifest
			}
		}
	}
	out := make([]PluginSummary, 0, len(order))
	for _, key := range order {
		s := byKey[key]
		if _, inService := gen.Plugin(key); inService {
			s.InService = true
		}
		out = append(out, *s)
	}
	return out, nil
}

// Activate 激活指定版本：事务翻 active（继承旧激活版本 enabled）→ 重建 generation。
// enabled 继承：同 key 热切换时新版本沿用旧版本的启用态，插件在 generation 中连续存在、
// 执行中的任务不掉线；旧版本处于禁用态则新版本保持禁用（激活不悄悄复活禁用插件）；
// 该 key 无旧 active 行（首次上线）保持目标行原值，仍需显式 Enable。
// 事务内持 advisory lock 串行化并发激活。跨插件模型同名（上游模型清单）与同形状
// 路由（/pjs/{key} 命名空间隔离）均为合法形态，不做跨插件冲突预检。
func (r *Registry) Activate(ctx context.Context, key, version string) error {
	target, err := r.loadRow(ctx, key, version)
	if err != nil || target == nil {
		return pluginOpError("插件 %s@%s 不存在", key, version)
	}
	var targetManifest contract.Manifest
	if err := json.Unmarshal(target.Manifest, &targetManifest); err != nil {
		return gerror.Wrap(err, "manifest 反序列化失败")
	}

	// 事务：advisory lock 串行化激活 → 清旧 active → 置新 active
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, lockErr := tx.Exec("SELECT pg_advisory_xact_lock(?)", registryAdvisoryLockKey); lockErr != nil {
			return gerror.Wrap(lockErr, "激活锁获取失败")
		}
		// 锁内读 active 行：enabled 继承依据（同 key 旧激活版本的启用态）
		others, err := r.loadActiveRowsTx(ctx, tx)
		if err != nil {
			return err
		}
		if _, err := tx.Model(tableName).Ctx(ctx).
			Where("plugin_key", key).Data(g.Map{"active": false, "updated_at": time.Now()}).Update(); err != nil {
			return err
		}
		if _, err := tx.Model(tableName).Ctx(ctx).
			Where("plugin_key", key).Where("version", version).
			Data(g.Map{"active": true, "enabled": inheritEnabled(key, target.Enabled, others), "updated_at": time.Now()}).Update(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return gerror.Wrap(err, "激活事务失败")
	}
	failed, err := r.rebuildGeneration(ctx)
	if err != nil {
		return err
	}
	r.PublishInvalidate(ctx) // 本实例已重建，广播其余实例（DB 已提交即广播，含装载失败的场景）
	// 事务已提交但目标插件装载失败（如配置解密失败）：明确告知管理员
	// 「已激活但未在服务」，而不是静默留下 DB 与运行时的分叉
	if loadErr, ok := failed[key]; ok {
		return gerror.Wrapf(loadErr, "插件 %s@%s 已激活但装载失败，暂不在服务中（其余插件不受影响）", key, version)
	}
	return nil
}

// inheritEnabled 计算目标行激活后的 enabled：有同 key 旧 active 行则继承其值
// （同 key 热切换不掉出 generation、禁用态不被激活悄悄复活），无旧 active 行
// 保持目标行原值（首个上线版本仍需显式 Enable）。others 为事务内读到的全部
// active 行（含同 key 旧行），无需额外查询。
func inheritEnabled(key string, targetEnabled bool, others []PluginVersionRow) bool {
	for i := range others {
		if others[i].PluginKey == key {
			return others[i].Enabled
		}
	}
	return targetEnabled
}

// Enable 启用/禁用插件（作用于 active 版本行）。禁用即从 generation 摘除，
// 池关闭；启用即重新装载。
func (r *Registry) Enable(ctx context.Context, key string, enabled bool) error {
	res, err := g.DB().Model(tableName).Ctx(ctx).
		Where("plugin_key", key).Where("active", true).
		Data(g.Map{"enabled": enabled, "updated_at": time.Now()}).Update()
	if err != nil {
		return gerror.Wrap(err, "更新启用状态失败")
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return pluginOpError("插件 %s 没有激活版本", key)
	}
	failed, err := r.rebuildGeneration(ctx)
	if err != nil {
		return err
	}
	r.PublishInvalidate(ctx) // 本实例已重建，广播其余实例
	if enabled {
		if loadErr, ok := failed[key]; ok {
			return gerror.Wrapf(loadErr, "插件 %s 已启用但装载失败，暂不在服务中（其余插件不受影响）", key)
		}
	}
	return nil
}

// VersionSource 返回指定版本的源码及元信息（管理后台只读查看用）。
func (r *Registry) VersionSource(ctx context.Context, key, version string) (*PluginVersionRow, error) {
	row, err := r.loadRow(ctx, key, version)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, pluginOpError("插件 %s@%s 不存在", key, version)
	}
	return row, nil
}

// GetPluginIcon 返回激活版本的品牌图标（优先上传图片 data URI，无则空）。
// 图标端点每请求都打到 DB，按 key 单行查询而不是全量 active 行内存过滤。
func (r *Registry) GetPluginIcon(ctx context.Context, key string) (string, error) {
	row, err := r.loadActiveRow(ctx, key)
	if err != nil {
		return "", err
	}
	if row == nil {
		return "", pluginOpError("插件 %s 没有激活版本", key)
	}
	return row.Icon, nil
}

// ActiveManifest 返回插件激活版本的 manifest（不要求 enabled——禁用状态下配置仍可读写）。
func (r *Registry) ActiveManifest(ctx context.Context, key string) (*contract.Manifest, error) {
	row, err := r.loadActiveRow(ctx, key)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, pluginOpError("插件 %s 没有激活版本", key)
	}
	var m contract.Manifest
	if err := json.Unmarshal(row.Manifest, &m); err != nil {
		return nil, gerror.Wrap(err, "manifest 反序列化失败")
	}
	return &m, nil
}

// ActiveManifests 返回全部 active 版本的 manifest（不筛 enabled——禁用插件的
// 历史统计与模型映射也需要）。manifest 解析失败的行跳过并记 Warning：
// 单行脏数据不应让整份统计失效（对齐 Bootstrap/rebuild 的跳过容错策略）。
func (r *Registry) ActiveManifests(ctx context.Context) ([]*contract.Manifest, error) {
	rows, err := r.loadActiveRows(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*contract.Manifest, 0, len(rows))
	for i := range rows {
		var m contract.Manifest
		if uErr := json.Unmarshal(rows[i].Manifest, &m); uErr != nil {
			glog.Warningf(ctx, "jsplugin manifest 解析失败（跳过该行） %s@%s: %v", rows[i].PluginKey, rows[i].Version, uErr)
			continue
		}
		out = append(out, &m)
	}
	return out, nil
}

// UpdatePluginConfig 更新插件配置并重建 generation（在服务中的插件即刻装载新值）。
func (r *Registry) UpdatePluginConfig(ctx context.Context, key string, values map[string]any) error {
	m, err := r.ActiveManifest(ctx, key)
	if err != nil {
		return err
	}
	if err := UpdateConfig(ctx, key, m, values); err != nil {
		return err
	}
	failed, err := r.rebuildGeneration(ctx)
	if err != nil {
		return err
	}
	r.PublishInvalidate(ctx) // 本实例已重建，广播其余实例
	if loadErr, ok := failed[key]; ok {
		// 部分成功状态：配置已落库但新实例装载失败，运行中仍是旧值——管理员必须
		// 在前端看到（带码透出），否则只能翻日志才能发现配置未生效
		return gerror.WrapCode(pluginOpCode(), loadErr,
			fmt.Sprintf("插件 %s 配置已保存但装载失败，运行中仍是旧配置（其余插件不受影响）", key))
	}
	return nil
}

// DeleteVersion 删除版本：内置不可删；active+enabled 拒绝删；删除激活中的
// 上传版本后，同 key 回落激活最新内置版本（若有）。
func (r *Registry) DeleteVersion(ctx context.Context, key, version string) error {
	row, err := r.loadRow(ctx, key, version)
	if err != nil || row == nil {
		return pluginOpError("插件 %s@%s 不存在", key, version)
	}
	if row.Origin == "builtin" {
		// 内置版本仅放行「过期」行（不在当前二进制内——删除后 seed 不会复活，
		// 供管理员清理历次升级遗留的版本记录）；当前二进制版本删除属假删除
		//（重启 seed 复活），始终拒绝
		live, err := r.builtinVersionSet()
		if err != nil {
			return gerror.Wrap(err, "读取内置插件清单失败")
		}
		if !isStaleBuiltin(*row, live) {
			return pluginOpError("当前二进制内置版本不可删除（重启会随 seed 复活）；历次升级遗留的过期内置版本可删除")
		}
	}
	if row.Active && row.Enabled {
		return pluginOpError("该版本激活且启用中，请先禁用或切换激活版本")
	}
	if _, err := g.DB().Model(tableName).Ctx(ctx).
		Where("plugin_key", key).Where("version", version).Delete(); err != nil {
		return gerror.Wrap(err, "删除版本失败")
	}
	if row.Active {
		if err := r.fallbackActivateBuiltin(ctx, key); err != nil {
			return err
		}
		_, rbErr := r.rebuildGeneration(ctx)
		if rbErr == nil {
			r.PublishInvalidate(ctx) // 本实例已重建，广播其余实例
		}
		return rbErr
	}
	return nil
}

// fallbackActivateBuiltin 删除激活中的上传版本后回落：key 失去 active 行时，
// 激活最新内置版本（enabled 保持该行原值——回落激活≠自动启用，启停仍是管理员动作）。
// 版本比较用 compareSemver 而非字符串 DESC——字典序会把 0.10.0 排在 0.9.0 之前、
// prerelease（1.0.0-beta）排在正式版之前，选错「最新」。
func (r *Registry) fallbackActivateBuiltin(ctx context.Context, key string) error {
	hasActive, err := r.hasActiveRow(ctx, key)
	if err != nil || hasActive {
		return err
	}
	var rows []PluginVersionRow
	if err := g.DB().Model(tableName).Ctx(ctx).
		Where("plugin_key", key).Where("origin", "builtin").Scan(&rows); err != nil {
		return gerror.Wrap(err, "回落内置版本失败")
	}
	if len(rows) == 0 {
		return nil
	}
	latest := rows[0]
	for _, row := range rows[1:] {
		if compareSemver(row.Version, latest.Version) > 0 {
			latest = row
		}
	}
	_, err = g.DB().Model(tableName).Ctx(ctx).
		Where("id", latest.Id).
		Data(g.Map{"active": true, "updated_at": time.Now()}).Update()
	return gerror.Wrap(err, "回落内置版本失败")
}

// PluginDeleteCheck 整删预检结果。
type PluginDeleteCheck struct {
	Allowed      bool
	Reason       string // 不允许时的中文原因（可直接透出给确认弹窗）
	VersionCount int
}

// checkDeletePluginRows 整删预检纯逻辑（无 DB 依赖，可单测）：
// key 不存在 / 含内置版本（平台基座，随启动 seed 会复活）/ 启用中 → 拒绝。
func checkDeletePluginRows(rows []PluginVersionRow) PluginDeleteCheck {
	if len(rows) == 0 {
		return PluginDeleteCheck{Allowed: false, Reason: "插件不存在或已删除"}
	}
	for i := range rows {
		if rows[i].Origin == "builtin" {
			return PluginDeleteCheck{Allowed: false,
				Reason: "该插件标识存在内置版本（平台基座），不可整删，请在版本历史中逐个删除版本（当前二进制内置版本除外，会随重启复活）"}
		}
		if rows[i].Active && rows[i].Enabled {
			return PluginDeleteCheck{Allowed: false, Reason: "插件启用中，请先禁用再删除"}
		}
	}
	return PluginDeleteCheck{Allowed: true, VersionCount: len(rows)}
}

// CheckDeletePlugin 整删预检（删除确认弹窗数据源，与 DeletePlugin 共用同一判定）。
func (r *Registry) CheckDeletePlugin(ctx context.Context, key string) (*PluginDeleteCheck, error) {
	rows, err := r.loadKeyRows(ctx, key)
	if err != nil {
		return nil, err
	}
	check := checkDeletePluginRows(rows)
	return &check, nil
}

// DeletePlugin 删除整个上传插件：全部版本行 + sys_options 的 plugin.{key}.* 配置键。
// 配置键必须连带清理——seedConfigDefaults「已存在的键不动」，残留键会让同 key
// 重新上传的新插件静默继承旧配置值（含加密密钥）。在途任务不阻断删除：轮询遇
// 插件缺席保持非终态等回归，由 30 分钟超时网（handleTimedOutTasks → 失败 + 退款）
// 兜底；历史任务 / 计费 / 用量 / 产物记录保留，不受影响。
func (r *Registry) DeletePlugin(ctx context.Context, key string) error {
	rows, err := r.loadKeyRows(ctx, key)
	if err != nil {
		return err
	}
	if check := checkDeletePluginRows(rows); !check.Allowed {
		return pluginOpError("%s", check.Reason)
	}
	// 版本行先删：配置键清理失败不回滚（残留键惰性无害，无人读取），告警留痕
	if _, err := g.DB().Model(tableName).Ctx(ctx).Where("plugin_key", key).Delete(); err != nil {
		return gerror.Wrap(err, "删除插件版本失败")
	}
	if err := deletePluginConfigKeys(ctx, key); err != nil {
		glog.Warningf(ctx, "jsplugin 整删 %s：配置键清理失败（版本已删除，残留键惰性无害）: %v", key, err)
	}
	_, rbErr := r.rebuildGeneration(ctx)
	if rbErr == nil {
		r.PublishInvalidate(ctx) // 本实例已重建，广播其余实例
	}
	return rbErr
}

// ---------------------------------------------------------------------------
// 内部
// ---------------------------------------------------------------------------

// materialize 从行构造 ActivePlugin（编译缓存命中则免编译：源码哈希未变的
// 重复装载直接复用 esbuild 产物，rebuild/Bootstrap 不再重复打包执行）。
func (r *Registry) materialize(ctx context.Context, row *PluginVersionRow) (*ActivePlugin, error) {
	compiled, err := r.compileCached(row.Source)
	if err != nil {
		return nil, err
	}
	// 配置值装载（含加密字段解密），随实例注入 utils.config.get
	configValues, err := loadPluginConfig(ctx, compiled.Manifest)
	if err != nil {
		return nil, err
	}
	opts := InstanceOptions{
		AllowedHosts: compiled.Manifest.AllowedHosts,
		ConfigValues: configValues,
	}
	// modelsFromConfig 扩展模型并入 Manifest 内存视图（副本——compiled.Manifest 为
	// 编译缓存共享对象，禁止原地改写）。解析失败仅告警降级静态清单：存量坏配置
	// 不应让整个插件装载失败（对齐 hookCache 探测失败的容错哲学）。
	manifestView := compiled.Manifest
	if compiled.Manifest.ModelsFromConfig != "" {
		view, cfgErr := applyConfigModels(compiled.Manifest, configValues)
		if cfgErr != nil {
			glog.Warningf(ctx, "jsplugin %s: 扩展模型配置解析失败（按静态清单装载）: %v", compiled.Key, cfgErr)
		} else {
			manifestView = view
		}
	}
	pool := NewPluginPool(r.engine, compiled, opts)
	return &ActivePlugin{
		Key:       compiled.Key,
		Version:   compiled.Version,
		Manifest:  manifestView,
		Compiled:  compiled,
		Pool:      pool,
		hookCache: buildHookCache(r.engine, compiled, opts),
	}, nil
}

// rebuildGeneration 从 DB 重建：active 且 enabled 的版本全部装载后原子发布。
// 池策略：整建整拆——重建时关闭旧池，简单且避免半开状态。
//
// 互斥 + 容错：rebuildMu 串行化重建（materialize 含数百 ms 编译，无锁时后完成者
// 可能带旧快照覆盖新发布）；单个插件装载失败跳过并记日志（对齐 Bootstrap）——
// 此前返回错误会让一个坏插件卡死所有插件的生命周期操作，且 Activate 事务已提交、
// generation 未发布，DB 与运行时持续分叉。跳过策略下受影响插件的调用方（Enable/
// Activate）会从 failed 集合得知自己的目标没装载，向管理员报错。
func (r *Registry) rebuildGeneration(ctx context.Context) (map[string]error, error) {
	r.rebuildMu.Lock()
	defer r.rebuildMu.Unlock()
	// 锁内重读 DB：排队期间的先行操作（激活/启停）不被旧快照吞掉
	rows, err := r.loadActiveRows(ctx)
	if err != nil {
		return nil, err
	}
	var plugins []*ActivePlugin
	failed := make(map[string]error)
	for i := range rows {
		row := rows[i]
		if !row.Enabled {
			continue
		}
		active, mErr := r.materialize(ctx, &row)
		if mErr != nil {
			glog.Errorf(ctx, "jsplugin %s@%s 装载失败（本次重建跳过）: %v", row.PluginKey, row.Version, mErr)
			failed[row.PluginKey] = mErr
			continue
		}
		plugins = append(plugins, active)
	}
	// 关旧池 → 发新 generation
	old := CurrentGeneration()
	publishGeneration(buildGeneration(plugins))
	for _, p := range old.Plugins() {
		p.Pool.Close()
	}
	return failed, nil
}

// compileCached 带缓存的编译：源码哈希 → CompiledPlugin。
// 缓存键是源码哈希而非 key+version——同 key 换版本源码不变（罕见但合法）也能命中。
func (r *Registry) compileCached(source string) (*CompiledPlugin, error) {
	hash := sourceHash(source)
	if v, ok := r.compileCache.Load(hash); ok {
		if compiled, ok := v.(*CompiledPlugin); ok {
			return compiled, nil
		}
	}
	compiled, err := r.engine.Compile(source)
	if err != nil {
		return nil, err
	}
	r.compileCache.Store(hash, compiled)
	return compiled, nil
}

func (r *Registry) loadRow(ctx context.Context, key, version string) (*PluginVersionRow, error) {
	var row PluginVersionRow
	err := g.DB().Model(tableName).Ctx(ctx).
		Where("plugin_key", key).Where("version", version).Scan(&row)
	// 结构体 Scan 空结果返回 sql.ErrNoRows——这是「不存在」而非失败，
	// 必须放行（此前把它当错误，导致空表首轮 seed 直接中断）
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, gerror.Wrap(err, "读取插件版本失败")
	}
	if row.Id == 0 {
		return nil, nil
	}
	return &row, nil
}

// loadKeyRows 取某 key 的全部版本行（整删预检 / 整删用）。
func (r *Registry) loadKeyRows(ctx context.Context, key string) ([]PluginVersionRow, error) {
	var rows []PluginVersionRow
	if err := g.DB().Model(tableName).Ctx(ctx).
		Where("plugin_key", key).Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "读取插件版本失败")
	}
	return rows, nil
}

func (r *Registry) loadActiveRows(ctx context.Context) ([]PluginVersionRow, error) {
	var rows []PluginVersionRow
	err := g.DB().Model(tableName).Ctx(ctx).Where("active", true).Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "读取激活插件失败")
	}
	return rows, nil
}

// loadActiveRow 按 key 读唯一 active 行（无则 nil）。
func (r *Registry) loadActiveRow(ctx context.Context, key string) (*PluginVersionRow, error) {
	var row PluginVersionRow
	err := g.DB().Model(tableName).Ctx(ctx).
		Where("plugin_key", key).Where("active", true).Scan(&row)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, gerror.Wrap(err, "读取激活插件失败")
	}
	if row.Id == 0 {
		return nil, nil
	}
	return &row, nil
}

// loadActiveRowsTx 事务内读 active 行（Activate 冲突预检用——锁内读，
// 并发激活互相可见，关闭 TOCTOU 窗口）。
func (r *Registry) loadActiveRowsTx(ctx context.Context, tx gdb.TX) ([]PluginVersionRow, error) {
	var rows []PluginVersionRow
	err := tx.Model(tableName).Ctx(ctx).Where("active", true).Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "读取激活插件失败")
	}
	return rows, nil
}

// compareSemver 语义化版本比较（a>b 返回 1，a<b 返回 -1，相等 0）。
// 主版本段按数字比较；prerelease 存在时低于同版本无 prerelease（1.0.0-beta < 1.0.0），
// prerelease 标识符按 semver 规则逐段比较（数字段之间按值、字母段之间按字典序、
// 数字段 < 字母段）。仅覆盖 versionPattern 允许的形态。
func compareSemver(a, b string) int {
	ap, apre := splitSemver(a)
	bp, bpre := splitSemver(b)
	for i := 0; i < 3; i++ {
		an, bn := semverDigit(ap, i), semverDigit(bp, i)
		if an != bn {
			if an > bn {
				return 1
			}
			return -1
		}
	}
	if apre == bpre {
		return 0
	}
	if apre == "" {
		return 1 // 无 prerelease 者更高（正式版）
	}
	if bpre == "" {
		return -1
	}
	// prerelease 标识符逐段比较（beta.2 < beta.10：数字段按值，不是字典序）
	as, bs := strings.Split(apre, "."), strings.Split(bpre, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		an, aErr := strconv.Atoi(as[i])
		bn, bErr := strconv.Atoi(bs[i])
		switch {
		case aErr == nil && bErr == nil:
			if an != bn {
				if an > bn {
					return 1
				}
				return -1
			}
		case aErr == nil:
			return -1 // 数字标识符低于字母数字标识符（semver 规则）
		case bErr == nil:
			return 1
		default:
			if c := strings.Compare(as[i], bs[i]); c != 0 {
				return c
			}
		}
	}
	// 前缀相同：段多者更高（beta.2 < beta.2.1）
	switch {
	case len(as) > len(bs):
		return 1
	case len(as) < len(bs):
		return -1
	}
	return 0
}

// splitSemver 拆出主版本段与 prerelease 后缀（1.0.0-beta.1 → ["1","0","0"], "beta.1"）。
func splitSemver(v string) ([]string, string) {
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		return strings.Split(v[:idx], "."), v[idx+1:]
	}
	return strings.Split(v, "."), ""
}

// semverDigit 取主版本第 i 段的数字值（越界/非数字按 0）。
func semverDigit(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	n, err := strconv.Atoi(parts[i])
	if err != nil {
		return 0
	}
	return n
}

func (r *Registry) hasActiveRow(ctx context.Context, key string) (bool, error) {
	count, err := g.DB().Model(tableName).Ctx(ctx).
		Where("plugin_key", key).Where("active", true).Count()
	if err != nil {
		return false, gerror.Wrap(err, "读取插件激活状态失败")
	}
	return count > 0, nil
}

func sourceHash(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

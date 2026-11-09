package jsplugin

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
	"github.com/qianfree/team-api/internal/logic/billing/formula"
)

// tableName JS 插件版本表（migrations/000031_js_plugin.sql）。
// 说明：按生成式 dao 惯例应由 `make dao` 生成访问层；该命令需可达数据库，
// 当前以 g.DB().Model 直访 + 本文件内的行结构承担，生成条件具备后可替换。
const tableName = "js_plugin_versions"

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
	return nil
}

// seedBuiltins 幂等装载内置插件（origin=builtin）：
//   - 同 key+version 已是 builtin 行且源码变化（二进制升级）→ 刷新内容，保留 enabled/active
//   - 同 key+version 是 upload 行 → 不动（上传优先，内置同版本被遮蔽）
//   - 无行 → 插入（origin=builtin，首版本自动 active，enabled 由管理员开启；
//     升级带来的新版本号 inactive 入库，不改变运行行为）
func (r *Registry) seedBuiltins(ctx context.Context) error {
	builtins, err := compileBuiltins(r.engine)
	if err != nil {
		return err
	}
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
		return nil, err
	}
	// 加密配置字段类型限制：加密仅支持 string/text（写入侧只对这两型加密、读取侧
	// 一律 GetSecret 解密），其余类型声明 encrypted 会明文落库——上传期拒绝。
	// 不放 Manifest.Validate：它在 Compile 内还服务存量行装载（materialize），
	// 会让 DB 里的坏行毒化整个 rebuildGeneration（全有或全无的热更新）。
	if err := validateEncryptedConfigTypes(compiled.Manifest); err != nil {
		return nil, err
	}
	hash := sourceHash(source)
	manifestJSON, _ := json.Marshal(*compiled.Manifest)

	// 幂等：key+version 已存在且源码一致 → 直接返回；不一致 → 拒绝
	// 品牌图标校验（失败拒绝上传）
	if iconDataURI != "" {
		if _, err := ValidateIconDataURI(iconDataURI); err != nil {
			return nil, gerror.Wrap(err, "图标校验失败")
		}
	}
	existing, err := r.loadRow(ctx, compiled.Key, compiled.Version)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.SourceHash != hash {
			return nil, gerror.Newf("插件 %s@%s 已存在且源码不同（同版本号不允许改内容，请升版本号）", compiled.Key, compiled.Version)
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
	Key          string                `json:"key"`
	Name         string                `json:"name"`
	Description  string                `json:"description"`
	Author       string                `json:"author"`
	Website      string                `json:"website"`
	Origin       string                `json:"origin"` // 激活版本来源：builtin | upload
	ActiveVer    string                `json:"active_version"`
	Enabled      bool                  `json:"enabled"`
	InService    bool                  `json:"in_service"` // 当前 generation 内
	Models       []string              `json:"models"`
	FetchMode    string                `json:"fetch_mode"`
	AllowedHosts []string              `json:"allowed_hosts"`
	UsageSchema  []contract.UsageField `json:"usage_schema"` // 计费单位（定价页参考）
	Icon         string                `json:"icon"`         // manifest 声明：text/text:label/图标名
	HasImage     bool                  `json:"has_image"`    // 激活版本是否带品牌图片（GET icon 端点）
	Manifest     json.RawMessage       `json:"manifest"`     // 激活版本 manifest 全量
	Versions     []PluginVersionRow    `json:"versions"`
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
	byKey := map[string]*PluginSummary{}
	var order []string
	for i := range rows {
		row := rows[i]
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
				s.UsageSchema = m.UsageSchema
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

// Activate 激活指定版本：冲突预检 → 事务翻 active → 重建 generation（若 enabled）。
// 预检在事务内持 advisory lock 进行——事务外预检存在 TOCTOU：两个管理员并发激活
// 路由/模型冲突的插件，互相看不到对方未提交的激活而双双通过，generation 内
// 模型映射静默后者覆盖前者、路由二义命中。
func (r *Registry) Activate(ctx context.Context, key, version string) error {
	target, err := r.loadRow(ctx, key, version)
	if err != nil || target == nil {
		return gerror.Newf("插件 %s@%s 不存在", key, version)
	}
	var targetManifest contract.Manifest
	if err := json.Unmarshal(target.Manifest, &targetManifest); err != nil {
		return gerror.Wrap(err, "manifest 反序列化失败")
	}

	// 事务：advisory lock 串行化激活 → 锁内冲突预检 → 清旧 active → 置新 active
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, lockErr := tx.Exec("SELECT pg_advisory_xact_lock(?)", registryAdvisoryLockKey); lockErr != nil {
			return gerror.Wrap(lockErr, "激活锁获取失败")
		}
		// 冲突预检（锁内读 active 行）：与「其他 key 的 active 版本」比路由与模型
		others, err := r.loadActiveRowsTx(ctx, tx)
		if err != nil {
			return err
		}
		for i := range others {
			other := others[i]
			if other.PluginKey == key {
				continue
			}
			var otherManifest contract.Manifest
			if err := json.Unmarshal(other.Manifest, &otherManifest); err != nil {
				continue
			}
			if conflictErr := checkManifestConflict(key, &targetManifest, other.PluginKey, &otherManifest); conflictErr != nil {
				return conflictErr
			}
		}
		if _, err := tx.Model(tableName).Ctx(ctx).
			Where("plugin_key", key).Data(g.Map{"active": false, "updated_at": time.Now()}).Update(); err != nil {
			return err
		}
		if _, err := tx.Model(tableName).Ctx(ctx).
			Where("plugin_key", key).Where("version", version).
			Data(g.Map{"active": true, "updated_at": time.Now()}).Update(); err != nil {
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
		return gerror.Newf("插件 %s 没有激活版本", key)
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
		return nil, gerror.Newf("插件 %s@%s 不存在", key, version)
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
		return "", gerror.Newf("插件 %s 没有激活版本", key)
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
		return nil, gerror.Newf("插件 %s 没有激活版本", key)
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
		return gerror.Wrapf(loadErr, "插件 %s 配置已保存但装载失败，运行中仍是旧配置（其余插件不受影响）", key)
	}
	return nil
}

// DeleteVersion 删除版本：内置不可删；active+enabled 拒绝删；删除激活中的
// 上传版本后，同 key 回落激活最新内置版本（若有）。
func (r *Registry) DeleteVersion(ctx context.Context, key, version string) error {
	row, err := r.loadRow(ctx, key, version)
	if err != nil || row == nil {
		return gerror.Newf("插件 %s@%s 不存在", key, version)
	}
	if row.Origin == "builtin" {
		return gerror.New("内置版本不可删除（可禁用，或激活上传版本覆盖）")
	}
	if row.Active && row.Enabled {
		return gerror.New("该版本激活且启用中，请先禁用或切换激活版本")
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
	// usageSchema number 型键注册进公式 facts 注册表（保存期校验 + u() 速查）。
	// 摘除策略见 formula.RegisterFactKeys 注释：禁用不摘除，保留无害。
	registerUsageFactKeys(compiled.Manifest)
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

// registerUsageFactKeys 把 usageSchema 的 number 型键注册进公式 u() 通道。
func registerUsageFactKeys(m *contract.Manifest) {
	keys := make(map[string]string, len(m.UsageSchema))
	for _, f := range m.UsageSchema {
		if f.Type != "number" {
			continue
		}
		desc := f.Description
		if desc == "" {
			desc = "用量事实"
		}
		keys[f.Key] = desc + "（JS 插件 " + m.Key + "）"
	}
	formula.RegisterFactKeys(keys)
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

// checkManifestConflict 两插件 manifest 冲突预检：路由形状冲突 + 模型名冲突。
// 模型同名在【双方都声明 sharedModels】时放行（同名多插件的合法形态：路由归属由
// 渠道 plugin_key 决定），任一方未声明仍拒绝——防拼写意外撞名产生归属歧义。
func checkManifestConflict(keyA string, a *contract.Manifest, keyB string, b *contract.Manifest) error {
	for _, ra := range a.Routes {
		for _, rb := range b.Routes {
			if ra.Method == rb.Method && routeShapeKey(ra.Path) == routeShapeKey(rb.Path) {
				return gerror.Newf("路由冲突：%s %s 与插件 %s 的 %s 形状重复（先禁用对方再激活）", ra.Method, ra.Path, keyB, rb.Path)
			}
		}
	}
	sharedOK := a.SharedModels && b.SharedModels
	for _, ma := range a.Models {
		for _, mb := range b.Models {
			if ma == mb {
				if !sharedOK {
					return gerror.Newf("模型 %s 已被插件 %s 声明（如需同名多插件经不同渠道服务，双方 manifest 均声明 sharedModels: true）", ma, keyB)
				}
				glog.Warningf(context.Background(),
					"jsplugin: 模型 %s 由插件 %s 与 %s 共享声明（双方已声明 sharedModels；路由归属由渠道 plugin_key 决定）", ma, keyA, keyB)
				continue
			}
			if strings.EqualFold(ma, mb) {
				if !sharedOK {
					return gerror.Newf("模型 %s 与插件 %s 的 %s 仅大小写不同", ma, keyB, mb)
				}
				glog.Warningf(context.Background(),
					"jsplugin: 模型 %s(%s) 与插件 %s 的 %s 仅大小写不同（双方已声明 sharedModels）", keyA, ma, keyB, mb)
			}
		}
	}
	return nil
}

// routeShapeKey 路由的结构形状键：参数段归一为 ":"（:action 与 :kind 同形状），
// 供冲突检测与 generation 匹配索引共用语义。
func routeShapeKey(path string) string {
	shape := contract.NormalizePathShape(path)
	if shape == "/" {
		return shape
	}
	segs := strings.Split(strings.TrimPrefix(shape, "/"), "/")
	for i, s := range segs {
		if strings.HasPrefix(s, ":") {
			segs[i] = ":"
		}
	}
	return strings.Join(segs, "/")
}

func sourceHash(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

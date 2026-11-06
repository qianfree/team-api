package jsplugin

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	uc "github.com/qianfree/team-api/internal/utility/crypto"
)

// JS 插件配置存 sys_options，键 = plugin.{pluginKey}.{fieldKey}（与 Go 插件/分析 AI
// 的配置体系同源）。加密字段 AES-256-GCM v1: 密文，更新空值/掩码值 = 保持现值。
// 读取走 ConfigService 缓存，解密语义对齐 config_secret.go（历史明文兼容返回）。

const configMask = "********"

// configKey 拼配置存储键。
func configKey(pluginKey, fieldKey string) string {
	return "plugin." + pluginKey + "." + fieldKey
}

// seedConfigDefaults 上传时种默认值（已存在的键不动；加密字段种空串占位）。
func seedConfigDefaults(ctx context.Context, m *contract.Manifest) error {
	for _, field := range m.Config {
		key := configKey(m.Key, field.Key)
		if existing := lcommon.Config().GetOption(ctx, key); existing != "" {
			continue
		}
		def := ""
		if !field.Encrypted && field.Default != nil {
			def = configValueToString(field.Default)
		}
		if err := lcommon.Config().SetOption(ctx, key, def); err != nil {
			return gerror.Wrapf(err, "写入配置默认值 %s 失败", key)
		}
	}
	return nil
}

// loadPluginConfig 装载插件配置值（加密字段解密），返回给实例注入 utils.config.get。
func loadPluginConfig(ctx context.Context, m *contract.Manifest) (map[string]any, error) {
	if len(m.Config) == 0 {
		return nil, nil
	}
	values := make(map[string]any, len(m.Config))
	for _, field := range m.Config {
		key := configKey(m.Key, field.Key)
		raw := lcommon.Config().GetOption(ctx, key)
		if raw == "" {
			if field.Default != nil && !field.Encrypted {
				values[field.Key] = field.Default
			} else {
				values[field.Key] = configZeroValue(field.Type)
			}
			continue
		}
		if field.Encrypted {
			plain := lcommon.Config().GetSecret(ctx, key)
			values[field.Key] = plain
			continue
		}
		values[field.Key] = parseConfigValue(field.Type, raw)
	}
	return values, nil
}

// ConfigView 管理端读取视图（加密字段掩码）。
type ConfigView struct {
	Key       string `json:"key"`
	Value     any    `json:"value"` // 加密字段：空串=未配置，掩码=已配置
	Encrypted bool   `json:"encrypted"`
}

// GetConfigView 管理端读取配置：schema + 当前值（加密掩码）。
func GetConfigView(ctx context.Context, pluginKey string, m *contract.Manifest) map[string]ConfigView {
	out := make(map[string]ConfigView, len(m.Config))
	for _, field := range m.Config {
		key := configKey(pluginKey, field.Key)
		raw := lcommon.Config().GetOption(ctx, key)
		view := ConfigView{Encrypted: field.Encrypted}
		if field.Encrypted {
			if raw != "" {
				view.Value = configMask
			} else {
				view.Value = ""
			}
		} else if raw == "" && field.Default != nil {
			view.Value = field.Default
		} else {
			view.Value = parseConfigValue(field.Type, raw)
		}
		out[field.Key] = view
	}
	return out
}

// validateEncryptedConfigTypes 校验加密配置字段类型：encrypted 仅支持 string/text
// （configValueFromAny 的加密分支只覆盖这两型，读取侧 loadPluginConfig 却对
// encrypted 一律 GetSecret 解密——其余类型声明加密会明文落库）。上传期与配置
// 写入期两个入口都调用。
func validateEncryptedConfigTypes(m *contract.Manifest) error {
	for _, f := range m.Config {
		if f.Encrypted && f.Type != "string" && f.Type != "text" {
			return gerror.Newf("配置项 %q 声明 encrypted，但加密仅支持 string/text 类型（当前 %q）", f.Key, f.Type)
		}
	}
	return nil
}

// UpdateConfig 管理端更新配置：空串/掩码 = 保持现值（加密字段），其余覆盖写入；
// 成功后由调用方触发 rebuildGeneration（新实例装载新值）。
// 两阶段执行：先对全部提交值完成「保持现值判定 + 类型校验/加密」，再统一落库——
// 中途失败不会留下半更新状态（加密/校验失败时一个键都不写）。
func UpdateConfig(ctx context.Context, pluginKey string, m *contract.Manifest, values map[string]any) error {
	if len(values) == 0 {
		return gerror.New("配置值不能为空")
	}
	// 写入期防御：拦截存量坏行（加密声明 + 非 string/text 类型——加密写路径
	// 只覆盖 string/text，其余类型会明文落库而读取侧却走解密）
	if err := validateEncryptedConfigTypes(m); err != nil {
		return err
	}
	for k := range values {
		if m.ConfigFieldByKey(k) == nil {
			return gerror.Newf("配置项 %q 未在 manifest.config 中声明", k)
		}
	}
	// modelsFromConfig 扩展模型预检：冲突在此拒绝，而不是落库后 generation 重建
	// 时静默遮蔽（详见 precheckModelsConfig）
	if err := precheckModelsConfig(ctx, pluginKey, m, values); err != nil {
		return err
	}
	type pendingWrite struct {
		key string
		val string
	}
	writes := make([]pendingWrite, 0, len(values))
	for _, field := range m.Config {
		v, ok := values[field.Key]
		if !ok {
			continue
		}
		// 掩码/空值 = 保持现值。必须在加密前对原始值判断：configValueFromAny 对加密
		// 字段返回的是密文，密文与掩码/空串永不相等——加密后判断会让整表单保存把
		// 真实密钥静默替换成 "********" 的密文（语义对齐 internal/plugin/config.go）。
		if keepCurrentValue(field, v) {
			continue
		}
		str, err := configValueFromAny(ctx, field, v)
		if err != nil {
			return gerror.Wrapf(err, "配置项 %q", field.Key)
		}
		writes = append(writes, pendingWrite{key: configKey(pluginKey, field.Key), val: str})
	}
	for _, w := range writes {
		if err := lcommon.Config().SetOption(ctx, w.key, w.val); err != nil {
			return gerror.Wrapf(err, "写入配置 %s 失败", w.key)
		}
	}
	return nil
}

// precheckModelsConfig modelsFromConfig 保存预检：合成保存后的有效模型集，与其他
// 激活且启用插件的有效集做独占比对（精确 + 大小写折叠），冲突拒绝保存——比让
// generation 重建时静默遮蔽（回落解析按后者）更早更明确。竞态窗口（两个管理员
// 并发保存）由 buildGeneration 的兜底告警暴露，属管理端低频操作可接受。
func precheckModelsConfig(ctx context.Context, pluginKey string, m *contract.Manifest, submitted map[string]any) error {
	if m.ModelsFromConfig == "" {
		return nil
	}
	// 合成保存后的本插件配置值：当前全量值 + 本次提交覆盖（保持现值的语义与主写入
	// 循环一致——掩码/加密空串不覆盖）
	current, err := loadPluginConfig(ctx, m)
	if err != nil {
		return gerror.Wrap(err, "读取插件配置失败")
	}
	merged := make(map[string]any, len(current)+len(submitted))
	for k, v := range current {
		merged[k] = v
	}
	for k, v := range submitted {
		if field := m.ConfigFieldByKey(k); field != nil && !keepCurrentValue(*field, v) {
			merged[k] = v
		}
	}
	setA, err := effectiveModelSet(m, merged)
	if err != nil {
		return gerror.Wrapf(err, "扩展模型配置 %q", m.ModelsFromConfig)
	}

	// 其他激活且启用插件的有效集（与 generation 实际装载口径一致；未启用插件
	// 不在运行时清单内，不构成冲突）。其他插件的配置扩展解析失败时按静态集
	// 参与（其装载侧同样会降级，口径一致）
	rows, err := DefaultRegistry().loadActiveRows(ctx)
	if err != nil {
		return gerror.Wrap(err, "读取激活插件失败")
	}
	others := make([]modelSetParty, 0, len(rows))
	for i := range rows {
		row := rows[i]
		if row.PluginKey == pluginKey || !row.Enabled {
			continue
		}
		var otherM contract.Manifest
		if json.Unmarshal(row.Manifest, &otherM) != nil {
			continue // 坏行不参与预检（装载侧自会跳过并告警）
		}
		otherSet := otherM.Models
		if otherM.ModelsFromConfig != "" {
			if cfg, cfgErr := loadPluginConfig(ctx, &otherM); cfgErr == nil {
				if eff, effErr := effectiveModelSet(&otherM, cfg); effErr == nil {
					otherSet = eff
				}
			}
		}
		others = append(others, modelSetParty{key: row.PluginKey, models: otherSet, shared: otherM.SharedModels})
	}
	return checkModelSetConflict(modelSetParty{key: pluginKey, models: setA, shared: m.SharedModels}, others)
}

// keepCurrentValue 判断提交值是否表示「保持现值」：加密字段空串/掩码、
// 非加密字段掩码（前端整表单回显掩码时不变更明文值）。
func keepCurrentValue(field contract.ConfigField, v any) bool {
	s, isStr := v.(string)
	if !isStr {
		return false
	}
	if s == configMask {
		return true
	}
	return field.Encrypted && strings.TrimSpace(s) == ""
}

// encryptConfigValue 加密写入口（UpdateConfig 加密分支经 SetOption 不会自动加密——
// settings registry 没有插件键的声明，这里显式处理）。
func encryptConfigValue(ctx context.Context, value string) (string, error) {
	hexKey := g.Cfg().MustGet(ctx, "crypto.encryptionKey").String()
	key, err := uc.GetEncryptionKey(hexKey)
	if err != nil {
		return "", gerror.Wrap(err, "加密密钥不可用")
	}
	return uc.EncryptString(key, value)
}

// configValueToString 默认值 → 存储字符串。
func configValueToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool, float64, int:
		b, _ := json.Marshal(t)
		return string(b)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// configValueFromAny 管理端提交值 → 存储字符串（按类型校验 + 加密处理）。
func configValueFromAny(ctx context.Context, field contract.ConfigField, v any) (string, error) {
	if v == nil {
		return "", nil
	}
	switch field.Type {
	case "string", "text":
		s, ok := v.(string)
		if !ok {
			return "", gerror.New("需要字符串")
		}
		if field.Encrypted {
			return encryptConfigValue(ctx, s)
		}
		return s, nil
	case "int":
		switch n := v.(type) {
		case float64:
			return strconv.FormatInt(int64(n), 10), nil
		case string:
			// 字符串形态必须是合法整数——原样入库会让 parseConfigValue 静默归 0
			if _, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64); err != nil {
				return "", gerror.Newf("需要整数，得到 %q", n)
			}
			return strings.TrimSpace(n), nil
		default:
			return "", gerror.New("需要整数")
		}
	case "bool":
		b, ok := v.(bool)
		if !ok {
			return "", gerror.New("需要布尔值")
		}
		if b {
			return "true", nil
		}
		return "false", nil
	case "select":
		s, ok := v.(string)
		if !ok {
			return "", gerror.New("需要字符串")
		}
		for _, opt := range field.Options {
			if opt == s {
				return s, nil
			}
		}
		return "", gerror.Newf("值 %q 不在选项内", s)
	case "json":
		b, err := json.Marshal(v)
		if err != nil {
			return "", gerror.New("需要 JSON 值")
		}
		return string(b), nil
	default:
		return "", gerror.Newf("类型 %q 不支持", field.Type)
	}
}

// parseConfigValue 存储字符串 → 类型化值（utils.config.get / 管理端回显共用）。
func parseConfigValue(fieldType, raw string) any {
	switch fieldType {
	case "int":
		var n int
		if err := json.Unmarshal([]byte(raw), &n); err == nil {
			return n
		}
		return 0
	case "bool":
		return raw == "true" || raw == "1"
	case "json":
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err == nil {
			return v
		}
		return nil
	default:
		return raw
	}
}

func configZeroValue(fieldType string) any {
	switch fieldType {
	case "int":
		return 0
	case "bool":
		return false
	case "json":
		return nil
	default:
		return ""
	}
}

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

// UpdateConfig 管理端更新配置：空串/掩码 = 保持现值（加密字段），其余覆盖写入；
// 成功后由调用方触发 rebuildGeneration（新实例装载新值）。
func UpdateConfig(ctx context.Context, pluginKey string, m *contract.Manifest, values map[string]any) error {
	if len(values) == 0 {
		return gerror.New("配置值不能为空")
	}
	for k := range values {
		if m.ConfigFieldByKey(k) == nil {
			return gerror.Newf("配置项 %q 未在 manifest.config 中声明", k)
		}
	}
	for _, field := range m.Config {
		v, ok := values[field.Key]
		if !ok {
			continue
		}
		str, err := configValueFromAny(ctx, field, v)
		if err != nil {
			return gerror.Wrapf(err, "配置项 %q", field.Key)
		}
		key := configKey(pluginKey, field.Key)
		// 空值/掩码 = 保持现值（加密字段掩码语义，与 Go 插件一致）
		if (field.Encrypted && strings.TrimSpace(str) == "") || str == configMask {
			continue
		}
		if err := lcommon.Config().SetOption(ctx, key, str); err != nil {
			return gerror.Wrapf(err, "写入配置 %s 失败", key)
		}
	}
	return nil
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
			return n, nil // gvalid 风格宽松处理
		default:
			b, _ := json.Marshal(n)
			return string(b), nil
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

package plugin

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	uc "github.com/qianfree/team-api/internal/utility/crypto"
)

// SecretMask 敏感配置字段的界面掩码值。
// 管理界面把该值原样提交回来时，框架理解为「保持现值不改动」。
const SecretMask = "********"

// encryptedSchemaKeys 返回插件 schema 中声明为敏感字段的键集合。
func encryptedSchemaKeys(pluginName string) map[string]bool {
	out := make(map[string]bool)
	for _, f := range GetConfigSchema(pluginName) {
		if f.Encrypted {
			out[f.Key] = true
		}
	}
	return out
}

// pluginEncryptionKey 读取插件配置加密密钥（与渠道 Key 共用 crypto.encryptionKey）。
func pluginEncryptionKey(ctx context.Context) ([]byte, error) {
	hexKey := g.Cfg().MustGet(ctx, "crypto.encryptionKey").String()
	if hexKey == "" {
		return nil, gerror.New("crypto.encryptionKey 未配置，无法处理插件敏感配置")
	}
	return uc.GetEncryptionKey(hexKey)
}

// maskSecretFields 把配置中的敏感字段替换为掩码（界面展示用）：
// 已配置显示 SecretMask，未配置显示空串——不泄露「是否已配置」以外的任何信息。
func maskSecretFields(secretKeys map[string]bool, config g.Map) g.Map {
	if len(secretKeys) == 0 || len(config) == 0 {
		return config
	}
	out := make(g.Map, len(config))
	for k, v := range config {
		if secretKeys[k] {
			s, _ := v.(string)
			if strings.TrimSpace(s) != "" {
				out[k] = SecretMask
			} else {
				out[k] = ""
			}
			continue
		}
		out[k] = v
	}
	return out
}

// mergeSecretFields 处理更新请求中的敏感字段（纯函数，密钥由调用方提供）：
//   - 空值或掩码值 → 沿用现值（界面掩码回显不得覆盖真实密钥）
//   - 其他值 → AES-256-GCM 加密后存储（已是密文则原样保留，支持重复提交幂等）
//
// 返回处理后的完整配置（非敏感字段原样透传）。
func mergeSecretFields(secretKeys map[string]bool, current, incoming g.Map, key []byte) (g.Map, error) {
	out := make(g.Map, len(incoming))
	for k, v := range incoming {
		if !secretKeys[k] {
			out[k] = v
			continue
		}
		s, _ := v.(string)
		s = strings.TrimSpace(s)
		if s == "" || s == SecretMask {
			if old, ok := current[k]; ok {
				out[k] = old
			}
			continue
		}
		if looksEncrypted(s) {
			out[k] = s
			continue
		}
		enc, err := uc.EncryptString(key, s)
		if err != nil {
			return nil, err
		}
		out[k] = enc
	}
	return out, nil
}

// decryptSecretFields 解密配置中的敏感字段（插件内部读取用）。
// 解密失败（如密钥轮换后的历史数据）不中断读取，返回空串并保留原值，调用方可给出明确错误。
func decryptSecretFields(secretKeys map[string]bool, config g.Map, key []byte) (g.Map, error) {
	if len(secretKeys) == 0 || len(config) == 0 {
		return config, nil
	}
	out := make(g.Map, len(config))
	for k, v := range config {
		if !secretKeys[k] {
			out[k] = v
			continue
		}
		s, _ := v.(string)
		if s == "" || !looksEncrypted(s) {
			out[k] = s // 兼容未经加密的历史明文
			continue
		}
		plain, err := uc.DecryptString(key, s)
		if err != nil {
			out[k] = ""
			continue
		}
		out[k] = plain
	}
	return out, nil
}

// looksEncrypted 判断值是否已是 v1 密文形态（与 utility/crypto 的版本前缀约定一致）。
func looksEncrypted(s string) bool {
	return strings.HasPrefix(s, "v1:")
}

// GetPluginConfig 获取插件全局配置（管理界面展示用：敏感字段以掩码显示）。
func GetPluginConfig(ctx context.Context, pluginName string) (g.Map, error) {
	config, err := loadRawConfig(ctx, pluginName)
	if err != nil {
		return nil, err
	}
	return maskSecretFields(encryptedSchemaKeys(pluginName), config), nil
}

// GetPluginConfigSecrets 获取插件全局配置（插件内部使用：敏感字段已解密）。
func GetPluginConfigSecrets(ctx context.Context, pluginName string) (g.Map, error) {
	config, err := loadRawConfig(ctx, pluginName)
	if err != nil {
		return nil, err
	}
	secretKeys := encryptedSchemaKeys(pluginName)
	if len(secretKeys) == 0 {
		return config, nil
	}
	key, err := pluginEncryptionKey(ctx)
	if err != nil {
		return nil, err
	}
	return decryptSecretFields(secretKeys, config, key)
}

// loadRawConfig 读取原始配置（不掩码、不解密）。
func loadRawConfig(ctx context.Context, pluginName string) (g.Map, error) {
	record, err := dao.SysPlugins.Ctx(ctx).
		Where("name", pluginName).
		Fields("config").
		One()
	if err != nil {
		return nil, err
	}
	if record == nil {
		return g.Map{}, nil
	}

	var config g.Map
	configStr := record["config"].String()
	if configStr == "" || configStr == "{}" {
		return g.Map{}, nil
	}
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		return nil, err
	}
	return config, nil
}

// GetPluginConfigForTenant 获取租户级插件配置（优先使用租户覆盖，否则使用全局配置）。
func GetPluginConfigForTenant(ctx context.Context, pluginName string, tenantID int64) (g.Map, error) {
	record, err := dao.TntTenantPlugins.Ctx(ctx).
		Where("tenant_id = ? AND plugin_name = ?", tenantID, pluginName).
		Fields("config").
		One()
	if err != nil {
		return nil, err
	}

	if record != nil {
		configStr := record["config"].String()
		if configStr != "" && configStr != "{}" {
			var config g.Map
			if err := json.Unmarshal([]byte(configStr), &config); err != nil {
				return nil, err
			}
			return config, nil
		}
	}

	return GetPluginConfig(ctx, pluginName)
}

// UpdatePluginConfig 更新插件全局配置。
//
// schema 声明为 Encrypted 的敏感字段在落库前加密；提交值为空或掩码时沿用现值，
// 界面掩码回显因此不会清掉真实密钥。租户级覆盖（UpdatePluginConfigForTenant）
// 暂不做敏感字段处理——当前没有租户级敏感配置的插件，需要时按同一模式扩展。
func UpdatePluginConfig(ctx context.Context, pluginName string, config g.Map) error {
	secretKeys := encryptedSchemaKeys(pluginName)
	if len(secretKeys) > 0 {
		key, err := pluginEncryptionKey(ctx)
		if err != nil {
			return err
		}
		current, err := loadRawConfig(ctx, pluginName)
		if err != nil {
			return err
		}
		config, err = mergeSecretFields(secretKeys, current, config, key)
		if err != nil {
			return err
		}
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = dao.SysPlugins.Ctx(ctx).
		Where("name", pluginName).
		Data(do.SysPlugins{Config: string(configJSON)}).
		Update()
	return err
}

// UpdatePluginConfigForTenant 更新租户级插件配置。
func UpdatePluginConfigForTenant(ctx context.Context, pluginName string, tenantID int64, config g.Map) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = dao.TntTenantPlugins.Ctx(ctx).
		Data(do.TntTenantPlugins{
			TenantId:   tenantID,
			PluginName: pluginName,
			Config:     string(configJSON),
		}).
		OnConflict("tenant_id,plugin_name").
		Save()
	return err
}

// GetConfigSchema 获取插件配置 schema（如果插件实现了 Configurable）。
func GetConfigSchema(pluginName string) []ConfigFieldDef {
	entry := GetPlugin(pluginName)
	if entry == nil {
		return nil
	}
	if c, ok := entry.Plugin.(Configurable); ok {
		return c.ConfigSchema()
	}
	return nil
}

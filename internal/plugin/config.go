package plugin

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

// GetPluginConfig 获取插件全局配置。
func GetPluginConfig(ctx context.Context, pluginName string) (g.Map, error) {
	record, err := g.DB().Model("sys_plugins").Ctx(ctx).
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
	// 先查租户级配置
	record, err := g.DB().Model("tnt_tenant_plugins").Ctx(ctx).
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

	// 回退到全局配置
	return GetPluginConfig(ctx, pluginName)
}

// UpdatePluginConfig 更新插件全局配置。
func UpdatePluginConfig(ctx context.Context, pluginName string, config g.Map) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = g.DB().Model("sys_plugins").Ctx(ctx).
		Where("name", pluginName).
		Data(g.Map{"config": string(configJSON)}).
		Update()
	return err
}

// UpdatePluginConfigForTenant 更新租户级插件配置。
func UpdatePluginConfigForTenant(ctx context.Context, pluginName string, tenantID int64, config g.Map) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = g.DB().Model("tnt_tenant_plugins").Ctx(ctx).
		Data(g.Map{
			"tenant_id":   tenantID,
			"plugin_name": pluginName,
			"config":      string(configJSON),
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

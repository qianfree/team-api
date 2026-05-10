package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/plugin"
)

func (s *sAdmin) PluginList(ctx context.Context, req *v1.PluginListReq) (*v1.PluginListRes, error) {
	entries := plugin.AllPlugins()
	items := make([]v1.PluginItem, 0, len(entries))

	for _, entry := range entries {
		if req.Category != "" && entry.Info.Category != req.Category {
			continue
		}
		if req.Status != "" && string(entry.Status) != req.Status {
			continue
		}

		config, _ := plugin.GetPluginConfig(ctx, entry.Info.Name)
		if config == nil {
			config = make(map[string]interface{})
		}

		items = append(items, v1.PluginItem{
			Name:        entry.Info.Name,
			Label:       entry.Info.Label,
			Description: entry.Info.Description,
			Version:     entry.Info.Version,
			Category:    entry.Info.Category,
			Author:      entry.Info.Author,
			Status:      string(entry.Status),
			Installed:   entry.Status != plugin.StatusRegistered,
			Config:      config,
		})
	}

	return &v1.PluginListRes{List: items}, nil
}

func (s *sAdmin) PluginDetail(ctx context.Context, req *v1.PluginDetailReq) (*v1.PluginDetailRes, error) {
	entry := plugin.GetPlugin(req.Name)
	if entry == nil {
		return nil, common.NewNotFoundError("插件")
	}

	config, _ := plugin.GetPluginConfig(ctx, req.Name)
	if config == nil {
		config = make(map[string]interface{})
	}

	return &v1.PluginDetailRes{
		Name:         entry.Info.Name,
		Label:        entry.Info.Label,
		Description:  entry.Info.Description,
		Version:      entry.Info.Version,
		Category:     entry.Info.Category,
		Author:       entry.Info.Author,
		Status:       string(entry.Status),
		Dependencies: entry.Info.Dependencies,
		Config:       config,
	}, nil
}

func (s *sAdmin) PluginInstall(ctx context.Context, req *v1.PluginInstallReq) (*v1.PluginInstallRes, error) {
	if err := plugin.Install(ctx, req.Name); err != nil {
		return nil, err
	}
	return &v1.PluginInstallRes{}, nil
}

func (s *sAdmin) PluginEnable(ctx context.Context, req *v1.PluginEnableReq) (*v1.PluginEnableRes, error) {
	if err := plugin.Enable(ctx, req.Name); err != nil {
		return nil, err
	}
	return &v1.PluginEnableRes{}, nil
}

func (s *sAdmin) PluginDisable(ctx context.Context, req *v1.PluginDisableReq) (*v1.PluginDisableRes, error) {
	if err := plugin.Disable(ctx, req.Name); err != nil {
		return nil, err
	}
	return &v1.PluginDisableRes{}, nil
}

func (s *sAdmin) PluginUninstall(ctx context.Context, req *v1.PluginUninstallReq) (*v1.PluginUninstallRes, error) {
	if err := plugin.Uninstall(ctx, req.Name, req.KeepData); err != nil {
		return nil, err
	}
	return &v1.PluginUninstallRes{}, nil
}

func (s *sAdmin) PluginUpgrade(ctx context.Context, req *v1.PluginUpgradeReq) (*v1.PluginUpgradeRes, error) {
	if err := plugin.Upgrade(ctx, req.Name); err != nil {
		return nil, err
	}
	return &v1.PluginUpgradeRes{}, nil
}

func (s *sAdmin) PluginConfigUpdate(ctx context.Context, req *v1.PluginConfigUpdateReq) (*v1.PluginConfigUpdateRes, error) {
	if err := plugin.UpdatePluginConfig(ctx, req.Name, req.Config); err != nil {
		return nil, err
	}
	return &v1.PluginConfigUpdateRes{}, nil
}

func (s *sAdmin) PluginConfigSchema(ctx context.Context, req *v1.PluginConfigSchemaReq) (*v1.PluginConfigSchemaRes, error) {
	fields := plugin.GetConfigSchema(req.Name)
	items := make([]v1.PluginConfigFieldItem, 0, len(fields))
	for _, f := range fields {
		items = append(items, v1.PluginConfigFieldItem{
			Key:         f.Key,
			Label:       f.Label,
			Type:        f.Type,
			Default:     f.Default,
			Options:     f.Options,
			Required:    f.Required,
			Description: f.Description,
		})
	}
	return &v1.PluginConfigSchemaRes{List: items}, nil
}

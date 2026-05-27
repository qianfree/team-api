package tenant

import (
	"context"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/middleware"
	"github.com/qianfree/team-api/internal/plugin"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

func (s *sTenant) TenantPluginList(ctx context.Context, req *v1.TenantPluginListReq) (*v1.TenantPluginListRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	entries := plugin.AllPlugins()
	items := make([]v1.TenantPluginItem, 0, len(entries))

	// 查询租户已启用的插件
	tenantPlugins, _ := g.DB().Model("tnt_tenant_plugins").Ctx(ctx).
		Where("tenant_id", tenantID).
		All()
	tenantEnabled := make(map[string]bool)
	for _, r := range tenantPlugins {
		tenantEnabled[r["plugin_name"].String()] = r["enabled"].Bool()
	}

	for _, entry := range entries {
		// 只展示已全局启用的插件
		if entry.Status != plugin.StatusEnabled {
			continue
		}

		items = append(items, v1.TenantPluginItem{
			Name:        entry.Info.Name,
			Label:       entry.Info.Label,
			Description: entry.Info.Description,
			Version:     entry.Info.Version,
			Category:    entry.Info.Category,
			Enabled:     tenantEnabled[entry.Info.Name],
		})
	}

	return &v1.TenantPluginListRes{List: items}, nil
}

func (s *sTenant) TenantPluginDetail(ctx context.Context, req *v1.TenantPluginDetailReq) (*v1.TenantPluginDetailRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	entry := plugin.GetPlugin(req.Name)
	if entry == nil {
		return nil, gerror.New("插件不存在")
	}

	config, _ := plugin.GetPluginConfigForTenant(ctx, req.Name, tenantID)
	if config == nil {
		config = make(map[string]interface{})
	}

	// 检查租户是否启用
	enabled := false
	record, _ := g.DB().Model("tnt_tenant_plugins").Ctx(ctx).
		Where("tenant_id = ? AND plugin_name = ?", tenantID, req.Name).
		One()
	if record != nil {
		enabled = record["enabled"].Bool()
	}

	return &v1.TenantPluginDetailRes{
		Name:        entry.Info.Name,
		Label:       entry.Info.Label,
		Description: entry.Info.Description,
		Version:     entry.Info.Version,
		Category:    entry.Info.Category,
		Enabled:     enabled,
		Config:      config,
	}, nil
}

func (s *sTenant) TenantPluginConfigUpdate(ctx context.Context, req *v1.TenantPluginConfigUpdateReq) (*v1.TenantPluginConfigUpdateRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	if err := plugin.UpdatePluginConfigForTenant(ctx, req.Name, tenantID, req.Config); err != nil {
		return nil, err
	}
	return &v1.TenantPluginConfigUpdateRes{}, nil
}

func (s *sTenant) TenantPluginEnable(ctx context.Context, req *v1.TenantPluginEnableReq) (*v1.TenantPluginEnableRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	if err := plugin.EnableForTenant(ctx, req.Name, tenantID); err != nil {
		return nil, err
	}
	return &v1.TenantPluginEnableRes{}, nil
}

func (s *sTenant) TenantPluginDisable(ctx context.Context, req *v1.TenantPluginDisableReq) (*v1.TenantPluginDisableRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	if err := plugin.DisableForTenant(ctx, req.Name, tenantID); err != nil {
		return nil, err
	}
	return &v1.TenantPluginDisableRes{}, nil
}

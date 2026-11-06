package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/jsplugin"
)

// JSPluginConfig 配置读取：manifest.config schema + 当前值（加密字段掩码），
// 形状与 Go 插件 PluginConfigModal 兼容（list/encrypted/group/half_width）。
func (s *sAdmin) JSPluginConfig(ctx context.Context, req *v1.JSPluginConfigReq) (*v1.JSPluginConfigRes, error) {
	manifest, err := jsplugin.DefaultRegistry().ActiveManifest(ctx, req.Key)
	if err != nil {
		return nil, err
	}
	view := jsplugin.GetConfigView(ctx, req.Key, manifest)
	out := make([]v1.JSPluginConfigField, 0, len(manifest.Config))
	for _, field := range manifest.Config {
		v := view[field.Key]
		out = append(out, v1.JSPluginConfigField{
			Key: field.Key, Label: field.Label, Type: field.Type,
			Default: field.Default, Options: field.Options, Required: field.Required,
			Description: field.Description, Encrypted: field.Encrypted,
			Group: field.Group, HalfWidth: field.HalfWidth, Value: v.Value,
		})
	}
	return &v1.JSPluginConfigRes{List: out}, nil
}

// JSPluginConfigUpdate 更新配置并重建 generation（在服务中的插件即刻装载新值）。
func (s *sAdmin) JSPluginConfigUpdate(ctx context.Context, req *v1.JSPluginConfigUpdateReq) (*v1.JSPluginConfigUpdateRes, error) {
	if err := jsplugin.DefaultRegistry().UpdatePluginConfig(ctx, req.Key, req.Config); err != nil {
		return nil, err
	}
	return &v1.JSPluginConfigUpdateRes{}, nil
}

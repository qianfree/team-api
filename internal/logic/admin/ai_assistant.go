package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/logic/aiassistant"
	"github.com/qianfree/team-api/internal/logic/common"
)

// AI 助手配置管理（对话端点在 internal/handler/admin，SSE 直写不走 controller）。
// 权限点 ai_assistant:use / ai_assistant:config 见 middleware/rbac.go。

// AiAssistantConfigGet 读取助手配置（与设置页读取同构，敏感字段掩码显示）。
func (s *sAdmin) AiAssistantConfigGet(ctx context.Context, _ *v1.AiAssistantConfigGetReq) (*v1.AiAssistantConfigGetRes, error) {
	items := common.Config().GetCategoryWithValues(ctx, "ai_assistant")
	list := make([]v1.AdminSettingItem, len(items))
	for i, item := range items {
		list[i] = v1.AdminSettingItem{
			Key:         item.Key,
			Value:       common.TypedValue(item.Type, item.Value, item.Default),
			Type:        string(item.Type),
			Label:       item.Label,
			Description: item.Description,
			Sensitive:   item.Sensitive,
			Validation:  item.Validation,
			Default:     common.TypedValue(item.Type, "", item.Default),
		}
	}
	return &v1.AiAssistantConfigGetRes{List: list}, nil
}

// AiAssistantConfigUpdate 更新助手配置。敏感字段为掩码/空值时保持现值
// （UpdateCategory 内统一处理）。
func (s *sAdmin) AiAssistantConfigUpdate(ctx context.Context, req *v1.AiAssistantConfigUpdateReq) (*v1.AiAssistantConfigUpdateRes, error) {
	// Normalize interface{} values to strings for storage
	strValues := make(map[string]string, len(req.Settings))
	for key, val := range req.Settings {
		strValues[key] = common.NormalizeSettingValue(val)
	}
	if err := common.Config().UpdateCategory(ctx, "ai_assistant", strValues); err != nil {
		return nil, err
	}
	return &v1.AiAssistantConfigUpdateRes{}, nil
}

// AiAssistantTest 用已保存配置发起一次最小补全做连通性测试，
// 失败原因原样返回给管理员定位问题（测试失败本身是有效结果，不返回 error）。
func (s *sAdmin) AiAssistantTest(ctx context.Context, _ *v1.AiAssistantTestReq) (*v1.AiAssistantTestRes, error) {
	cfg := aiassistant.LoadConfig(ctx)
	if err := cfg.Validate(); err != nil {
		return nil, common.NewBadRequestError(err.Error())
	}
	result := aiassistant.NewClient(cfg).TestConnection(ctx)
	return &v1.AiAssistantTestRes{
		OK:         result.OK,
		Model:      result.Model,
		LatencyMs:  result.LatencyMs,
		TokensUsed: result.TokensUsed,
		Reply:      result.Reply,
		Error:      result.Error,
	}, nil
}

package common

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/dao"
)

// AvailableModelItem 租户可用模型基本信息（公共结构体，不含价格）
type AvailableModelItem struct {
	ModelDBID        int64  `json:"model_db_id"`
	ModelId          string `json:"model_id"`
	ModelName        string `json:"model_name"`
	Category         string `json:"category"`
	MaxContextTokens int    `json:"max_context_tokens"`
	MaxOutputTokens  int    `json:"max_output_tokens"`
	Description      string `json:"description"`
	Tags             string `json:"tags"`
	Capabilities     string `json:"capabilities"`
	Source           string `json:"source"` // "explicit" 或 "group"
}

// GetTenantAvailableModels 计算租户实际可用的所有模型（显式分配 + 分组来源，去重）
// 返回模型基本信息列表，不含价格。可被管理后台和租户控制台共用。
func GetTenantAvailableModels(ctx context.Context, tenantID int64, category, search string) ([]AvailableModelItem, error) {
	// 阶段1：获取显式分配的模型
	var explicitModels []struct {
		ModelDBID        int64  `json:"model_db_id"`
		ModelId          string `json:"model_id"`
		ModelName        string `json:"model_name"`
		Category         string `json:"category"`
		MaxContextTokens int    `json:"max_context_tokens"`
		MaxOutputTokens  int    `json:"max_output_tokens"`
		Description      string `json:"description"`
		Tags             string `json:"tags"`
		Capabilities     string `json:"capabilities"`
	}

	explicitQuery := dao.MdlTenantModels.Ctx(ctx).
		LeftJoin("mdl_models m ON mdl_tenant_models.model_id = m.id").
		Where("mdl_tenant_models.tenant_id", tenantID).
		Where("mdl_tenant_models.enabled", true).
		Where("m.status", "active")

	if category != "" {
		explicitQuery = explicitQuery.Where("m.category", category)
	}
	if search != "" {
		explicitQuery = explicitQuery.Where("m.model_id LIKE ? OR m.model_name LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	err := explicitQuery.
		Fields("mdl_tenant_models.model_id AS model_db_id, m.model_id, m.model_name, m.category, m.max_context_tokens, m.max_output_tokens, m.description, m.tags, m.capabilities").
		OrderAsc("m.category").
		OrderAsc("m.model_id").
		Scan(&explicitModels)
	if err != nil {
		return nil, err
	}

	result := make([]AvailableModelItem, 0, len(explicitModels))
	explicitModelIDs := make(map[int64]bool, len(explicitModels))
	for _, r := range explicitModels {
		explicitModelIDs[r.ModelDBID] = true
		result = append(result, AvailableModelItem{
			ModelDBID:        r.ModelDBID,
			ModelId:          r.ModelId,
			ModelName:        r.ModelName,
			Category:         r.Category,
			MaxContextTokens: r.MaxContextTokens,
			MaxOutputTokens:  r.MaxOutputTokens,
			Description:      r.Description,
			Tags:             r.Tags,
			Capabilities:     r.Capabilities,
			Source:           "explicit",
		})
	}

	// 阶段2：获取分组来源的模型（排除已在显式分配中的）
	var groupModels []struct {
		ModelDBID        int64  `json:"model_db_id"`
		ModelId          string `json:"model_id"`
		ModelName        string `json:"model_name"`
		Category         string `json:"category"`
		MaxContextTokens int    `json:"max_context_tokens"`
		MaxOutputTokens  int    `json:"max_output_tokens"`
		Description      string `json:"description"`
		Tags             string `json:"tags"`
		Capabilities     string `json:"capabilities"`
	}

	groupQuery := g.DB().Model("mdl_group_models gm").Ctx(ctx).
		InnerJoin("mdl_model_groups g ON gm.group_id = g.id").
		InnerJoin("mdl_models m ON gm.model_id = m.id").
		InnerJoin("mdl_tenant_groups tg ON tg.group_id = g.id").
		Where("tg.tenant_id", tenantID).
		Where("g.status", "active").
		Where("m.status", "active")

	if len(explicitModelIDs) > 0 {
		ids := make([]int64, 0, len(explicitModelIDs))
		for id := range explicitModelIDs {
			ids = append(ids, id)
		}
		groupQuery = groupQuery.WhereNotIn("m.id", ids)
	}

	if category != "" {
		groupQuery = groupQuery.Where("m.category", category)
	}
	if search != "" {
		groupQuery = groupQuery.Where("m.model_id LIKE ? OR m.model_name LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	_ = groupQuery.
		Fields("DISTINCT m.id AS model_db_id, m.model_id, m.model_name, m.category, m.max_context_tokens, m.max_output_tokens, m.description, m.tags, m.capabilities").
		OrderAsc("m.category").
		OrderAsc("m.model_id").
		Scan(&groupModels)

	for _, gm := range groupModels {
		result = append(result, AvailableModelItem{
			ModelDBID:        gm.ModelDBID,
			ModelId:          gm.ModelId,
			ModelName:        gm.ModelName,
			Category:         gm.Category,
			MaxContextTokens: gm.MaxContextTokens,
			MaxOutputTokens:  gm.MaxOutputTokens,
			Description:      gm.Description,
			Tags:             gm.Tags,
			Capabilities:     gm.Capabilities,
			Source:           "group",
		})
	}

	return result, nil
}

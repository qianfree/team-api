package tenant

import (
	"context"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
)

// ListAvailableModels 获取租户可用的模型列表
func (s *sTenant) ListAvailableModels(ctx context.Context, req *v1.TenantAvailableModelsReq) (*v1.TenantAvailableModelsRes, error) {
	tenantID := ctxTenantID(ctx)

	query := dao.MdlTenantModels.Ctx(ctx).
		LeftJoin("mdl_models m ON mdl_tenant_models.model_id = m.id").
		Where("mdl_tenant_models.tenant_id", tenantID).
		Where("mdl_tenant_models.enabled", true).
		Where("m.status", "active")

	if req.Category != "" {
		query = query.Where("m.category", req.Category)
	}
	if req.Search != "" {
		query = query.Where("m.model_id LIKE ? OR m.model_name LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
	}

	var results []struct {
		ID               int64    `json:"id"`
		ModelId          string   `json:"model_id"`
		ModelName        string   `json:"model_name"`
		Category         string   `json:"category"`
		MaxContextTokens int      `json:"max_context_tokens"`
		MaxOutputTokens  int      `json:"max_output_tokens"`
		Description      string   `json:"description"`
		Tags             string   `json:"tags"`
		Capabilities     string   `json:"capabilities"`
		BillingMode      *string  `json:"billing_mode"`
		PerRequestPrice  *float64 `json:"per_request_price"`
		DiscountRatio    *float64 `json:"discount_ratio"`
		MaxConcurrency   *int     `json:"max_concurrency"`
	}

	err := query.
		Fields("mdl_tenant_models.id, m.model_id, m.model_name, m.category, m.max_context_tokens, m.max_output_tokens, m.description, m.tags, m.capabilities, mdl_tenant_models.billing_mode, mdl_tenant_models.per_request_price, mdl_tenant_models.discount_ratio, mdl_tenant_models.max_concurrency").
		OrderAsc("m.category").
		OrderAsc("m.model_id").
		Scan(&results)
	if err != nil {
		return nil, err
	}

	list := make([]v1.TenantAvailableModelItem, 0, len(results))
	for _, r := range results {
		list = append(list, v1.TenantAvailableModelItem{
			ID:              r.ID,
			ModelId:         r.ModelId,
			ModelName:       r.ModelName,
			Category:        r.Category,
			MaxContext:      r.MaxContextTokens,
			MaxOutput:       r.MaxOutputTokens,
			Description:     r.Description,
			Tags:            r.Tags,
			Capabilities:    r.Capabilities,
			BillingMode:     r.BillingMode,
			PerRequestPrice: r.PerRequestPrice,
			DiscountRatio:   r.DiscountRatio,
			MaxConcurrency:  r.MaxConcurrency,
		})
	}

	return &v1.TenantAvailableModelsRes{List: list}, nil
}

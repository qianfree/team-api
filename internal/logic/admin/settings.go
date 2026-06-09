package admin

import (
	"context"
	"strings"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/logic/common"
)

// GetSettingsCategories returns all available setting categories.
func (s *sAdmin) GetSettingsCategories(ctx context.Context, _ *v1.AdminSettingsCategoriesReq) (*v1.AdminSettingsCategoriesRes, error) {
	categories := common.Categories
	list := make([]v1.SettingCategoryItem, len(categories))
	for i, c := range categories {
		list[i] = v1.SettingCategoryItem{
			Key:   c.Key,
			Label: c.Label,
			Icon:  c.Icon,
			Order: c.Order,
		}
	}
	return &v1.AdminSettingsCategoriesRes{List: list}, nil
}

// GetSettings retrieves settings with schema for a given category.
func (s *sAdmin) GetSettings(ctx context.Context, req *v1.AdminSettingsGetReq) (*v1.AdminSettingsGetRes, error) {
	if !isValidCategory(req.Category) {
		return nil, common.NewBadRequestError("无效的设置分类")
	}

	items := common.Config().GetCategoryWithValues(ctx, req.Category)
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
	return &v1.AdminSettingsGetRes{List: list}, nil
}

// UpdateSettings batch-updates settings for a given category.
func (s *sAdmin) UpdateSettings(ctx context.Context, req *v1.AdminSettingsUpdateReq) (*v1.AdminSettingsUpdateRes, error) {
	if !isValidCategory(req.Category) {
		return nil, common.NewBadRequestError("无效的设置分类")
	}

	// Normalize interface{} values to strings for storage
	strValues := make(map[string]string, len(req.Settings))
	for key, val := range req.Settings {
		strValues[key] = common.NormalizeSettingValue(val)
	}

	if err := s.validateCrossFieldSettings(ctx, req.Category, strValues); err != nil {
		return nil, err
	}

	if err := common.Config().UpdateCategory(ctx, req.Category, strValues); err != nil {
		return nil, err
	}
	return nil, nil
}

// validateCrossFieldSettings validates interdependent settings within a category.
func (s *sAdmin) validateCrossFieldSettings(ctx context.Context, category string, values map[string]string) error {
	if category != "security" {
		return nil
	}

	enabled, ok := values["turnstile_enabled"]
	if !ok || (enabled != "true" && enabled != "1") {
		return nil
	}

	siteKey := values["turnstile_site_key"]
	secretKey := values["turnstile_secret_key"]

	// If keys not in the current request, read existing values from DB
	if siteKey == "" {
		siteKey = common.Config().GetString(ctx, "turnstile_site_key")
	}
	if secretKey == "" || secretKey == "******" {
		secretKey = common.Config().GetString(ctx, "turnstile_secret_key")
	}

	if strings.TrimSpace(siteKey) == "" || strings.TrimSpace(secretKey) == "" {
		return common.NewBadRequestError("启用 Turnstile 前必须先配置 Site Key 和 Secret Key")
	}

	return nil
}

func isValidCategory(category string) bool {
	for _, c := range common.Categories {
		if c.Key == category {
			return true
		}
	}
	return false
}

package admin

import (
	"context"
	"fmt"

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
		return nil, fmt.Errorf("未知的设置分类: %s", req.Category)
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
		return nil, fmt.Errorf("未知的设置分类: %s", req.Category)
	}

	// Normalize interface{} values to strings for storage
	strValues := make(map[string]string, len(req.Settings))
	for key, val := range req.Settings {
		strValues[key] = common.NormalizeSettingValue(val)
	}
	if err := common.Config().UpdateCategory(ctx, req.Category, strValues); err != nil {
		return nil, err
	}
	return nil, nil
}

func isValidCategory(category string) bool {
	for _, c := range common.Categories {
		if c.Key == category {
			return true
		}
	}
	return false
}

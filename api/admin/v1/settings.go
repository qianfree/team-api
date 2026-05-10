package v1

import "github.com/gogf/gf/v2/frame/g"

// AdminSettingsCategoriesReq 获取所有设置分类
type AdminSettingsCategoriesReq struct {
	g.Meta `path:"/settings/categories" method:"get" mime:"json" tags:"管理后台-系统设置" summary:"获取设置分类列表"`
}

type SettingCategoryItem struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Icon  string `json:"icon,omitempty"`
	Order int    `json:"order"`
}

type AdminSettingsCategoriesRes struct {
	List []SettingCategoryItem `json:"list"`
}

// AdminSettingsGetReq 获取指定分类的设置（含 schema + 当前值）
type AdminSettingsGetReq struct {
	g.Meta   `path:"/settings/{category}" method:"get" mime:"json" tags:"管理后台-系统设置" summary:"获取分类设置"`
	Category string `json:"category" in:"path" v:"required" dc:"设置分类"`
}

type AdminSettingItem struct {
	Key         string      `json:"key"`
	Value       interface{} `json:"value"`
	Type        string      `json:"type"`
	Label       string      `json:"label"`
	Description string      `json:"description,omitempty"`
	Sensitive   bool        `json:"sensitive,omitempty"`
	Validation  string      `json:"validation,omitempty"`
	Default     interface{} `json:"default"`
}

type AdminSettingsGetRes struct {
	List []AdminSettingItem `json:"list"`
}

// AdminSettingsUpdateReq 更新指定分类的设置
type AdminSettingsUpdateReq struct {
	g.Meta   `path:"/settings/{category}" method:"put" mime:"json" tags:"管理后台-系统设置" summary:"更新分类设置"`
	Category string                 `json:"category" in:"path" v:"required" dc:"设置分类"`
	Settings map[string]interface{} `json:"settings" v:"required" dc:"设置键值对"`
}

type AdminSettingsUpdateRes struct{}

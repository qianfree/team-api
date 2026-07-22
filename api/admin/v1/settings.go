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
	Key         string `json:"key"`
	Value       any    `json:"value"`
	Type        string `json:"type"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Sensitive   bool   `json:"sensitive,omitempty"`
	Validation  string `json:"validation,omitempty"`
	Default     any    `json:"default"`
}

type AdminSettingsGetRes struct {
	List []AdminSettingItem `json:"list"`
}

// AdminSettingsUpdateReq 更新指定分类的设置
type AdminSettingsUpdateReq struct {
	g.Meta   `path:"/settings/{category}" method:"put" mime:"json" tags:"管理后台-系统设置" summary:"更新分类设置"`
	Category string         `json:"category" in:"path" v:"required" dc:"设置分类"`
	Settings map[string]any `json:"settings" v:"required" dc:"设置键值对"`
}

type AdminSettingsUpdateRes struct{}

// AdminStorageTestReq 测试对象存储（OSS/S3/COS）配置连通性。
// 携带表单中的存储配置（可含未保存的改动）；access_key_id / access_key_secret 为空或掩码
// "******" 时后端回落到已保存的值，从而支持「未保存直接测试」。
type AdminStorageTestReq struct {
	g.Meta      `path:"/settings/storage/test" method:"post" mime:"json" tags:"管理后台-系统设置" summary:"测试对象存储配置"`
	Provider    string `json:"storage_provider" dc:"存储供应商"`
	Endpoint    string `json:"storage_endpoint" dc:"存储端点"`
	Region      string `json:"storage_region" dc:"存储区域"`
	Bucket      string `json:"storage_bucket" dc:"存储桶名称"`
	AccessKeyID string `json:"storage_access_key_id" dc:"Access Key ID，空/掩码回落已保存值"`
	SecretKey   string `json:"storage_access_key_secret" dc:"Access Key Secret，空/掩码回落已保存值"`
	UseSSL      bool   `json:"storage_use_ssl" dc:"是否启用 SSL"`
	PathPrefix  string `json:"storage_path_prefix" dc:"路径前缀"`
}

type AdminStorageTestRes struct {
	Uploaded   bool   `json:"uploaded"`   // 测试图片上传成功
	Downloaded bool   `json:"downloaded"` // 下载并校验内容一致
	Deleted    bool   `json:"deleted"`    // 测试对象清理成功
	ElapsedMs  int64  `json:"elapsed_ms"` // 往返耗时(毫秒)
	Message    string `json:"message"`    // 结果描述
}

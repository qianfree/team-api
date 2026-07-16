package common

// SettingType defines the data type of a setting value.
type SettingType string

const (
	SettingTypeString SettingType = "string"
	SettingTypeInt    SettingType = "int"
	SettingTypeFloat  SettingType = "float"
	SettingTypeBool   SettingType = "bool"
	SettingTypeJSON   SettingType = "json"
)

// SettingCategory defines a logical group of settings.
type SettingCategory struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Icon  string `json:"icon,omitempty"`
	Order int    `json:"order"`
}

// SettingDef defines the schema for a single configuration item.
type SettingDef struct {
	Key         string      `json:"key"`
	Type        SettingType `json:"type"`
	Default     string      `json:"default"`
	Category    string      `json:"category"`
	Label       string      `json:"label"`
	Description string      `json:"description,omitempty"`
	Sensitive   bool        `json:"sensitive,omitempty"`
	Validation  string      `json:"validation,omitempty"` // "enum:a,b,c" or "min:1,max:100"
	IsPublic    bool        `json:"is_public,omitempty"`
}

// Categories defines all setting categories in display order.
var Categories = []SettingCategory{
	{Key: "general", Label: "基础配置", Icon: "settings", Order: 1},
	{Key: "oauth", Label: "第三方登录", Icon: "lock", Order: 2},
	{Key: "email", Label: "邮件配置", Icon: "email", Order: 3},
	{Key: "security", Label: "安全配置", Icon: "shield", Order: 4},
	{Key: "audit", Label: "审计配置", Icon: "audit", Order: 5},
	{Key: "payment", Label: "支付配置", Icon: "payment", Order: 6},
	{Key: "performance", Label: "性能配置", Icon: "speed", Order: 7},
	{Key: "content_filter", Label: "内容过滤", Icon: "filter", Order: 8},
	{Key: "channel", Label: "渠道配置", Icon: "channel", Order: 9},
	{Key: "storage", Label: "存储配置", Icon: "cloud", Order: 10},
	{Key: "data_governance", Label: "数据治理", Icon: "database", Order: 11},
	{Key: "agreement", Label: "用户协议", Icon: "document", Order: 12},
}

// Registry is the central definition of all configuration items.
var Registry = []SettingDef{
	// ── General ──
	{Key: "site_name", Type: SettingTypeString, Default: "Team-API", Category: "general",
		Label: "站点名称", Description: "显示在页面标题和邮件中", IsPublic: true},
	{Key: "site_description", Type: SettingTypeString, Default: "", Category: "general",
		Label: "站点描述", IsPublic: true},
	{Key: "register_enabled", Type: SettingTypeBool, Default: "true", Category: "general",
		Label: "开放注册", Description: "是否允许新用户注册", IsPublic: true},
	{Key: "register_email_verification", Type: SettingTypeBool, Default: "false", Category: "general",
		Label: "注册邮箱验证", Description: "注册时是否需要邮箱验证码，关闭时使用滑块验证", IsPublic: true},
	{Key: "register_ip_limit_per_hour", Type: SettingTypeInt, Default: "50", Category: "general",
		Label: "IP每小时注册限制", Description: "同一IP每小时最多注册次数（0表示不限制）", Validation: "min:0,max:200"},
	{Key: "register_ip_limit_per_day", Type: SettingTypeInt, Default: "500", Category: "general",
		Label: "IP每天注册限制", Description: "同一IP每天最多注册次数（0表示不限制）", Validation: "min:0,max:1000"},
	{Key: "register_global_limit_per_minute", Type: SettingTypeInt, Default: "50", Category: "general",
		Label: "全局每分钟注册限制", Description: "全系统每分钟最多注册次数（0表示不限制）", Validation: "min:0,max:200"},
	{Key: "maintenance_mode", Type: SettingTypeBool, Default: "false", Category: "general",
		Label: "维护模式", Description: "开启后控制台显示维护提示", IsPublic: true},
	{Key: "maintenance_message", Type: SettingTypeString, Default: "", Category: "general",
		Label: "维护提示信息", Description: "维护模式下显示的提示文字", IsPublic: true},
	{Key: "maintenance_duration", Type: SettingTypeString, Default: "", Category: "general",
		Label: "预计维护时长", Description: "维护模式预计持续时间，展示给用户", IsPublic: true},
	{Key: "api_maintenance_enabled", Type: SettingTypeBool, Default: "false", Category: "general",
		Label: "全局 API 维护", Description: "开启后 API 代理返回 503，叠加维护模式使用", IsPublic: true},
	{Key: "tenant_console_url", Type: SettingTypeString, Default: "", Category: "general",
		Label: "租户控制台地址", Description: "租户控制台的完整 URL，如 https://console.example.com，用于生成邀请链接等"},

	// ── OAuth (第三方登录) ──
	{Key: "oauth_auto_register", Type: SettingTypeBool, Default: "false", Category: "oauth",
		Label: "OAuth 自动注册", Description: "首次 OAuth 登录自动创建租户账号", IsPublic: true},
	{Key: "oauth_tenant_code", Type: SettingTypeString, Default: "", Category: "oauth",
		Label: "OAuth 默认租户代码", Description: "OAuth 自动注册时使用的租户代码（需预先创建）"},
	{Key: "oauth_github_enabled", Type: SettingTypeBool, Default: "false", Category: "oauth",
		Label: "启用 GitHub 登录", IsPublic: true},
	{Key: "oauth_github_client_id", Type: SettingTypeString, Default: "", Category: "oauth",
		Label: "GitHub Client ID", Sensitive: true},
	{Key: "oauth_github_client_secret", Type: SettingTypeString, Default: "", Category: "oauth",
		Label: "GitHub Client Secret", Sensitive: true},
	{Key: "oauth_google_enabled", Type: SettingTypeBool, Default: "false", Category: "oauth",
		Label: "启用 Google 登录", IsPublic: true},
	{Key: "oauth_google_client_id", Type: SettingTypeString, Default: "", Category: "oauth",
		Label: "Google Client ID", Sensitive: true},
	{Key: "oauth_google_client_secret", Type: SettingTypeString, Default: "", Category: "oauth",
		Label: "Google Client Secret", Sensitive: true},

	// ── Email ──
	{Key: "email_smtp_host", Type: SettingTypeString, Default: "", Category: "email",
		Label: "SMTP 服务器"},
	{Key: "email_smtp_port", Type: SettingTypeInt, Default: "587", Category: "email",
		Label: "SMTP 端口", Validation: "min:1,max:65535"},
	{Key: "email_smtp_username", Type: SettingTypeString, Default: "", Category: "email",
		Label: "SMTP 用户名"},
	{Key: "email_smtp_password", Type: SettingTypeString, Default: "", Category: "email",
		Label: "SMTP 密码", Sensitive: true},
	{Key: "email_smtp_from", Type: SettingTypeString, Default: "", Category: "email",
		Label: "发件人地址", Description: "格式: noreply@example.com"},
	{Key: "email_smtp_tls", Type: SettingTypeBool, Default: "true", Category: "email",
		Label: "启用 TLS", Description: "587/465 端口建议开启"},

	// ── Security ──
	{Key: "max_sessions_per_user", Type: SettingTypeInt, Default: "10", Category: "security",
		Label: "租户用户最大会话数", Validation: "min:1,max:100"},
	{Key: "admin_max_sessions", Type: SettingTypeInt, Default: "5", Category: "security",
		Label: "管理员最大会话数", Validation: "min:1,max:50"},
	{Key: "login_max_attempts", Type: SettingTypeInt, Default: "5", Category: "security",
		Label: "登录最大尝试次数", Validation: "min:1,max:30"},
	{Key: "login_lockout_minutes", Type: SettingTypeInt, Default: "30", Category: "security",
		Label: "登录锁定时长(分钟)", Validation: "min:1,max:1440"},
	{Key: "password_min_length", Type: SettingTypeInt, Default: "8", Category: "security",
		Label: "密码最小长度", Validation: "min:6,max:32"},
	{Key: "turnstile_enabled", Type: SettingTypeBool, Default: "false", Category: "security",
		Label: "启用 Turnstile 人机验证", IsPublic: true},
	{Key: "turnstile_site_key", Type: SettingTypeString, Default: "", Category: "security",
		Label: "Turnstile Site Key", Sensitive: false, IsPublic: true},
	{Key: "turnstile_secret_key", Type: SettingTypeString, Default: "", Category: "security",
		Label: "Turnstile Secret Key", Sensitive: true},
	{Key: "captcha_expire_seconds", Type: SettingTypeInt, Default: "300", Category: "security",
		Label: "验证码有效期(秒)", Validation: "min:60,max:600"},
	{Key: "new_device_notification", Type: SettingTypeBool, Default: "true", Category: "security",
		Label: "新设备登录通知"},
	{Key: "register_forbidden_words", Type: SettingTypeString, Default: "admin,system,root,api,test,administrator,管理员,系统", Category: "security",
		Label: "注册禁用词", Description: "组织名称、组织代码、用户名中包含这些词时禁止注册（不区分大小写），多个禁用词用英文逗号分隔"},
	{Key: "sandbox_enabled", Type: SettingTypeBool, Default: "true", Category: "performance",
		Label: "启用沙箱模式"},
	{Key: "sandbox_default_quota", Type: SettingTypeInt, Default: "100", Category: "performance",
		Label: "沙箱每月默认额度"},

	// ── Audit ──
	{Key: "audit_level", Type: SettingTypeString, Default: "full", Category: "audit",
		Label: "全局审计级别", Validation: "enum:full,full_text,masked,question_only,none",
		Description: "full=完整记录, full_text=全量文本, masked=脱敏记录, question_only=仅提问, none=不记录"},
	{Key: "audit_retention_days", Type: SettingTypeInt, Default: "90", Category: "audit",
		Label: "审计日志保留天数", Validation: "min:7,max:3650"},
	{Key: "operation_log_retention_days", Type: SettingTypeInt, Default: "365", Category: "audit",
		Label: "操作日志保留天数", Validation: "min:30,max:3650"},

	// ── Payment ──
	{Key: "payment_amount_options", Type: SettingTypeJSON, Default: "[10,20,50,100,200,500]", Category: "payment",
		Label: "充值金额选项", Description: "JSON 数组，预设充值面额"},
	{Key: "payment_amount_discount", Type: SettingTypeJSON, Default: "{}", Category: "payment",
		Label: "充值折扣", Description: "JSON 对象，如 {\"100\":0.9} 表示充100享9折"},
	{Key: "payment_min_topup", Type: SettingTypeFloat, Default: "1", Category: "payment",
		Label: "最低充值金额", Validation: "min:0.01"},
	{Key: "payment_currency", Type: SettingTypeString, Default: "CNY", Category: "payment",
		Label: "货币单位", Validation: "enum:CNY,USD"},
	{Key: "payment_callback_base_url", Type: SettingTypeString, Default: "", Category: "payment",
		Label: "支付回调基础URL", Description: "为空则使用请求 Host"},
	{Key: "payment_exchange_rate_cny_to_usd", Type: SettingTypeFloat, Default: "0.14", Category: "payment",
		Label: "CNY → USD 兑换比例", Description: "1 人民币兑换多少美元", Validation: "min:0.001,max:100"},
	{Key: "payment_exchange_rate_usd_to_cny", Type: SettingTypeFloat, Default: "7.25", Category: "payment",
		Label: "USD → CNY 兑换比例", Description: "1 美元兑换多少人民币", Validation: "min:0.001,max:1000"},

	// ── Performance ──
	{Key: "global_qps_limit", Type: SettingTypeInt, Default: "10000", Category: "performance",
		Label: "系统级 QPS 上限", Validation: "min:0"},
	{Key: "tenant_qps_limit", Type: SettingTypeInt, Default: "1000", Category: "performance",
		Label: "租户级 QPS 上限", Validation: "min:0"},
	{Key: "user_qps_limit", Type: SettingTypeInt, Default: "100", Category: "performance",
		Label: "用户级 QPS 上限", Validation: "min:0"},
	{Key: "key_qps_limit", Type: SettingTypeInt, Default: "60", Category: "performance",
		Label: "Key 级 QPS 上限", Validation: "min:0"},
	{Key: "tenant_concurrency_limit", Type: SettingTypeInt, Default: "0", Category: "performance",
		Label: "租户级并发上限", Validation: "min:0"},
	{Key: "request_timeout_seconds", Type: SettingTypeInt, Default: "120", Category: "performance",
		Label: "请求超时(秒)", Validation: "min:5,max:600"},
	{Key: "batch_write_size", Type: SettingTypeInt, Default: "64", Category: "performance",
		Label: "批量写入大小", Validation: "min:1,max:1000"},
	{Key: "channel_affinity_ttl", Type: SettingTypeInt, Default: "300", Category: "performance",
		Label: "渠道亲和性TTL(秒)", Validation: "min:0,max:3600"},
	{Key: "auto_test_interval", Type: SettingTypeInt, Default: "300", Category: "performance",
		Label: "渠道自动测试间隔(秒)", Validation: "min:60,max:86400"},
	{Key: "streaming_timeout_seconds", Type: SettingTypeInt, Default: "300", Category: "performance",
		Label: "流式超时(秒)", Validation: "min:30,max:3600",
		Description: "流式响应最大持续时间"},
	{Key: "cache_enabled", Type: SettingTypeBool, Default: "true", Category: "performance",
		Label: "缓存开关", Description: "是否启用 API Key 缓存"},
	{Key: "cache_ttl_seconds", Type: SettingTypeInt, Default: "300", Category: "performance",
		Label: "缓存过期时间(秒)", Validation: "min:30,max:86400",
		Description: "API Key 缓存有效期"},
	{Key: "batch_write_interval_ms", Type: SettingTypeInt, Default: "1000", Category: "performance",
		Label: "批量写入间隔(ms)", Validation: "min:100,max:30000",
		Description: "日志批量写入的间隔时间"},
	{Key: "global_concurrency_limit", Type: SettingTypeInt, Default: "1000", Category: "performance",
		Label: "系统级并发上限", Validation: "min:0"},
	{Key: "pprof_port", Type: SettingTypeInt, Default: "0", Category: "performance",
		Label: "pprof 端口", Validation: "min:0,max:65535",
		Description: "性能分析端口，0=关闭"},

	// ── Content Filter ──
	{Key: "content_filter_mode", Type: SettingTypeString, Default: "off", Category: "content_filter",
		Label: "过滤策略", Validation: "enum:off,log,replace,block",
		Description: "off=关闭, log=仅记录, replace=替换敏感词, block=拦截请求"},
	{Key: "content_filter_words", Type: SettingTypeJSON, Default: "[]", Category: "content_filter",
		Label: "敏感词列表", Description: "JSON 数组，支持通配符 *"},
	{Key: "content_filter_replacement", Type: SettingTypeString, Default: "***", Category: "content_filter",
		Label: "替换文本", Description: "replace 策略下用于替换敏感词的文本"},
	{Key: "content_filter_response", Type: SettingTypeString, Default: "内容包含敏感词，请修改后重试", Category: "content_filter",
		Label: "拦截提示", Description: "block 策略下返回给客户端的提示信息"},

	// ── Channel ──
	{Key: "channel_auto_test_enabled", Type: SettingTypeBool, Default: "true", Category: "channel",
		Label: "渠道自动探测", Description: "定期向活跃渠道发送测试请求，检测连通性并更新健康度（会消耗少量 Token）"},
	{Key: "channel_auto_test_recovery_enabled", Type: SettingTypeBool, Default: "true", Category: "channel",
		Label: "自动恢复探测", Description: "定期测试已自动禁用的渠道，测试通过则恢复启用（依赖自动探测开启）"},
	{Key: "channel_auto_disable_enabled", Type: SettingTypeBool, Default: "false", Category: "channel",
		Label: "渠道自动禁用", Description: "连续失败达到阈值时自动禁用渠道"},
	{Key: "channel_auto_disable_threshold", Type: SettingTypeInt, Default: "5", Category: "channel",
		Label: "自动禁用失败阈值", Validation: "min:2,max:50"},
	{Key: "health_snapshot_retention_days", Type: SettingTypeInt, Default: "7", Category: "channel",
		Label: "健康快照保留天数", Validation: "min:1,max:90"},
	{Key: "channel_proxy_url", Type: SettingTypeString, Default: "", Category: "channel",
		Label: "代理地址", Description: "全局代理 URL，支持 http:// 和 socks5://，如 http://127.0.0.1:7890。启用代理的渠道会通过此代理转发请求"},
	{Key: "sync_image_async_enabled", Type: SettingTypeBool, Default: "true", Category: "channel",
		Label: "同步图片厂商异步化", Description: "开启后，同步阻塞返回的图片厂商（如 OpenAI/DALL·E）走 /v1/images/generations/async 时由后台 worker 池异步处理，客户端提交即拿 task_id 后轮询取图；关闭则该端点对同步厂商返回不支持"},
	{Key: "sync_image_rehost_url", Type: SettingTypeBool, Default: "false", Category: "channel",
		Label: "同步图片 URL 转存对象存储", Description: "开启后，上游返回图片 URL 时下载并转存对象存储（返回 24h 稳定链接，需已配置存储）；关闭则直接透传上游 URL（部分厂商约 1h 过期）。b64_json 始终转存"},

	// ── Storage ──
	{Key: "storage_provider", Type: SettingTypeString, Default: "minio", Category: "storage",
		Label: "存储供应商", Validation: "enum:s3,minio,oss,cos",
		Description: "对象存储供应商类型"},
	{Key: "storage_endpoint", Type: SettingTypeString, Default: "", Category: "storage",
		Label:       "存储端点",
		Description: "S3/MinIO: https://s3.amazonaws.com, OSS: https://oss-cn-hangzhou.aliyuncs.com, COS: https://cos.ap-guangzhou.myqcloud.com"},
	{Key: "storage_region", Type: SettingTypeString, Default: "", Category: "storage",
		Label: "存储区域", Description: "AWS Region / OSS Region / COS Region"},
	{Key: "storage_bucket", Type: SettingTypeString, Default: "", Category: "storage",
		Label: "存储桶名称"},
	{Key: "storage_access_key_id", Type: SettingTypeString, Default: "", Category: "storage",
		Label: "Access Key ID", Sensitive: true},
	{Key: "storage_access_key_secret", Type: SettingTypeString, Default: "", Category: "storage",
		Label: "Access Key Secret", Sensitive: true},
	{Key: "storage_use_ssl", Type: SettingTypeBool, Default: "true", Category: "storage",
		Label: "启用 SSL"},
	{Key: "storage_path_prefix", Type: SettingTypeString, Default: "team-api", Category: "storage",
		Label: "路径前缀", Description: "存储路径前缀，用于隔离不同环境"},

	// Data Governance
	{Key: "data_retention_api_logs_days", Type: SettingTypeInt, Default: "90", Category: "data_governance",
		Label: "API调用日志保留天数", Validation: "min:7,max:3650"},
	{Key: "usage_log_retention_days", Type: SettingTypeInt, Default: "90", Category: "data_governance",
		Label: "用量日志保留天数", Validation: "min:7,max:3650"},
	{Key: "data_retention_operation_logs_days", Type: SettingTypeInt, Default: "365", Category: "data_governance",
		Label: "操作日志保留天数", Validation: "min:30,max:3650"},
	{Key: "data_retention_temp_data_days", Type: SettingTypeInt, Default: "1", Category: "data_governance",
		Label: "临时数据保留天数", Validation: "min:1,max:30"},
	{Key: "data_export_expiry_days", Type: SettingTypeInt, Default: "7", Category: "data_governance",
		Label: "数据导出文件保留天数", Validation: "min:1,max:30"},
	{Key: "data_deletion_completion_days", Type: SettingTypeInt, Default: "30", Category: "data_governance",
		Label: "GDPR删除请求完成天数", Validation: "min:7,max:90"},
	{Key: "file_retention_enabled", Type: SettingTypeBool, Default: "true", Category: "data_governance",
		Label: "启用文件保留期检查"},

	// ── Agreement (用户协议) ──
	{Key: "agreement_enabled", Type: SettingTypeBool, Default: "false", Category: "agreement",
		Label: "启用用户协议", Description: "启用后，管理员可管理协议版本，用户登录时需接受协议", IsPublic: true},
}

// registryMap is a lookup index for fast access by key.
var registryMap map[string]*SettingDef

func init() {
	registryMap = make(map[string]*SettingDef, len(Registry))
	for i := range Registry {
		registryMap[Registry[i].Key] = &Registry[i]
	}
}

// GetSettingDef returns the definition for a given key, or nil if not found.
func GetSettingDef(key string) *SettingDef {
	return registryMap[key]
}

// GetCategorySettings returns all settings for a given category.
func GetCategorySettings(category string) []SettingDef {
	var result []SettingDef
	for _, def := range Registry {
		if def.Category == category {
			result = append(result, def)
		}
	}
	return result
}

// IsRegisteredKey returns true if the key is defined in the registry.
func IsRegisteredKey(key string) bool {
	_, ok := registryMap[key]
	return ok
}

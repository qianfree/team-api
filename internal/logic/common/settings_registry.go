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
	// Encrypted 敏感值加密存储：写入前 AES-256-GCM 加密（v1: 前缀密文），
	// 读取走 Config().GetSecret 解密；界面回显掩码同 Sensitive。密钥与渠道 Key 共用
	Encrypted bool `json:"encrypted,omitempty"`
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
	{Key: "billing", Label: "计费配置", Icon: "record", Order: 13},
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
		Label: "预计维护时长", Description: "维护模式预计持续时间（Go 时长格式，如 2h、30m、1h30m），展示给用户", IsPublic: true},
	{Key: "api_maintenance_enabled", Type: SettingTypeBool, Default: "false", Category: "general",
		Label: "全局 API 维护", Description: "开启后 API 代理返回 503，叠加维护模式使用", IsPublic: true},
	{Key: "tenant_console_url", Type: SettingTypeString, Default: "", Category: "general",
		Label: "租户控制台地址", Description: "租户控制台的完整 URL，如 https://console.example.com，用于生成邀请链接等"},
	{Key: "site_url", Type: SettingTypeString, Default: "", Category: "general",
		Label: "站点后端地址", Description: "平台后端 API 的对外访问地址，如 https://api.example.com，用于构造 OAuth 登录回调地址（须与 GitHub/Google 应用后台登记的回调域名一致）；为空时按当前请求 Host 推断"},
	{Key: "pricing_time_timezone", Type: SettingTypeString, Default: "Asia/Shanghai", Category: "general",
		Label: "计费时区", Description: "模型时段定价（峰谷/促销）按此时区解释，IANA 时区名，如 Asia/Shanghai"},

	// ── Billing (计费) ──
	// 工具调用计费（联网搜索/云端工具按次附加费）：价格表为 sys_options JSON，
	// 查找与附加费计算见 internal/logic/billing/tool_pricing.go
	{Key: "tool_call_prices", Type: SettingTypeJSON, Default: "{}", Category: "billing",
		Label: "工具调用价格表", Description: "云端工具/联网搜索按次计费单价（每次调用，本位币）。key=工具名或 工具名:模型前缀*（最长前缀匹配），如 {\"web_search\":0.01,\"web_search:claude-*\":0.01}；0=关闭该工具计费；未列出的工具不计费（免费）"},

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
	{Key: "email_smtp_from_name", Type: SettingTypeString, Default: "", Category: "email",
		Label: "发件人名称", Description: "收件人看到的发件人显示名，如 aifree；留空则使用站点名称"},
	{Key: "email_smtp_tls", Type: SettingTypeBool, Default: "true", Category: "email",
		Label: "启用 TLS", Description: "587/465 端口建议开启"},

	// ── Security ──
	// 会话数上限不入注册表：并发会话限制由配置文件 jwt.adminMaxSessions / jwt.tenantMaxSessions 管理
	{Key: "login_max_attempts", Type: SettingTypeInt, Default: "5", Category: "security",
		Label: "登录最大尝试次数", Description: "连续密码错误达到该次数后锁定账号（管理后台 + 租户控制台）", Validation: "min:1,max:30"},
	{Key: "login_lockout_minutes", Type: SettingTypeInt, Default: "30", Category: "security",
		Label: "登录锁定时长(分钟)", Description: "触发锁定后的持续时长（管理后台 + 租户控制台）", Validation: "min:1,max:1440"},
	{Key: "turnstile_enabled", Type: SettingTypeBool, Default: "false", Category: "security",
		Label: "启用 Turnstile 人机验证", IsPublic: true},
	{Key: "turnstile_site_key", Type: SettingTypeString, Default: "", Category: "security",
		Label: "Turnstile Site Key", Sensitive: false, IsPublic: true},
	{Key: "turnstile_secret_key", Type: SettingTypeString, Default: "", Category: "security",
		Label: "Turnstile Secret Key", Sensitive: true},
	{Key: "captcha_expire_seconds", Type: SettingTypeInt, Default: "300", Category: "security",
		Label: "验证码有效期(秒)", Validation: "min:60,max:600"},
	{Key: "register_forbidden_words", Type: SettingTypeString, Default: "admin,system,root,api,test,administrator,管理员,系统", Category: "security",
		Label: "注册禁用词", Description: "组织名称、组织代码、用户名中包含这些词时禁止注册（不区分大小写），多个禁用词用英文逗号分隔"},
	{Key: "ip_blacklist_enabled", Type: SettingTypeBool, Default: "false", Category: "security",
		Label: "启用 IP 黑名单", Description: "开启后，命中黑名单的 IP 访问任何端点（控制台 / AI 代理）都会被直接拒绝，保存后即时生效"},
	{Key: "ip_blacklist_list", Type: SettingTypeJSON, Default: "[]", Category: "security",
		Label: "IP 黑名单列表", Description: "JSON 字符串数组，支持精确 IP 与 CIDR 网段，如 [\"1.2.3.4\", \"10.0.0.0/8\"]；拦截次数统计在仪表盘展示（存于缓存，不落库）"},

	// ── Audit ──
	{Key: "audit_level", Type: SettingTypeString, Default: "full", Category: "audit",
		Label: "全局审计级别", Validation: "enum:full,full_text,masked,question_only,meta_only,none",
		Description: "full=完整记录, full_text=全量文本, masked=脱敏记录, question_only=仅提问, meta_only=仅留痕（不记内容，出错时记录错误原因）, none=不记录"},

	// ── Payment ──
	{Key: "payment_amount_options", Type: SettingTypeJSON, Default: "[10,20,50,100,200,500]", Category: "payment",
		Label: "充值金额选项", Description: "JSON 数组，预设充值面额"},
	{Key: "payment_amount_discount", Type: SettingTypeJSON, Default: "{}", Category: "payment",
		Label: "充值折扣", Description: "JSON 对象，如 {\"100\":0.9} 表示充100享9折"},
	{Key: "payment_min_topup", Type: SettingTypeFloat, Default: "1", Category: "payment",
		Label: "最低充值金额", Validation: "min:0.01"},
	{Key: "billing_currency", Type: SettingTypeString, Default: "USD", Category: "payment",
		Label: "本位币", Description: "系统内部记账与全站显示的货币（USD/CNY）。仅在系统初始化向导中选定，初始化后不可更改；存量部署默认 USD", Validation: "enum:USD,CNY", IsPublic: true},
	{Key: "payment_callback_base_url", Type: SettingTypeString, Default: "", Category: "payment",
		Label: "支付回调基础URL", Description: "为空则使用请求 Host"},
	{Key: "payment_exchange_rate_cny_to_usd", Type: SettingTypeFloat, Default: "0.14", Category: "payment",
		Label: "CNY → USD 兑换比例", Description: "1 人民币兑换多少美元（USD → CNY 自动取倒数，确保往返闭合）", Validation: "min:0.001,max:100", IsPublic: true},
	{Key: "payment_notice", Type: SettingTypeString, Default: "", Category: "payment",
		Label: "支付说明", Description: "展示在租户端钱包充值页的说明文案，支持纯文本或 HTML 代码（如提示充值后不支持退款、指定渠道折扣活动等）。纯文本换行需用 <br>，富文本请直接粘贴 HTML"},

	// ── Performance ──
	{Key: "global_qps_limit", Type: SettingTypeInt, Default: "10000", Category: "performance",
		Label: "系统级 QPS 上限", Validation: "min:0"},
	{Key: "tenant_qps_limit", Type: SettingTypeInt, Default: "1000", Category: "performance",
		Label: "租户级 QPS 上限", Validation: "min:0"},
	{Key: "user_qps_limit", Type: SettingTypeInt, Default: "100", Category: "performance",
		Label: "用户级 QPS 上限", Validation: "min:0"},
	{Key: "key_qps_limit", Type: SettingTypeInt, Default: "60", Category: "performance",
		Label: "Key 级 QPS 上限", Validation: "min:0"},
	{Key: "global_concurrency_limit", Type: SettingTypeInt, Default: "0", Category: "performance",
		Label: "系统级并发上限", Validation: "min:0",
		Description: "全平台同时在途请求的总闸门（多副本共享计数）；流式长请求占用连接数分钟，QPS 限流拦不住堆积，须靠并发上限兜底；0=不限制，保存后约 5 秒生效"},
	{Key: "tenant_concurrency_limit", Type: SettingTypeInt, Default: "0", Category: "performance",
		Label: "租户级并发上限", Validation: "min:0"},
	{Key: "request_timeout_seconds", Type: SettingTypeInt, Default: "180", Category: "performance",
		Label: "请求超时(秒)", Validation: "min:5,max:600",
		Description: "非流式请求的全局默认总超时；渠道级超时设置优先，未配置的渠道使用此值，保存后即时生效"},
	{Key: "streaming_timeout_seconds", Type: SettingTypeInt, Default: "300", Category: "performance",
		Label: "流式空闲超时(秒)", Validation: "min:30,max:3600",
		Description: "流式响应两个数据块之间的最大空闲间隔（非总时长）的全局默认；渠道级设置优先，保存后即时生效"},
	{Key: "batch_write_size", Type: SettingTypeInt, Default: "64", Category: "performance",
		Label: "批量写入大小", Validation: "min:1,max:1000",
		Description: "审计日志批量落库条数，重启后生效"},
	{Key: "batch_write_interval_ms", Type: SettingTypeInt, Default: "1000", Category: "performance",
		Label: "批量写入间隔(ms)", Validation: "min:100,max:30000",
		Description: "审计日志批量落库周期，重启后生效"},
	{Key: "pprof_port", Type: SettingTypeInt, Default: "0", Category: "performance",
		Label: "pprof 端口", Validation: "min:0,max:65535",
		Description: "性能分析端口，仅监听本机回环（127.0.0.1），0=关闭，重启后生效"},
	{Key: "resource_memory_limit_percent", Type: SettingTypeInt, Default: "90", Category: "performance",
		Label: "内存占用上限(%)", Validation: "min:0,max:100",
		Description: "服务器内存占用超过该百分比时，新进大模型代理请求在计费前被拒绝（503 + Retry-After），防止服务器被拖垮；0=不启用，保存后即时生效"},
	{Key: "resource_disk_limit_percent", Type: SettingTypeInt, Default: "95", Category: "performance",
		Label: "磁盘占用上限(%)", Validation: "min:0,max:100",
		Description: "服务器磁盘占用超过该百分比时行为同上（检测路径为根分区）；0=不启用，保存后即时生效"},

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
		Label: "渠道自动探测", Description: "定期向活跃渠道发送测试请求，检测连通性并更新健康度（会消耗少量 Token）。禁用渠道不自动探测，由管理员手动测试确认后再启用"},
	{Key: "channel_auto_test_interval_minutes", Type: SettingTypeInt, Default: "5", Category: "channel",
		Label: "自动探测间隔（分钟）", Validation: "min:1,max:1440",
		Description: "两轮自动探测之间的间隔，保存后即时生效。探测是串行的（每渠道最多 30 秒），渠道多或上游超时多时单轮耗时会变长；上一轮未结束时下一轮自动跳过，因此间隔不宜小于「渠道数 × 30 秒」"},
	{Key: "channel_auto_disable_enabled", Type: SettingTypeBool, Default: "false", Category: "channel",
		Label: "渠道自动禁用", Description: "渠道熔断持续超过路由策略 breaker.autoDisableAfterSeconds（默认 10 分钟）未恢复时自动禁用"},
	{Key: "channel_routing_policy", Type: SettingTypeJSON, Default: "", Category: "channel",
		Label: "路由策略覆盖（JSON）", Description: "渠道调度引擎策略，部分字段覆盖内置默认值，保存后 30 秒内热生效；留空使用全部默认。高级功能，非必要不要修改；字段说明见管理后台渠道配置页路由策略的说明弹窗"},
	{Key: "health_snapshot_retention_days", Type: SettingTypeInt, Default: "7", Category: "channel",
		Label: "健康快照保留天数", Validation: "min:1,max:90"},
	{Key: "channel_proxy_url", Type: SettingTypeString, Default: "", Category: "channel",
		Label: "代理地址", Description: "全局代理 URL，支持 http:// 和 socks5://，如 http://127.0.0.1:7890。启用代理的渠道会通过此代理转发请求；OAuth 出站请求同样走此代理。保存后即时生效，无需重启"},
	{Key: "sync_image_async_enabled", Type: SettingTypeBool, Default: "false", Category: "channel",
		Label: "同步图片厂商异步化", Description: "开启后，同步阻塞返回的图片厂商（如 OpenAI/DALL·E）走 /v1/images/generations/async 时由后台 worker 池异步处理，客户端提交即拿 task_id 后轮询取图；关闭则该端点对同步厂商返回不支持。依赖对象存储保存生成结果，开启前需先在「存储配置」中配置对象存储（OSS/S3/COS）"},
	{Key: "sync_image_rehost_url", Type: SettingTypeBool, Default: "false", Category: "channel",
		Label: "同步图片 URL 转存对象存储", Description: "开启后，上游返回图片 URL 时下载并转存对象存储（返回 24h 稳定链接，需已配置存储）；关闭则直接透传上游 URL（部分厂商约 1h 过期）。b64_json 始终转存"},

	// ── Storage ──
	{Key: "storage_provider", Type: SettingTypeString, Default: "minio", Category: "storage",
		Label: "存储供应商", Validation: "enum:s3,minio,r2,oss,cos,local",
		Description: "对象存储供应商类型；local 表示本地磁盘存储（仅适合单实例部署）"},
	{Key: "storage_endpoint", Type: SettingTypeString, Default: "", Category: "storage",
		Label:       "存储端点",
		Description: "S3/MinIO: https://s3.amazonaws.com, OSS: https://oss-cn-hangzhou.aliyuncs.com, COS: https://cos.ap-guangzhou.myqcloud.com, R2: https://<account_id>.r2.cloudflarestorage.com"},
	{Key: "storage_region", Type: SettingTypeString, Default: "", Category: "storage",
		Label: "存储区域", Description: "AWS Region / OSS Region / COS Region（R2 固定填 auto）"},
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
	{Key: "storage_local_dir", Type: SettingTypeString, Default: "./data/files", Category: "storage",
		Label: "本地存储目录", Description: "本地磁盘存储根目录，仅 provider=local（或对象存储未配置降级）时生效。相对路径基于可执行文件目录。仅适合单实例部署；多实例需改用对象存储或挂载共享卷（PVC/NFS），否则不同副本间文件不互通"},
	{Key: "relay_files_url_ttl_minutes", Type: SettingTypeInt, Default: "60", Category: "storage",
		Label: "Files签名URL有效期(分钟)", Validation: "min:5,max:10080",
		Description: "/v1/files 上传响应及查询返回的预签名 URL 有效期（分钟）。URL 会被客户端持有并可能转交上游拉取，过期后需重新查询获取新 URL；file_id 引用由网关在转发时实时签发不受影响"},
	{Key: "task_reference_url_ttl_minutes", Type: SettingTypeInt, Default: "1440", Category: "storage",
		Label: "视频参考素材签名有效期(分钟)", Validation: "min:60,max:10080",
		Description: "/v1/videos 以 file_id 引用 /v1/files 上传素材（参考图/视频/音频）时，网关替换为签名 URL 的有效期（分钟）。上游视频任务可能排队后才拉取素材，需覆盖排队时长；实际取值不低于「Files签名URL有效期」"},
	{Key: "task_content_url_ttl_minutes", Type: SettingTypeInt, Default: "1440", Category: "storage",
		Label: "任务产物链接有效期(分钟)", Validation: "min:5,max:43200",
		Description: "异步任务（视频/图片/音频）统一产物链接 /v1/tasks/{id}/content 的签名有效期（分钟）。每次查询任务都会签发新链接，过期后重新查询即可；产物是否转存 OSS 不影响链接形态"},
	{Key: "relay_files_max_tenant_mb", Type: SettingTypeInt, Default: "1024", Category: "storage",
		Label: "Files每租户上传上限(MB)", Validation: "min:0,max:1048576",
		Description: "/v1/files 每租户累计上传总量上限（MB），超出后拒绝新上传；0 表示不限制。文件删除后配额即时释放"},
	{Key: "relay_files_allowed_types", Type: SettingTypeString, Default: "image/*", Category: "storage",
		Label: "Files允许的文件类型", Description: "/v1/files 上传类型白名单，逗号分隔，支持 MIME 前缀（image/*、application/pdf）与扩展名（jpg、.png）两种写法，扩展名或声明的 MIME 任一命中即放行；留空 = 不限制类型"},
	{Key: "relay_files_max_size_mb", Type: SettingTypeInt, Default: "32", Category: "storage",
		Label: "Files单文件大小上限(MB)", Validation: "min:1,max:2048",
		Description: "/v1/files 单个文件上传上限（MB）；实际生效上限为该值与全局请求体上限 clientMaxBodySize 的较小者"},

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
	{Key: "file_retention_enabled", Type: SettingTypeBool, Default: "true", Category: "data_governance",
		Label: "启用文件保留期检查"},
	{Key: "file_image_retention_days", Type: SettingTypeInt, Default: "0", Category: "data_governance",
		Label: "AI图片保留天数(0=不清理)", Description: "AI re-host 图片超过该天数后自动清理，0 表示不自动删除", Validation: "min:0,max:3650"},

	// ── Agreement (用户协议) ──
	{Key: "agreement_enabled", Type: SettingTypeBool, Default: "false", Category: "agreement",
		Label: "启用用户协议", Description: "启用后，管理员可管理协议版本，用户登录时需接受协议", IsPublic: true},

	// ── AI 请求分析 ──
	// 审计域核心功能（原 ai-req-analyze 插件转系统原生）：配置存 sys_options 但不进
	// Categories（设置页无此分类页签），读写走功能页自有的 /ai-analysis/config 端点；
	// 业务读取方为 internal/logic/aianalysis。
	{Key: "ai_analysis_base_url", Type: SettingTypeString, Default: "", Category: "ai_analysis",
		Label:       "分析 AI 端点",
		Description: "OpenAI 兼容端点地址（填法取决于「端点模式」）。建议内网自部署，防止分析内容二次泄露"},
	{Key: "ai_analysis_endpoint_mode", Type: SettingTypeString, Default: "base", Category: "ai_analysis",
		Label: "端点模式", Validation: "enum:base,full",
		Description: "base：端点填基地址（如 http://ollama:11434/v1，自动拼接 /chat/completions）；full：端点填完整请求地址（非标准路径用）"},
	{Key: "ai_analysis_api_key", Type: SettingTypeString, Default: "", Category: "ai_analysis",
		Label: "分析 AI 密钥", Sensitive: true, Encrypted: true,
		Description: "端点访问密钥，AES-256 加密存储；内网无鉴权部署（如 Ollama）可留空"},
	{Key: "ai_analysis_model", Type: SettingTypeString, Default: "", Category: "ai_analysis",
		Label: "分析模型", Description: "用于分析的模型名（如 qwen2.5:32b、gpt-4o-mini）"},
	{Key: "ai_analysis_timeout_ms", Type: SettingTypeInt, Default: "60000", Category: "ai_analysis",
		Label: "调用超时（毫秒）", Validation: "min:1000,max:600000",
		Description: "单次分析调用的超时时间"},
	{Key: "ai_analysis_max_tokens", Type: SettingTypeInt, Default: "4096", Category: "ai_analysis",
		Label: "单次输出上限", Validation: "min:64,max:128000",
		Description: "单次分析调用的 max_tokens"},
	{Key: "ai_analysis_agent_prompt", Type: SettingTypeString, Default: "", Category: "ai_analysis",
		Label:       "智能体提示词",
		Description: "自定义分析智能体的角色设定与审计维度（如专属行业合规口径），留空使用内置提示词。判定质量要求与 JSON 输出格式由系统自动拼接，无需编写"},
	{Key: "ai_analysis_custom_rules", Type: SettingTypeString, Default: "", Category: "ai_analysis",
		Label: "自定义合规规则", Description: "追加到分析系统提示词的额外规则（如行业监管要求），可选"},
	{Key: "ai_analysis_units_per_batch", Type: SettingTypeInt, Default: "6", Category: "ai_analysis",
		Label: "每批分析单元数", Validation: "min:1,max:50",
		Description: "每批送入 AI 的会话/独立单元数。单元含多轮对话，比逐请求的批量小一个量级"},
	{Key: "ai_analysis_snippet_max_len", Type: SettingTypeInt, Default: "4000", Category: "ai_analysis",
		Label: "单元截断长度（字符）", Validation: "min:200,max:100000",
		Description: "单个分析单元送入 AI 的最大长度，超出截断"},
	{Key: "ai_analysis_patrol_enabled", Type: SettingTypeBool, Default: "false", Category: "ai_analysis",
		Label: "启用定时巡查", Description: "按计划自动对巡查范围内的租户产出分析报告"},
	{Key: "ai_analysis_patrol_cron", Type: SettingTypeString, Default: "0 3 * * *", Category: "ai_analysis",
		Label: "巡查计划", Description: "cron 表达式，默认每天凌晨 3 点；保存后自动重排生效"},
	{Key: "ai_analysis_patrol_scope", Type: SettingTypeJSON, Default: `{"tenants":[]}`, Category: "ai_analysis",
		Label: "巡查范围", Description: "指定租户 ID 列表，空数组表示全部活跃租户"},
	{Key: "ai_analysis_patrol_window_h", Type: SettingTypeInt, Default: "24", Category: "ai_analysis",
		Label: "巡查回看窗口（小时）", Validation: "min:1,max:720",
		Description: "每次巡查分析的回看时间窗口"},
	{Key: "ai_analysis_retention_days", Type: SettingTypeInt, Default: "90", Category: "ai_analysis",
		Label: "报告保留天数", Validation: "min:1,max:3650",
		Description: "报告及发现明细的保留期，超期由清理任务删除"},

	// ── AI 助手（管理后台全局运营助手）──
	// 与 ai_analysis 同模式：不进 Categories（设置页无通用页签），读写走功能页自有的
	// /ai-assistant/config 端点；业务读取方为 internal/logic/aiassistant。
	{Key: "ai_assistant_enabled", Type: SettingTypeBool, Default: "false", Category: "ai_assistant",
		Label: "启用 AI 助手", IsPublic: true,
		Description: "开启后管理后台顶栏出现 AI 助手入口（需同时配置端点与模型）"},
	{Key: "ai_assistant_base_url", Type: SettingTypeString, Default: "", Category: "ai_assistant",
		Label:       "助手 AI 端点",
		Description: "OpenAI 兼容端点地址（填法取决于「端点模式」）"},
	{Key: "ai_assistant_endpoint_mode", Type: SettingTypeString, Default: "base", Category: "ai_assistant",
		Label: "端点模式", Validation: "enum:base,full",
		Description: "base：端点填基地址（如 https://api.example.com/v1，自动拼接 /chat/completions）；full：端点填完整请求地址（非标准路径用）"},
	{Key: "ai_assistant_api_key", Type: SettingTypeString, Default: "", Category: "ai_assistant",
		Label: "助手 AI 密钥", Sensitive: true, Encrypted: true,
		Description: "端点访问密钥，AES-256 加密存储"},
	{Key: "ai_assistant_model", Type: SettingTypeString, Default: "", Category: "ai_assistant",
		Label: "助手模型", Description: "对话使用的模型名（如 deepseek-chat、gpt-4o-mini）"},
	{Key: "ai_assistant_system_prompt", Type: SettingTypeString, Default: "", Category: "ai_assistant",
		Label:       "系统提示词",
		Description: "自定义助手人设与规则，留空使用内置默认（面向平台运营者的中文助手）"},
	{Key: "ai_assistant_max_tokens", Type: SettingTypeInt, Default: "4096", Category: "ai_assistant",
		Label: "单次输出上限", Validation: "min:64,max:128000",
		Description: "单次回复的 max_tokens"},
	{Key: "ai_assistant_temperature", Type: SettingTypeFloat, Default: "0.7", Category: "ai_assistant",
		Label: "采样温度", Validation: "min:0,max:2",
		Description: "0 更确定性，越高越发散"},
	{Key: "ai_assistant_timeout_ms", Type: SettingTypeInt, Default: "120000", Category: "ai_assistant",
		Label: "调用超时（毫秒）", Validation: "min:1000,max:600000",
		Description: "单次对话调用的总超时"},
	{Key: "ai_assistant_history_limit", Type: SettingTypeInt, Default: "20", Category: "ai_assistant",
		Label: "历史消息上限", Validation: "min:2,max:200",
		Description: "每次请求携带的历史消息条数上限（超出按最近保留，后端截断）"},
	{Key: "ai_assistant_rate_limit_per_min", Type: SettingTypeInt, Default: "30", Category: "ai_assistant",
		Label: "每分钟调用上限", Validation: "min:0,max:1000",
		Description: "每管理员每分钟对话次数上限，0 表示不限制"},
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

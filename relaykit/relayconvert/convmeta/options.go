package convmeta

// Options 是转换器查询的单次请求宿主配置快照。
// 宿主在构造 Meta 时从其设置系统填充该结构（见 relaycommon.RelayInfo.ConvOptions）；
// relaykit 用户则直接填充。零值 = 所有适配均被禁用，不应用任何默认值。
type Options struct {
	Claude ClaudeOptions
	Gemini GeminiOptions

	// OpenRouterDialect 标记上游为 OpenRouter 的 OpenAI 兼容接口，
	// 该接口接受转换器仅在该方言下才会输出的额外字段（reasoning 配置、system parts 上的 cache_control）。
	// 宿主根据渠道类型设置此项。
	OpenRouterDialect bool

	// PreserveThinkingSuffix 返回哪些模型在发出的模型名上必须保留
	// -thinking/-nothinking/effort 后缀（宿主黑名单查询）。
	// Nil 表示"从不保留"。
	PreserveThinkingSuffix func(modelName string) bool
}

type ClaudeOptions struct {
	// ThinkingAdapterEnabled 将带 "-thinking" 后缀的 OpenAI 模型名
	// 转换为 Claude 扩展思考请求。
	ThinkingAdapterEnabled bool
	// ThinkingAdapterBudgetTokensPercentage 在适配器触发时，
	// 按 max_tokens 的一定比例确定 thinking budget_tokens 的大小。
	ThinkingAdapterBudgetTokensPercentage float64
	// DefaultMaxTokens 返回当源请求未携带 max_tokens 时要注入的 max_tokens 值。
	// Claude Messages API 要求必须提供 max_tokens（省略会返回 400），
	// 因此当该 hook 为 nil 且没有其他路径提供该值时，
	// OpenAI→Claude 请求转换会以显式错误失败，而不是发出一个上游必然拒绝的请求。
	// new-api 宿主总是会提供该 hook；
	// 独立使用 relaykit 的用户必须自行提供一个，或保证每个请求都带有 max_tokens。
	DefaultMaxTokens func(modelName string) int
}

type GeminiOptions struct {
	// ThinkingAdapterEnabled 将 -thinking/-nothinking/effort 后缀映射到
	// Gemini 的 thinkingConfig。
	ThinkingAdapterEnabled bool
	// ThinkingAdapterBudgetTokensPercentage 在适配器触发时，
	// 按 maxOutputTokens 的一定比例确定 thinkingBudget 的大小。
	ThinkingAdapterBudgetTokensPercentage float64
	// FunctionCallThoughtSignatureEnabled 将 thoughtSignature 绕过值
	// 附带在 function-call parts 上。
	FunctionCallThoughtSignatureEnabled bool
	// SupportsImagine 返回模型是否支持图像生成
	// （切换响应模态）。Nil 表示"从不"。
	SupportsImagine func(modelName string) bool
	// SafetySetting 返回某个类别对应的伤害阈值。
	// 返回 nil 或空字符串表示不附带 safetySettings。
	SafetySetting func(category string) string
}

func (o *ClaudeOptions) DefaultMaxTokensFor(modelName string) (int, bool) {
	if o == nil || o.DefaultMaxTokens == nil {
		return 0, false
	}
	return o.DefaultMaxTokens(modelName), true
}

func (o *GeminiOptions) SupportsImagineModel(modelName string) bool {
	return o != nil && o.SupportsImagine != nil && o.SupportsImagine(modelName)
}

func (o *GeminiOptions) SafetySettingFor(category string) string {
	if o == nil || o.SafetySetting == nil {
		return ""
	}
	return o.SafetySetting(category)
}

func (o *Options) ShouldPreserveThinkingSuffix(modelName string) bool {
	return o != nil && o.PreserveThinkingSuffix != nil && o.PreserveThinkingSuffix(modelName)
}

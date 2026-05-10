package common

import "github.com/qianfree/team-api/relay/dto"

// TokenDetails Token 使用量细分
type TokenDetails struct {
	CachedTokens             int `json:"cached_tokens,omitempty"`
	CachedCreationTokens     int `json:"cached_creation_tokens,omitempty"`    // Claude cache_creation_input_tokens
	CachedCreation5mTokens   int `json:"cached_creation_5m_tokens,omitempty"` // Claude 5分钟缓存创建
	CachedCreation1hTokens   int `json:"cached_creation_1h_tokens,omitempty"` // Claude 1小时缓存创建
	AudioTokens              int `json:"audio_tokens,omitempty"`
	TextTokens               int `json:"text_tokens,omitempty"`
	ImageTokens              int `json:"image_tokens,omitempty"`
	ReasoningTokens          int `json:"reasoning_tokens,omitempty"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens,omitempty"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens,omitempty"`
}

// Usage Token 使用量信息
type Usage struct {
	PromptTokens           int           `json:"prompt_tokens"`
	CompletionTokens       int           `json:"completion_tokens"`
	TotalTokens            int           `json:"total_tokens"`
	CacheCreationTokens    int           `json:"cache_creation_tokens,omitempty"` // 缓存创建 token 总量
	PromptTokensDetails    *TokenDetails `json:"prompt_tokens_details,omitempty"`
	CompletionTokenDetails *TokenDetails `json:"completion_token_details,omitempty"`
	// CacheIncludedInPrompt 标记 PromptTokens 是否包含 cache tokens。
	// true: PromptTokens 是总量，cache 是其子集，计费时需扣减避免重复计费（OpenAI 原生 API）
	// false（默认）: PromptTokens 与 cache 独立，不扣减（Claude API、第三方兼容 API）
	CacheIncludedInPrompt bool `json:"-"`
}

// DtoTokenDetailsToCommon 将 dto.TokenDetails 转换为 common.TokenDetails
func DtoTokenDetailsToCommon(d *dto.TokenDetails) *TokenDetails {
	if d == nil {
		return nil
	}
	return &TokenDetails{
		CachedTokens:             d.CachedTokens,
		CachedCreationTokens:     d.CachedCreationTokens,
		CachedCreation5mTokens:   d.CachedCreation5mTokens,
		CachedCreation1hTokens:   d.CachedCreation1hTokens,
		AudioTokens:              d.AudioTokens,
		TextTokens:               d.TextTokens,
		ImageTokens:              d.ImageTokens,
		ReasoningTokens:          d.ReasoningTokens,
		AcceptedPredictionTokens: d.AcceptedPredictionTokens,
		RejectedPredictionTokens: d.RejectedPredictionTokens,
	}
}

// CommonTokenDetailsToDto 将 common.TokenDetails 转换为 dto.TokenDetails
func CommonTokenDetailsToDto(c *TokenDetails) *dto.TokenDetails {
	if c == nil {
		return nil
	}
	return &dto.TokenDetails{
		CachedTokens:             c.CachedTokens,
		CachedCreationTokens:     c.CachedCreationTokens,
		CachedCreation5mTokens:   c.CachedCreation5mTokens,
		CachedCreation1hTokens:   c.CachedCreation1hTokens,
		AudioTokens:              c.AudioTokens,
		TextTokens:               c.TextTokens,
		ImageTokens:              c.ImageTokens,
		ReasoningTokens:          c.ReasoningTokens,
		AcceptedPredictionTokens: c.AcceptedPredictionTokens,
		RejectedPredictionTokens: c.RejectedPredictionTokens,
	}
}

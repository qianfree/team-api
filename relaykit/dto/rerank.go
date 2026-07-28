package dto

// RerankRequest 重排请求
type RerankRequest struct {
	Model           string `json:"model"`
	Query           string `json:"query"`
	Documents       []any  `json:"documents"`
	TopN            *int   `json:"top_n,omitempty"`
	ReturnDocuments *bool  `json:"return_documents,omitempty"`
	MaxChunksPerDoc *int   `json:"max_chunks_per_doc,omitempty"`
	OverlapTokens   *int   `json:"overlap_tokens,omitempty"`
}

// RerankResponse 重排响应
type RerankResponse struct {
	Results []RerankResponseResult `json:"results"`
	Usage   RerankUsage            `json:"usage,omitempty"`
	ID      string                 `json:"id,omitempty"`
	Meta    *RerankMeta            `json:"meta,omitempty"`
}

// RerankResponseResult 单个重排结果
type RerankResponseResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
	Document       any     `json:"document,omitempty"`
}

// RerankUsage 重排使用量
type RerankUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
	SearchUnits  int `json:"search_units,omitempty"`
}

// RerankMeta 重排元信息
type RerankMeta struct {
	BilledUnits RerankBilledUnits `json:"billed_units,omitempty"`
}

// RerankBilledUnits 计费单位
type RerankBilledUnits struct {
	SearchDocuments int `json:"search_documents,omitempty"`
	TotalTokens     int `json:"total_tokens,omitempty"`
}

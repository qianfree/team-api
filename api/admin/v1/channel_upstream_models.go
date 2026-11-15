package v1

import "github.com/gogf/gf/v2/frame/g"

// ChannelUpstreamModelsReq 拉取渠道上游可用模型列表请求。
// 只读探测：用渠道自身的 base_url + 有效 Key 请求上游的模型列表接口，
// 供管理后台「添加模型能力」时挑选，不写入任何状态。
type ChannelUpstreamModelsReq struct {
	g.Meta    `path:"/channels/{channel_id}/upstream_models" method:"get" mime:"json" tags:"管理后台-渠道" summary:"获取上游可用模型列表"`
	ChannelID int64 `json:"channel_id" in:"path" v:"required" dc:"渠道ID"`
}

// ChannelUpstreamModelsRes 上游模型列表响应
type ChannelUpstreamModelsRes struct {
	List      []UpstreamModelItem `json:"list"`
	Total     int                 `json:"total"`
	Protocol  string              `json:"protocol" dc:"实际使用的列表协议：openai/claude/gemini"`
	ElapsedMs int64               `json:"elapsed_ms" dc:"上游响应耗时（毫秒）"`
}

// UpstreamModelItem 上游模型条目
type UpstreamModelItem struct {
	ID      string `json:"id" dc:"上游模型 ID（可直接作为映射来源名）"`
	OwnedBy string `json:"owned_by,omitempty" dc:"归属方，部分供应商返回"`
}

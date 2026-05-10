package coze

// CozeCreateRequest Coze v3 创建对话请求
type CozeCreateRequest struct {
	ConversationID string `json:"conversation_id,omitempty"`
	BotID          string `json:"bot_id"`
	User           string `json:"user"`
	Query          string `json:"query"`
	Stream         bool   `json:"stream"`
}

// CozeCreateResponse Coze 创建对话响应
type CozeCreateResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ID             string `json:"id"`
		ConversationID string `json:"conversation_id"`
		Status         string `json:"status"`
	} `json:"data"`
}

// CozeRetrieveResponse Coze 查询对话状态响应
type CozeRetrieveResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Status string `json:"status"`
	} `json:"data"`
}

// CozeMessageListResponse Coze 消息列表响应
type CozeMessageListResponse struct {
	Code int           `json:"code"`
	Msg  string        `json:"msg"`
	Data []CozeMessage `json:"data"`
}

// CozeMessage Coze 消息
type CozeMessage struct {
	Role        string `json:"role"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
}

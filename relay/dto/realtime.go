package dto

import "encoding/json"

// RealtimeEvent Realtime API WebSocket 事件
type RealtimeEvent struct {
	EventID  string            `json:"event_id,omitempty"`
	Type     string            `json:"type"`
	Session  *RealtimeSession  `json:"session,omitempty"`
	Item     *RealtimeItem     `json:"item,omitempty"`
	Response *RealtimeResponse `json:"response,omitempty"`
	Delta    json.RawMessage   `json:"delta,omitempty"`
	Error    *RealtimeError    `json:"error,omitempty"`
	Audio    string            `json:"audio,omitempty"`
}

// RealtimeSession 会话配置
type RealtimeSession struct {
	Model                   string                   `json:"model,omitempty"`
	Modalities              []string                 `json:"modalities,omitempty"`
	Instructions            string                   `json:"instructions,omitempty"`
	Voice                   string                   `json:"voice,omitempty"`
	InputAudioFormat        string                   `json:"input_audio_format,omitempty"`
	OutputAudioFormat       string                   `json:"output_audio_format,omitempty"`
	InputAudioTranscription *InputAudioTranscription `json:"input_audio_transcription,omitempty"`
	TurnDetection           json.RawMessage          `json:"turn_detection,omitempty"`
	Tools                   []RealTimeTool           `json:"tools,omitempty"`
	ToolChoice              any                      `json:"tool_choice,omitempty"`
	Temperature             *float64                 `json:"temperature,omitempty"`
	MaxResponseOutputTokens any                      `json:"max_response_output_tokens,omitempty"`
}

// InputAudioTranscription 输入音频转录配置
type InputAudioTranscription struct {
	Model string `json:"model,omitempty"`
}

// RealtimeItem 会话项（消息、函数调用等）
type RealtimeItem struct {
	ID        string            `json:"id,omitempty"`
	Type      string            `json:"type,omitempty"`
	Status    string            `json:"status,omitempty"`
	Role      string            `json:"role,omitempty"`
	Content   []RealtimeContent `json:"content,omitempty"`
	CallID    string            `json:"call_id,omitempty"`
	Name      string            `json:"name,omitempty"`
	Arguments string            `json:"arguments,omitempty"`
	Output    string            `json:"output,omitempty"`
}

// RealtimeContent 内容块
type RealtimeContent struct {
	Type       string `json:"type,omitempty"`
	Text       string `json:"text,omitempty"`
	Audio      string `json:"audio,omitempty"`
	Transcript string `json:"transcript,omitempty"`
}

// RealtimeResponse 响应对象
type RealtimeResponse struct {
	ID            string          `json:"id,omitempty"`
	Object        string          `json:"object,omitempty"`
	Status        string          `json:"status,omitempty"`
	StatusDetails string          `json:"status_details,omitempty"`
	Output        []RealtimeItem  `json:"output,omitempty"`
	Usage         *RealtimeUsage  `json:"usage,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

// RealtimeUsage Realtime API 使用量
type RealtimeUsage struct {
	TotalTokens        int                   `json:"total_tokens"`
	InputTokens        int                   `json:"input_tokens"`
	OutputTokens       int                   `json:"output_tokens"`
	InputTokenDetails  *RealtimeTokenDetails `json:"input_token_details,omitempty"`
	OutputTokenDetails *RealtimeTokenDetails `json:"output_token_details,omitempty"`
}

// RealtimeTokenDetails Token 类型细分
type RealtimeTokenDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
	TextTokens   int `json:"text_tokens,omitempty"`
	AudioTokens  int `json:"audio_tokens,omitempty"`
}

// RealtimeError 错误信息
type RealtimeError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// RealTimeTool 工具定义
type RealTimeTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

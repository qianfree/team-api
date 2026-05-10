package dto

import "encoding/json"

// AudioRequest TTS/STT/翻译统一请求结构
type AudioRequest struct {
	Model                  string          `json:"model"`
	Input                  string          `json:"input,omitempty"`                 // TTS 输入文本
	Voice                  string          `json:"voice,omitempty"`                 // TTS 语音（alloy/echo/fable/onyx/nova/shimmer）
	Instructions           string          `json:"instructions,omitempty"`          // TTS 指令
	ResponseFormat         string          `json:"response_format,omitempty"`       // TTS 输出格式（mp3/opus/aac/flac/wav/pcm）
	Speed                  *float64        `json:"speed,omitempty"`                 // TTS 速度（0.25-4.0）
	StreamFormat           string          `json:"stream_format,omitempty"`         // 流式音频格式
	Language               string          `json:"language,omitempty"`              // STT 源语言（可选）
	Prompt                 string          `json:"prompt,omitempty"`                // STT 提示词（可选）
	TimestampGranularities []string        `json:"timestamp_grularities,omitempty"` // STT 时间戳粒度（word/segment）
	Metadata               json.RawMessage `json:"metadata,omitempty"`              // 自定义元数据
}

// AudioResponse STT/翻译响应
type AudioResponse struct {
	Text     string        `json:"text"`
	Language string        `json:"language,omitempty"`
	Duration float64       `json:"duration,omitempty"`
	Words    []WordSegment `json:"words,omitempty"`
	Segments []Segment     `json:"segments,omitempty"`
}

// WordSegment 词级别时间戳
type WordSegment struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// Segment 片段级别时间戳
type Segment struct {
	ID    int     `json:"id"`
	Seek  int     `json:"seek"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// WhisperVerboseJSONResponse STT 详细响应格式（verbose_json）
type WhisperVerboseJSONResponse struct {
	Task     string        `json:"task"`
	Language string        `json:"language"`
	Duration float64       `json:"duration"`
	Text     string        `json:"text"`
	Segments []Segment     `json:"segments"`
	Words    []WordSegment `json:"words,omitempty"`
}

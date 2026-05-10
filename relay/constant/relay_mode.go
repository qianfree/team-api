package constant

import (
	"strings"
)

// RelayMode 定义代理请求的模式类型
type RelayMode int

const (
	RelayModeUnknown RelayMode = iota
	RelayModeChatCompletions
	RelayModeCompletions
	RelayModeEmbeddings
	RelayModeImagesGenerations
	RelayModeAudioSpeech
	RelayModeAudioTranscription
	RelayModeRerank
	RelayModeResponses
	RelayModeRealtime
	RelayModeClaudeMessages   // Claude 原生 /v1/messages 端点
	RelayModeGeminiChat       // Gemini /v1beta/models/{model}:generateContent 端点
	RelayModeAudioTranslation // /v1/audio/translations
	RelayModeVideoGenerations // POST /v1/video/generations
	RelayModeVideoFetch       // GET  /v1/video/generations/:id
	RelayModeSunoSubmit       // POST /suno/submit/:action
	RelayModeSunoFetch        // POST /suno/fetch 或 GET /suno/fetch/:id
	RelayModeModerations      // POST /v1/moderations
	RelayModeImagesEdits      // POST /v1/images/edits
	RelayModeMjSubmit         // POST /mj/submit/:action
	RelayModeMjFetch          // GET  /mj/task/:id/fetch
	RelayModeMjImage          // GET  /mj/image/:id
	RelayModeResponsesCompact // POST /v1/responses/compact
)

// Path2RelayMode 根据请求路径判断 RelayMode
// 使用前缀匹配以支持带查询参数的路径
func Path2RelayMode(path string) RelayMode {
	// 去除前导 /v1/ 或 /v1beta/ 前缀（如果有）
	path = strings.TrimPrefix(path, "/v1beta")
	path = strings.TrimPrefix(path, "/v1")

	// 检查非 /v1 前缀的路径（如 /mj/..., /suno/...）
	if strings.HasPrefix(path, "/mj/") {
		switch {
		case strings.HasPrefix(path, "/mj/submit/"):
			return RelayModeMjSubmit
		case strings.HasPrefix(path, "/mj/task/") && strings.HasSuffix(path, "/fetch"):
			return RelayModeMjFetch
		case strings.HasPrefix(path, "/mj/image/"):
			return RelayModeMjImage
		}
	}

	if strings.HasPrefix(path, "/suno/") {
		switch {
		case strings.HasPrefix(path, "/suno/submit/"):
			return RelayModeSunoSubmit
		case path == "/suno/fetch" || strings.HasPrefix(path, "/suno/fetch/"):
			return RelayModeSunoFetch
		}
	}

	// 分离查询参数
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	var result RelayMode
	switch {
	case strings.HasSuffix(path, "/chat/completions"):
		result = RelayModeChatCompletions
	case strings.HasSuffix(path, "/completions"):
		result = RelayModeCompletions
	case strings.HasSuffix(path, "/embeddings"):
		result = RelayModeEmbeddings
	case strings.HasSuffix(path, "/images/edits"):
		result = RelayModeImagesEdits
	case strings.HasSuffix(path, "/images/generations"):
		result = RelayModeImagesGenerations
	case strings.HasSuffix(path, "/audio/speech"):
		result = RelayModeAudioSpeech
	case strings.HasSuffix(path, "/audio/transcriptions"):
		result = RelayModeAudioTranscription
	case strings.HasSuffix(path, "/audio/translations"):
		result = RelayModeAudioTranslation
	case strings.HasSuffix(path, "/rerank"):
		result = RelayModeRerank
	// /responses/compact 必须在 /responses 之前匹配，避免被更短的后缀截获
	case strings.HasPrefix(path, "/responses/compact"):
		result = RelayModeResponsesCompact
	case strings.HasSuffix(path, "/responses"):
		result = RelayModeResponses
	case strings.HasSuffix(path, "/realtime"):
		result = RelayModeRealtime
	case strings.HasSuffix(path, "/messages"):
		result = RelayModeClaudeMessages
	case strings.HasSuffix(path, ":generateContent"), strings.HasSuffix(path, ":streamGenerateContent"):
		result = RelayModeGeminiChat
	case strings.HasSuffix(path, "/video/generations"):
		result = RelayModeVideoGenerations
	case strings.Contains(path, "/video/generations/"):
		result = RelayModeVideoFetch
	case strings.HasSuffix(path, "/moderations"):
		result = RelayModeModerations
	default:
		result = RelayModeUnknown
	}

	return result
}

// String 返回 RelayMode 的字符串表示
func (m RelayMode) String() string {
	switch m {
	case RelayModeChatCompletions:
		return "chat_completions"
	case RelayModeCompletions:
		return "completions"
	case RelayModeEmbeddings:
		return "embeddings"
	case RelayModeImagesGenerations:
		return "images_generations"
	case RelayModeAudioSpeech:
		return "audio_speech"
	case RelayModeAudioTranscription:
		return "audio_transcriptions"
	case RelayModeAudioTranslation:
		return "audio_translations"
	case RelayModeRerank:
		return "rerank"
	case RelayModeResponses:
		return "responses"
	case RelayModeResponsesCompact:
		return "responses_compact"
	case RelayModeRealtime:
		return "realtime"
	case RelayModeClaudeMessages:
		return "claude_messages"
	case RelayModeGeminiChat:
		return "gemini_generate_content"
	case RelayModeVideoGenerations:
		return "video_generations"
	case RelayModeVideoFetch:
		return "video_fetch"
	case RelayModeSunoSubmit:
		return "suno_submit"
	case RelayModeSunoFetch:
		return "suno_fetch"
	case RelayModeModerations:
		return "moderations"
	case RelayModeImagesEdits:
		return "images_edits"
	case RelayModeMjSubmit:
		return "mj_submit"
	case RelayModeMjFetch:
		return "mj_fetch"
	case RelayModeMjImage:
		return "mj_image"
	default:
		return "unknown"
	}
}

package dto

// 从 relaykit/dto 导入所有类型，保持现有代码兼容
// 使用类型别名机制，现有代码无需修改
import relaykitdto "github.com/qianfree/team-api/relaykit/dto"

// ==================== OpenAI 类型别名 ====================

type GeneralOpenAIRequest = relaykitdto.GeneralOpenAIRequest
type ImageConfigDTO = relaykitdto.ImageConfigDTO
type Message = relaykitdto.Message
type ContentPart = relaykitdto.ContentPart
type ImageURL = relaykitdto.ImageURL
type InputAudio = relaykitdto.InputAudio
type FileData = relaykitdto.FileData
type Tool = relaykitdto.Tool
type FunctionDef = relaykitdto.FunctionDef
type ToolCall = relaykitdto.ToolCall
type FunctionCall = relaykitdto.FunctionCall
type StreamOptions = relaykitdto.StreamOptions
type ResponseFormat = relaykitdto.ResponseFormat
type ChatCompletionResponse = relaykitdto.ChatCompletionResponse
type Choice = relaykitdto.Choice
type ChatCompletionStreamResponse = relaykitdto.ChatCompletionStreamResponse
type StreamChoice = relaykitdto.StreamChoice
type ModelsResponse = relaykitdto.ModelsResponse
type ModelDTO = relaykitdto.ModelDTO
type ModelDetailResponse = relaykitdto.ModelDetailResponse
type ModelModalities = relaykitdto.ModelModalities

// ==================== Claude 类型别名 ====================

type ClaudeRequest = relaykitdto.ClaudeRequest
type ClaudeThinking = relaykitdto.ClaudeThinking
type ClaudeMessage = relaykitdto.ClaudeMessage
type ClaudeContentBlock = relaykitdto.ClaudeContentBlock
type ClaudeSource = relaykitdto.ClaudeSource
type ClaudeCacheControl = relaykitdto.ClaudeCacheControl
type ClaudeTool = relaykitdto.ClaudeTool
type ClaudeToolChoice = relaykitdto.ClaudeToolChoice
type ClaudeResponse = relaykitdto.ClaudeResponse
type ClaudeDelta = relaykitdto.ClaudeDelta
type ClaudeMessageInfo = relaykitdto.ClaudeMessageInfo
type ClaudeCacheUsage = relaykitdto.ClaudeCacheUsage
type ClaudeServerToolUsage = relaykitdto.ClaudeServerToolUsage
type ClaudeUsage = relaykitdto.ClaudeUsage

// ==================== Gemini 类型别名 ====================

type GeminiChatRequest = relaykitdto.GeminiChatRequest
type GeminiContent = relaykitdto.GeminiContent
type GeminiPart = relaykitdto.GeminiPart
type GeminiInlineData = relaykitdto.GeminiInlineData
type GeminiFileData = relaykitdto.GeminiFileData
type GeminiFunctionCall = relaykitdto.GeminiFunctionCall
type GeminiFunctionResponse = relaykitdto.GeminiFunctionResponse
type GeminiExecutableCode = relaykitdto.GeminiExecutableCode
type GeminiCodeExecutionResult = relaykitdto.GeminiCodeExecutionResult
type GeminiVideoMetadata = relaykitdto.GeminiVideoMetadata
type GeminiGenerationConfig = relaykitdto.GeminiGenerationConfig
type GeminiImageConfig = relaykitdto.GeminiImageConfig
type GeminiThinkingConfig = relaykitdto.GeminiThinkingConfig
type GeminiSafetySetting = relaykitdto.GeminiSafetySetting
type GeminiChatResponse = relaykitdto.GeminiChatResponse
type GeminiCandidate = relaykitdto.GeminiCandidate
type GeminiPromptFeedback = relaykitdto.GeminiPromptFeedback
type GeminiUsageMetadata = relaykitdto.GeminiUsageMetadata
type GeminiModalityTokenCount = relaykitdto.GeminiModalityTokenCount
type GeminiFunctionDeclaration = relaykitdto.GeminiFunctionDeclaration
type GeminiModel = relaykitdto.GeminiModel
type GeminiModelsResponse = relaykitdto.GeminiModelsResponse
type GeminiImageRequest = relaykitdto.GeminiImageRequest
type GeminiImageInstance = relaykitdto.GeminiImageInstance
type GeminiImageParameters = relaykitdto.GeminiImageParameters
type GeminiImageResponse = relaykitdto.GeminiImageResponse
type GeminiImagePrediction = relaykitdto.GeminiImagePrediction

// ==================== Audio 类型别名 ====================

type AudioRequest = relaykitdto.AudioRequest
type AudioResponse = relaykitdto.AudioResponse
type WordSegment = relaykitdto.WordSegment
type Segment = relaykitdto.Segment
type WhisperVerboseJSONResponse = relaykitdto.WhisperVerboseJSONResponse

// ==================== Rerank 类型别名 ====================

type RerankRequest = relaykitdto.RerankRequest
type RerankResponse = relaykitdto.RerankResponse
type RerankResponseResult = relaykitdto.RerankResponseResult
type RerankUsage = relaykitdto.RerankUsage
type RerankMeta = relaykitdto.RerankMeta
type RerankBilledUnits = relaykitdto.RerankBilledUnits

// ==================== Realtime 类型别名 ====================

type RealtimeEvent = relaykitdto.RealtimeEvent
type RealtimeSession = relaykitdto.RealtimeSession
type InputAudioTranscription = relaykitdto.InputAudioTranscription
type RealtimeItem = relaykitdto.RealtimeItem
type RealtimeContent = relaykitdto.RealtimeContent
type RealtimeResponse = relaykitdto.RealtimeResponse
type RealtimeUsage = relaykitdto.RealtimeUsage
type RealtimeTokenDetails = relaykitdto.RealtimeTokenDetails
type RealtimeError = relaykitdto.RealtimeError
type RealTimeTool = relaykitdto.RealTimeTool

// ==================== Task 类型别名 ====================

type TaskSubmitRequest = relaykitdto.TaskSubmitRequest
type TaskSubmitResponse = relaykitdto.TaskSubmitResponse
type TaskFetchResponse = relaykitdto.TaskFetchResponse
type SunoSubmitRequest = relaykitdto.SunoSubmitRequest
type SunoFetchResponse = relaykitdto.SunoFetchResponse

// ==================== Usage 类型别名 ====================

type Usage = relaykitdto.Usage
type UsageWithDetails = relaykitdto.UsageWithDetails
type TokenDetails = relaykitdto.TokenDetails
type CompletionsRequest = relaykitdto.CompletionsRequest
type CompletionsResponse = relaykitdto.CompletionsResponse
type CompletionsChoice = relaykitdto.CompletionsChoice
type CompletionsStreamResponse = relaykitdto.CompletionsStreamResponse
type CompletionsStreamChoice = relaykitdto.CompletionsStreamChoice
type EmbeddingRequest = relaykitdto.EmbeddingRequest
type EmbeddingResponse = relaykitdto.EmbeddingResponse
type Embedding = relaykitdto.Embedding
type ImageRequest = relaykitdto.ImageRequest
type ImageResponse = relaykitdto.ImageResponse
type ImageData = relaykitdto.ImageData
type ImageUsage = relaykitdto.ImageUsage
type ImageTokenDetails = relaykitdto.ImageTokenDetails

package dto

// Responses API 类型已下沉至 relaykit/dto（类型权威实现），此处保留类型别名，
// 现有宿主代码无需修改。对齐 relay/dto/relaykit_types.go 的既有别名模式。
import relaykitdto "github.com/qianfree/team-api/relaykit/dto"

// ========== Responses API 请求 ==========

type OpenAIResponsesRequest = relaykitdto.OpenAIResponsesRequest
type Reasoning = relaykitdto.Reasoning

// ========== Responses API 响应 ==========

type OpenAIResponsesResponse = relaykitdto.OpenAIResponsesResponse
type IncompleteDetails = relaykitdto.IncompleteDetails
type ResponsesUsage = relaykitdto.ResponsesUsage
type ResponsesReasoning = relaykitdto.ResponsesReasoning
type ResponsesText = relaykitdto.ResponsesText
type ResponsesTextFormat = relaykitdto.ResponsesTextFormat
type InputTokenDetails = relaykitdto.InputTokenDetails
type OutputTokenDetails = relaykitdto.OutputTokenDetails
type ResponsesOutput = relaykitdto.ResponsesOutput
type ResponsesOutputContent = relaykitdto.ResponsesOutputContent
type ResponsesAnnotation = relaykitdto.ResponsesAnnotation
type ResponsesWebSearchAction = relaykitdto.ResponsesWebSearchAction
type ResponsesWebSearchSource = relaykitdto.ResponsesWebSearchSource
type ResponsesFileSearchResult = relaykitdto.ResponsesFileSearchResult
type ResponsesSafetyCheck = relaykitdto.ResponsesSafetyCheck
type ResponsesCodeInterpreterOutput = relaykitdto.ResponsesCodeInterpreterOutput
type ResponsesShellAction = relaykitdto.ResponsesShellAction
type ResponsesPatchAction = relaykitdto.ResponsesPatchAction
type ResponsesMCPTool = relaykitdto.ResponsesMCPTool

// ========== Responses API 流式响应 ==========

type ResponsesStreamResponse = relaykitdto.ResponsesStreamResponse
type ResponsesStreamEvent = relaykitdto.ResponsesStreamEvent
type ResponsesSummaryPart = relaykitdto.ResponsesSummaryPart

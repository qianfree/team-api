package dto

// Responses API 类型已下沉至 relaykit/dto（协议转换迁移），此处保留类型别名以兼容现有代码。
// 使用类型别名机制，现有代码无需修改；方法（如 OpenAIResponsesResponse.HasError）随类型定义
// 一并迁移，别名可直接调用。
import relaykitdto "github.com/qianfree/team-api/relaykit/dto"

// ==================== Responses API 请求 ====================

type OpenAIResponsesRequest = relaykitdto.OpenAIResponsesRequest
type Reasoning = relaykitdto.Reasoning

// ==================== Responses API 响应 ====================

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

// ==================== Responses API 流式响应 ====================

type ResponsesStreamResponse = relaykitdto.ResponsesStreamResponse
type ResponsesSummaryPart = relaykitdto.ResponsesSummaryPart

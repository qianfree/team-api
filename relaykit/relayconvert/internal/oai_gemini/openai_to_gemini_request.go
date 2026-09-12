package oai_gemini

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/shared"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIToGeminiRequestConverter 将 OpenAI Chat Completions 请求转换为 Gemini Generate Content 请求。
type OpenAIToGeminiRequestConverter struct{}

func (c *OpenAIToGeminiRequestConverter) ID() string {
	return relayconvert.ConverterOpenAIChatToGeminiContent
}

func (c *OpenAIToGeminiRequestConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToGeminiRequestConverter) To() types.RelayFormat {
	return types.RelayFormatGemini
}

func (c *OpenAIToGeminiRequestConverter) Quality() relayconvert.RequestConverterQuality {
	return relayconvert.RequestConverterQualityGood
}

func (c *OpenAIToGeminiRequestConverter) ConvertRequest(
	ctx context.Context,
	info convmeta.Meta,
	request any,
) (any, error) {
	openaiReq, ok := request.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, fmt.Errorf("expected *dto.GeneralOpenAIRequest, got %T", request)
	}

	geminiReq := &dto.GeminiChatRequest{
		Contents: make([]dto.GeminiContent, 0, len(openaiReq.Messages)),
		GenerationConfig: &dto.GeminiGenerationConfig{
			Temperature: openaiReq.Temperature,
			TopP:        openaiReq.TopP,
		},
	}

	// TopK
	if openaiReq.TopK != nil && *openaiReq.TopK > 0 {
		v := float64(*openaiReq.TopK)
		geminiReq.GenerationConfig.TopK = &v
	}

	// MaxOutputTokens
	maxTokens := 0
	if openaiReq.MaxTokens != nil {
		maxTokens = int(*openaiReq.MaxTokens)
	} else if openaiReq.MaxCompletionTokens != nil {
		maxTokens = int(*openaiReq.MaxCompletionTokens)
	}
	if maxTokens > 0 {
		v := uint(maxTokens)
		geminiReq.GenerationConfig.MaxOutputTokens = &v
	}

	// StopSequences（限制为 5 个）
	if stops := parseStopSequences(openaiReq.Stop); len(stops) > 0 {
		if len(stops) > 5 {
			stops = stops[:5]
		}
		geminiReq.GenerationConfig.StopSequences = stops
	}

	// Seed
	if openaiReq.Seed != nil {
		v := int64(*openaiReq.Seed)
		geminiReq.GenerationConfig.Seed = &v
	}

	// PresencePenalty
	if openaiReq.PresencePenalty != nil {
		geminiReq.GenerationConfig.PresencePenalty = openaiReq.PresencePenalty
	}

	// FrequencyPenalty
	if openaiReq.FrequencyPenalty != nil {
		geminiReq.GenerationConfig.FrequencyPenalty = openaiReq.FrequencyPenalty
	}

	// CandidateCount（来自 N 字段）
	if openaiReq.N != nil && *openaiReq.N > 0 {
		geminiReq.GenerationConfig.CandidateCount = openaiReq.N
	}

	// Logprobs
	if openaiReq.LogProbs != nil && *openaiReq.LogProbs {
		geminiReq.GenerationConfig.ResponseLogprobs = openaiReq.LogProbs
	}
	if openaiReq.TopLogProbs != nil && *openaiReq.TopLogProbs > 0 {
		geminiReq.GenerationConfig.Logprobs = openaiReq.TopLogProbs
	}

	// ServiceTier
	if openaiReq.ServiceTier != "" {
		geminiReq.ServiceTier = openaiReq.ServiceTier
	}

	// ReasoningEffort → ThinkingConfig
	if openaiReq.ReasoningEffort != "" {
		geminiReq.GenerationConfig.ThinkingConfig = convertReasoningEffort(openaiReq.ReasoningEffort)
	}

	// ResponseFormat → ResponseMimeType + ResponseSchema
	if openaiReq.ResponseFormat != nil {
		if openaiReq.ResponseFormat.Type == "json_schema" || openaiReq.ResponseFormat.Type == "json_object" {
			geminiReq.GenerationConfig.ResponseMimeType = "application/json"
			if openaiReq.ResponseFormat.JSONSchema != nil {
				geminiReq.GenerationConfig.ResponseSchema = convertResponseSchema(openaiReq.ResponseFormat.JSONSchema)
			}
		}
	}

	// 默认安全设置（宽松）
	geminiReq.SafetySettings = []dto.GeminiSafetySetting{
		{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_ONLY_HIGH"},
		{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_ONLY_HIGH"},
		{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_ONLY_HIGH"},
		{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_ONLY_HIGH"},
	}

	// Tools conversion
	if len(openaiReq.Tools) > 0 {
		geminiTools, err := convertTools(openaiReq.Tools)
		if err != nil {
			return nil, fmt.Errorf("convert tools: %w", err)
		}
		if len(geminiTools) > 0 {
			toolsJSON, err := json.Marshal(geminiTools)
			if err != nil {
				return nil, fmt.Errorf("marshal gemini tools: %w", err)
			}
			geminiReq.Tools = toolsJSON
		}

		// ToolChoice conversion
		if openaiReq.ToolChoice != nil {
			geminiReq.ToolConfig = convertToolChoice(openaiReq.ToolChoice)
		}
	}

	// 服务端联网搜索：chat 的 web_search_options → Gemini 原生 googleSearch。
	//
	// 与其他目标格式不同，本方向受渠道开关 WebSearchToGoogleSearch 控制：
	// googleSearch 是**用 Google 的 grounding 替代客户端请求的搜索**，Google 按搜索
	// 次数另行计价，因此由运营者显式开启（与 Claude→Gemini 方向同一开关、同一理由）。
	//
	// 保守策略同样对齐 Claude→Gemini：仅在无 functionDeclarations 时附加——
	// 部分 Gemini 模型代际不接受 googleSearch 与函数声明同请求，混用时宁可丢搜索
	// 也不让整个请求 400。
	if convmeta.OptionsOf(info).Gemini.WebSearchToGoogleSearch && len(geminiReq.Tools) == 0 {
		if spec := shared.DetectWebSearchFromOpenAI(openaiReq.WebSearchOptions); spec != nil {
			entries := []map[string]any{shared.GeminiGoogleSearchEntry()}
			raw, err := json.Marshal(entries)
			if err != nil {
				return nil, fmt.Errorf("marshal googleSearch tool: %w", err)
			}
			geminiReq.Tools = raw
		}
	}

	// Messages conversion
	toolCallIDs := make(map[string]string) // toolCallID -> functionName
	var systemParts []dto.GeminiPart

	for _, msg := range openaiReq.Messages {
		switch msg.Role {
		case "system", "developer":
			text := extractText(msg.Content)
			if text != "" {
				systemParts = append(systemParts, dto.GeminiPart{Text: text})
			}

		case "user":
			parts := convertUserParts(msg.Content)
			if len(parts) > 0 {
				geminiReq.Contents = append(geminiReq.Contents, dto.GeminiContent{
					Role:  "user",
					Parts: parts,
				})
			}

		case "assistant":
			parts := convertAssistantParts(msg, toolCallIDs)
			if len(parts) > 0 {
				geminiReq.Contents = append(geminiReq.Contents, dto.GeminiContent{
					Role:  "model",
					Parts: parts,
				})
			}

		case "tool":
			// 确保最后一条 content 是 user 角色（Gemini 要求 functionResponse 必须位于 user content 中）
			if len(geminiReq.Contents) == 0 || geminiReq.Contents[len(geminiReq.Contents)-1].Role == "model" {
				geminiReq.Contents = append(geminiReq.Contents, dto.GeminiContent{Role: "user"})
			}
			lastIdx := len(geminiReq.Contents) - 1

			name := msg.Name
			if name == "" {
				name = toolCallIDs[msg.ToolCallID]
			}

			// 工具输出归一化：functionResponse.response 必须是 JSON 对象（Struct），
			// 纯文本（shell 输出等）会被 protojson 直接 400，统一经 shared 包包装
			response := shared.ToolResultToGeminiResponse(extractText(msg.Content))

			geminiReq.Contents[lastIdx].Parts = append(geminiReq.Contents[lastIdx].Parts, dto.GeminiPart{
				FunctionResponse: &dto.GeminiFunctionResponse{
					Name:     name,
					Response: response,
				},
			})
		}
	}

	// SystemInstruction
	if len(systemParts) > 0 {
		geminiReq.SystemInstruction = &dto.GeminiContent{
			Parts: systemParts,
		}
	}

	return geminiReq, nil
}

// 辅助函数

func parseStopSequences(stop any) []string {
	if stop == nil {
		return nil
	}
	switch v := stop.(type) {
	case string:
		if v != "" {
			return []string{v}
		}
	case []string:
		return v
	case []any:
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func extractText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if m["type"] == "text" {
					if t, ok := m["text"].(string); ok {
						parts = append(parts, t)
					}
				}
			}
		}
		if len(parts) > 0 {
			return parts[0] // 简化处理，取第一个 text part
		}
	}
	return ""
}

func convertUserParts(content any) []dto.GeminiPart {
	switch v := content.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []dto.GeminiPart{{Text: v}}

	case []dto.ContentPart:
		// 链式转换（如 Responses→OpenAI→Gemini）产出的类型化部件列表：
		// 经 JSON 往返降为 []any 复用下方 wire 形态逻辑，避免两套多模态分支漂移
		data, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		var anyParts []any
		if err := json.Unmarshal(data, &anyParts); err != nil {
			return nil
		}
		return convertUserParts(anyParts)

	case []any:
		var parts []dto.GeminiPart
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			switch m["type"] {
			case "text":
				if text, ok := m["text"].(string); ok && text != "" {
					parts = append(parts, dto.GeminiPart{Text: text})
				}
			case "image_url":
				if imageURL, ok := m["image_url"].(map[string]any); ok {
					if url, ok := imageURL["url"].(string); ok {
						if mimeType, data, ok := parseDataURL(url); ok {
							parts = append(parts, dto.GeminiPart{
								InlineData: &dto.GeminiInlineData{
									MimeType: mimeType,
									Data:     data,
								},
							})
						}
					}
				}
			case "input_audio":
				if audioData, ok := m["input_audio"].(map[string]any); ok {
					if data, ok := audioData["data"].(string); ok {
						mimeType := "audio/wav"
						if fmt, ok := audioData["format"].(string); ok && fmt != "" {
							mimeType = "audio/" + fmt
						}
						parts = append(parts, dto.GeminiPart{
							InlineData: &dto.GeminiInlineData{
								MimeType: mimeType,
								Data:     data,
							},
						})
					}
				}
			}
		}
		return parts
	}
	return nil
}

func parseDataURL(dataURL string) (mimeType, data string, ok bool) {
	if len(dataURL) < 11 || dataURL[:5] != "data:" {
		return "", "", false
	}
	// 查找分号分隔符
	semiIdx := -1
	for i := 5; i < len(dataURL); i++ {
		if dataURL[i] == ';' {
			semiIdx = i
			break
		}
	}
	if semiIdx == -1 {
		return "", "", false
	}
	mimeType = dataURL[5:semiIdx]
	afterSemi := dataURL[semiIdx+1:]
	if len(afterSemi) < 7 || afterSemi[:7] != "base64," {
		return "", "", false
	}
	data = afterSemi[7:]
	return mimeType, data, true
}

func convertAssistantParts(msg dto.Message, toolCallIDs map[string]string) []dto.GeminiPart {
	var parts []dto.GeminiPart

	text := extractText(msg.Content)
	if text != "" {
		parts = append(parts, dto.GeminiPart{Text: text})
	}

	// ReasoningContent → thought
	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		t := true
		parts = append(parts, dto.GeminiPart{
			Text:    *msg.ReasoningContent,
			Thought: &t,
		})
	}

	// ToolCalls → FunctionCall
	for _, tc := range msg.ToolCalls {
		args := map[string]any{}
		if tc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]any{"raw": tc.Function.Arguments}
			}
		}
		parts = append(parts, dto.GeminiPart{
			FunctionCall: &dto.GeminiFunctionCall{
				FunctionName: tc.Function.Name,
				Arguments:    args,
			},
		})
		toolCallIDs[tc.ID] = tc.Function.Name
	}

	return parts
}

type geminiTool struct {
	FunctionDeclarations []functionDecl `json:"functionDeclarations,omitempty"`
}

type functionDecl struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

func convertTools(tools []dto.Tool) ([]geminiTool, error) {
	var funcDecls []functionDecl
	for _, t := range tools {
		if t.Type != "function" {
			continue
		}
		cleanedParams := shared.CleanGeminiToolParams(t.Function.Parameters)
		funcDecls = append(funcDecls, functionDecl{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  cleanedParams,
		})
	}
	if len(funcDecls) == 0 {
		return nil, nil
	}
	return []geminiTool{{FunctionDeclarations: funcDecls}}, nil
}

func convertToolChoice(toolChoice any) any {
	if toolChoice == nil {
		return nil
	}
	switch v := toolChoice.(type) {
	case string:
		switch v {
		case "auto":
			return map[string]any{"functionCallingConfig": map[string]any{"mode": "AUTO"}}
		case "none":
			return map[string]any{"functionCallingConfig": map[string]any{"mode": "NONE"}}
		case "required":
			return map[string]any{"functionCallingConfig": map[string]any{"mode": "ANY"}}
		default:
			return map[string]any{"functionCallingConfig": map[string]any{"mode": "AUTO"}}
		}
	case map[string]any:
		if v["type"] == "function" {
			config := map[string]any{
				"functionCallingConfig": map[string]any{"mode": "ANY"},
			}
			if fn, ok := v["function"].(map[string]any); ok {
				if name, ok := fn["name"].(string); ok && name != "" {
					config["functionCallingConfig"].(map[string]any)["allowedFunctionNames"] = []string{name}
				}
			}
			return config
		}
	}
	return nil
}

// convertReasoningEffort 将 reasoning_effort 档位展开为 Gemini thinkingBudget。
// 只下发 thinkingBudget、不下发 thinkingLevel：二者互斥（同时携带上游返回 400），
// 且 Gemini 3 对 thinkingBudget 向后兼容。
func convertReasoningEffort(effort string) *dto.GeminiThinkingConfig {
	var budget int
	switch effort {
	case "low":
		budget = 1024
	case "medium":
		budget = 8192
	case "high":
		budget = 32768
	default:
		budget = 8192
	}
	return &dto.GeminiThinkingConfig{
		IncludeThoughts: true,
		ThinkingBudget:  &budget,
	}
}

// convertResponseSchema 把 chat 的 response_format.json_schema 包装（{name,schema,strict}）
// 解包为 Gemini 的 response_schema——必须是**裸 schema**，带着 name/strict 外壳发出会被
// protojson 拒绝（线上实测：Unknown name "name" at 'request.generation_config.response_schema'）。
// 解包后经 CleanGeminiToolParams 归一化：Gemini 对 response_schema 与工具参数 schema
// 应用同一套约束（白名单关键字、每节点须有 type、数组须带 items）。
func convertResponseSchema(schema any) any {
	if schema == nil {
		return nil
	}
	if m, ok := schema.(map[string]any); ok {
		if inner, ok := m["schema"]; ok {
			schema = inner
		}
	}
	return shared.CleanGeminiToolParams(schema)
}

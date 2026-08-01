package oai_gemini

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIToGeminiRequestConverter converts OpenAI Chat Completions request to Gemini Generate Content request.
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

	// StopSequences (limit to 5)
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

	// CandidateCount (from N)
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

	// Default safety settings (permissive)
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
			// Ensure last content is user (Gemini requires functionResponse in user content)
			if len(geminiReq.Contents) == 0 || geminiReq.Contents[len(geminiReq.Contents)-1].Role == "model" {
				geminiReq.Contents = append(geminiReq.Contents, dto.GeminiContent{Role: "user"})
			}
			lastIdx := len(geminiReq.Contents) - 1

			name := msg.Name
			if name == "" {
				name = toolCallIDs[msg.ToolCallID]
			}

			contentStr := extractText(msg.Content)
			var response any = contentStr
			if contentStr != "" {
				var parsed any
				if json.Unmarshal([]byte(contentStr), &parsed) == nil {
					response = parsed
				}
			}

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

// Helper functions

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
			return parts[0] // Use first text part for simplicity
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
	// Find semicolon separator
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
		cleanedParams := cleanParams(t.Function.Parameters)
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

// cleanParams removes Gemini-unsupported JSON Schema fields
func cleanParams(params any) any {
	if params == nil {
		return nil
	}
	m, ok := params.(map[string]any)
	if !ok {
		return params
	}
	cleaned := make(map[string]any)
	for k, v := range m {
		switch k {
		case "type", "description", "properties", "required", "items",
			"anyOf", "default", "enum", "format", "maxLength", "minLength",
			"maximum", "minimum", "pattern", "title", "nullable",
			"maxItems", "minItems", "maxProperties", "minProperties", "example":
			cleaned[k] = v
		}
	}
	return cleaned
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

func convertReasoningEffort(effort string) *dto.GeminiThinkingConfig {
	var budget int
	var level string
	switch effort {
	case "low":
		budget = 1024
		level = "LOW"
	case "medium":
		budget = 8192
		level = "MEDIUM"
	case "high":
		budget = 32768
		level = "HIGH"
	default:
		budget = 8192
		level = "MEDIUM"
	}
	return &dto.GeminiThinkingConfig{
		IncludeThoughts: true,
		ThoughtBudget:   &budget,
		ThinkingLevel:   level,
	}
}

func convertResponseSchema(schema any) any {
	if schema == nil {
		return nil
	}

	// Handle json_schema wrapper: {"type":"json_schema","json_schema":{"schema":{...}}}
	if m, ok := schema.(map[string]any); ok {
		if js, ok := m["json_schema"].(map[string]any); ok {
			if innerSchema, ok := js["schema"]; ok {
				return convertSchemaMap(innerSchema)
			}
		}
		return convertSchemaMap(m)
	}

	return schema
}

// convertSchemaMap recursively converts JSON Schema type names to Gemini format
func convertSchemaMap(schema any) any {
	m, ok := schema.(map[string]any)
	if !ok {
		return schema
	}

	result := make(map[string]any, len(m))
	for k, v := range m {
		switch k {
		case "type":
			if s, ok := v.(string); ok {
				result["type"] = mapSchemaType(s)
			} else {
				result[k] = v
			}
		case "properties":
			if props, ok := v.(map[string]any); ok {
				converted := make(map[string]any, len(props))
				for pk, pv := range props {
					converted[pk] = convertSchemaMap(pv)
				}
				result["properties"] = converted
			} else {
				result[k] = v
			}
		case "items":
			result["items"] = convertSchemaMap(v)
		case "anyOf", "oneOf", "allOf":
			if arr, ok := v.([]any); ok {
				converted := make([]any, len(arr))
				for i, item := range arr {
					converted[i] = convertSchemaMap(item)
				}
				result[k] = converted
			} else {
				result[k] = v
			}
		default:
			result[k] = v
		}
	}
	return result
}

// mapSchemaType maps JSON Schema type names to Gemini Schema type names
func mapSchemaType(t string) string {
	switch t {
	case "string":
		return "STRING"
	case "number":
		return "NUMBER"
	case "integer":
		return "INTEGER"
	case "boolean":
		return "BOOLEAN"
	case "object":
		return "OBJECT"
	case "array":
		return "ARRAY"
	case "null":
		return "NULL"
	default:
		return t
	}
}

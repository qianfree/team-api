package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
)

// ConvertOpenAIToGemini 将 OpenAI 格式请求转换为 Gemini API 格式。
func ConvertOpenAIToGemini(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	var openaiReq dto.GeneralOpenAIRequest
	if err := json.Unmarshal(requestBody, &openaiReq); err != nil {
		return nil, fmt.Errorf("parse openai request: %w", err)
	}

	geminiReq := dto.GeminiChatRequest{
		Contents: make([]dto.GeminiContent, 0, len(openaiReq.Messages)),
		GenerationConfig: &dto.GeminiGenerationConfig{
			Temperature: openaiReq.Temperature,
			TopP:        openaiReq.TopP,
		},
	}

	// topK
	if openaiReq.TopK != nil && *openaiReq.TopK > 0 {
		v := float64(*openaiReq.TopK)
		geminiReq.GenerationConfig.TopK = &v
	}

	// maxOutputTokens
	maxTokens := 0
	if openaiReq.MaxTokens != nil {
		maxTokens = int(*openaiReq.MaxTokens)
	} else if openaiReq.MaxCompletionTokens != nil {
		maxTokens = int(*openaiReq.MaxCompletionTokens)
	}
	if maxTokens > 0 {
		geminiReq.GenerationConfig.MaxOutputTokens = o2gUintPtr(uint(maxTokens))
	}

	// stopSequences
	if stops := o2gParseStopSequences(openaiReq.Stop); len(stops) > 0 {
		if len(stops) > 5 {
			stops = stops[:5]
		}
		geminiReq.GenerationConfig.StopSequences = stops
	}

	// seed
	if openaiReq.Seed != nil {
		geminiReq.GenerationConfig.Seed = o2gInt64Ptr(int64(*openaiReq.Seed))
	}

	// presencePenalty
	if openaiReq.PresencePenalty != nil {
		geminiReq.GenerationConfig.PresencePenalty = openaiReq.PresencePenalty
	}

	// frequencyPenalty
	if openaiReq.FrequencyPenalty != nil {
		geminiReq.GenerationConfig.FrequencyPenalty = openaiReq.FrequencyPenalty
	}

	// n → candidateCount
	if openaiReq.N != nil && *openaiReq.N > 0 {
		n := *openaiReq.N
		geminiReq.GenerationConfig.CandidateCount = &n
	}

	// logprobs
	if openaiReq.LogProbs != nil && *openaiReq.LogProbs {
		geminiReq.GenerationConfig.ResponseLogprobs = openaiReq.LogProbs
	}
	if openaiReq.TopLogProbs != nil && *openaiReq.TopLogProbs > 0 {
		v := *openaiReq.TopLogProbs
		geminiReq.GenerationConfig.Logprobs = &v
	}

	// serviceTier
	if openaiReq.ServiceTier != "" {
		geminiReq.ServiceTier = openaiReq.ServiceTier
	}

	// reasoning_effort → thinkingConfig
	if openaiReq.ReasoningEffort != "" {
		geminiReq.GenerationConfig.ThinkingConfig = o2gConvertReasoningEffort(openaiReq.ReasoningEffort)
	}

	// response_format → responseMimeType + responseSchema
	if openaiReq.ResponseFormat != nil {
		if openaiReq.ResponseFormat.Type == "json_schema" || openaiReq.ResponseFormat.Type == "json_object" {
			geminiReq.GenerationConfig.ResponseMimeType = "application/json"
			if openaiReq.ResponseFormat.JSONSchema != nil {
				geminiReq.GenerationConfig.ResponseSchema = o2gConvertResponseSchema(openaiReq.ResponseFormat.JSONSchema)
			}
		}
	}

	// Chat 内生图：为支持图片输出的模型注入 ResponseModalities
	if isGeminiImageModel(info.ChannelMeta.UpstreamModelName) {
		geminiReq.GenerationConfig.ResponseModalities = []string{"TEXT", "IMAGE"}
		// 透传 imageConfig（宽高比、分辨率）
		if openaiReq.ImageConfig != nil {
			geminiReq.GenerationConfig.ImageConfig = &dto.GeminiImageConfig{
				AspectRatio: openaiReq.ImageConfig.AspectRatio,
				ImageSize:   openaiReq.ImageConfig.ImageSize,
			}
		}
	}

	// 默认安全设置（较宽松，避免过度过滤）
	geminiReq.SafetySettings = []dto.GeminiSafetySetting{
		{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_ONLY_HIGH"},
		{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_ONLY_HIGH"},
		{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_ONLY_HIGH"},
		{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_ONLY_HIGH"},
	}

	// tools 转换
	if len(openaiReq.Tools) > 0 {
		geminiTools, err := o2gConvertTools(openaiReq.Tools)
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

		// tool_choice 转换
		if openaiReq.ToolChoice != nil {
			geminiReq.ToolConfig = o2gConvertToolChoice(openaiReq.ToolChoice)
		}
	}

	// messages 转换
	toolCallIDs := make(map[string]string) // toolCallID -> functionName
	var systemParts []dto.GeminiPart

	for _, msg := range openaiReq.Messages {
		switch msg.Role {
		case "system", "developer":
			text := o2gExtractText(msg.Content)
			if text != "" {
				systemParts = append(systemParts, dto.GeminiPart{Text: text})
			}

		case "user":
			parts := o2gConvertUserParts(msg.Content)
			if len(parts) > 0 {
				geminiReq.Contents = append(geminiReq.Contents, dto.GeminiContent{
					Role:  "user",
					Parts: parts,
				})
			}

		case "assistant":
			parts := o2gConvertAssistantParts(msg, toolCallIDs)
			if len(parts) > 0 {
				geminiReq.Contents = append(geminiReq.Contents, dto.GeminiContent{
					Role:  "model",
					Parts: parts,
				})
			}

		case "tool":
			// 确保最后一个 content 是 user（Gemini 要求 functionResponse 在 user content 中）
			if len(geminiReq.Contents) == 0 || geminiReq.Contents[len(geminiReq.Contents)-1].Role == "model" {
				geminiReq.Contents = append(geminiReq.Contents, dto.GeminiContent{Role: "user"})
			}
			lastIdx := len(geminiReq.Contents) - 1

			name := msg.Name
			if name == "" {
				name = toolCallIDs[msg.ToolCallID]
			}

			contentStr := o2gExtractText(msg.Content)
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

	// system instructions
	if len(systemParts) > 0 {
		geminiReq.SystemInstruction = &dto.GeminiContent{
			Parts: systemParts,
		}
	}

	result, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}
	return bytes.NewReader(result), nil
}

// ConvertClaudeToGemini 将 Claude 格式请求转换为 Gemini 格式。
// 链式转换：Claude → OpenAI → Gemini
func ConvertClaudeToGemini(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	openaiReader, err := openai.ConvertClaudeToOpenAI(requestBody, info)
	if err != nil {
		return nil, fmt.Errorf("claude to openai: %w", err)
	}
	openaiBody, err := io.ReadAll(openaiReader)
	if err != nil {
		return nil, fmt.Errorf("read openai intermediate: %w", err)
	}
	return ConvertOpenAIToGemini(openaiBody, info)
}

// ConvertResponsesToGemini 将 Responses API 格式请求转换为 Gemini 格式。
// 链式转换：Responses → OpenAI → Gemini
func ConvertResponsesToGemini(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	openaiReader, err := openai.ConvertResponsesToOpenAI(requestBody, info)
	if err != nil {
		return nil, fmt.Errorf("responses to openai: %w", err)
	}
	openaiBody, err := io.ReadAll(openaiReader)
	if err != nil {
		return nil, fmt.Errorf("read openai intermediate: %w", err)
	}
	return ConvertOpenAIToGemini(openaiBody, info)
}

// ===== OpenAI → Gemini 内部工具函数 =====

// o2gGeminiTool Gemini 工具定义（内部序列化用）
type o2gGeminiTool struct {
	FunctionDeclarations []o2gFunctionDecl `json:"functionDeclarations,omitempty"`
}

// o2gFunctionDecl Gemini 函数声明
type o2gFunctionDecl struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

func o2gConvertTools(tools []dto.Tool) ([]o2gGeminiTool, error) {
	var funcDecls []o2gFunctionDecl
	for _, t := range tools {
		if t.Type != "function" {
			continue
		}
		cleanedParams := o2gCleanParams(t.Function.Parameters)
		funcDecls = append(funcDecls, o2gFunctionDecl{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  cleanedParams,
		})
	}
	if len(funcDecls) == 0 {
		return nil, nil
	}
	return []o2gGeminiTool{{FunctionDeclarations: funcDecls}}, nil
}

// o2gCleanParams 清理函数参数，移除 Gemini 不支持的字段
func o2gCleanParams(params any) any {
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

func o2gConvertToolChoice(toolChoice any) any {
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

func o2gParseStopSequences(stop any) []string {
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

func o2gExtractText(content any) string {
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
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func o2gConvertUserParts(content any) []dto.GeminiPart {
	switch v := content.(type) {
	case string:
		if v == "" {
			return nil
		}
		return o2gParseMarkdownImages(v)
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
					parts = append(parts, o2gParseMarkdownImages(text)...)
				}
			case "image_url":
				if imageURL, ok := m["image_url"].(map[string]any); ok {
					if url, ok := imageURL["url"].(string); ok {
						if mimeType, data, ok := o2gParseDataURL(url); ok {
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
			case "file":
				if fileData, ok := m["file"].(map[string]any); ok {
					if url, ok := fileData["url"].(string); ok {
						mimeType := "application/octet-stream"
						if mt, ok := fileData["mime_type"].(string); ok && mt != "" {
							mimeType = mt
						}
						parts = append(parts, dto.GeminiPart{
							FileData: &dto.GeminiFileData{
								MimeType: mimeType,
								FileURI:  url,
							},
						})
					}
				}
			}
		}
		return parts
	default:
		return nil
	}
}

func o2gParseMarkdownImages(text string) []dto.GeminiPart {
	var parts []dto.GeminiPart

	for {
		startIdx := strings.Index(text, "![")
		if startIdx == -1 {
			break
		}
		bracketIdx := strings.Index(text[startIdx:], "](data:")
		if bracketIdx == -1 {
			break
		}
		bracketIdx += startIdx
		closeIdx := strings.Index(text[bracketIdx+2:], ")")
		if closeIdx == -1 {
			break
		}
		closeIdx += bracketIdx + 2

		if startIdx > 0 {
			before := text[:startIdx]
			if before != "" {
				parts = append(parts, dto.GeminiPart{Text: before})
			}
		}

		dataURL := text[bracketIdx+2 : closeIdx]
		if mimeType, data, ok := o2gParseDataURL(dataURL); ok {
			parts = append(parts, dto.GeminiPart{
				InlineData: &dto.GeminiInlineData{MimeType: mimeType, Data: data},
			})
		}

		text = text[closeIdx+1:]
	}

	if len(parts) == 0 {
		if text != "" {
			return []dto.GeminiPart{{Text: text}}
		}
		return nil
	}
	if text != "" {
		parts = append(parts, dto.GeminiPart{Text: text})
	}
	return parts
}

func o2gParseDataURL(dataURL string) (mimeType, data string, ok bool) {
	if len(dataURL) < 11 || dataURL[:5] != "data:" {
		return "", "", false
	}
	rest := dataURL[5:]
	semiIdx := strings.Index(rest, ";")
	if semiIdx == -1 {
		return "", "", false
	}
	mimeType = rest[:semiIdx]
	afterSemi := rest[semiIdx+1:]
	if len(afterSemi) < 7 || afterSemi[:7] != "base64," {
		return "", "", false
	}
	data = afterSemi[7:]
	return mimeType, data, true
}

func o2gConvertAssistantParts(msg dto.Message, toolCallIDs map[string]string) []dto.GeminiPart {
	var parts []dto.GeminiPart

	text := o2gExtractText(msg.Content)
	if text != "" {
		parts = append(parts, dto.GeminiPart{Text: text})
	}

	// reasoning_content → thought (for multi-turn)
	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		t := true
		parts = append(parts, dto.GeminiPart{
			Text:    *msg.ReasoningContent,
			Thought: &t,
		})
	}

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

func o2gUintPtr(v uint) *uint    { return &v }
func o2gInt64Ptr(v int64) *int64 { return &v }

// o2gConvertReasoningEffort 将 OpenAI reasoning_effort 转换为 Gemini thinkingConfig
func o2gConvertReasoningEffort(effort string) *dto.GeminiThinkingConfig {
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

// o2gConvertResponseSchema 将 OpenAI 的 JSON Schema 转换为 Gemini Schema 格式。
// OpenAI 使用小写类型名 ("string")，Gemini 使用大写 ("STRING")。
// 还处理 OpenAI 的 json_schema 包装格式：{"type":"json_schema","json_schema":{"schema":{...}}}
func o2gConvertResponseSchema(schema any) any {
	if schema == nil {
		return nil
	}

	// 处理 json_schema 包装格式
	if m, ok := schema.(map[string]any); ok {
		// 检查是否是 {"type":"json_schema","json_schema":{"schema":{...}}} 格式
		if js, ok := m["json_schema"].(map[string]any); ok {
			if innerSchema, ok := js["schema"]; ok {
				return o2gConvertSchemaMap(innerSchema)
			}
		}
		return o2gConvertSchemaMap(m)
	}

	return schema
}

// o2gConvertSchemaMap 递归转换 JSON Schema 类型名为 Gemini 格式
func o2gConvertSchemaMap(schema any) any {
	m, ok := schema.(map[string]any)
	if !ok {
		return schema
	}

	result := make(map[string]any, len(m))
	for k, v := range m {
		switch k {
		case "type":
			if s, ok := v.(string); ok {
				result["type"] = o2gMapSchemaType(s)
			} else {
				result[k] = v
			}
		case "properties":
			if props, ok := v.(map[string]any); ok {
				converted := make(map[string]any, len(props))
				for pk, pv := range props {
					converted[pk] = o2gConvertSchemaMap(pv)
				}
				result["properties"] = converted
			} else {
				result[k] = v
			}
		case "items":
			result["items"] = o2gConvertSchemaMap(v)
		case "anyOf", "oneOf", "allOf":
			if arr, ok := v.([]any); ok {
				converted := make([]any, len(arr))
				for i, item := range arr {
					converted[i] = o2gConvertSchemaMap(item)
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

// o2gMapSchemaType 将 JSON Schema 类型名映射为 Gemini Schema 类型名
func o2gMapSchemaType(t string) string {
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

// isGeminiImageModel 检测模型是否支持 Chat 内生图，需要注入 ResponseModalities
func isGeminiImageModel(model string) bool {
	imageModels := []string{
		"gemini-2.0-flash-exp-image-generation",
		"gemini-2.0-flash-exp",
		"gemini-3-pro-image-preview",
		"gemini-2.5-flash-image",
		"gemini-3.1-flash-image-preview",
		// Nano Banana 别名
		"nano-banana",
		"nano-banana-2-preview",
		"nano-banana-pro-preview",
	}
	for _, m := range imageModels {
		if model == m || strings.HasPrefix(model, m+"-") {
			return true
		}
	}
	return false
}

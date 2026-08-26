package oai_gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

	// ReasoningEffort → ThinkingConfig（请求体显式指定）
	if openaiReq.ReasoningEffort != "" {
		geminiReq.GenerationConfig.ThinkingConfig = convertReasoningEffort(openaiReq.ReasoningEffort)
	} else if info != nil {
		// 模型名 thinking 后缀（-thinking/-low 等）：gemini adaptor 的 injectGeminiThinking
		// 在桥接路径不执行，此处吸收该语义；请求体已显式设置 effort 时以请求体为准
		thinkingInfo := shared.ParseThinkingSuffix(info.GetUpstreamModelName())
		if thinkingInfo.IsThinking || thinkingInfo.IsNoThinking || thinkingInfo.EffortLevel != "" {
			shared.ApplyThinkingToGemini(geminiReq.GenerationConfig, thinkingInfo, convmeta.OptionsOf(info).Gemini)
		}
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

	// web_search_options（responses 入站的 web_search 托管工具经 r2c 提取）→
	// Gemini googleSearch grounding；独立于上方 tools 块（无 function 工具时搜索仍应生效）。
	// 与已有 functionDeclarations 合并为追加的 tool 条目
	if len(openaiReq.WebSearchOptions) > 0 {
		var existing []geminiTool
		if len(geminiReq.Tools) > 0 {
			_ = json.Unmarshal(geminiReq.Tools, &existing)
		}
		existing = append(existing, geminiTool{GoogleSearch: &googleSearchTool{}})
		toolsJSON, err := json.Marshal(existing)
		if err != nil {
			return nil, fmt.Errorf("marshal gemini tools with google search: %w", err)
		}
		geminiReq.Tools = toolsJSON
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

			contentStr := extractText(msg.Content)
			var response any = contentStr
			if contentStr != "" {
				var parsed any
				if json.Unmarshal([]byte(contentStr), &parsed) == nil {
					response = parsed
				}
			}
			// Gemini 要求 functionResponse.response 必须是 JSON 对象（Struct），
			// 纯文本/数组/标量等非对象结果统一包进 {"result": ...}，否则上游按 map 解析会直接报错
			if _, isMap := response.(map[string]any); !isMap {
				response = map[string]any{"result": response}
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
	case []dto.ContentPart:
		// 链式第一跳的 typed 切片（system/assistant/tool 消息文本经此提取）
		for _, part := range v {
			if part.Type == "text" {
				return part.Text
			}
		}
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
		// 链式转换第一跳（claude→openai / responses→openai）产出的 typed 切片——
		// dispatch 原样传对象无 JSON 往返，缺失此分支会整条丢弃消息（含文字）
		var parts []dto.GeminiPart
		for _, part := range v {
			switch part.Type {
			case "text":
				if part.Text != "" {
					parts = append(parts, dto.GeminiPart{Text: part.Text})
				}
			case "image_url":
				if part.ImageURL != nil {
					if mimeType, data, ok := parseDataURL(part.ImageURL.URL); ok {
						parts = append(parts, dto.GeminiPart{
							InlineData: &dto.GeminiInlineData{
								MimeType: mimeType,
								Data:     data,
							},
						})
					}
				}
			case "input_audio":
				if part.InputAudio != nil && part.InputAudio.Data != "" {
					mimeType := "audio/wav"
					if part.InputAudio.Format != "" {
						mimeType = "audio/" + part.InputAudio.Format
					}
					parts = append(parts, dto.GeminiPart{
						InlineData: &dto.GeminiInlineData{
							MimeType: mimeType,
							Data:     part.InputAudio.Data,
						},
					})
				}
			}
		}
		return parts

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
	textIdx, thoughtIdx := -1, -1

	text := extractText(msg.Content)
	if text != "" {
		parts = append(parts, dto.GeminiPart{Text: text})
		textIdx = len(parts) - 1
	}

	// ReasoningContent → thought
	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		t := true
		parts = append(parts, dto.GeminiPart{
			Text:    *msg.ReasoningContent,
			Thought: &t,
		})
		thoughtIdx = len(parts) - 1
	}

	// ToolCalls → FunctionCall（工具级 thoughtSignature 直挂对应 part）
	firstBareFCIdx := -1
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
			ThoughtSignature: tc.ThoughtSignature,
		})
		if tc.ThoughtSignature == "" && firstBareFCIdx == -1 {
			firstBareFCIdx = len(parts) - 1
		}
		toolCallIDs[tc.ID] = tc.Function.Name
	}

	// 消息级 thoughtSignature 回填（claude 链的 thinking 块签名走消息级）：
	// 优先首个无签名 functionCall part（Gemini 3 的强校验点），其次 thought part，
	// 最后 text part（纯文本轮次的签名附着位）
	if msg.ThoughtSignature != "" {
		switch {
		case firstBareFCIdx >= 0:
			parts[firstBareFCIdx].ThoughtSignature = msg.ThoughtSignature
		case thoughtIdx >= 0:
			parts[thoughtIdx].ThoughtSignature = msg.ThoughtSignature
		case textIdx >= 0:
			parts[textIdx].ThoughtSignature = msg.ThoughtSignature
		}
	}

	return parts
}

type geminiTool struct {
	FunctionDeclarations []functionDecl `json:"functionDeclarations,omitempty"`
	// GoogleSearch grounding 托管搜索（web_search_options 经 r2c 提取后映射至此；
	// 与 functionDeclarations 分属不同 tool 条目，由调用方分别 append）
	GoogleSearch *googleSearchTool `json:"googleSearch,omitempty"`
}

// googleSearchTool Gemini googleSearch grounding 配置（空对象即启用）
type googleSearchTool struct{}

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

// geminiSchemaAllowedFields Gemini Schema 支持的 OpenAPI 子集字段白名单。
// additionalProperties/$schema/oneOf 等 JSON Schema 扩展不在其中，透传会被
// Gemini 以 400 "Unknown name ... Cannot find field" 拒绝。
var geminiSchemaAllowedFields = map[string]struct{}{
	"anyOf":            {},
	"default":          {},
	"description":      {},
	"enum":             {},
	"example":          {},
	"format":           {},
	"items":            {},
	"maxItems":         {},
	"maxLength":        {},
	"maxProperties":    {},
	"maximum":          {},
	"minItems":         {},
	"minLength":        {},
	"minProperties":    {},
	"minimum":          {},
	"nullable":         {},
	"pattern":          {},
	"properties":       {},
	"propertyOrdering": {},
	"required":         {},
	"title":            {},
	"type":             {},
}

// geminiSchemaMaxDepth schema 递归清洗深度上限，超过后浅清洗（防深嵌套栈溢出/DoS）
const geminiSchemaMaxDepth = 64

// cleanParams 清洗工具参数 schema 为 Gemini Schema 子集。
// 必须递归到 properties/items/anyOf 的每一层：codex 等 CLI 的工具 schema（schemars 生成）
// 在每个嵌套 object 上都携带 additionalProperties:false，仅清洗顶层会让嵌套层透传，
// 导致上游 400 "Unknown name additionalProperties"（顶层已洗掉、嵌套漏网的形态）。
func cleanParams(params any) any {
	return cleanGeminiSchema(params, 0)
}

// cleanGeminiSchema 递归剔除 Gemini 不支持的 schema 字段并规范化 type。
// 行为对齐 new-api 的 shared/gemini.CleanFunctionParameters。
func cleanGeminiSchema(schema any, depth int) any {
	if schema == nil {
		return nil
	}
	if depth >= geminiSchemaMaxDepth {
		return cleanGeminiSchemaShallow(schema)
	}
	switch v := schema.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(v))
		for k, val := range v {
			if _, ok := geminiSchemaAllowedFields[k]; ok {
				cleaned[k] = val
			}
		}
		normalizeGeminiSchemaType(cleaned)

		if props, ok := cleaned["properties"].(map[string]any); ok && props != nil {
			cleanedProps := make(map[string]any, len(props))
			for name, pv := range props {
				cleanedProps[name] = cleanGeminiSchema(pv, depth+1)
			}
			cleaned["properties"] = cleanedProps
		}
		if items, ok := cleaned["items"].(map[string]any); ok && items != nil {
			cleaned["items"] = cleanGeminiSchema(items, depth+1)
		}
		// draft-2020 之前的 items 数组形态：以第一个元素为元素 schema
		if itemsArr, ok := cleaned["items"].([]any); ok && len(itemsArr) > 0 {
			cleaned["items"] = cleanGeminiSchema(itemsArr[0], depth+1)
		}
		if nested, ok := cleaned["anyOf"].([]any); ok && nested != nil {
			cleanedAnyOf := make([]any, len(nested))
			for i, item := range nested {
				cleanedAnyOf[i] = cleanGeminiSchema(item, depth+1)
			}
			cleaned["anyOf"] = cleanedAnyOf
		}
		return cleaned
	case []any:
		cleanedArr := make([]any, len(v))
		for i, item := range v {
			cleanedArr[i] = cleanGeminiSchema(item, depth+1)
		}
		return cleanedArr
	default:
		return schema
	}
}

// cleanGeminiSchemaShallow 深度超限后的兜底：仅过滤本层字段、不再下钻
func cleanGeminiSchemaShallow(schema any) any {
	switch v := schema.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(v))
		for k, val := range v {
			if _, ok := geminiSchemaAllowedFields[k]; ok {
				cleaned[k] = val
			}
		}
		normalizeGeminiSchemaType(cleaned)
		delete(cleaned, "properties")
		delete(cleaned, "items")
		delete(cleaned, "anyOf")
		return cleaned
	case []any:
		return []any{}
	default:
		return schema
	}
}

// normalizeGeminiSchemaType 规范化 type 字段：小写 JSON Schema 类型名 → Gemini 大写枚举名；
// 数组形态（schemars 对 Option<T> 生成的 ["string","null"]）折叠为单一 type + nullable:true；
// 纯 "null" 折叠为 nullable:true（Gemini 枚举无 NULL 类型）。
func normalizeGeminiSchemaType(schema map[string]any) {
	rawType, ok := schema["type"]
	if !ok || rawType == nil {
		return
	}

	normalize := func(t string) (string, bool) {
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "object":
			return "OBJECT", false
		case "array":
			return "ARRAY", false
		case "string":
			return "STRING", false
		case "integer":
			return "INTEGER", false
		case "number":
			return "NUMBER", false
		case "boolean":
			return "BOOLEAN", false
		case "null":
			return "", true
		default:
			return t, false
		}
	}

	switch typed := rawType.(type) {
	case string:
		normalized, isNull := normalize(typed)
		if isNull {
			schema["nullable"] = true
			delete(schema, "type")
			return
		}
		schema["type"] = normalized
	case []any:
		nullable := false
		var chosen string
		for _, item := range typed {
			s, ok := item.(string)
			if !ok {
				continue
			}
			normalized, isNull := normalize(s)
			if isNull {
				nullable = true
				continue
			}
			if chosen == "" {
				chosen = normalized
			}
		}
		if nullable {
			schema["nullable"] = true
		}
		if chosen != "" {
			schema["type"] = chosen
		} else {
			delete(schema, "type")
		}
	}
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

// convertResponseSchema 将 response_format.json_schema 的 schema 转换为 Gemini ResponseSchema。
// 与工具参数共用 cleanGeminiSchema：structured output 的 strict 模式 schema 顶层即携带
// additionalProperties:false，未清洗同样会被上游以 "Unknown name" 拒绝。
func convertResponseSchema(schema any) any {
	if schema == nil {
		return nil
	}

	// 处理 json_schema 包装结构：{"type":"json_schema","json_schema":{"schema":{...}}}
	if m, ok := schema.(map[string]any); ok {
		if js, ok := m["json_schema"].(map[string]any); ok {
			if innerSchema, ok := js["schema"]; ok {
				return cleanGeminiSchema(innerSchema, 0)
			}
		}
		return cleanGeminiSchema(m, 0)
	}

	return schema
}

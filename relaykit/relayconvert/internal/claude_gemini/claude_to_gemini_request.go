package claude_gemini

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

const (
	// claudeMaxTokensFallback 源请求未携带 max_tokens 时的兜底值（与 Claude→OpenAI 跳同口径）
	claudeMaxTokensFallback = 4096
	// geminiMaxStopSequences Gemini generationConfig.stopSequences 上限
	geminiMaxStopSequences = 5
)

// ClaudeToGeminiRequestConverter 将 Claude Messages 请求直连转换为 Gemini GenerateContent 请求。
//
// 与「Claude→OpenAI→Gemini」步骤链相比，直连不经中枢格式，故能保住 OpenAI 无法承载的信息：
//   - thinking.signature → part.thoughtSignature：Gemini 多轮思考续传的凭据，
//     链式在 Claude→OpenAI 跳被丢弃，回传时思考块缺签名会被上游判为非法请求
//   - thinking.budget_tokens → thoughtBudget 精确值：链式先折叠为 low/medium/high
//     再展开为 1024/8192/32768，5000 会被放大成 8192
//   - top_k → generationConfig.topK：OpenAI chat 无该字段，链式直接丢失
//   - tool_use.id → functionCall.id：链式经 OpenAI tool_calls 后被 Gemini 跳丢弃
//   - system 文本块边界：链式压平为单个字符串
type ClaudeToGeminiRequestConverter struct{}

func (c *ClaudeToGeminiRequestConverter) ID() string {
	return relayconvert.ConverterClaudeMessagesToGeminiContent
}

func (c *ClaudeToGeminiRequestConverter) From() types.RelayFormat {
	return types.RelayFormatClaude
}

func (c *ClaudeToGeminiRequestConverter) To() types.RelayFormat {
	return types.RelayFormatGemini
}

func (c *ClaudeToGeminiRequestConverter) Quality() relayconvert.RequestConverterQuality {
	return relayconvert.RequestConverterQualityFair
}

func (c *ClaudeToGeminiRequestConverter) ConvertRequest(
	ctx context.Context,
	info convmeta.Meta,
	request any,
) (any, error) {
	claudeReq, ok := request.(*dto.ClaudeRequest)
	if !ok {
		return nil, fmt.Errorf("expected *dto.ClaudeRequest, got %T", request)
	}

	// Gemini 的目标模型经 URL 路径传递（请求体无 model 字段），故无需写入上游模型名
	geminiReq := &dto.GeminiChatRequest{
		Contents: convertClaudeMessagesToContents(claudeReq.Messages),
		GenerationConfig: &dto.GeminiGenerationConfig{
			Temperature: claudeReq.Temperature,
			TopP:        claudeReq.TopP,
		},
	}

	maxTokens := claudeMaxTokensFallback
	if claudeReq.MaxTokens != nil {
		maxTokens = int(*claudeReq.MaxTokens)
	}
	if maxTokens > 0 {
		v := uint(maxTokens)
		geminiReq.GenerationConfig.MaxOutputTokens = &v
	}

	// top_k → topK（Claude 与 Gemini 均有该字段，链式因中枢缺字段而丢失）
	if claudeReq.TopK != nil && *claudeReq.TopK > 0 {
		v := float64(*claudeReq.TopK)
		geminiReq.GenerationConfig.TopK = &v
	}

	// stop_sequences：Gemini 上限 5 个，超出截断（与链式同口径）
	if len(claudeReq.StopSequences) > 0 {
		stops := claudeReq.StopSequences
		if len(stops) > geminiMaxStopSequences {
			stops = stops[:geminiMaxStopSequences]
		}
		geminiReq.GenerationConfig.StopSequences = stops
	}

	if thinking := claudeThinkingToGemini(claudeReq.Thinking); thinking != nil {
		geminiReq.GenerationConfig.ThinkingConfig = thinking
	}

	// 默认宽松安全设置（与 OpenAI→Gemini 路径同口径，渠道策略不因入站格式而变）
	geminiReq.SafetySettings = defaultGeminiSafetySettings()

	if len(claudeReq.Tools) > 0 {
		toolsJSON, err := claudeToolsToGemini(claudeReq.Tools)
		if err != nil {
			return nil, fmt.Errorf("convert tools: %w", err)
		}
		if len(toolsJSON) > 0 {
			geminiReq.Tools = toolsJSON
			if claudeReq.ToolChoice != nil {
				geminiReq.ToolConfig = claudeToolChoiceToGemini(claudeReq.ToolChoice)
			}
		}
	}

	if system := claudeSystemToGemini(claudeReq.System); system != nil {
		geminiReq.SystemInstruction = system
	}

	return geminiReq, nil
}

// ========== generationConfig 映射 ==========

// claudeThinkingToGemini 将 Claude thinking 配置转换为 Gemini thinkingConfig。
// 与链式（budget→reasoning_effort→budget）不同，此处 budget 原值直传；
// thinkingLevel 仍按同一套档位阈值给出，供只认 thinkingLevel 的模型使用。
// type 非 enabled（disabled / 缺省）时不附加 thinkingConfig，与链式同口径。
func claudeThinkingToGemini(thinking *dto.ClaudeThinking) *dto.GeminiThinkingConfig {
	if thinking == nil || thinking.Type != "enabled" {
		return nil
	}
	cfg := &dto.GeminiThinkingConfig{
		IncludeThoughts: true,
		ThinkingLevel:   claudeThinkingBudgetToLevel(thinking.BudgetTokens),
	}
	if thinking.BudgetTokens != nil && *thinking.BudgetTokens > 0 {
		cfg.ThoughtBudget = intPtr(*thinking.BudgetTokens)
	}
	return cfg
}

// claudeThinkingBudgetToLevel 按 budget 档位给出 Gemini thinkingLevel（LOW/MEDIUM/HIGH）。
// 阈值与 Claude→OpenAI 跳的 reasoning_effort 折算保持一致。
func claudeThinkingBudgetToLevel(budget *int) string {
	if budget == nil {
		return "MEDIUM"
	}
	switch {
	case *budget <= 2048:
		return "LOW"
	case *budget <= 16384:
		return "MEDIUM"
	default:
		return "HIGH"
	}
}

// defaultGeminiSafetySettings 返回宽松的默认安全设置。
func defaultGeminiSafetySettings() []dto.GeminiSafetySetting {
	return []dto.GeminiSafetySetting{
		{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_ONLY_HIGH"},
		{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_ONLY_HIGH"},
		{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_ONLY_HIGH"},
		{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_ONLY_HIGH"},
	}
}

// ========== tools / tool_choice 映射 ==========

// claudeToolsToGemini 将 Claude tools 序列化为 Gemini 的 tools 字段
// （json.RawMessage 形态的 {functionDeclarations: [...]}）。
// Claude 的服务端内置工具（web_search / computer 等）无 input_schema，Gemini 无法表达，跳过。
func claudeToolsToGemini(tools []dto.ClaudeTool) (json.RawMessage, error) {
	decls := make([]dto.GeminiFunctionDeclaration, 0, len(tools))
	for _, t := range tools {
		if t.Name == "" || (t.Type != "" && t.Type != "custom") {
			continue
		}
		decls = append(decls, dto.GeminiFunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  shared.CleanGeminiToolParams(t.InputSchema),
		})
	}
	if len(decls) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(map[string]any{"functionDeclarations": decls})
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// claudeToolChoiceToGemini 将 Claude tool_choice 转换为 Gemini toolConfig。
func claudeToolChoiceToGemini(toolChoice any) any {
	tcType, name := claudeToolChoiceFields(toolChoice)
	functionCallingConfig := map[string]any{"mode": "AUTO"}
	switch tcType {
	case "any":
		functionCallingConfig["mode"] = "ANY"
	case "none":
		functionCallingConfig["mode"] = "NONE"
	case "tool":
		functionCallingConfig["mode"] = "ANY"
		if name != "" {
			functionCallingConfig["allowedFunctionNames"] = []string{name}
		}
	}
	return map[string]any{"functionCallingConfig": functionCallingConfig}
}

// claudeToolChoiceFields 读取 tool_choice 的 type / name，兼容字符串与对象两种入站形态。
func claudeToolChoiceFields(toolChoice any) (tcType, name string) {
	switch v := toolChoice.(type) {
	case string:
		return v, ""
	case map[string]any:
		t, _ := v["type"].(string)
		n, _ := v["name"].(string)
		return t, n
	case *dto.ClaudeToolChoice:
		if v == nil {
			return "", ""
		}
		return v.Type, v.Name
	case dto.ClaudeToolChoice:
		return v.Type, v.Name
	}
	return "", ""
}

// ========== system / messages 映射 ==========

// claudeSystemToGemini 将 Claude system 转换为 Gemini systemInstruction。
// 与链式（压平为单个字符串）不同，此处保留文本块的划分。
func claudeSystemToGemini(system any) *dto.GeminiContent {
	var texts []string
	switch v := system.(type) {
	case nil:
		return nil
	case string:
		if v != "" {
			texts = append(texts, v)
		}
	case []any:
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok || m["type"] != "text" {
				continue
			}
			if text, ok := m["text"].(string); ok && text != "" {
				texts = append(texts, text)
			}
		}
	case []dto.ClaudeContentBlock:
		for i := range v {
			if v[i].Type == "text" && v[i].Text != nil && *v[i].Text != "" {
				texts = append(texts, *v[i].Text)
			}
		}
	}
	if len(texts) == 0 {
		return nil
	}
	parts := make([]dto.GeminiPart, 0, len(texts))
	for _, text := range texts {
		parts = append(parts, dto.GeminiPart{Text: text})
	}
	return &dto.GeminiContent{Parts: parts}
}

// convertClaudeMessagesToContents 将 Claude messages 转换为 Gemini contents。
// Claude 的 user/assistant 分别落到 Gemini 的 user/model。
func convertClaudeMessagesToContents(messages []dto.ClaudeMessage) []dto.GeminiContent {
	// tool_result 块只带 tool_use_id，而 Gemini 的 functionResponse 用函数名关联，
	// 需先扫一遍 assistant 的 tool_use 块建立 id→name 映射。
	toolNames := claudeToolUseNames(messages)

	contents := make([]dto.GeminiContent, 0, len(messages))
	for i := range messages {
		blocks := c2gNormalizeContent(messages[i].Content)
		var parts []dto.GeminiPart
		role := ""
		switch messages[i].Role {
		case "user":
			role, parts = "user", claudeUserParts(blocks, toolNames)
		case "assistant":
			role, parts = "model", claudeModelParts(blocks)
		}
		// Gemini 拒绝 parts 为空的 content，整条消息无可搬运内容时跳过
		if len(parts) > 0 {
			contents = append(contents, dto.GeminiContent{Role: role, Parts: parts})
		}
	}
	return contents
}

// claudeToolUseNames 建立 tool_use_id → 函数名映射（供 functionResponse 关联）。
func claudeToolUseNames(messages []dto.ClaudeMessage) map[string]string {
	names := make(map[string]string)
	for i := range messages {
		if messages[i].Role != "assistant" {
			continue
		}
		for _, b := range c2gNormalizeContent(messages[i].Content) {
			if b.Type == "tool_use" && b.ID != "" && b.Name != "" {
				names[b.ID] = b.Name
			}
		}
	}
	return names
}

// claudeUserParts 转换 Claude user 消息的内容块。
// tool_result 落到 functionResponse，Gemini 要求其位于 user content 内，
// 而本函数产出的整个 content 即 role=user，故文本块与工具结果可共存于同一条 content。
func claudeUserParts(blocks []c2gBlock, toolNames map[string]string) []dto.GeminiPart {
	parts := make([]dto.GeminiPart, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				parts = append(parts, dto.GeminiPart{Text: b.Text})
			}
		case "image", "document":
			if part, ok := claudeMediaPart(b.Source); ok {
				parts = append(parts, part)
			}
		case "tool_result":
			name := toolNames[b.ToolUseID]
			if name == "" {
				// 找不到对应 tool_use（历史被截断等）：Gemini 只能按名关联，无法搬运
				continue
			}
			parts = append(parts, dto.GeminiPart{
				FunctionResponse: &dto.GeminiFunctionResponse{
					ID:       b.ToolUseID,
					Name:     name,
					Response: claudeToolResultToResponse(b.Result),
				},
			})
		}
	}
	return parts
}

// claudeModelParts 转换 Claude assistant 消息的内容块。
func claudeModelParts(blocks []c2gBlock) []dto.GeminiPart {
	parts := make([]dto.GeminiPart, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				parts = append(parts, dto.GeminiPart{Text: b.Text})
			}
		case "thinking":
			if b.Thinking == "" {
				continue
			}
			parts = append(parts, dto.GeminiPart{
				Text:             b.Thinking,
				Thought:          boolPtr(true),
				ThoughtSignature: b.Signature,
			})
		case "redacted_thinking":
			// Claude 的加密思考块，Gemini 无对应物，丢弃（与响应侧同口径）
		case "tool_use":
			parts = append(parts, dto.GeminiPart{
				FunctionCall: &dto.GeminiFunctionCall{
					ID:           b.ID,
					FunctionName: b.Name,
					Arguments:    claudeToolInput(b.Input),
				},
			})
		}
	}
	return parts
}

// claudeMediaPart 将 Claude 的多模态 source 转换为 Gemini part。
// base64 数据走 inlineData；URL 走 fileData（Gemini 的 fileData 官方只接受
// Files API URI，非 Files URI 的地址由上游判定，此处原样搬运而非静默丢弃）。
func claudeMediaPart(source *dto.ClaudeSource) (dto.GeminiPart, bool) {
	if source == nil {
		return dto.GeminiPart{}, false
	}
	switch {
	case source.Data != "":
		return dto.GeminiPart{InlineData: &dto.GeminiInlineData{
			MimeType: source.MediaType,
			Data:     source.Data,
		}}, true
	case source.URL != "":
		return dto.GeminiPart{FileData: &dto.GeminiFileData{
			MimeType: source.MediaType,
			FileURI:  source.URL,
		}}, true
	}
	return dto.GeminiPart{}, false
}

// claudeToolResultToResponse 归一化 tool_result.content 为 Gemini functionResponse.response。
// Gemini 要求 response 是对象，而 Claude 允许字符串或内容块数组，
// 非对象结果统一包成 {"result": ...}（链式原样传字符串会被上游拒绝）。
func claudeToolResultToResponse(content any) any {
	switch v := content.(type) {
	case nil:
		return map[string]any{}
	case map[string]any:
		return v
	case string:
		if v == "" {
			return map[string]any{}
		}
		var parsed any
		if json.Unmarshal([]byte(v), &parsed) == nil {
			if m, ok := parsed.(map[string]any); ok {
				return m
			}
		}
		return map[string]any{"result": v}
	case []any:
		var texts []string
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok || m["type"] != "text" {
				continue
			}
			if text, ok := m["text"].(string); ok {
				texts = append(texts, text)
			}
		}
		return map[string]any{"result": c2gJoinText(texts)}
	case []dto.ClaudeContentBlock:
		var texts []string
		for i := range v {
			if v[i].Text != nil {
				texts = append(texts, *v[i].Text)
			}
		}
		return map[string]any{"result": c2gJoinText(texts)}
	default:
		return map[string]any{"result": fmt.Sprintf("%v", v)}
	}
}

// ========== 内容块归一化 ==========

// c2gBlock Claude 内容块的归一化视图。
// 入站 message.content 在宿主侧解码后有 []any（原始 JSON）与 []dto.ClaudeContentBlock
// （已解析类型）两种形态，统一为一种形态，避免下游逐处断言。
type c2gBlock struct {
	Type      string
	Text      string
	Thinking  string
	Signature string
	ID        string
	Name      string
	Input     any
	ToolUseID string
	Result    any
	Source    *dto.ClaudeSource
}

// c2gNormalizeContent 将 Claude message.content 归一化为内容块列表，
// 字符串形态视为单个 text 块。
func c2gNormalizeContent(content any) []c2gBlock {
	switch v := content.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []c2gBlock{{Type: "text", Text: v}}
	case []any:
		blocks := make([]c2gBlock, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				blocks = append(blocks, c2gBlockFromMap(m))
			}
		}
		return blocks
	case []map[string]any:
		blocks := make([]c2gBlock, 0, len(v))
		for _, m := range v {
			blocks = append(blocks, c2gBlockFromMap(m))
		}
		return blocks
	case []dto.ClaudeContentBlock:
		blocks := make([]c2gBlock, 0, len(v))
		for i := range v {
			blocks = append(blocks, c2gBlockFromTyped(&v[i]))
		}
		return blocks
	}
	return nil
}

// c2gBlockFromMap 从原始 JSON map 形态提取内容块。
func c2gBlockFromMap(m map[string]any) c2gBlock {
	b := c2gBlock{
		Type:      c2gString(m["type"]),
		Text:      c2gString(m["text"]),
		Thinking:  c2gString(m["thinking"]),
		Signature: c2gString(m["signature"]),
		ID:        c2gString(m["id"]),
		Name:      c2gString(m["name"]),
		Input:     m["input"],
		ToolUseID: c2gString(m["tool_use_id"]),
		Result:    m["content"],
	}
	if source, ok := m["source"].(map[string]any); ok {
		b.Source = &dto.ClaudeSource{
			Type:      c2gString(source["type"]),
			MediaType: c2gString(source["media_type"]),
			Data:      c2gString(source["data"]),
			URL:       c2gString(source["url"]),
		}
	}
	return b
}

// c2gBlockFromTyped 从已解析的类型化形态提取内容块。
func c2gBlockFromTyped(block *dto.ClaudeContentBlock) c2gBlock {
	b := c2gBlock{
		Type:      block.Type,
		Signature: block.Signature,
		ID:        block.ID,
		Name:      block.Name,
		Input:     block.Input,
		ToolUseID: block.ToolUseID,
		Result:    block.Content,
		Source:    block.Source,
	}
	if block.Text != nil {
		b.Text = *block.Text
	}
	if block.Thinking != nil {
		b.Thinking = *block.Thinking
	}
	return b
}

// c2gString 读取值为字符串，非字符串返回空串。
func c2gString(v any) string {
	s, _ := v.(string)
	return s
}

// c2gJoinText 用换行符拼接文本片段。
func c2gJoinText(parts []string) string {
	joined := make([]byte, 0, 64)
	for i, p := range parts {
		if i > 0 {
			joined = append(joined, '\n')
		}
		joined = append(joined, p...)
	}
	return string(joined)
}

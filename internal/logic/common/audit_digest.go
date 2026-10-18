package common

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/qianfree/team-api/relaykit/dispatch"
)

// ============================================================
// 会话关联摘要（设计见 docs/会话关联与完整会话提取方案.md §5）
//
// 无状态对话协议下客户端每轮重发全部历史，本文件在审计写入时从
// 原始请求/响应体提取 O(1) 单记录特征，供查询侧把同一会话的
// 记录分组串联并还原完整对话：
//   - 会话键：显式会话信号（X-Session-Id 头 / Claude metadata.user_id /
//     Responses previous_response_id）派生，复用路由侧同一解析链
//   - 会话指纹：system + 首条 user 消息哈希，同会话各轮共享
//   - 衔接哈希对：本条响应的 assistant 消息哈希（resp_msg_hash）与
//     请求 messages 末条 assistant 消息哈希（last_asst_msg_hash），
//     下一轮的后者 == 上一轮的前者 ⇒ 两轮衔接
//
// 关键约束：哈希必须在 ApplyAuditLevel / truncateBody 之前的原始值上
// 计算，关联能力才不受审计级别与 2 KiB 截断影响。
// ============================================================

// SessionDigest 写时计算的会话关联摘要，全部为单记录特征，不依赖跨记录查询
type SessionDigest struct {
	SessionKey      string // sk:<source>:<sha256>，仅显式会话信号命中时非空
	SessionSource   string // header / anthropic / openai
	ConvFingerprint string // fp1:<sha256>，system+首条 user 消息指纹
	RespMsgHash     string // mh1:<sha256>，响应 assistant 消息哈希
	LastAsstMsgHash string // mh1:<sha256>，请求末条 assistant 消息哈希
	UserField       string // OpenAI user 字段原文（hint，不参与键解析）
	IsContextReset  bool   // 写时检测到上下文压缩/重写标记
}

// canonMsg 规范化消息：请求侧与响应侧必须共用同一规范化（衔接哈希可匹配的前提）。
// 哈希只关心三类锚点消息（system / 首条 user / 末条 assistant），
// 工具结果（Claude tool_result 块、OpenAI role=tool 消息）不参与任何哈希，天然被跳过。
// 文本拼接一律直接连接不加分隔符——响应侧按流式分片拼接、请求侧按
// 客户端回传的分段拼接，两侧语义必须一致。
// ToolCallID 仅供渲染路径标注工具结果与调用的对应关系（omitempty，
// 哈希锚点消息不含该字段，序列化输出不变，存量哈希不受影响）。
type canonMsg struct {
	Role       string      `json:"role"`
	Text       string      `json:"text"`
	ToolCalls  []canonTool `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// canonTool 规范化工具调用：请求侧 tool_use/tool_calls/functionCall 与
// 响应侧（含流式分片拼接结果）汇聚到同一结构。Args 为参数 JSON 文本，
// 各协议原样取值（OpenAI 协议本身是字符串、Claude/Gemini 取 RawMessage 原文）
type canonTool struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Args string `json:"args,omitempty"`
}

const (
	mediaPlaceholder  = "<media>"       // 非文本内容段占位（媒体负载不参与哈希，与长度无关保证确定性）
	toolResultMarker  = "<tool_result>" // 工具结果块标记（保留消息边界信息）
	sseScannerMaxLine = 1024 * 1024     // SSE 单行上限（防长 data 行截断，大于 ExtractStreamText 的 64KB）
)

// 哈希版本前缀：规范化规则升级时递增，新旧哈希按前缀天然隔离不混用
const (
	fingerprintPrefix = "fp1:"
	msgHashPrefix     = "mh1:"
)

// contextResetMarkers 上下文压缩/续接标记（写时 best-effort 识别；
// 权威判定在会话重组查询时按相邻记录消息量骤降进行）
var contextResetMarkers = []*regexp.Regexp{
	regexp.MustCompile(`\[Conversation summarized`),
	regexp.MustCompile(`\[Request interrupted[^\]]*compaction\]`),
	regexp.MustCompile(`This session is being continued from a previous conversation`),
}

// convShape 请求侧会话形状（指纹与衔接哈希的原料）
type convShape struct {
	system    *canonMsg
	firstUser *canonMsg
	lastAsst  *canonMsg
}

// ExtractSessionDigest 计算会话关联摘要。必须在审计级别处理与截断之前、
// 以原始请求/响应体调用。非对话端点（embeddings/images/任务等）只解析
// 会话键，内容字段留空。
func ExtractSessionDigest(path, rawReqBody, rawRespBody string, isStream bool, headerSessionID string, tenantID int64, model string) SessionDigest {
	var d SessionDigest

	// 会话键：用会话稳定键解析（header → Claude metadata → conversation_id），
	// 与调度亲和键（ResolveSessionKey）优先级不同——previous_response_id 每轮
	// 漂移只适合调度亲和，做审计会话键会导致各轮键互不相同、聚合失败。
	// 无稳定信号时零值（Source 空）即无键，身份推断属查询时行为不落键
	var reqMap map[string]json.RawMessage
	if rawReqBody != "" {
		_ = json.Unmarshal([]byte(rawReqBody), &reqMap) // 解析失败（二进制体等）按无协议信号处理
	}
	sig := dispatch.SessionSignals{HeaderSessionID: headerSessionID}
	if reqMap != nil {
		extractProtocolSignals(reqMap, &sig)
	}
	key := dispatch.ResolveConversationStableKey(dispatch.RequestProfile{
		TenantID: tenantID,
		Model:    model,
		Signals:  sig,
	})
	if key.Source != "" {
		d.SessionKey = key.Key
		d.SessionSource = sessionSourceName(key.Source)
	}

	if !isConversationPath(path) {
		return d
	}

	shape := parseConversationRequest(path, reqMap)
	d.ConvFingerprint = fingerprint(shape.system, shape.firstUser)
	if shape.lastAsst != nil {
		d.LastAsstMsgHash = msgHash(shape.lastAsst)
	}
	if m := extractRespAssistant(path, rawRespBody, isStream); m != nil {
		d.RespMsgHash = msgHash(m)
	}
	if reqMap != nil {
		if v, ok := reqMap["user"]; ok {
			_ = json.Unmarshal(v, &d.UserField)
		}
	}
	for _, re := range contextResetMarkers {
		if re.MatchString(rawReqBody) {
			d.IsContextReset = true
			break
		}
	}
	return d
}

// extractProtocolSignals 从请求体提取协议内会话信号。
// 与 relay/handler 的 extractSessionSignals 字段口径保持一致（那边是
// 包内函数无法复用，协议字段变化时两处同步）
func extractProtocolSignals(reqMap map[string]json.RawMessage, sig *dispatch.SessionSignals) {
	if v, ok := reqMap["metadata"]; ok {
		var meta struct {
			UserID string `json:"user_id"`
		}
		if json.Unmarshal(v, &meta) == nil {
			sig.AnthropicUserID = meta.UserID
		}
	}
	if v, ok := reqMap["previous_response_id"]; ok {
		_ = json.Unmarshal(v, &sig.PreviousResponseID)
	}
	if v, ok := reqMap["conversation_id"]; ok {
		_ = json.Unmarshal(v, &sig.ConversationID)
	}
}

// sessionSourceName 会话键来源落库名（dispatch 侧 hdr 对齐列注释的 header 口径）
func sessionSourceName(src dispatch.SessionSource) string {
	if src == dispatch.SourceHeader {
		return "header"
	}
	return string(src)
}

// isConversationPath 对话端点判定（path 形态与 recordAudit 传入一致：
// /chat/completions、/messages、/responses、/models/{model}:generateContent）
func isConversationPath(path string) bool {
	switch {
	case strings.HasSuffix(path, "/chat/completions"),
		strings.HasSuffix(path, "/messages"),
		strings.HasSuffix(path, "/responses"),
		isGeminiPath(path):
		return true
	}
	return false
}

// isGeminiPath Gemini 端点判定。流式 action 为 streamGenerateContent
// （大写 G），与 generateContent 大小写不一致，统一小写比较
func isGeminiPath(path string) bool {
	return strings.Contains(strings.ToLower(path), "generatecontent")
}

// parseConversationRequest 按协议分派解析请求侧会话形状
func parseConversationRequest(path string, reqMap map[string]json.RawMessage) convShape {
	var shape convShape
	if reqMap == nil {
		return shape
	}
	switch {
	case strings.HasSuffix(path, "/messages"):
		shape = parseClaudeRequest(reqMap)
	case strings.HasSuffix(path, "/responses"):
		shape = parseResponsesRequest(reqMap)
	case isGeminiPath(path):
		shape = parseGeminiRequest(reqMap)
	default:
		shape = parseOpenAIChatRequest(reqMap)
	}
	return shape
}

// ============ 哈希 ============

// msgHash 规范化消息哈希；空文本且无工具调用的退化消息不产哈希，
// 避免大量空哈希互相误链（衔接失败自然降级到下一层关联）
func msgHash(m *canonMsg) string {
	if m == nil || (m.Text == "" && len(m.ToolCalls) == 0) {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return msgHashPrefix + hex.EncodeToString(sum[:])
}

// fingerprint 会话指纹：system + 首条 user 消息哈希。
// 首条 user 缺失（纯 system 或无消息体）不产指纹——只靠 system 会把
// 所有同系统提示词的对话错误合并
func fingerprint(system, firstUser *canonMsg) string {
	if firstUser == nil {
		return ""
	}
	type material struct {
		System *canonMsg `json:"sys,omitempty"`
		First  *canonMsg `json:"first,omitempty"`
	}
	b, err := json.Marshal(material{System: system, First: firstUser})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return fingerprintPrefix + hex.EncodeToString(sum[:])
}

// ============ 请求侧：OpenAI chat completions ============

// oaiMessage 仅声明哈希所需字段：content 用 RawMessage 延迟解析，
// 媒体载荷字段不进结构体，编码 json 跳过未知字段不会拷贝 base64
type oaiMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	ToolCallID string          `json:"tool_call_id"` // role=tool 消息的结果归属（渲染标注用）
	ToolCalls  []struct {
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"`
}

func parseOpenAIChatRequest(reqMap map[string]json.RawMessage) convShape {
	var shape convShape
	raw, ok := reqMap["messages"]
	if !ok {
		return shape
	}
	var msgs []oaiMessage
	if json.Unmarshal(raw, &msgs) != nil {
		return shape
	}
	for i := range msgs {
		m := &msgs[i]
		cm := canonMsg{Role: m.Role, Text: contentText(m.Content)}
		for _, tc := range m.ToolCalls {
			cm.ToolCalls = append(cm.ToolCalls, canonTool{ID: tc.ID, Name: tc.Function.Name, Args: tc.Function.Arguments})
		}
		switch m.Role {
		case "system", "developer":
			if shape.system == nil {
				c := cm
				shape.system = &c
			}
		case "user":
			if shape.firstUser == nil {
				c := cm
				shape.firstUser = &c
			}
		case "assistant":
			c := cm
			shape.lastAsst = &c
		}
	}
	return shape
}

// contentText OpenAI 系 content（string 或分段数组）→ 文本：
// text 段直接连接（响应侧为整串文本，分段拼接语义必须一致），
// 非文本段（image_url/file 等）媒体占位
func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range parts {
		if p.Text != "" {
			sb.WriteString(p.Text)
		} else {
			sb.WriteString(mediaPlaceholder)
		}
	}
	return sb.String()
}

// ============ 请求侧：Claude messages ============

type claudeMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// claudeBlock content 块：input 用 RawMessage 原样取参数 JSON 文本，
// Content 为 tool_result 的结果载荷（字符串或块数组）
type claudeBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

func parseClaudeRequest(reqMap map[string]json.RawMessage) convShape {
	var shape convShape
	if raw, ok := reqMap["system"]; ok {
		text, _ := claudeContent(raw)
		if text != "" {
			shape.system = &canonMsg{Role: "system", Text: text}
		}
	}
	raw, ok := reqMap["messages"]
	if !ok {
		return shape
	}
	var msgs []claudeMessage
	if json.Unmarshal(raw, &msgs) != nil {
		return shape
	}
	for i := range msgs {
		text, tools := claudeContent(msgs[i].Content)
		cm := canonMsg{Role: msgs[i].Role, Text: text, ToolCalls: tools}
		switch msgs[i].Role {
		case "user":
			if shape.firstUser == nil {
				c := cm
				shape.firstUser = &c
			}
		case "assistant":
			c := cm
			shape.lastAsst = &c
		}
	}
	return shape
}

// claudeContent Claude content（字符串或块数组）→ 文本与工具调用。
// thinking/redacted_thinking 跳过（响应侧同理），tool_use 进工具结构，
// tool_result 与媒体块以标记占位保留边界信息
func claudeContent(raw json.RawMessage) (text string, tools []canonTool) {
	if len(raw) == 0 {
		return "", nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, nil
	}
	var blocks []claudeBlock
	if json.Unmarshal(raw, &blocks) != nil {
		return "", nil
	}
	var sb strings.Builder
	for _, b := range blocks {
		switch b.Type {
		case "text":
			sb.WriteString(b.Text)
		case "thinking", "redacted_thinking":
			// 思考块不参与哈希：两侧一致跳过
		case "tool_use":
			tools = append(tools, canonTool{ID: b.ID, Name: b.Name, Args: string(b.Input)})
		case "tool_result":
			sb.WriteString(toolResultMarker)
		default:
			sb.WriteString(mediaPlaceholder)
		}
	}
	return sb.String(), tools
}

// ============ 请求侧：Gemini generateContent ============

type geminiPart struct {
	Text         string `json:"text"`
	FunctionCall *struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	} `json:"functionCall"`
	FunctionResponse *struct {
		Name     string          `json:"name"`
		Response json.RawMessage `json:"response"`
	} `json:"functionResponse"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

func parseGeminiRequest(reqMap map[string]json.RawMessage) convShape {
	var shape convShape
	if raw, ok := reqMap["systemInstruction"]; ok {
		if text := geminiInstructionText(raw); text != "" {
			shape.system = &canonMsg{Role: "system", Text: text}
		}
	} else if raw, ok := reqMap["system_instruction"]; ok {
		if text := geminiInstructionText(raw); text != "" {
			shape.system = &canonMsg{Role: "system", Text: text}
		}
	}
	raw, ok := reqMap["contents"]
	if !ok {
		return shape
	}
	var contents []geminiContent
	if json.Unmarshal(raw, &contents) != nil {
		return shape
	}
	for i := range contents {
		cm := geminiContentCanon(contents[i])
		switch cm.Role {
		case "user":
			if shape.firstUser == nil {
				c := cm
				shape.firstUser = &c
			}
		case "assistant":
			c := cm
			shape.lastAsst = &c
		}
	}
	return shape
}

// geminiContentCanon Gemini content → 规范化消息。
// role: model→assistant；缺省 role 视为 user（单轮请求可省略 role）。
// 文本 parts 直接连接（与流式分片拼接语义一致），functionCall 各自成调用，
// 其余段（inlineData/functionResponse）媒体占位
func geminiContentCanon(c geminiContent) canonMsg {
	role := c.Role
	switch role {
	case "model":
		role = "assistant"
	case "":
		role = "user"
	}
	var sb strings.Builder
	var tools []canonTool
	for _, p := range c.Parts {
		if p.FunctionCall != nil {
			tools = append(tools, canonTool{Name: p.FunctionCall.Name, Args: string(p.FunctionCall.Args)})
			continue
		}
		if p.Text != "" {
			sb.WriteString(p.Text)
		} else {
			sb.WriteString(mediaPlaceholder)
		}
	}
	return canonMsg{Role: role, Text: sb.String(), ToolCalls: tools}
}

// geminiInstructionText systemInstruction（字符串或 {parts:[...]}）→ 文本
func geminiInstructionText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var c struct {
		Parts []geminiPart `json:"parts"`
	}
	if json.Unmarshal(raw, &c) != nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range c.Parts {
		if p.Text != "" {
			sb.WriteString(p.Text)
		}
	}
	return sb.String()
}

// ============ 请求侧：OpenAI Responses ============

func parseResponsesRequest(reqMap map[string]json.RawMessage) convShape {
	var shape convShape
	if raw, ok := reqMap["instructions"]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil && s != "" {
			shape.system = &canonMsg{Role: "system", Text: s}
		}
	}
	raw, ok := reqMap["input"]
	if !ok {
		return shape
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if s != "" {
			shape.firstUser = &canonMsg{Role: "user", Text: s}
		}
		return shape
	}
	var items []struct {
		Type    string          `json:"type"`
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if json.Unmarshal(raw, &items) != nil {
		return shape
	}
	for i := range items {
		it := &items[i]
		if it.Type != "" && it.Type != "message" {
			continue // function_call / reasoning 等项不参与锚点哈希
		}
		cm := canonMsg{Role: it.Role, Text: contentText(it.Content)}
		switch it.Role {
		case "user":
			if shape.firstUser == nil {
				c := cm
				shape.firstUser = &c
			}
		case "assistant":
			c := cm
			shape.lastAsst = &c
		}
	}
	return shape
}

// ============ 完整消息序列解析（会话渲染用） ============

// parseConversationBody 从原始请求体字符串解析完整消息序列（解析失败返回 nil）
func parseConversationBody(path, rawBody string) []canonMsg {
	if rawBody == "" {
		return nil
	}
	var reqMap map[string]json.RawMessage
	if json.Unmarshal([]byte(rawBody), &reqMap) != nil {
		return nil
	}
	return parseConversationMessages(path, reqMap)
}

// parseConversationMessages 按协议解析请求的完整消息序列（含历史），
// 供会话重组渲染。system（各协议的顶字段）作为首条消息插入；
// Responses 的非 message 项（function_call/reasoning）跳过。
// 解析失败返回 nil——哈希列不受影响，仅该记录不贡献渲染消息。
func parseConversationMessages(path string, reqMap map[string]json.RawMessage) []canonMsg {
	if reqMap == nil {
		return nil
	}
	switch {
	case strings.HasSuffix(path, "/messages"):
		return claudeAllMessages(reqMap)
	case strings.HasSuffix(path, "/responses"):
		return responsesAllMessages(reqMap)
	case isGeminiPath(path):
		return geminiAllMessages(reqMap)
	default:
		return openAIAllMessages(reqMap)
	}
}

func openAIAllMessages(reqMap map[string]json.RawMessage) []canonMsg {
	raw, ok := reqMap["messages"]
	if !ok {
		return nil
	}
	var msgs []oaiMessage
	if json.Unmarshal(raw, &msgs) != nil {
		return nil
	}
	out := make([]canonMsg, 0, len(msgs))
	for i := range msgs {
		m := &msgs[i]
		cm := canonMsg{Role: m.Role, Text: contentText(m.Content), ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			cm.ToolCalls = append(cm.ToolCalls, canonTool{ID: tc.ID, Name: tc.Function.Name, Args: tc.Function.Arguments})
		}
		out = append(out, cm)
	}
	return out
}

// claudeAllMessages 渲染路径：与哈希路径（claudeContent）分离——
// tool_result 块拆成独立 role=tool 消息并提取实际结果内容
// （审核需要看到工具返回了什么），user 消息保留纯文本部分。
func claudeAllMessages(reqMap map[string]json.RawMessage) []canonMsg {
	var out []canonMsg
	if raw, ok := reqMap["system"]; ok {
		if text, _ := claudeContent(raw); text != "" {
			out = append(out, canonMsg{Role: "system", Text: text})
		}
	}
	raw, ok := reqMap["messages"]
	if !ok {
		return out
	}
	var msgs []claudeMessage
	if json.Unmarshal(raw, &msgs) != nil {
		return out
	}
	for i := range msgs {
		// 非 user 消息（assistant 等）的 content 无 tool_result，直接走共用规范化
		if msgs[i].Role != "user" {
			text, tools := claudeContent(msgs[i].Content)
			out = append(out, canonMsg{Role: msgs[i].Role, Text: text, ToolCalls: tools})
			continue
		}
		var blocks []claudeBlock
		if json.Unmarshal(msgs[i].Content, &blocks) != nil {
			// 字符串 content：无工具结果
			var s string
			if json.Unmarshal(msgs[i].Content, &s) == nil && s != "" {
				out = append(out, canonMsg{Role: "user", Text: s})
			}
			continue
		}
		var textParts []string
		var tools []canonTool
		for _, b := range blocks {
			switch b.Type {
			case "text":
				textParts = append(textParts, b.Text)
			case "tool_result":
				out = append(out, canonMsg{
					Role:       "tool",
					Text:       claudeToolResultText(b),
					ToolCallID: b.ToolUseID,
				})
			case "tool_use":
				tools = append(tools, canonTool{ID: b.ID, Name: b.Name, Args: string(b.Input)})
			case "thinking", "redacted_thinking":
			default:
				textParts = append(textParts, mediaPlaceholder)
			}
		}
		text := strings.Join(textParts, "")
		if text != "" || len(tools) > 0 {
			out = append(out, canonMsg{Role: "user", Text: text, ToolCalls: tools})
		}
	}
	return out
}

// claudeToolResultText 提取 tool_result 的结果载荷：
// 字符串原样；块数组取 text 块拼接，媒体块占位
func claudeToolResultText(b claudeBlock) string {
	if len(b.Content) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(b.Content, &s) == nil {
		return s
	}
	var blocks []claudeBlock
	if json.Unmarshal(b.Content, &blocks) != nil {
		return ""
	}
	var sb strings.Builder
	for _, rb := range blocks {
		switch rb.Type {
		case "text":
			sb.WriteString(rb.Text)
		default:
			sb.WriteString(mediaPlaceholder)
		}
	}
	return sb.String()
}

// geminiAllMessages 渲染路径：functionResponse 拆成独立 role=tool 消息
// （结果对象按 JSON 文本展示），其余走共用规范化
func geminiAllMessages(reqMap map[string]json.RawMessage) []canonMsg {
	var out []canonMsg
	if raw, ok := reqMap["systemInstruction"]; ok {
		if text := geminiInstructionText(raw); text != "" {
			out = append(out, canonMsg{Role: "system", Text: text})
		}
	} else if raw, ok := reqMap["system_instruction"]; ok {
		if text := geminiInstructionText(raw); text != "" {
			out = append(out, canonMsg{Role: "system", Text: text})
		}
	}
	raw, ok := reqMap["contents"]
	if !ok {
		return out
	}
	var contents []geminiContent
	if json.Unmarshal(raw, &contents) != nil {
		return out
	}
	for i := range contents {
		c := contents[i]
		role := c.Role
		switch role {
		case "model":
			role = "assistant"
		case "":
			role = "user"
		}
		var textParts []string
		var tools []canonTool
		for _, p := range c.Parts {
			switch {
			case p.FunctionCall != nil:
				tools = append(tools, canonTool{Name: p.FunctionCall.Name, Args: string(p.FunctionCall.Args)})
			case p.FunctionResponse != nil:
				respText := string(p.FunctionResponse.Response)
				if respText == "" {
					respText = "{}"
				}
				out = append(out, canonMsg{
					Role:       "tool",
					Text:       respText,
					ToolCallID: p.FunctionResponse.Name,
				})
			case p.Text != "":
				textParts = append(textParts, p.Text)
			default:
				textParts = append(textParts, mediaPlaceholder)
			}
		}
		out = append(out, canonMsg{Role: role, Text: strings.Join(textParts, ""), ToolCalls: tools})
	}
	return out
}

func responsesAllMessages(reqMap map[string]json.RawMessage) []canonMsg {
	var out []canonMsg
	if raw, ok := reqMap["instructions"]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil && s != "" {
			out = append(out, canonMsg{Role: "system", Text: s})
		}
	}
	raw, ok := reqMap["input"]
	if !ok {
		return out
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if s != "" {
			out = append(out, canonMsg{Role: "user", Text: s})
		}
		return out
	}
	var items []struct {
		Type    string          `json:"type"`
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
		CallID  string          `json:"call_id"`
		Name    string          `json:"name"`
		Args    string          `json:"arguments"`
		Output  json.RawMessage `json:"output"`
	}
	if json.Unmarshal(raw, &items) != nil {
		return out
	}
	for i := range items {
		it := &items[i]
		switch {
		case it.Type == "function_call":
			// 渲染路径保留工具调用项（哈希路径跳过——两侧一致跳过不影响锚点）
			out = append(out, canonMsg{Role: "assistant", ToolCalls: []canonTool{{
				ID: it.CallID, Name: it.Name, Args: it.Args,
			}}})
		case it.Type == "function_call_output":
			out = append(out, canonMsg{Role: "tool", Text: string(it.Output), ToolCallID: it.CallID})
		default:
			// message 项与无 type 项（兼容裸数组形态）
			if it.Type != "" && it.Type != "message" {
				continue
			}
			out = append(out, canonMsg{Role: it.Role, Text: contentText(it.Content)})
		}
	}
	return out
}

// ============ 响应侧（与请求侧共用 canonMsg 规范化） ============

// extractRespAssistant 从原始响应提取 assistant 规范化消息：
// 非流式直接解析；流式按协议汇聚增量（含工具调用分片拼接）。
// 提取失败/无 assistant 内容返回 nil，该轮衔接哈希留空降级到下一层关联
func extractRespAssistant(path, rawRespBody string, isStream bool) *canonMsg {
	if rawRespBody == "" {
		return nil
	}
	if isStream {
		return extractRespAssistantStream(path, rawRespBody)
	}
	var respMap map[string]json.RawMessage
	if json.Unmarshal([]byte(rawRespBody), &respMap) != nil {
		return nil
	}
	switch {
	case strings.HasSuffix(path, "/messages"):
		return claudeRespAssistant(respMap)
	case strings.HasSuffix(path, "/responses"):
		return responsesRespAssistant(respMap)
	case isGeminiPath(path):
		return geminiRespAssistant(respMap)
	default:
		return openAIRespAssistant(respMap)
	}
}

func extractRespAssistantStream(path, rawRespBody string) *canonMsg {
	switch {
	case strings.HasSuffix(path, "/messages"):
		return claudeRespAssistantStream(rawRespBody)
	case strings.HasSuffix(path, "/responses"):
		return responsesRespAssistantStream(rawRespBody)
	case isGeminiPath(path):
		return geminiRespAssistantStream(rawRespBody)
	default:
		return openAIRespAssistantStream(rawRespBody)
	}
}

// openAIRespAssistant 非流式：末个含 message 的 choice
func openAIRespAssistant(respMap map[string]json.RawMessage) *canonMsg {
	raw, ok := respMap["choices"]
	if !ok {
		return nil
	}
	var choices []struct {
		Message *oaiMessage `json:"message"`
	}
	if json.Unmarshal(raw, &choices) != nil {
		return nil
	}
	for i := len(choices) - 1; i >= 0; i-- {
		if m := choices[i].Message; m != nil {
			cm := canonMsg{Role: "assistant", Text: contentText(m.Content)}
			for _, tc := range m.ToolCalls {
				cm.ToolCalls = append(cm.ToolCalls, canonTool{ID: tc.ID, Name: tc.Function.Name, Args: tc.Function.Arguments})
			}
			return &cm
		}
	}
	return nil
}

// claudeRespAssistant 非流式：content 块数组
func claudeRespAssistant(respMap map[string]json.RawMessage) *canonMsg {
	raw, ok := respMap["content"]
	if !ok {
		return nil
	}
	text, tools := claudeContent(raw)
	return &canonMsg{Role: "assistant", Text: text, ToolCalls: tools}
}

// geminiRespAssistant 非流式：末个含 content 的 candidate
func geminiRespAssistant(respMap map[string]json.RawMessage) *canonMsg {
	raw, ok := respMap["candidates"]
	if !ok {
		return nil
	}
	var candidates []struct {
		Content geminiContent `json:"content"`
	}
	if json.Unmarshal(raw, &candidates) != nil {
		return nil
	}
	for i := len(candidates) - 1; i >= 0; i-- {
		if len(candidates[i].Content.Parts) == 0 {
			continue
		}
		cm := geminiContentCanon(candidates[i].Content)
		cm.Role = "assistant"
		return &cm
	}
	return nil
}

// responsesRespAssistant 非流式：output 数组末条 assistant message 项。
// function_call 项两侧一致跳过，暂不参与衔接哈希——函数调用密集的
// Responses 流量依赖会话键/指纹层关联
func responsesRespAssistant(respMap map[string]json.RawMessage) *canonMsg {
	raw, ok := respMap["output"]
	if !ok {
		return nil
	}
	var items []struct {
		Type    string          `json:"type"`
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if json.Unmarshal(raw, &items) != nil {
		return nil
	}
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].Type == "message" && items[i].Role == "assistant" {
			return &canonMsg{Role: "assistant", Text: contentText(items[i].Content)}
		}
	}
	return nil
}

// ============ 响应侧流式汇聚 ============

// sseScanner SSE 行扫描器（沿用 ExtractStreamText 的行式遍历模式，
// 单行缓冲放大到 1MB 防长 data 行被静默截断）
func sseScanner(s string) *bufio.Scanner {
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 64*1024), sseScannerMaxLine)
	return sc
}

// dataPayload 取 SSE data 行负载；非 data 行返回 false
func dataPayload(line string) (string, bool) {
	if !strings.HasPrefix(line, "data: ") {
		return "", false
	}
	return strings.TrimPrefix(line, "data: "), true
}

// openAIRespAssistantStream 汇聚 OpenAI 流式 chunk：delta.content 拼接文本；
// delta.tool_calls 按 index 拼接（id/name 首片携带，arguments 逐片追加）
func openAIRespAssistantStream(s string) *canonMsg {
	var text strings.Builder
	type toolAcc struct{ id, name, args strings.Builder }
	tools := map[int]*toolAcc{}
	var order []int
	sc := sseScanner(s)
	for sc.Scan() {
		payload, ok := dataPayload(sc.Text())
		if !ok || payload == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   *string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(payload), &chunk) != nil {
			continue
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.Content != nil {
				text.WriteString(*ch.Delta.Content)
			}
			for _, tc := range ch.Delta.ToolCalls {
				acc, exists := tools[tc.Index]
				if !exists {
					acc = &toolAcc{}
					tools[tc.Index] = acc
					order = append(order, tc.Index)
				}
				if tc.ID != "" {
					acc.id.WriteString(tc.ID)
				}
				if tc.Function.Name != "" {
					acc.name.WriteString(tc.Function.Name)
				}
				acc.args.WriteString(tc.Function.Arguments)
			}
		}
	}
	cm := canonMsg{Role: "assistant", Text: text.String()}
	for _, idx := range order {
		cm.ToolCalls = append(cm.ToolCalls, canonTool{
			ID:   tools[idx].id.String(),
			Name: tools[idx].name.String(),
			Args: tools[idx].args.String(),
		})
	}
	return &cm
}

// claudeRespAssistantStream 汇聚 Claude SSE：content_block_start 开块
// （tool_use 携带 id/name）、content_block_delta 逐片追加（text_delta /
// input_json_delta）、content_block_end 收块；thinking 增量跳过
func claudeRespAssistantStream(s string) *canonMsg {
	var text strings.Builder
	var tools []canonTool
	var curArgs strings.Builder
	var curTool *canonTool
	sc := sseScanner(s)
	for sc.Scan() {
		payload, ok := dataPayload(sc.Text())
		if !ok {
			continue
		}
		var ev struct {
			Type         string `json:"type"`
			ContentBlock *struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"content_block"`
			Delta *struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
		}
		if json.Unmarshal([]byte(payload), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "content_block_start":
			if ev.ContentBlock != nil && ev.ContentBlock.Type == "tool_use" {
				curTool = &canonTool{ID: ev.ContentBlock.ID, Name: ev.ContentBlock.Name}
				curArgs.Reset()
			}
		case "content_block_delta":
			if ev.Delta == nil {
				continue
			}
			switch ev.Delta.Type {
			case "text_delta":
				text.WriteString(ev.Delta.Text)
			case "input_json_delta":
				curArgs.WriteString(ev.Delta.PartialJSON)
			}
		case "content_block_end":
			if curTool != nil {
				curTool.Args = curArgs.String()
				tools = append(tools, *curTool)
				curTool = nil
			}
		}
	}
	cm := canonMsg{Role: "assistant", Text: text.String(), ToolCalls: tools}
	return &cm
}

// geminiRespAssistantStream 汇聚 Gemini 流式 chunk：每片含完整 parts，
// 文本直接连接、functionCall 各自成调用
func geminiRespAssistantStream(s string) *canonMsg {
	var sb strings.Builder
	var tools []canonTool
	sc := sseScanner(s)
	for sc.Scan() {
		payload, ok := dataPayload(sc.Text())
		if !ok {
			continue
		}
		var chunk struct {
			Candidates []struct {
				Content geminiContent `json:"content"`
			} `json:"candidates"`
		}
		if json.Unmarshal([]byte(payload), &chunk) != nil {
			continue
		}
		for _, cand := range chunk.Candidates {
			for _, p := range cand.Content.Parts {
				if p.FunctionCall != nil {
					tools = append(tools, canonTool{Name: p.FunctionCall.Name, Args: string(p.FunctionCall.Args)})
					continue
				}
				sb.WriteString(p.Text)
			}
		}
	}
	return &canonMsg{Role: "assistant", Text: sb.String(), ToolCalls: tools}
}

// responsesRespAssistantStream 从流式事件取末个 response.completed 的完整
// 响应对象，复用非流式提取（Responses 事件序列结尾必发 completed；流被
// 截断/中断时找不到则返回 nil，该轮衔接哈希留空降级）
func responsesRespAssistantStream(s string) *canonMsg {
	sc := sseScanner(s)
	var last string
	for sc.Scan() {
		payload, ok := dataPayload(sc.Text())
		if !ok {
			continue
		}
		var ev struct {
			Type     string          `json:"type"`
			Response json.RawMessage `json:"response"`
		}
		if json.Unmarshal([]byte(payload), &ev) == nil && ev.Type == "response.completed" && len(ev.Response) > 0 {
			last = string(ev.Response)
		}
	}
	if last == "" {
		return nil
	}
	var respMap map[string]json.RawMessage
	if json.Unmarshal([]byte(last), &respMap) != nil {
		return nil
	}
	return responsesRespAssistant(respMap)
}

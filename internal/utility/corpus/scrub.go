package corpus

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// 语料脱敏。
//
// 核心判断：**协议转换器只关心结构，不关心内容**。因此自由文本可以激进替换，
// 而不损失任何测试价值。但「激进」必须止步于两类东西，否则语料会变成
// 假阳性的来源——测试失败了，却不是代码的错：
//
//  1. **引用一致性**：tool_call_id 在 assistant.tool_calls 与后续 tool_result 之间
//     必须替换后仍然相等；Gemini 的 functionCall ↔ functionResponse 靠**函数名**关联。
//     打断任何一条，工具循环链就断了，而工具循环正是转换最容易出 bug 的地方。
//  2. **判别式与行为相关字段**：type / role / finish_reason 决定转换分支；
//     model 决定模型相关分支（Claude 默认 max_tokens、Vertex 按名选端点）；
//     协议内置工具名（web_search 等）决定服务端工具识别。这些一律保留原值。
//
// 另有两处形态约束：tool 的 arguments 是**JSON 字符串**，替换后必须仍是合法 JSON
// （否则转换器走进解析失败分支，行为与线上不同）；base64 内联数据替换为可解码的
// 占位图片（而非任意字符串），使图片处理路径仍能正常工作。

// ScrubOptions 脱敏选项。
type ScrubOptions struct {
	// Enabled 为 false 时所有 Scrub 方法原样返回。
	Enabled bool
	// KeepToolNames 保留自定义工具名原值。默认 false（按映射一致替换）——
	// 自定义工具名可能泄漏业务逻辑（如 query_internal_crm）。
	// 协议内置工具名（web_search 等）不受此选项影响，始终保留。
	KeepToolNames bool
	// MaxTextLen 单个字符串值的填充长度上限（字节，0=不限制）。
	// 等长替换本为覆盖「token 估算/截断阈值」等长度敏感路径，但对转换器测试而言
	// 长度的**量级**即足够；不设上限时，携带完整会话历史的请求（多轮工具循环）
	// 会让语料膨胀到数百 MB（实测 340MB），此处让超长文本在上限处截断填充。
	MaxTextLen int
}

// Scrubber 单条记录范围的脱敏器。
//
// 作用域是「一条记录」而非「一次提取」：同一记录的请求与响应共用映射表，
// 保证记录内的标识符引用关系完整；跨记录无需一致（每个语料文件独立使用）。
type Scrubber struct {
	opts    ScrubOptions
	idMap   map[string]string
	nameMap map[string]string
}

// NewScrubber 创建脱敏器。
func NewScrubber(opts ScrubOptions) *Scrubber {
	return &Scrubber{
		opts:    opts,
		idMap:   make(map[string]string),
		nameMap: make(map[string]string),
	}
}

// ---------- 字段分类 ----------

// idKeys 承载标识符的字段。这些值必须**一致映射**（同一原值 → 同一新值），
// 否则跨消息的引用关系断裂。
var idKeys = map[string]bool{
	"id": true, "call_id": true, "tool_call_id": true, "tool_use_id": true,
	"item_id": true, "message_id": true, "response_id": true, "responseId": true,
	"previous_response_id": true, "container_id": true, "approval_request_id": true,
	// 会话级标识符（codex 等客户端把同一会话 UUID 塞进大量字段，含
	// x-codex-* 这类 header 风格的连字符 key）：值随会话变化且属隐私，需脱敏；
	// 无跨字段引用语义，一致映射即可。key 名无法穷举，裸 UUID 另按值形态兜底
	"prompt_cache_key": true, "session_id": true, "thread_id": true,
	"turn_id": true, "root_turn_id": true,
	"x-codex-window-id": true, "x-codex-installation-id": true,
}

// nameKeys 承载工具名的字段。Gemini 的 functionCall ↔ functionResponse 仅靠名字关联，
// 因此同样必须一致映射。
var nameKeys = map[string]bool{"name": true, "tool_name": true}

// reservedToolNames 协议内置（服务端）工具名，转换器按名识别，必须原样保留。
var reservedToolNames = map[string]bool{
	"web_search": true, "web_search_preview": true, "file_search": true,
	"code_execution": true, "code_interpreter": true, "computer": true,
	"computer_use_preview": true, "bash": true, "text_editor": true,
	"str_replace_editor": true, "str_replace_based_edit_tool": true,
	"googleSearch": true, "googleSearchRetrieval": true,
}

// behaviorKeys 影响转换分支的字段，保留原值。
//
// model 尤其重要：Claude 默认 max_tokens 按模型名分档、Vertex 按模型名是否含 "claude"
// 选择端点——替换掉会让语料不再触发这些分支。
var behaviorKeys = map[string]bool{
	"model": true, "type": true, "role": true, "object": true, "status": true,
	"finish_reason": true, "stop_reason": true, "finishReason": true,
	"mimeType": true, "media_type": true, "mime_type": true, "encoding_format": true,
	"effort": true, "summary": true, "bridge": true, "format": true,
	"detail": true, "service_tier": true, "search_context_size": true,
	"done_reason": true, "blockReason": true, "category": true, "threshold": true,
}

// signatureKeys 不透明签名/加密块，无结构语义，替换为固定标记。
var signatureKeys = map[string]bool{
	"signature": true, "thoughtSignature": true, "thought_signature": true,
	"encrypted_content": true,
}

// userDataKeys 进入这些字段后，其**整棵子树**的字符串都视为用户数据强制脱敏
// （不再按「短且无空格即枚举」的启发式放行）——工具参数里的
// {"location":"Boston"} 正是靠这条才不会漏。
var userDataKeys = map[string]bool{
	"arguments": true, "args": true, "input": true, "parameters": true,
	"input_schema": true, "inputSchema": true, "response": true,
	"content": true, "text": true, "output": true, "metadata": true,
}

// secretPattern 常见凭证形态：无论在哪个字段、是否「看起来像枚举」，一律脱敏。
// 用户完全可能把密钥粘在提示词里。
var secretPattern = regexp.MustCompile(`(?i)\b(sk-[A-Za-z0-9_-]{16,}|AIza[A-Za-z0-9_-]{20,}|gh[pousr]_[A-Za-z0-9]{20,}|xox[baprs]-[A-Za-z0-9-]{10,}|Bearer\s+[A-Za-z0-9._-]{20,}|eyJ[A-Za-z0-9._-]{20,})`)

// dataURLPattern data: URL 的前缀部分（保留 mime 与 base64 标记，替换载荷）。
var dataURLPattern = regexp.MustCompile(`^(data:[^;,]*;base64,)`)

// ---------- 入口 ----------

// ScrubJSON 脱敏 JSON 报文体。非 JSON 或未启用时原样返回。
//
// 注意：脱敏经过「解析 → 改写 → 重新序列化」，输出的 key 顺序与空白与原文不同。
// 语料的用途是喂给转换器（它本就要重新解析），因此语义等价即可；
// 但这意味着脱敏后的语料不再适合做字节级的 wire 保真校验。
func (s *Scrubber) ScrubJSON(body []byte) []byte {
	if !s.opts.Enabled || len(body) == 0 {
		return body
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return body
	}
	out, err := json.Marshal(s.scrubValue(v, "", false))
	if err != nil {
		return body
	}
	return out
}

// ScrubStream 脱敏 SSE / NDJSON 流：逐帧脱敏 data 负载，保留事件名与帧结构。
// [DONE] 等非 JSON 哨兵原样保留（它们是协议的一部分）。
func (s *Scrubber) ScrubStream(body []byte) []byte {
	if !s.opts.Enabled || len(body) == 0 {
		return body
	}
	text := string(body)
	if !strings.Contains(text, "data:") {
		return s.scrubNDJSON(text)
	}

	var sb strings.Builder
	for _, block := range strings.Split(text, "\n\n") {
		if strings.TrimSpace(block) == "" {
			continue
		}
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "data:"):
				payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				sb.WriteString("data: " + string(s.ScrubJSON([]byte(payload))) + "\n")
			case strings.TrimSpace(line) == "":
				// 帧内空行忽略
			default:
				// event: / id: / retry: 等元信息行原样保留
				sb.WriteString(line + "\n")
			}
		}
		sb.WriteString("\n")
	}
	return []byte(sb.String())
}

func (s *Scrubber) scrubNDJSON(text string) []byte {
	var sb strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		sb.Write(s.ScrubJSON([]byte(line)))
		sb.WriteString("\n")
	}
	return []byte(sb.String())
}

// ---------- 递归改写 ----------

// scrubValue 递归脱敏。key 为当前值所在字段名；inUserData 表示已处于用户数据子树内
// （此时字符串一律脱敏，不再走枚举启发式）。
func (s *Scrubber) scrubValue(v any, key string, inUserData bool) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = s.scrubValue(val, k, inUserData || userDataKeys[k])
		}
		return out

	case []any:
		out := make([]any, 0, len(t))
		for _, el := range t {
			// 数组元素继承父字段名：tools[] 的元素仍按 tools 语境处理
			out = append(out, s.scrubValue(el, key, inUserData))
		}
		return out

	case string:
		return s.scrubString(t, key, inUserData)

	default:
		// 数字 / 布尔 / null：不含隐私且影响转换行为（max_tokens、usage 等），保留
		return v
	}
}

func (s *Scrubber) scrubString(val, key string, inUserData bool) string {
	if val == "" {
		return val
	}

	// ① 标识符：一致映射，保住跨消息引用。裸 UUID 形态的值无论 key 名一律
	// 按标识符处理——客户端会把会话/安装标识塞进任意命名的字段，key 不可枚举
	if idKeys[key] || uuidPattern.MatchString(val) {
		return s.mapID(val)
	}
	// ② 工具名：一致映射（内置名保留）
	if nameKeys[key] {
		return s.mapToolName(val)
	}
	// ③ 签名/加密块：固定标记
	if signatureKeys[key] {
		return "scrubbed-signature"
	}
	// ④ 行为相关字段：原样保留
	if behaviorKeys[key] {
		return val
	}
	// ⑤ URL / 内联数据
	if isURLLike(key, val) {
		return scrubURL(val)
	}
	// ⑥ 工具参数等「JSON 字符串」：保持其仍为合法 JSON
	if key == "arguments" || key == "partial_json" {
		return s.scrubEmbeddedJSON(val)
	}
	// ⑦ 凭证形态：无条件脱敏
	if secretPattern.MatchString(val) {
		return filler(len(val))
	}
	// ⑧ 裸 base64 载荷：图片/音频等二进制内联数据（Claude 的 source.data、Gemini 的
	// inlineData.data——不带 data: 前缀的形态）。等长文本填充有两个问题：体积随原图
	// 膨胀（实测单文件 1.2MB、语料总量 154MB），且填充含空格后不再是合法 base64，
	// 图片处理路径会走进解码失败分支。换成可解码的 1×1 占位图（与 data: URL 同款）：
	// 内容对协议转换毫无影响（转换器只搬运 mimeType 与 data 字段），体积归一。
	if isBase64Payload(val) {
		return placeholderPNG
	}
	// ⑨ 用户数据子树内一律脱敏；子树外用「短且无空格」启发式放过枚举值。
	// 超长文本按 MaxTextLen 截断填充（等长原则对量级敏感而非字节敏感，见选项说明）
	if inUserData || !isEnumLike(key, val) {
		return filler(s.cappedLen(len(val)))
	}
	return val
}

// scrubEmbeddedJSON 脱敏「以字符串承载的 JSON」（tool arguments）。
//
// 必须保持结果仍是合法 JSON：转换器会解析它，替换成普通文本会让转换走进
// 解析失败分支，语料反映的就不再是线上行为。解析失败（如流式增量的半截 JSON）
// 时退化为等长填充。
func (s *Scrubber) scrubEmbeddedJSON(val string) string {
	var inner any
	if err := json.Unmarshal([]byte(val), &inner); err != nil {
		return filler(len(val))
	}
	out, err := json.Marshal(s.scrubValue(inner, "", true))
	if err != nil {
		return filler(len(val))
	}
	return string(out)
}

// mapID 标识符一致映射。保留已知前缀（call_ / toolu_ / msg_ 等）——
// 部分测试与转换器按前缀模式识别合成 ID，改掉前缀会影响判定。
func (s *Scrubber) mapID(orig string) string {
	if mapped, ok := s.idMap[orig]; ok {
		return mapped
	}
	mapped := fmt.Sprintf("%s%04d", idPrefixOf(orig), len(s.idMap)+1)
	s.idMap[orig] = mapped
	return mapped
}

var knownIDPrefixes = []string{
	"srvtoolu_", "toolu_", "chatcmpl-", "call_", "msg_", "resp_", "rs_",
	"fc_", "ws_", "item_", "req_",
}

func idPrefixOf(orig string) string {
	for _, p := range knownIDPrefixes {
		if strings.HasPrefix(orig, p) {
			return p
		}
	}
	return "id_"
}

// mapToolName 工具名一致映射。协议内置工具名原样保留——转换器按名识别服务端工具，
// 改掉会让语料不再触发 web_search 等能力分支。
func (s *Scrubber) mapToolName(orig string) string {
	if reservedToolNames[orig] {
		return orig
	}
	if s.opts.KeepToolNames {
		return orig
	}
	if mapped, ok := s.nameMap[orig]; ok {
		return mapped
	}
	mapped := fmt.Sprintf("tool_%d", len(s.nameMap)+1)
	s.nameMap[orig] = mapped
	return mapped
}

func isURLLike(key, val string) bool {
	if key == "url" || key == "image_url" || key == "uri" || key == "fileUri" || key == "file_uri" {
		return true
	}
	return strings.HasPrefix(val, "data:") || strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://")
}

// base64PayloadPattern 裸 base64 载荷形态（标准字母表 + 末尾可选填充）。
var base64PayloadPattern = regexp.MustCompile(`^[A-Za-z0-9+/]+={0,2}$`)

// base64PayloadMinLen 判定为二进制载荷的最小长度。真实图片/音频的 base64 动辄数千字符，
// 短于该值的串不可能是二进制载荷（也避免误伤碰巧只含 base64 字符的短文本/ID——
// 它们仍走等长填充，长度语义不受影响）。
const base64PayloadMinLen = 256

// isBase64Payload 判断值是否为裸 base64 二进制载荷（无 data: 前缀的内联数据形态）。
func isBase64Payload(val string) bool {
	return len(val) >= base64PayloadMinLen && base64PayloadPattern.MatchString(val)
}

// scrubURL 脱敏 URL。
//   - data: URL 保留 mime 前缀、载荷换成可解码的 1×1 PNG（图片处理路径仍能工作）
//   - http(s) URL 换成固定占位域名
func scrubURL(val string) string {
	if m := dataURLPattern.FindStringSubmatch(val); m != nil {
		return m[1] + placeholderPNG
	}
	if strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://") {
		return "https://scrubbed.example/resource"
	}
	return filler(len(val))
}

// filler 生成指定**字节长度**的占位文本。
//
// 保留长度量级而非替换成固定短串：截断阈值、token 估算（2 字符/token）、
// grounding 的字节偏移校正等逻辑都与长度相关，长度塌缩会让语料不再覆盖这些路径。
func filler(n int) string {
	if n <= 0 {
		return ""
	}
	if n <= 3 {
		return strings.Repeat("x", n)
	}
	const word = "scrubbed "
	var sb strings.Builder
	for sb.Len() < n {
		sb.WriteString(word)
	}
	return sb.String()[:n]
}

// cappedLen 应用 MaxTextLen 上限后的填充长度。
func (s *Scrubber) cappedLen(n int) int {
	if s.opts.MaxTextLen > 0 && n > s.opts.MaxTextLen {
		return s.opts.MaxTextLen
	}
	return n
}

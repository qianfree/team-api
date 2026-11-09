package aiassistant

import (
	"regexp"
	"strings"
)

// FenceSplitter 四反引号围栏流式切分器：把模型流式输出切成「正文 delta」与
// 「代码 delta」两类事件，代码走专用通道渲染（不经 markdown），从机制上杜绝
// 插件源码内的反引号把代码块拆碎。
//
// 围栏协议（system prompt 与模型约定）：
//   - 开栏行：行首（≤3 空格缩进）连续 ≥4 个反引号，后跟可选语言标记（如 ````js）
//   - 闭栏行：行首连续 ≥4 个反引号（≥ 开栏长度，统一按 4 判定——代码内禁止出现
//     连续 4 个反引号，提示词硬约束）
//   - 一条回复只认第一个围栏；后续围栏降级为正文（防模型多段输出）
//
// 流式实现按行处理：Feed 的 chunk 拼进缓冲，只消费完整行，未完成行留待下个
// chunk 或 Finish——围栏标记可能被 SSE 分片从中间劈开，不能按 chunk 直接判。
type FenceSplitter struct {
	buf    string // 当前未完成行的缓冲
	inside bool   // 是否处于围栏内
	opened bool   // 是否已识别过第一个围栏
}

// FenceEventKind 事件类型。
type FenceEventKind string

const (
	FenceText      FenceEventKind = "text"       // 正文增量（markdown 渲染）
	FenceCodeStart FenceEventKind = "code_start" // 围栏开启（此后为代码增量）
	FenceCodeDelta FenceEventKind = "code_delta" // 代码增量（专用代码卡片渲染）
	FenceCodeEnd   FenceEventKind = "code_end"   // 围栏闭合
)

// FenceEvent 单个切分事件。Lang 仅在 CodeStart 时有值（开栏行的语言标记，
// 用于把代码归属到对应技能，如 js → js_plugin）。
type FenceEvent struct {
	Kind FenceEventKind `json:"kind"`
	Text string         `json:"text"`
	Lang string         `json:"lang,omitempty"`
}

var (
	fenceOpenRe  = regexp.MustCompile("^ {0,3}`{4,}[A-Za-z0-9_-]{0,30}[ \t]*$")
	fenceCloseRe = regexp.MustCompile("^ {0,3}`{4,}[ \t]*$")
)

// NewFenceSplitter 构造切分器。
func NewFenceSplitter() *FenceSplitter { return &FenceSplitter{} }

// Feed 消费一个流式分片，返回由此产生的完整事件（可能为空）。
func (s *FenceSplitter) Feed(chunk string) []FenceEvent {
	if chunk == "" {
		return nil
	}
	s.buf += chunk
	var events []FenceEvent
	for {
		idx := strings.IndexByte(s.buf, '\n')
		if idx < 0 {
			break
		}
		line := s.buf[:idx]
		s.buf = s.buf[idx+1:]
		events = append(events, s.handleLine(line))
	}
	return events
}

// Finish 流结束：冲刷未完成行；围栏未闭合时按隐式闭合补 CodeEnd
// （部分端点截断输出的形态，代码卡片仍要完整呈现并走校验）。
func (s *FenceSplitter) Finish() []FenceEvent {
	var events []FenceEvent
	if s.buf != "" {
		events = append(events, s.handleLine(s.buf))
		s.buf = ""
	}
	if s.inside {
		s.inside = false
		events = append(events, FenceEvent{Kind: FenceCodeEnd})
	}
	return events
}

// handleLine 处理一行（不含换行符），恒返回 1 个事件。
func (s *FenceSplitter) handleLine(line string) FenceEvent {
	trimmed := strings.TrimSuffix(line, "\r")
	if !s.inside {
		// 只认第一个围栏；已开过围栏后的 ```` 行按正文降级
		if !s.opened && fenceOpenRe.MatchString(trimmed) {
			s.inside = true
			s.opened = true
			return FenceEvent{Kind: FenceCodeStart, Lang: extractFenceLang(trimmed)}
		}
		return FenceEvent{Kind: FenceText, Text: line + "\n"}
	}
	if fenceCloseRe.MatchString(trimmed) {
		s.inside = false
		return FenceEvent{Kind: FenceCodeEnd}
	}
	return FenceEvent{Kind: FenceCodeDelta, Text: line + "\n"}
}

// extractFenceLang 取开栏行的语言标记（去缩进与反引号前缀后的剩余部分）。
func extractFenceLang(line string) string {
	s := strings.TrimLeft(line, " \t")
	s = strings.TrimLeft(s, "`")
	return strings.TrimSpace(s)
}

// fallbackCodeRe 模型不守约（未输出四反引号围栏）时的兜底提取：
// 取全文最后一个三反引号围栏的内容。
var fallbackCodeRe = regexp.MustCompile("```[A-Za-z0-9_-]*[ \t]*\r?\n([\\s\\S]*?)```")

// ExtractCodeFallback 从全文提取代码块（兜底路径），返回（代码, 是否提取到）。
func ExtractCodeFallback(text string) (string, bool) {
	matches := fallbackCodeRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return "", false
	}
	return strings.TrimSuffix(matches[len(matches)-1][1], "\n"), true
}

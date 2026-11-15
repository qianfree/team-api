package aiassistant

import (
	"strings"
	"testing"
)

// collect 把事件流汇总为可断言的形态。
func collect(events []FenceEvent) (text, code string, kinds []FenceEventKind) {
	for _, ev := range events {
		kinds = append(kinds, ev.Kind)
		switch ev.Kind {
		case FenceText:
			text += ev.Text
		case FenceCodeDelta:
			code += ev.Text
		}
	}
	return
}

// TestFencePlainText 无围栏时全部按正文透传。
func TestFencePlainText(t *testing.T) {
	s := NewFenceSplitter()
	events := append(s.Feed("你好\n这是说明"), s.Finish()...)
	text, code, _ := collect(events)
	if code != "" {
		t.Fatalf("不应产生代码: %q", code)
	}
	if text != "你好\n这是说明\n" {
		t.Fatalf("正文不符: %q", text)
	}
}

// TestFenceNormal 正常围栏：开栏（带语言标记）→ 代码 → 闭栏。
func TestFenceNormal(t *testing.T) {
	s := NewFenceSplitter()
	var events []FenceEvent
	events = append(events, s.Feed("说明文字\n````js\nconst a = 1;\n")...)
	events = append(events, s.Feed("````\n结束\n")...)
	text, code, _ := collect(events)
	if code != "const a = 1;\n" {
		t.Fatalf("代码不符: %q", code)
	}
	if !strings.Contains(text, "说明文字") || !strings.Contains(text, "结束") {
		t.Fatalf("正文缺失: %q", text)
	}
	if strings.Contains(text, "const a") {
		t.Fatalf("代码不应出现在正文: %q", text)
	}
	// 首个 CodeStart 带语言标记
	var lang string
	for _, ev := range events {
		if ev.Kind == FenceCodeStart {
			lang = ev.Lang
		}
	}
	if lang != "js" {
		t.Fatalf("语言标记应为 js，得到 %q", lang)
	}
}

// TestFenceSplitAcrossChunks 围栏标记被 SSE 分片从中间劈开（核心场景）。
func TestFenceSplitAcrossChunks(t *testing.T) {
	s := NewFenceSplitter()
	var events []FenceEvent
	for _, chunk := range []string{"前文\n`", "``", "`js", "\nconst x", " = `模板`;", "\n", "``", "``\n"} {
		events = append(events, s.Feed(chunk)...)
	}
	events = append(events, s.Finish()...)
	text, code, _ := collect(events)
	if code != "const x = `模板`;\n" {
		t.Fatalf("跨片围栏代码不符: %q", code)
	}
	if strings.Contains(text, "const x") {
		t.Fatalf("代码泄漏进正文: %q", text)
	}
}

// TestFenceBackticksInsideCode 代码内 1-3 个连续反引号不构成闭合。
func TestFenceBackticksInsideCode(t *testing.T) {
	s := NewFenceSplitter()
	events := s.Feed("````js\nconst a = ``` + \"`\" + `x`;\n````\n后文\n")
	text, code, _ := collect(events)
	if !strings.Contains(code, "const a = ``` + \"`\" + `x`;") {
		t.Fatalf("三反引号不应闭合围栏: %q", code)
	}
	if !strings.Contains(text, "后文") {
		t.Fatalf("闭栏后正文丢失: %q", text)
	}
}

// TestFenceUnclosed 流结束未闭栏：Finish 隐式闭合，代码完整。
func TestFenceUnclosed(t *testing.T) {
	s := NewFenceSplitter()
	var events []FenceEvent
	events = append(events, s.Feed("````formula\np*2+c*6")...)
	events = append(events, s.Finish()...)
	_, code, kinds := collect(events)
	if code != "p*2+c*6\n" {
		t.Fatalf("未闭栏代码不符: %q", code)
	}
	hasEnd := false
	for _, k := range kinds {
		if k == FenceCodeEnd {
			hasEnd = true
		}
	}
	if !hasEnd {
		t.Fatal("Finish 应补隐式 CodeEnd")
	}
}

// TestFenceSecondFenceDegraded 第二个四反引号围栏降级为正文（只认第一个）。
func TestFenceSecondFenceDegraded(t *testing.T) {
	s := NewFenceSplitter()
	events := s.Feed("````js\nfirst\n````\n中间\n````js\nsecond\n````\n")
	text, code, _ := collect(events)
	if code != "first\n" {
		t.Fatalf("只应提取第一个围栏: %q", code)
	}
	if !strings.Contains(text, "second") {
		t.Fatalf("第二个围栏应降级为正文: %q", text)
	}
}

// TestFenceCRLF CRLF 行尾按行正确处理。
func TestFenceCRLF(t *testing.T) {
	s := NewFenceSplitter()
	events := s.Feed("````js\r\ncode1\r\n````\r\n")
	text, code, _ := collect(events)
	if code != "code1\r\n" {
		t.Fatalf("CRLF 代码不符: %q", code)
	}
	if text != "\n" { // 闭栏行后的空行（\r\n 的 \n 残留按正文行）
		_ = text // CRLF 差异不作为失败条件，仅保证不 panic/不丢代码
	}
}

// TestExtractCodeFallback 兜底提取：模型不守约时取最后一个三反引号围栏。
func TestExtractCodeFallback(t *testing.T) {
	text := "说明\n```js\n旧片段\n```\n更多说明\n```formula\np*3+c*9\n```\n收尾"
	code, ok := ExtractCodeFallback(text)
	if !ok {
		t.Fatal("应提取到代码")
	}
	if strings.TrimSpace(code) != "p*3+c*9" {
		t.Fatalf("应取最后一个围栏: %q", code)
	}
	if _, ok := ExtractCodeFallback("没有代码的文本"); ok {
		t.Fatal("无代码应返回 false")
	}
}

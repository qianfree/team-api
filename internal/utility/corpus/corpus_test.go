package corpus

import (
	"strings"
	"testing"
	"time"
)

// ---------- 结构指纹 ----------

// TestFingerprint_IgnoresFreeText 指纹必须无视自由文本：
// 内容不同但结构相同的请求算同一形状，否则每条真实请求都是新形状、去重完全失效。
func TestFingerprint_IgnoresFreeText(t *testing.T) {
	a := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"今天天气怎么样"}]}`)
	b := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"帮我写一段 Go 代码，要求实现快速排序"}]}`)

	ha, ok := Fingerprint(a)
	if !ok {
		t.Fatal("应能解析")
	}
	hb, _ := Fingerprint(b)
	if ha != hb {
		t.Errorf("内容不同、结构相同的请求指纹应一致\n a=%s\n b=%s", ha, hb)
	}
}

// TestFingerprint_DistinguishesStructure 判别式字段与结构差异必须区分开，
// 否则不同的转换分支会被折叠成一条语料，覆盖面反而收窄。
func TestFingerprint_DistinguishesStructure(t *testing.T) {
	base := []byte(`{"messages":[{"role":"user","content":"x"}]}`)

	cases := map[string]string{
		"content 由字符串变数组": `{"messages":[{"role":"user","content":[{"type":"text","text":"x"}]}]}`,
		"多了图片块":           `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"x"}}]}]}`,
		"role 不同":         `{"messages":[{"role":"assistant","content":"x"}]}`,
		"多了 tools 字段":     `{"messages":[{"role":"user","content":"x"}],"tools":[{"type":"function"}]}`,
	}

	hBase, _ := Fingerprint(base)
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			h, ok := Fingerprint([]byte(body))
			if !ok {
				t.Fatal("应能解析")
			}
			if h == hBase {
				t.Error("结构不同的请求不应算作同一形状")
			}
		})
	}
}

// TestFingerprint_ArrayLengthBucketing 数组长度按 1/2/3/many 分桶：
// 「单轮」「多轮」是结构差异要区分，而「50 轮」与「51 轮」是长度噪音要折叠。
func TestFingerprint_ArrayLengthBucketing(t *testing.T) {
	mk := func(n int) []byte {
		items := make([]string, n)
		for i := range items {
			items[i] = `{"role":"user","content":"x"}`
		}
		return []byte(`{"messages":[` + strings.Join(items, ",") + `]}`)
	}

	h1, _ := Fingerprint(mk(1))
	h2, _ := Fingerprint(mk(2))
	h50, _ := Fingerprint(mk(50))
	h51, _ := Fingerprint(mk(51))

	if h1 == h2 {
		t.Error("1 条与 2 条消息应算不同形状")
	}
	if h50 != h51 {
		t.Error("50 条与 51 条消息应折叠为同一形状（都落在 many 桶）")
	}
	if h2 == h50 {
		t.Error("2 条与 50 条消息应算不同形状")
	}
}

func TestFingerprint_NonJSON(t *testing.T) {
	if _, ok := Fingerprint([]byte("data: {...}\n\n")); ok {
		t.Error("非 JSON 应返回 ok=false，交由 FingerprintStream 处理")
	}
}

// ---------- 流式指纹 ----------

// TestFingerprintStream_CollapsesDeltaCount 流式指纹必须折叠增量帧数量：
// 一次回答 3 个 token 还是 300 个，结构上是同一件事。
func TestFingerprintStream_CollapsesDeltaCount(t *testing.T) {
	mk := func(n int) []byte {
		var sb strings.Builder
		sb.WriteString("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
		for i := 0; i < n; i++ {
			sb.WriteString("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n")
		}
		sb.WriteString("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
		return []byte(sb.String())
	}

	if FingerprintStream(mk(5)) != FingerprintStream(mk(500)) {
		t.Error("增量帧数量不同、结构相同的流应算同一形状")
	}
}

// TestFingerprintStream_DistinguishesEventShape 帧类型/事件名不同必须区分——
// 带工具调用的流与纯文本流是两种转换路径。
func TestFingerprintStream_DistinguishesEventShape(t *testing.T) {
	text := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n")
	tool := []byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"name\":\"f\"}}]}}]}\n\n")
	if FingerprintStream(text) == FingerprintStream(tool) {
		t.Error("文本流与工具调用流应算不同形状")
	}

	// SSE 事件名是 Claude/Responses 协议的一部分，必须进指纹
	e1 := []byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\"}\n\n")
	e2 := []byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	if FingerprintStream(e1) == FingerprintStream(e2) {
		t.Error("不同事件名的流应算不同形状")
	}
}

func TestFingerprintStream_NDJSON(t *testing.T) {
	a := []byte("{\"message\":{\"content\":\"a\"},\"done\":false}\n{\"done\":true}\n")
	b := []byte("{\"message\":{\"content\":\"完全不同的内容\"},\"done\":false}\n{\"done\":true}\n")
	if FingerprintStream(a) != FingerprintStream(b) {
		t.Error("NDJSON（Ollama）内容不同、结构相同应算同一形状")
	}
}

// ---------- 特征识别 ----------

func TestDetectFeatures(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"纯文本", `{"messages":[{"role":"user","content":"hi"}]}`, nil},
		{"带工具", `{"messages":[],"tools":[{"type":"function"}]}`, []string{"tools"}},
		{
			"工具循环",
			`{"messages":[{"role":"assistant","tool_calls":[{"id":"c"}]},{"role":"tool","content":"r"}]}`,
			[]string{"toolloop"},
		},
		{"图片", `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"x"}}]}]}`, []string{"image"}},
		{"Gemini 图片", `{"contents":[{"parts":[{"inlineData":{"mimeType":"image/png"}}]}]}`, []string{"image"}},
		{"思考", `{"messages":[],"reasoning_effort":"high"}`, []string{"thinking"}},
		{"Claude 思考", `{"messages":[],"thinking":{"type":"enabled"}}`, []string{"thinking"}},
		{"联网搜索", `{"messages":[],"web_search_options":{}}`, []string{"websearch"}},
		{"Gemini 搜索", `{"contents":[],"tools":[{"googleSearch":{}}]}`, []string{"tools", "websearch"}},
		{
			"多轮",
			`{"messages":[{"role":"user"},{"role":"assistant"},{"role":"user"}]}`,
			[]string{"multiturn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFeatures([]byte(tt.body))
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("DetectFeatures() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectStreamFeatures(t *testing.T) {
	stream := []byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"functionCall\":{\"name\":\"f\"}}]},\"groundingMetadata\":{}}]}\n\n")
	got := DetectStreamFeatures(stream)
	joined := strings.Join(got, ",")
	if !strings.Contains(joined, "toolloop") || !strings.Contains(joined, "websearch") {
		t.Errorf("流式特征识别遗漏: %v", got)
	}
}

// ---------- 文件名 ----------

func TestSampleFileName(t *testing.T) {
	s := Sample{
		Kind:      KindRequest,
		Format:    "openai",
		Features:  []string{"image", "tools"},
		ShapeHash: "abcdef0123456789",
	}
	if got := s.FileName(); got != "openai__image-tools_abcdef01.json" {
		t.Errorf("FileName() = %q", got)
	}
	if got := s.RelPath(); got != "real_inputs/openai__image-tools_abcdef01.json" {
		t.Errorf("RelPath() = %q", got)
	}

	// 无特征时用 basic，与手写语料的命名习惯一致
	s.Features = nil
	if got := s.FileName(); got != "openai__basic_abcdef01.json" {
		t.Errorf("无特征时 FileName() = %q", got)
	}

	// 流式语料落在独立目录、用 .txt（SSE 原文不是 JSON）
	s.Kind = KindStreamResponse
	if got := s.RelPath(); got != "real_stream_inputs/openai__basic_abcdef01.txt" {
		t.Errorf("流式 RelPath() = %q", got)
	}
}

// ---------- 去重收集 ----------

func mkRecord(id int64, clientBody, respBody string, stream bool) Record {
	return Record{
		ID:               id,
		RequestID:        "req",
		RelayMode:        "chat_completions",
		IsStream:         stream,
		UpstreamStatus:   200,
		CapturedAt:       time.Now(),
		ClientReqBody:    []byte(clientBody),
		UpstreamRespBody: []byte(respBody),
		Conversion:       Conversion{ClientFormat: "openai", UpstreamFormat: "claude"},
	}
}

// TestCollector_DedupByShape 同形状只保留代表样本，但出现次数要累计——
// occurrences 反映真实流量权重，是判断哪些形状值得提升进精选金样本的依据。
func TestCollector_DedupByShape(t *testing.T) {
	c := NewCollector(1, ScrubOptions{})

	// 三条内容不同、结构相同的请求
	c.Add(mkRecord(1, `{"messages":[{"role":"user","content":"问题一"}]}`, "", false))
	c.Add(mkRecord(2, `{"messages":[{"role":"user","content":"另一个完全不同的问题"}]}`, "", false))
	c.Add(mkRecord(3, `{"messages":[{"role":"user","content":"第三个"}]}`, "", false))
	// 一条结构不同的
	c.Add(mkRecord(4, `{"messages":[{"role":"user","content":"x"}],"tools":[{"type":"function"}]}`, "", false))

	samples := c.Samples()
	if len(samples) != 2 {
		t.Fatalf("应去重为 2 个样本，实际 %d: %+v", len(samples), samples)
	}

	stats := c.Stats()
	if stats.Scanned != 4 {
		t.Errorf("Scanned = %d, want 4", stats.Scanned)
	}
	if stats.Shapes != 2 {
		t.Errorf("Shapes = %d, want 2", stats.Shapes)
	}

	// 第一种形状出现 3 次
	var basic *Sample
	for i := range samples {
		if len(samples[i].Features) == 0 {
			basic = &samples[i]
		}
	}
	if basic == nil {
		t.Fatal("未找到无特征样本")
	}
	if basic.Meta.Occurrences != 3 {
		t.Errorf("Occurrences = %d, want 3（同形状出现次数需累计）", basic.Meta.Occurrences)
	}
}

func TestCollector_PerShapeLimit(t *testing.T) {
	c := NewCollector(2, ScrubOptions{})
	for i := 0; i < 5; i++ {
		c.Add(mkRecord(int64(i), `{"messages":[{"role":"user","content":"x"}]}`, "", false))
	}
	if got := len(c.Samples()); got != 2 {
		t.Errorf("per-shape=2 应保留 2 条，实际 %d", got)
	}
}

// TestCollector_SkipsRowsWithoutConversion 缺 conversion 元数据的行无法判断协议方向，
// 计入 SkippedNoConv 而非静默丢弃——统计要能反映采集质量。
func TestCollector_SkipsRowsWithoutConversion(t *testing.T) {
	c := NewCollector(1, ScrubOptions{})
	r := mkRecord(1, `{"messages":[]}`, "", false)
	r.Conversion = Conversion{}
	c.Add(r)

	if len(c.Samples()) != 0 {
		t.Error("缺 conversion 的记录不应产出语料")
	}
	if c.Stats().SkippedNoConv != 1 {
		t.Errorf("SkippedNoConv = %d, want 1", c.Stats().SkippedNoConv)
	}
}

// TestBuild_StreamVsNonStream is_stream 决定响应语料落进流式目录还是非流式目录。
func TestBuild_StreamVsNonStream(t *testing.T) {
	nonStream := mkRecord(1, `{"messages":[]}`, `{"id":"c","choices":[{"message":{"content":"hi"}}]}`, false)
	kinds := map[SampleKind]bool{}
	for _, s := range Build(nonStream, nil) {
		kinds[s.Kind] = true
	}
	if !kinds[KindRequest] || !kinds[KindResponse] {
		t.Errorf("非流式应产出 request + response 两类语料: %v", kinds)
	}

	stream := mkRecord(2, `{"messages":[]}`, "data: {\"type\":\"message_stop\"}\n\n", true)
	kinds = map[SampleKind]bool{}
	for _, s := range Build(stream, nil) {
		kinds[s.Kind] = true
	}
	if !kinds[KindStreamResponse] {
		t.Errorf("流式应产出 stream_response 语料: %v", kinds)
	}
	if kinds[KindResponse] {
		t.Error("流式不应产出非流式响应语料")
	}
}

// TestBuild_SkipsFailedUpstream 上游非 200 时段3 是错误体，不能当作响应语料。
func TestBuild_SkipsFailedUpstream(t *testing.T) {
	r := mkRecord(1, `{"messages":[]}`, `{"error":{"message":"boom"}}`, false)
	r.UpstreamStatus = 500
	for _, s := range Build(r, nil) {
		if s.Kind == KindResponse {
			t.Error("上游失败的响应体不应成为语料")
		}
	}
}

// ---------- 体积控制 ----------

func TestShrinkInlineData(t *testing.T) {
	long := strings.Repeat("A", 20000)
	body := []byte(`{"image_url":{"url":"data:image/png;base64,` + long + `"}}`)

	got := ShrinkInlineData(body, 8192)
	if len(got) >= len(body) {
		t.Errorf("超长内联数据未被压缩: %d → %d", len(body), len(got))
	}
	if !strings.Contains(string(got), placeholderPNG) {
		t.Error("应替换为占位 PNG")
	}
	// 结构必须保留（转换器靠 mimeType 与字段位置工作）
	if !strings.Contains(string(got), "data:image/png;base64,") {
		t.Error("data URL 前缀应保留")
	}

	// 阈值内的数据不动
	short := []byte(`{"data":"` + strings.Repeat("A", 100) + `"}`)
	if string(ShrinkInlineData(short, 8192)) != string(short) {
		t.Error("阈值内的数据不应改动")
	}
	// 关闭
	if string(ShrinkInlineData(body, 0)) != string(body) {
		t.Error("maxBytes=0 应原样返回")
	}
}

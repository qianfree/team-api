package common

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// ============================================================
// 会话重组算法测试（纯内存，覆盖开发计划 T2.2 用例矩阵）。
// ConvRecord 的哈希列经 ExtractSessionDigest 计算，与生产写路径同源，
// 保证测试链路与真实数据的衔接语义一致。
// ============================================================

var convTestBase = time.Date(2026, 10, 11, 10, 0, 0, 0, time.Local)

// oaiReq 构造 OpenAI chat 请求体（role/content 对）
func oaiReq(msgs ...[2]string) string {
	var sb strings.Builder
	sb.WriteString(`{"model":"gpt-4o","messages":[`)
	for i, m := range msgs {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"role":"` + m[0] + `","content":` + jsonString(m[1]) + `}`)
	}
	sb.WriteString(`]}`)
	return sb.String()
}

// oaiResp 构造 OpenAI 非流式响应体
func oaiResp(text string) string {
	return `{"choices":[{"message":{"role":"assistant","content":` + jsonString(text) + `}}]}`
}

func jsonString(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// newConvRec 构造记录：哈希列由真实提取器计算，mutate 允许覆盖字段
// （清空哈希模拟 legacy 行、设置会话键等）
func newConvRec(id int64, at time.Time, req, resp string, mutate func(*ConvRecord)) *ConvRecord {
	return newConvRecPath(id, at, "/chat/completions", req, resp, mutate)
}

// newConvRecPath 指定协议 path 的构造变体（Claude/Gemini/Responses 用）
func newConvRecPath(id int64, at time.Time, path, req, resp string, mutate func(*ConvRecord)) *ConvRecord {
	d := ExtractSessionDigest(path, req, resp, false, "", 1, "gpt-4o")
	r := &ConvRecord{
		ID:              id,
		RequestID:       fmt.Sprintf("req-%d", id),
		CreatedAt:       at,
		Path:            path,
		ModelName:       "gpt-4o",
		StatusCode:      200,
		RespMsgHash:     d.RespMsgHash,
		LastAsstMsgHash: d.LastAsstMsgHash,
		ConvFingerprint: d.ConvFingerprint,
		IsContextReset:  d.IsContextReset,
		ReqBody:         req,
		RespBody:        resp,
	}
	if mutate != nil {
		mutate(r)
	}
	return r
}

func withSessionKey(r *ConvRecord) {
	r.SessionKey = "sk:hdr:abcdef1234567890abcdef1234567890"
	r.SessionSource = "header"
}

func rolesOf(msgs []ConversationMessage) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.Role)
	}
	return out
}

func textAt(msgs []ConversationMessage, i int) string {
	return msgs[i].Text
}

// TestRebuildThreeTurnChain 三轮直线：单段、完整消息序列、逐条溯源
func TestRebuildThreeTurnChain(t *testing.T) {
	r1 := newConvRec(1, convTestBase, oaiReq([2]string{"system", "S"}, [2]string{"user", "u1"}), oaiResp("a1"), nil)
	r2 := newConvRec(2, convTestBase.Add(time.Minute), oaiReq(
		[2]string{"system", "S"}, [2]string{"user", "u1"},
		[2]string{"assistant", "a1"}, [2]string{"user", "u2"}), oaiResp("a2"), nil)
	r3 := newConvRec(3, convTestBase.Add(2*time.Minute), oaiReq(
		[2]string{"system", "S"}, [2]string{"user", "u1"},
		[2]string{"assistant", "a1"}, [2]string{"user", "u2"},
		[2]string{"assistant", "a2"}, [2]string{"user", "u3"}), oaiResp("a3"), nil)

	v := RebuildConversation(r3, []*ConvRecord{r1, r2, r3}, CorrelationModeContentChain)

	if v.CorrelationMode != CorrelationModeContentChain {
		t.Errorf("模式不符: %s", v.CorrelationMode)
	}
	if len(v.Segments) != 1 || v.RecordCount != 3 {
		t.Fatalf("应单段 3 条记录，实际 %d 段 %d 条", len(v.Segments), v.RecordCount)
	}
	got := rolesOf(v.Messages)
	want := []string{"system", "user", "assistant", "user", "assistant", "user", "assistant"}
	if len(got) != len(want) {
		t.Fatalf("消息数不符: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("角色序列不符: %v", got)
		}
	}
	// 逐条溯源：u1/a1 首现于 r1，u2/a2 首现于 r2，u3/a3 首现于 r3
	for i, wantSrc := range map[int]int64{0: 1, 1: 1, 2: 1, 3: 2, 4: 2, 5: 3, 6: 3} {
		if v.Messages[i].SourceRecordID != wantSrc {
			t.Errorf("消息 %d(%s) 来源应 %d 实际 %d", i, got[i], wantSrc, v.Messages[i].SourceRecordID)
		}
	}
	if len(v.Missing) != 0 || v.Truncated {
		t.Errorf("直线会话不应有缺失/截断: missing=%v truncated=%v", v.Missing, v.Truncated)
	}
}

// TestRebuildSlidingWindow 滑动窗口裁剪：链不断（衔接保留），早期消息由回溯恢复
func TestRebuildSlidingWindow(t *testing.T) {
	r1 := newConvRec(1, convTestBase, oaiReq([2]string{"system", "S"}, [2]string{"user", "u1"}), oaiResp("a1"), nil)
	r2 := newConvRec(2, convTestBase.Add(time.Minute), oaiReq(
		[2]string{"system", "S"}, [2]string{"user", "u1"},
		[2]string{"assistant", "a1"}, [2]string{"user", "u2"}), oaiResp("a2"), nil)
	// r3 起客户端丢最旧一对（system 保留，u1/a1 被裁）
	r3 := newConvRec(3, convTestBase.Add(2*time.Minute), oaiReq(
		[2]string{"system", "S"},
		[2]string{"assistant", "a1"}, [2]string{"user", "u2"},
		[2]string{"assistant", "a2"}, [2]string{"user", "u3"}), oaiResp("a3"), nil)

	v := RebuildConversation(r3, []*ConvRecord{r1, r2, r3}, CorrelationModeContentChain)

	if len(v.Segments) != 1 {
		t.Fatalf("滑动窗口不应断段，实际 %d 段", len(v.Segments))
	}
	full := strings.Join(textsOf(v.Messages), "|")
	if !strings.Contains(full, "u1") || !strings.Contains(full, "a1") {
		t.Errorf("被裁剪的早期消息应被回溯恢复: %v", full)
	}
	if full != "S|u1|a1|u2|a2|u3|a3" {
		t.Errorf("完整序列不符: %s", full)
	}
}

func textsOf(msgs []ConversationMessage) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.Text)
	}
	return out
}

// TestRebuildCompressionBreak 摘要压缩：断段 + 原因 + 摘要提示，两段消息都渲染
func TestRebuildCompressionBreak(t *testing.T) {
	r1 := newConvRec(1, convTestBase, oaiReq([2]string{"system", "S"}, [2]string{"user", "长期任务的第 1 轮"}), oaiResp("第 1 轮回答"), withSessionKey)
	r2 := newConvRec(2, convTestBase.Add(time.Minute), oaiReq(
		[2]string{"system", "S"}, [2]string{"user", "长期任务的第 1 轮"},
		[2]string{"assistant", "第 1 轮回答"}, [2]string{"user", "第 2 轮输入"}),
		oaiResp("第 2 轮回答"), withSessionKey)
	// 压缩后：全新上下文（摘要块 + 新输入），无衔接 + 写时标记命中
	r3 := newConvRec(3, convTestBase.Add(2*time.Minute), oaiReq(
		[2]string{"system", "S"}, [2]string{"user", "[Conversation summarized as of 10:02] 此前讨论了长期任务。继续：第 3 轮输入"}),
		oaiResp("第 3 轮回答"), withSessionKey)

	v := RebuildConversation(r3, []*ConvRecord{r1, r2, r3}, CorrelationModeSessionKey)

	if v.CorrelationMode != CorrelationModeSessionKey || v.SessionKeyMasked == "" {
		t.Fatalf("应走会话键模式且输出掩码键: %+v", v)
	}
	if len(v.Segments) != 2 {
		t.Fatalf("压缩应产生 2 段，实际 %d", len(v.Segments))
	}
	if v.Segments[1].ResetReason != ResetReasonCompression {
		t.Errorf("断段原因应为压缩，实际 %s", v.Segments[1].ResetReason)
	}
	if !strings.Contains(v.Segments[1].SummaryText, "summarized") {
		t.Errorf("压缩段应携带摘要提示: %q", v.Segments[1].SummaryText)
	}
	// 两段消息都渲染：段 1（u1/a1/u2/a2）+ 段 2（摘要 user/a3）
	full := strings.Join(textsOf(v.Messages), "|")
	for _, want := range []string{"第 1 轮", "第 2 轮回答", "summarized", "第 3 轮回答"} {
		if !strings.Contains(full, want) {
			t.Errorf("渲染缺少 %q: %s", want, full)
		}
	}
	// 段归属：压缩后消息的 segment_index 应为 2
	last := v.Messages[len(v.Messages)-1]
	if last.SegmentIndex != 2 {
		t.Errorf("压缩后消息应属段 2，实际 %d", last.SegmentIndex)
	}
}

// TestRebuildBranch 分支（regenerate）：兄弟后继保留在段内、标记分支、不参与渲染
func TestRebuildBranch(t *testing.T) {
	r1 := newConvRec(1, convTestBase, oaiReq([2]string{"system", "S"}, [2]string{"user", "u1"}), oaiResp("a1"), nil)
	// 两个后继都回传 a1（regenerate 两个版本）
	r2a := newConvRec(2, convTestBase.Add(time.Minute), oaiReq(
		[2]string{"system", "S"}, [2]string{"user", "u1"},
		[2]string{"assistant", "a1"}, [2]string{"user", "u2-版本A"}), oaiResp("a2A"), nil)
	r2b := newConvRec(3, convTestBase.Add(2*time.Minute), oaiReq(
		[2]string{"system", "S"}, [2]string{"user", "u1"},
		[2]string{"assistant", "a1"}, [2]string{"user", "u2-版本B"}), oaiResp("a2B"), nil)

	v := RebuildConversation(r2b, []*ConvRecord{r1, r2a, r2b}, CorrelationModeContentChain)

	if v.RecordCount != 3 {
		t.Fatalf("分支记录应保留在视图中，实际 %d 条", v.RecordCount)
	}
	full := strings.Join(textsOf(v.Messages), "|")
	if strings.Contains(full, "版本A") {
		t.Errorf("分支消息不应参与主链渲染: %s", full)
	}
	if !strings.Contains(full, "版本B") {
		t.Errorf("最新后继应在主链: %s", full)
	}
	// 分支标记：r2a 在段记录中标 IsBranch
	var branchMarked, anchorMarked bool
	for _, seg := range v.Segments {
		for _, rec := range seg.Records {
			if rec.ID == 2 {
				branchMarked = rec.IsBranch
			}
			if rec.ID == 3 {
				anchorMarked = rec.IsAnchor
			}
		}
	}
	if !branchMarked {
		t.Error("regenerate 兄弟后继应标记 IsBranch")
	}
	if !anchorMarked {
		t.Error("锚点应标记 IsAnchor")
	}
}

// TestRebuildFingerprintCollision 指纹碰撞：同开场白的不同会话被连通性校验剔除
func TestRebuildFingerprintCollision(t *testing.T) {
	// 会话 A 与会话 B 用完全相同的开场白（指纹相同）
	r1 := newConvRec(1, convTestBase, oaiReq([2]string{"system", "S"}, [2]string{"user", "你好"}), oaiResp("会话A回答"), nil)
	r2 := newConvRec(2, convTestBase.Add(time.Minute), oaiReq(
		[2]string{"system", "S"}, [2]string{"user", "你好"},
		[2]string{"assistant", "会话A回答"}, [2]string{"user", "会话A第二轮"}), oaiResp("A2"), nil)
	r3 := newConvRec(3, convTestBase.Add(2*time.Minute), oaiReq([2]string{"system", "S"}, [2]string{"user", "你好"}), oaiResp("会话B回答"), nil)
	r4 := newConvRec(4, convTestBase.Add(3*time.Minute), oaiReq(
		[2]string{"system", "S"}, [2]string{"user", "你好"},
		[2]string{"assistant", "会话B回答"}, [2]string{"user", "会话B第二轮"}), oaiResp("B2"), nil)

	if r1.ConvFingerprint == "" || r1.ConvFingerprint != r3.ConvFingerprint {
		t.Fatalf("前置失败：两会话应同指纹")
	}
	v := RebuildConversation(r2, []*ConvRecord{r1, r2, r3, r4}, CorrelationModeContentChain)

	if v.RecordCount != 2 {
		t.Errorf("碰撞会话应被连通性剔除，实际 %d 条", v.RecordCount)
	}
	full := strings.Join(textsOf(v.Messages), "|")
	if strings.Contains(full, "会话B") {
		t.Errorf("无关会话消息不应渲染: %s", full)
	}
}

// TestRebuildIdentityInferred 身份推断：无键无指纹（legacy 行），时间窗切分
func TestRebuildIdentityInferred(t *testing.T) {
	clearDigest := func(r *ConvRecord) { r.SessionKey, r.SessionSource, r.ConvFingerprint = "", "", "" }
	r1 := newConvRec(1, convTestBase, oaiReq([2]string{"user", "片段一提问"}), oaiResp("回答一"), clearDigest)
	r2 := newConvRec(2, convTestBase.Add(5*time.Minute), oaiReq(
		[2]string{"user", "片段一提问"}, [2]string{"assistant", "回答一"}, [2]string{"user", "追问"}),
		oaiResp("回答二"), clearDigest)
	// 距 r2 40 分钟：超出推断间隔阈值，切段
	r3 := newConvRec(3, convTestBase.Add(45*time.Minute), oaiReq([2]string{"user", "新片段提问"}), oaiResp("回答三"), clearDigest)

	v := RebuildConversation(r2, []*ConvRecord{r1, r2, r3}, CorrelationModeIdentity)

	if v.CorrelationMode != CorrelationModeIdentity {
		t.Fatalf("应为推断模式: %s", v.CorrelationMode)
	}
	if len(v.Segments) != 2 {
		t.Fatalf("超窗应切 2 段，实际 %d", len(v.Segments))
	}
	if v.Segments[1].ResetReason != ResetReasonInferredGap {
		t.Errorf("推断模式断段原因应为超窗: %s", v.Segments[1].ResetReason)
	}
	// 同窗内衔接成立（哈希仍在）：r1-r2 渲染合并正常；
	// r3 与对话流无内容关联（可能是并发另一会话）——记录保留在段列表，
	// 但消息不硬拼进对话流，missing 标注
	full := strings.Join(textsOf(v.Messages), "|")
	if full != "片段一提问|回答一|追问|回答二" {
		t.Errorf("推断模式渲染序列不符（无关联段不应硬拼）: %s", full)
	}
	if len(v.Missing) != 1 || v.Missing[0].RecordID != r3.ID || v.Missing[0].Reason != "unaligned_inferred" {
		t.Errorf("无关联记录应标注 unaligned_inferred: %+v", v.Missing)
	}
	if v.RecordCount != 3 {
		t.Errorf("段记录应保留全部 3 条（含未并入流的）: %d", v.RecordCount)
	}
}

// TestRebuildModelSwitch 模型切换：内容链不含模型维度，切换后仍关联
func TestRebuildModelSwitch(t *testing.T) {
	r1 := newConvRec(1, convTestBase, oaiReq([2]string{"user", "hi"}), oaiResp("hello"), nil)
	r2 := newConvRec(2, convTestBase.Add(time.Minute), oaiReq(
		[2]string{"user", "hi"}, [2]string{"assistant", "hello"}, [2]string{"user", "again"}), oaiResp("sure"), nil)
	r2.ModelName = "claude-sonnet-4" // 中途切换模型

	v := RebuildConversation(r2, []*ConvRecord{r1, r2}, CorrelationModeContentChain)
	if v.RecordCount != 2 || len(v.Segments) != 1 {
		t.Fatalf("模型切换不应断链: %d 条 %d 段", v.RecordCount, len(v.Segments))
	}
	if v.Segments[0].Records[1].ModelName != "claude-sonnet-4" {
		t.Errorf("段记录应保留各自模型名")
	}
}

// TestRebuildToolResultRender 工具结果渲染：Claude tool_result 块拆成独立
// tool 消息并携带实际内容（而非仅边界标记），user 消息保留纯文本部分；
// 哈希路径不受渲染改造影响（衔接哈希仍成立）
func TestRebuildToolResultRender(t *testing.T) {
	// 第 1 轮：user 提问 → 响应为工具调用
	req1 := `{"model":"claude-sonnet-4","system":"S","messages":[{"role":"user","content":"查北京和上海天气"}]}`
	resp1 := `{"content":[{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{"city":"北京"}},{"type":"tool_use","id":"toolu_2","name":"get_weather","input":{"city":"上海"}}]}`
	// 第 2 轮：user 消息带两个 tool_result（含实际内容）+ 新文本
	req2 := `{"model":"claude-sonnet-4","system":"S","messages":[` +
		`{"role":"user","content":"查北京和上海天气"},` +
		`{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{"city":"北京"}},{"type":"tool_use","id":"toolu_2","name":"get_weather","input":{"city":"上海"}}]},` +
		`{"role":"user","content":[` +
		`{"type":"tool_result","tool_use_id":"toolu_1","content":"北京 25℃ 晴"},` +
		`{"type":"tool_result","tool_use_id":"toolu_2","content":[{"type":"text","text":"上海 28℃ 多云"},{"type":"image","source":{"type":"base64"}}]}]},` +
		`{"role":"user","content":"汇总一下"}]}`
	resp2 := `{"content":[{"type":"text","text":"北京 25℃ 晴，上海 28℃ 多云"}]}`

	r1 := newConvRecPath(1, convTestBase, "/messages", req1, resp1, nil)
	r2 := newConvRecPath(2, convTestBase.Add(time.Minute), "/messages", req2, resp2, nil)

	// 前置：哈希路径不受影响——衔接哈希成立（响应侧工具调用 == 请求侧末条 assistant）
	if r1.RespMsgHash == "" || r1.RespMsgHash != r2.LastAsstMsgHash {
		t.Fatalf("渲染改造不应影响衔接哈希: %s != %s", r1.RespMsgHash, r2.LastAsstMsgHash)
	}

	v := RebuildConversation(r2, []*ConvRecord{r1, r2}, CorrelationModeContentChain)

	// 工具结果应为独立 tool 消息且携带实际内容与归属 ID
	var toolMsgs []ConversationMessage
	for _, m := range v.Messages {
		if m.Role == "tool" {
			toolMsgs = append(toolMsgs, m)
		}
	}
	if len(toolMsgs) != 2 {
		t.Fatalf("应拆出 2 条工具结果消息，实际 %d（全部: %v）", len(toolMsgs), textsOf(v.Messages))
	}
	if toolMsgs[0].Text != "北京 25℃ 晴" || toolMsgs[0].ToolCallID != "toolu_1" {
		t.Errorf("工具结果 1 内容/归属不符: %+v", toolMsgs[0])
	}
	if toolMsgs[1].Text != "上海 28℃ 多云<media>" || toolMsgs[1].ToolCallID != "toolu_2" {
		t.Errorf("工具结果 2 内容/归属不符: %+v", toolMsgs[1])
	}
	// user 消息不含工具结果标记，纯文本保留
	for _, m := range v.Messages {
		if m.Role == "user" && strings.Contains(m.Text, toolResultMarker) {
			t.Errorf("渲染路径 user 消息不应含工具结果标记: %q", m.Text)
		}
	}
	full := strings.Join(textsOf(v.Messages), "|")
	if !strings.Contains(full, "查北京和上海天气") || !strings.Contains(full, "汇总一下") {
		t.Errorf("user 文本缺失: %s", full)
	}
	if !strings.Contains(full, "北京 25℃ 晴，上海 28℃ 多云") {
		t.Errorf("最终回复缺失: %s", full)
	}
}

// TestRebuildRecordLimit 消息上限：超限截断保尾部并标注
func TestRebuildRecordLimit(t *testing.T) {
	// 260 轮逐轮增长的真实链（渲染总消息 521 > 500 上限）
	const turns = 260
	var recs []*ConvRecord
	history := [][2]string{}
	for i := 1; i <= turns; i++ {
		msgs := append(append([][2]string{}, history...), [2]string{"user", fmt.Sprintf("u%d", i)})
		recs = append(recs, newConvRec(int64(i), convTestBase.Add(time.Duration(i)*time.Second),
			oaiReq(msgs...), oaiResp(fmt.Sprintf("a%d", i)), nil))
		history = append(history, [2]string{"user", fmt.Sprintf("u%d", i)},
			[2]string{"assistant", fmt.Sprintf("a%d", i)})
	}
	v := RebuildConversation(recs[turns-1], recs, CorrelationModeContentChain)
	if v.RecordCount != turns {
		t.Fatalf("记录数不符: %d", v.RecordCount)
	}
	if !v.Truncated || len(v.Messages) != ConversationMaxMessages {
		t.Errorf("消息超限应截断保尾部: truncated=%v len=%d", v.Truncated, len(v.Messages))
	}
	// 保尾部：截掉前 20 条（u1,a1..u10,a10）后首条为 u11，末条为最后一轮响应
	if v.Messages[0].Text != "u11" {
		t.Errorf("截断后首条应为 u11: %s", v.Messages[0].Text)
	}
	if v.Messages[len(v.Messages)-1].Text != fmt.Sprintf("a%d", turns) {
		t.Errorf("末条应为最后一轮响应: %s", v.Messages[len(v.Messages)-1].Text)
	}
}

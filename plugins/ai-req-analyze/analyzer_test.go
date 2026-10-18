package aireqanalyze

import (
	"strings"
	"testing"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

func TestParseAIBatch(t *testing.T) {
	// 直接 JSON
	res, err := parseAIBatch(`{"findings":[{"index":1,"dimension":"leakage","severity":"high","category":"credential_leak","evidence":"AKIA...","explanation":"提交了云密钥"}],"batch_summary":"发现 1 条泄露"}`)
	if err != nil {
		t.Fatalf("直接解析失败: %v", err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Index != 1 || res.Findings[0].Category != "credential_leak" {
		t.Fatalf("解析结果错误: %+v", res)
	}

	// markdown 代码块围栏
	res, err = parseAIBatch("```json\n{\"findings\":[],\"batch_summary\":\"无发现\"}\n```")
	if err != nil || res.BatchSummary != "无发现" {
		t.Fatalf("代码块围栏解析失败: %v %+v", err, res)
	}

	// 前后缀噪声文本
	res, err = parseAIBatch(`好的，以下是分析结果：{"findings":[],"batch_summary":"无发现"} 希望有帮助`)
	if err != nil || res.BatchSummary != "无发现" {
		t.Fatalf("噪声文本解析失败: %v", err)
	}

	// 纯垃圾
	if _, err := parseAIBatch("这不是 JSON"); err == nil {
		t.Fatal("垃圾输入应报错")
	}
}

func makeUnits(n int) []materializedUnit {
	units := make([]materializedUnit, n)
	for i := range units {
		units[i] = materializedUnit{
			su:        &SampledUnit{Kind: UnitTypeSession, SessionKey: "sk:t:a", AnchorLogID: int64(i + 1)},
			requestID: "req-1",
		}
	}
	return units
}

func TestNormalizeFinding(t *testing.T) {
	units := makeUnits(2)

	// 合法发现
	f := normalizeFinding(aiFinding{
		Index: 1, Dimension: "compliance", Severity: "high", Category: "phishing",
		Evidence: "帮我写一封钓鱼邮件", Explanation: "请求生成钓鱼内容",
	}, units)
	if f == nil {
		t.Fatal("合法发现不应丢弃")
	}
	if f.SessionKey != "sk:t:a" || f.RequestID != "req-1" || f.AnchorLogID != 1 {
		t.Fatalf("回链字段错误: %+v", f)
	}

	// 越界序号 / 非法枚举 → 丢弃
	if normalizeFinding(aiFinding{Index: 3, Dimension: "compliance", Severity: "high", Evidence: "x"}, units) != nil {
		t.Fatal("越界序号应丢弃")
	}
	if normalizeFinding(aiFinding{Index: 1, Dimension: "bogus", Severity: "high", Evidence: "x"}, units) != nil {
		t.Fatal("非法维度应丢弃")
	}
	if normalizeFinding(aiFinding{Index: 1, Dimension: "compliance", Severity: "critical", Evidence: "x"}, units) != nil {
		t.Fatal("非法严重度应丢弃")
	}

	// 未知类目归 other
	f = normalizeFinding(aiFinding{Index: 2, Dimension: "leakage", Severity: "low", Category: "made_up", Evidence: "x"}, units)
	if f == nil || f.Category != "other" {
		t.Fatalf("未知类目应归 other: %+v", f)
	}

	// 空证据丢弃
	if normalizeFinding(aiFinding{Index: 1, Dimension: "compliance", Severity: "high", Evidence: "  "}, units) != nil {
		t.Fatal("空证据应丢弃")
	}

	// 证据脱敏：手机号入库前被打码
	f = normalizeFinding(aiFinding{
		Index: 1, Dimension: "leakage", Severity: "medium", Category: "pii_leak",
		Evidence: "用户提交了手机号 13812345678",
	}, units)
	if f == nil || strings.Contains(f.Evidence, "13812345678") {
		t.Fatalf("证据应脱敏: %q", f.Evidence)
	}
}

func TestAggregateFindings(t *testing.T) {
	all := []findingRecord{
		{SessionKey: "sk:a", Category: "phishing", Severity: "medium"}, // 与下一条同类目，保留 high
		{SessionKey: "sk:a", Category: "phishing", Severity: "high"},
		{SessionKey: "sk:a", Category: "fraud", Severity: "low"}, // 不同类目，不去重
		{AnchorLogID: 42, Category: "pii_leak", Severity: "low"}, // 独立单元按锚点聚合
	}
	risk, final := aggregateFindings(all)
	if risk != "high" {
		t.Fatalf("整体风险应为 high: %s", risk)
	}
	if len(final) != 3 {
		t.Fatalf("同键同类目应去重，期望 3 条实际 %d: %+v", len(final), final)
	}
	hasHigh := false
	for _, f := range final {
		if f.SessionKey == "sk:a" && f.Category == "phishing" && f.Severity == "high" {
			hasHigh = true
		}
	}
	if !hasHigh {
		t.Fatal("去重应保留最高严重度")
	}

	// 无发现 → clean
	risk, final = aggregateFindings(nil)
	if risk != "clean" || len(final) != 0 {
		t.Fatalf("空输入应为 clean: %s %d", risk, len(final))
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	base := buildSystemPrompt("", "")
	if !strings.Contains(base, "compliance") || !strings.Contains(base, "leakage") {
		t.Fatal("系统提示词缺少双维度定义")
	}
	if !strings.Contains(base, "以 JSON 输出") || !strings.Contains(base, `"batch_summary"`) {
		t.Fatal("系统提示词缺少输出契约")
	}
	if strings.Contains(base, "平台补充规则") {
		t.Fatal("无自定义规则时不应包含补充规则段")
	}

	withRules := buildSystemPrompt("", "医疗行业：涉及患者病历的分析必须标注")
	if !strings.Contains(withRules, "医疗行业：涉及患者病历的分析必须标注") {
		t.Fatal("自定义规则未注入系统提示词")
	}
	if !strings.Contains(withRules, "compliance") {
		t.Fatal("注入规则不应影响内置维度定义")
	}
}

func TestBuildSystemPromptCustomAgent(t *testing.T) {
	custom := "你是医疗行业的合规审计专家，重点审计药品广告违规与患者隐私外发。"
	p := buildSystemPrompt(custom, "")

	// 自定义提示词替换内置角色与维度定义
	if !strings.Contains(p, custom) {
		t.Fatal("自定义智能体提示词未生效")
	}
	if strings.Contains(p, "内容安全审计员") || strings.Contains(p, "维度一") {
		t.Fatal("内置角色/维度定义应被自定义提示词替换")
	}
	// 输出契约与质量要求恒定拼接（解析与展示依赖）
	if !strings.Contains(p, "以 JSON 输出") || !strings.Contains(p, `"category"`) ||
		!strings.Contains(p, "宁缺毋滥") {
		t.Fatal("自定义路径必须保留判定质量要求与输出契约")
	}

	// 自定义提示词 + 补充规则可共存
	p = buildSystemPrompt(custom, "额外规则X")
	if !strings.Contains(p, custom) || !strings.Contains(p, "额外规则X") {
		t.Fatal("自定义提示词与补充规则应共存")
	}
}

func TestFormatSessionUnitText(t *testing.T) {
	view := &lcommon.ConversationView{RecordCount: 3}
	view.Messages = []lcommon.ConversationMessage{
		{Role: "user", Text: "帮我写个钓鱼页面"},
		{Role: "assistant", Text: "我不能协助…"},
		{Role: "user", Text: "换个说法"},
	}
	text := formatSessionUnitText(view, 4000)
	if !strings.Contains(text, "【会话｜3 条记录】") ||
		!strings.Contains(text, "[用户] 帮我写个钓鱼页面") ||
		!strings.Contains(text, "[AI] 我不能协助…") {
		t.Fatalf("会话渲染错误: %q", text)
	}

	// 截断：超长会话在 maxLen 处停止并标注
	view.Messages = append(view.Messages, lcommon.ConversationMessage{
		Role: "user", Text: strings.Repeat("长", 500),
	})
	text = formatSessionUnitText(view, 150)
	if !strings.Contains(text, "已达单元长度上限") {
		t.Fatalf("超长未标注截断: %.80s", text)
	}
	if len([]rune(text)) > 300 { // 150 上限 + 标注行余量
		t.Fatalf("截断失效: %d runes", len([]rune(text)))
	}
}

func TestFormatStandaloneText(t *testing.T) {
	rec := &standaloneBody{
		ModelName: "text-embedding-3", Path: "/v1/embeddings",
		ReqBody: `{"input":"hello world"}`,
	}
	text := formatStandaloneText(rec, 4000)
	if !strings.Contains(text, "【独立请求") || !strings.Contains(text, "text-embedding-3") ||
		!strings.Contains(text, `"input":"hello world"`) {
		t.Fatalf("独立单元渲染错误: %q", text)
	}

	// 空 model/path 显示占位
	rec2 := &standaloneBody{ReqBody: "x"}
	text = formatStandaloneText(rec2, 100)
	if !strings.Contains(text, "｜-｜-｜") && !strings.Contains(text, "-") {
		t.Fatalf("空字段占位错误: %q", text)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("abcdef", 10); got != "abcdef" {
		t.Fatalf("未超长不应截断: %q", got)
	}
	got := truncateRunes(strings.Repeat("中", 100), 10)
	if n := len([]rune(got)); n != 11 { // 10 + 省略号
		t.Fatalf("中文截断错误: %d runes", n)
	}
}

package aireqanalyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

// 分析流水线：单元采样 → 分批物化（会话重组/独立正文）→ 调 AI → 校验归一 →
// 聚合去重 → 落报告。设计文档 5.5/5.6 节的实现。
//
// 提示词注入防护：AI 输入不含任何标识符（单元只有序号，会话键不进 prompt），
// 序号到单元的映射在插件侧完成，被分析内容无法伪造与真实记录的关联。

// ── AI 输出契约 ──

// aiFinding AI 输出的单条发现（index 由插件映射回单元）。
type aiFinding struct {
	Index       int    `json:"index"`
	Dimension   string `json:"dimension"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Evidence    string `json:"evidence"`
	Explanation string `json:"explanation"`
}

// aiBatchResult 单批分析结果。
type aiBatchResult struct {
	Findings     []aiFinding `json:"findings"`
	BatchSummary string      `json:"batch_summary"`

	tokensUsed int      // 本批 token 消耗（插件侧计量，非 AI 输出）
	notes      []string // 批内降级产生的单元级失败明细（整批错误时为空，错误走返回值）
}

// 枚举白名单（超出归 other 或整条丢弃，防 AI 编造值污染统计）。
var (
	validDimensions = map[string]bool{DimensionCompliance: true, DimensionLeakage: true}
	validSeverities = map[string]bool{"high": true, "medium": true, "low": true}
	validCategories = map[string]bool{
		"phishing": true, "fraud": true, "malware": true, "illegal_content": true,
		"evasion": true, "credential_leak": true, "source_code_leak": true,
		"pii_leak": true, "internal_doc_leak": true, "other": true,
	}
	severityRank = map[string]int{"high": 3, "medium": 2, "low": 1}
)

// 证据/理由的防御性截断长度（AI 约定 200 字/一句话，超限说明输出失控）。
const (
	maxEvidenceLen    = 500
	maxExplanationLen = 300
)

// defaultAgentPrompt 内置智能体提示词：角色设定 + 双维度审计定义。
// 管理员配置 agent_prompt 后整段被自定义提示词替换。
const defaultAgentPrompt = `你是一个 AI 网关平台的内容安全审计员。以下是某租户用户在一段时间内通过平台
调用大模型的使用单元：多轮对话已按会话去重还原为完整对话（单元标注「会话」），
无会话语义的请求为独立单条（单元标注「独立请求」）。请从两个维度逐单元分析：

【维度一：合规（compliance）】判断用户是否在用 AI 从事违规活动，包括但不限于：
生成钓鱼/诈骗文案、编写恶意代码或攻击工具、生成违法违规内容、批量伪造、
绕过安全审查的尝试。

【维度二：泄露（leakage）】判断用户是否向 AI 提交了不应外传的敏感数据，包括
但不限于：内部源代码、API 密钥/凭证/密码、内部文档或机密信息、批量个人信息
（姓名+身份证+手机号等组合）。`

// promptQualityAndContract 判定质量要求与 JSON 输出契约。
// 无论内置还是自定义提示词都强制拼接在末尾：输出契约决定插件能否解析结果
// （index/dimension/severity/category 枚举），质量要求决定 evidence 可信度
// 与 severity 展示语义——放开自定义会破坏解析与回链，这段不参与自定义。
const promptQualityAndContract = `判定要求：
- 只报告有明确证据的单元，宁缺毋滥；无法确定时不要编造
- evidence 必须是单元内原文的直接摘录（不是转述），长度不超过 200 字
- severity：high=明确违规/明确敏感数据泄露；medium=高度可疑；
  low=轻度可疑或需人工复核

以 JSON 输出，不要输出任何其他内容：
{
  "findings": [
    {
      "index": <该单元在输入中的序号，从 1 开始>,
      "dimension": "compliance" | "leakage",
      "severity": "high" | "medium" | "low",
      "category": "phishing|fraud|malware|illegal_content|evasion|credential_leak|source_code_leak|pii_leak|internal_doc_leak|other",
      "evidence": "原文摘录",
      "explanation": "判定理由（中文，一句话）"
    }
  ],
  "batch_summary": "本批次整体评估（中文，两三句话，无发现则说明）"
}`

// buildSystemPrompt 构造分析系统提示词。
// agentPrompt 非空时替换内置的角色与维度定义（管理员自定义智能体），
// custom_rules 作为补充规则注入；判定质量要求与输出契约恒定拼接。
func buildSystemPrompt(agentPrompt, customRules string) string {
	var extra string
	if strings.TrimSpace(customRules) != "" {
		extra = "\n平台补充规则（与其他判定要求同等效力）：\n" + strings.TrimSpace(customRules) + "\n"
	}

	role := defaultAgentPrompt
	if p := strings.TrimSpace(agentPrompt); p != "" {
		role = p
	}
	return role + "\n\n" + extra + promptQualityAndContract
}

// materializedUnit 已物化正文的单元（送审形态）。
type materializedUnit struct {
	su        *SampledUnit
	text      string
	requestID string // 回链用：锚点/独立记录的 request_id
}

// formatSessionUnitText 把会话重组结果渲染为送审文本。
// 逐消息拼接，达到 maxLen 即截断标注，不逐条截断（保留上下文连贯性）。
func formatSessionUnitText(view *lcommon.ConversationView, maxLen int) string {
	var b strings.Builder
	segNote := ""
	if len(view.Segments) > 1 {
		segNote = fmt.Sprintf("，%d 段", len(view.Segments))
	}
	fmt.Fprintf(&b, "【会话｜%d 条记录%s】\n", view.RecordCount, segNote)
	if view.Truncated {
		b.WriteString("（会话超长，早期内容已截断）\n")
	}

	roleLabel := map[string]string{
		"user": "用户", "assistant": "AI", "system": "系统", "tool": "工具结果",
	}
	for _, msg := range view.Messages {
		label := roleLabel[msg.Role]
		if label == "" {
			label = msg.Role
		}
		line := "[" + label + "] " + strings.TrimSpace(msg.Text) + "\n"
		if b.Len()+len(line) > maxLen {
			b.WriteString("…（已达单元长度上限，后续内容截断）\n")
			break
		}
		b.WriteString(line)
	}
	return b.String()
}

// standaloneBody 独立单元的正文记录。
type standaloneBody struct {
	ID        int64
	RequestID string
	ReqBody   string
	RespBody  string
	ModelName string
	Path      string
	CreatedAt time.Time
}

// formatStandaloneText 把独立请求渲染为送审文本。
func formatStandaloneText(rec *standaloneBody, maxLen int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "【独立请求｜%s｜模型 %s｜%s】\n",
		rec.CreatedAt.Format("2006-01-02 15:04"), orDash(rec.ModelName), orDash(rec.Path))

	req := strings.TrimSpace(rec.ReqBody)
	if req != "" {
		line := "请求: " + req + "\n"
		if b.Len()+len(line) > maxLen {
			line = truncateRunes("请求: "+req, maxLen-b.Len()) + "\n"
			b.WriteString(line)
			return b.String()
		}
		b.WriteString(line)
	}
	resp := strings.TrimSpace(rec.RespBody)
	if resp != "" && b.Len() < maxLen {
		line := "回答: " + resp + "\n"
		if b.Len()+len(line) > maxLen {
			line = truncateRunes("回答: "+resp, maxLen-b.Len()) + "\n"
		}
		b.WriteString(line)
	}
	return b.String()
}

// materializeBatch 物化一批单元的送审文本。
// 会话单元逐锚点调 LoadConversationView（失败降级为独立单元用锚点单条正文），
// 独立单元一次 IN 查询取正文。
func materializeBatch(ctx context.Context, units []*SampledUnit, maxLen int) ([]materializedUnit, error) {
	out := make([]materializedUnit, 0, len(units))
	var standaloneIDs []int64
	for _, su := range units {
		if su.Kind == UnitTypeStandalone {
			standaloneIDs = append(standaloneIDs, su.AnchorLogID)
		}
	}

	// 独立单元正文：单次 IN 查询
	bodies := make(map[int64]*standaloneBody, len(standaloneIDs))
	if len(standaloneIDs) > 0 {
		all, err := lcommon.AuditModelCtx(ctx, "aud_request_logs").
			WhereIn("id", standaloneIDs).
			Fields("id, request_id, request_body, response_body, model_name, path, created_at").
			All()
		if err != nil {
			return nil, err
		}
		for _, r := range all {
			bodies[r["id"].Int64()] = &standaloneBody{
				ID:        r["id"].Int64(),
				RequestID: r["request_id"].String(),
				ReqBody:   r["request_body"].String(),
				RespBody:  r["response_body"].String(),
				ModelName: r["model_name"].String(),
				Path:      r["path"].String(),
				CreatedAt: r["created_at"].Time(),
			}
		}
	}

	for _, su := range units {
		if su.Kind == UnitTypeStandalone {
			rec := bodies[su.AnchorLogID]
			if rec == nil {
				continue // 记录已被清理，跳过该单元
			}
			out = append(out, materializedUnit{su: su, text: formatStandaloneText(rec, maxLen), requestID: rec.RequestID})
			continue
		}

		// 会话/内容链单元：重组（admin 全局视角 tenantID=0，平台侧正文副本）
		view, err := lcommon.LoadConversationView(ctx, su.AnchorLogID, 0, lcommon.SysBodyColumns())
		if err == nil && view != nil {
			out = append(out, materializedUnit{
				su: su, text: formatSessionUnitText(view, maxLen),
				requestID: anchorRequestID(view),
			})
			continue
		}
		// 重组失败（锚点被清理/时间越界等）→ 降级为独立单元，用锚点单条正文
		if err != nil {
			glog.Warningf(ctx, "ai-req-analyze: 会话重组失败，锚点 %d 降级为独立单元: %v", su.AnchorLogID, err)
		}
		rec := bodies[su.AnchorLogID]
		if rec == nil {
			one, ferr := fetchOneLog(ctx, su.AnchorLogID)
			if ferr != nil {
				continue
			}
			rec = one
		}
		deg := *su
		deg.Kind = UnitTypeStandalone
		out = append(out, materializedUnit{su: &deg, text: formatStandaloneText(rec, maxLen), requestID: rec.RequestID})
	}
	return out, nil
}

// anchorRequestID 从重组结果中找锚点记录的 request_id（段记录元信息里带）。
func anchorRequestID(view *lcommon.ConversationView) string {
	for _, seg := range view.Segments {
		for _, rec := range seg.Records {
			if rec.ID == view.AnchorRecordID {
				return rec.RequestID
			}
		}
	}
	return ""
}

// fetchOneLog 取单条日志正文（会话重组失败的降级查询）。
func fetchOneLog(ctx context.Context, logID int64) (*standaloneBody, error) {
	one, err := lcommon.AuditModelCtx(ctx, "aud_request_logs").
		Where("id", logID).
		Fields("id, request_id, request_body, response_body, model_name, path, created_at").
		One()
	if err != nil {
		return nil, err
	}
	if one.IsEmpty() {
		return nil, fmt.Errorf("日志 %d 不存在", logID)
	}
	return &standaloneBody{
		ID:        one["id"].Int64(),
		RequestID: one["request_id"].String(),
		ReqBody:   one["request_body"].String(),
		RespBody:  one["response_body"].String(),
		ModelName: one["model_name"].String(),
		Path:      one["path"].String(),
		CreatedAt: one["created_at"].Time(),
	}, nil
}

// parseAIBatch 解析 AI 批次输出：直接解析失败时提取首个 JSON 对象再试
// （容忍模型输出 markdown 代码块或前后缀文本）。
func parseAIBatch(content string) (*aiBatchResult, error) {
	content = strings.TrimSpace(content)
	// 剥离 ```json ... ``` 代码块围栏
	if strings.HasPrefix(content, "```") {
		if i := strings.IndexByte(content, '\n'); i > 0 {
			content = content[i+1:]
		}
		content = strings.TrimSuffix(strings.TrimSpace(content), "```")
		content = strings.TrimSpace(content)
	}

	var result aiBatchResult
	if err := json.Unmarshal([]byte(content), &result); err == nil {
		return &result, nil
	}
	// 兜底：截取首个 '{' 到最后一个 '}' 之间的内容
	start := strings.IndexByte(content, '{')
	end := strings.LastIndexByte(content, '}')
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(content[start:end+1]), &result); err == nil {
			return &result, nil
		}
	}
	return nil, fmt.Errorf("AI 输出无法解析为 JSON: %.120s", content)
}

// findingRecord 归一后的发现（逻辑运算用类型化字段，落库前转 FindingDO）。
type findingRecord struct {
	UnitType    string
	SessionKey  string
	RequestID   string
	AnchorLogID int64
	Dimension   string
	Severity    string
	Category    string
	Evidence    string
	Explanation string
}

// normalizeFinding 校验并归一一条 AI 发现，映射回单元；非法条目返回 nil。
// evidence/explanation 落库前过 MaskSensitiveData 脱敏（设计文档第七节）。
func normalizeFinding(f aiFinding, units []materializedUnit) *findingRecord {
	if f.Index < 1 || f.Index > len(units) {
		return nil
	}
	if !validDimensions[f.Dimension] || !validSeverities[f.Severity] {
		return nil
	}
	category := f.Category
	if !validCategories[category] {
		category = "other"
	}

	unit := units[f.Index-1]
	evidence := truncateRunes(strings.TrimSpace(f.Evidence), maxEvidenceLen)
	explanation := truncateRunes(strings.TrimSpace(f.Explanation), maxExplanationLen)
	if evidence == "" {
		return nil // 无证据的发现没有复核价值，宁可丢弃
	}

	return &findingRecord{
		UnitType:    unit.su.Kind,
		SessionKey:  unit.su.SessionKey,
		RequestID:   unit.requestID,
		AnchorLogID: unit.su.AnchorLogID,
		Dimension:   f.Dimension,
		Severity:    f.Severity,
		Category:    category,
		Evidence:    lcommon.MaskSensitiveData(evidence),
		Explanation: lcommon.MaskSensitiveData(explanation),
	}
}

// aggregateFindings 聚合去重：同（会话键或锚点）+ 同类目保留最高严重度。
func aggregateFindings(all []findingRecord) (riskLevel string, final []findingRecord) {
	type key struct {
		unit     string // 会话键非空用会话键，否则用锚点
		category string
	}
	best := make(map[key]*findingRecord)
	for i := range all {
		f := &all[i]
		k := key{unit: unitKeyOf(f), category: f.Category}
		if prev, ok := best[k]; !ok || severityRank[f.Severity] > severityRank[prev.Severity] {
			best[k] = f
		}
	}

	maxRank := 0
	for _, f := range best {
		final = append(final, *f)
		if r := severityRank[f.Severity]; r > maxRank {
			maxRank = r
		}
	}

	switch {
	case maxRank == severityRank["high"]:
		riskLevel = "high"
	case maxRank == severityRank["medium"]:
		riskLevel = "medium"
	case maxRank == severityRank["low"]:
		riskLevel = "low"
	default:
		riskLevel = "clean"
	}
	return riskLevel, final
}

func unitKeyOf(f *findingRecord) string {
	if f.SessionKey != "" {
		return "sk:" + f.SessionKey
	}
	return fmt.Sprintf("anchor:%d", f.AnchorLogID)
}

// runAnalysis 执行完整分析并落库，返回报告 ID。
func runAnalysis(ctx context.Context, job *JobEntity, cfg *Config, client *AIClient) (int64, error) {
	// gtime.Time 内嵌 time.Time，直接取字段（.Time() 方法被内嵌字段遮蔽）
	start, end := job.TimeStart.Time, job.TimeEnd.Time

	// 数据完整性声明：平台侧分析记录全局审计级别
	globalLevel, _ := lcommon.GetAuditLevels(ctx, job.TenantId)

	sample, err := sampleUnits(ctx, job.TenantId, job.TargetUserId, start, end)
	if err != nil {
		return 0, err
	}

	// 空窗口：不调 AI，直接产出说明性报告
	if sample.LogCount == 0 {
		return persistReport(ctx, job, cfg, &reportPayload{
			riskLevel: "clean", auditLevel: globalLevel,
			summary: "分析窗口内无审计数据。若预期应有数据，请确认该租户的审计级别未设为 none。",
		})
	}

	// 会话单元在前（信息密度高），独立单元在后
	units := append(sample.SessionUnits, sample.StandaloneUnits...)

	var (
		allFindings []findingRecord
		batchNotes  []string
		tokensUsed  int
		batchErrors int
	)
	totalBatches := (len(units) + cfg.UnitsPerBatch - 1) / cfg.UnitsPerBatch
	processed := 0

	for batchStart := 0; batchStart < len(units); batchStart += cfg.UnitsPerBatch {
		batchEnd := batchStart + cfg.UnitsPerBatch
		if batchEnd > len(units) {
			batchEnd = len(units)
		}
		batch := units[batchStart:batchEnd]
		processed++

		mats, err := materializeBatch(ctx, batch, cfg.SnippetMaxLen)
		if err != nil {
			batchErrors++
			batchNotes = append(batchNotes, fmt.Sprintf("第 %d 批物化失败：%v", processed, err))
			updateJobProgress(ctx, job.Id, totalBatches, processed)
			continue
		}
		if len(mats) == 0 {
			updateJobProgress(ctx, job.Id, totalBatches, processed)
			continue
		}

		res, perr := analyzeBatch(ctx, client, cfg, mats)
		if perr != nil {
			batchErrors++
			batchNotes = append(batchNotes, fmt.Sprintf("第 %d 批分析失败：%v", processed, perr))
		} else {
			for _, f := range res.Findings {
				if fd := normalizeFinding(f, mats); fd != nil {
					allFindings = append(allFindings, *fd)
				}
			}
			if s := strings.TrimSpace(res.BatchSummary); s != "" {
				batchNotes = append(batchNotes, s)
			}
			// 逐单元降级产生的单元级失败明细（含被拒单元锚点，可回审计日志复核）
			batchNotes = append(batchNotes, res.notes...)
			tokensUsed += res.tokensUsed
		}

		updateJobProgress(ctx, job.Id, totalBatches, processed)
	}

	riskLevel, finalFindings := aggregateFindings(allFindings)

	summary, err := buildFinalSummary(ctx, client, cfg, batchNotes, riskLevel)
	if err != nil {
		summary = strings.Join(batchNotes, "\n") // 汇总调用失败时退化为批次摘要拼接
	}

	payload := &reportPayload{
		riskLevel:        riskLevel,
		auditLevel:       globalLevel,
		logCount:         sample.LogCount,
		sessionCount:     len(sample.SessionUnits),
		standaloneCount:  len(sample.StandaloneUnits),
		standaloneCapped: sample.StandaloneCapped,
		batchErrors:      batchErrors,
		tokensUsed:       tokensUsed,
		summary:          summary,
		findings:         finalFindings,
	}
	return persistReport(ctx, job, cfg, payload)
}

// analyzeBatch 对一批物化单元执行一次 AI 调用；JSON 解析失败整批重试一次。
func analyzeBatch(ctx context.Context, client *AIClient, cfg *Config, mats []materializedUnit) (*aiBatchResult, error) {
	res, err := callAI(ctx, client, cfg, mats)
	if err == nil {
		return res, nil
	}

	// 整批被端点拒绝（HTTP 400，典型为内容安全审查拦截送审文本）→ 逐单元降级：
	// 正常单元仍得到分析，被拒单元精确定位到锚点日志（被审查拦截的内容
	// 本身就是高价值线索，管理员可凭锚点 ID 直接回审计日志人工复核）。
	// 其余错误（传输故障/5xx/密钥错误等）不降级，避免放大对故障端点的调用。
	var rejected *AIRejectedError
	if errors.As(err, &rejected) && rejected.Status == 400 && len(mats) > 1 {
		return analyzePerUnit(ctx, client, cfg, mats, err)
	}
	return nil, err
}

// callAI 对一组单元执行一次 AI 调用；JSON 解析失败整批重试一次。
func callAI(ctx context.Context, client *AIClient, cfg *Config, mats []materializedUnit) (*aiBatchResult, error) {
	var b strings.Builder
	for i, m := range mats {
		fmt.Fprintf(&b, "===== 单元 %d =====\n%s\n", i+1, m.text)
	}
	messages := []ChatMessage{
		{Role: "system", Content: buildSystemPrompt(cfg.AgentPrompt, cfg.CustomRules)},
		{Role: "user", Content: b.String()},
	}

	opts := ChatOptions{MaxTokens: cfg.AIMaxTokens, Temperature: 0.1, ResponseJSON: true}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		res, err := client.Chat(ctx, messages, opts)
		if err != nil {
			return nil, err // 传输/HTTP 错误不重试（调用方记批次错误），避免对故障端点双倍压力
		}
		parsed, perr := parseAIBatch(res.Content)
		if perr == nil {
			// token 计量挂在解析成功的批次上
			parsed.tokensUsed = res.TotalTokens
			return parsed, nil
		}
		lastErr = perr
	}
	return nil, lastErr
}

// analyzePerUnit 逐单元降级分析：每个单元独立调用，成功的发现按原批序号
// 重映射后合并；被拒单元的失败明细记入 notes 供报告与任务错误展示。
func analyzePerUnit(ctx context.Context, client *AIClient, cfg *Config, mats []materializedUnit, batchErr error) (*aiBatchResult, error) {
	out := &aiBatchResult{}
	var summaries []string

	for i, m := range mats {
		res, err := callAI(ctx, client, cfg, []materializedUnit{m})
		if err != nil {
			// 精确记录被拒单元与原因（含锚点，可回审计日志复核）
			out.notes = append(out.notes, fmt.Sprintf("单元 %d（%s）被分析端点拒绝: %v",
				i+1, unitLabel(m), err))
			continue
		}
		// 单单元调用的 findings index 恒为 1，重映射回原批序号供 normalizeFinding 定位
		for j := range res.Findings {
			res.Findings[j].Index = i + 1
		}
		out.Findings = append(out.Findings, res.Findings...)
		if s := strings.TrimSpace(res.BatchSummary); s != "" {
			summaries = append(summaries, s)
		}
		out.tokensUsed += res.tokensUsed
	}

	if len(summaries) > 0 {
		out.BatchSummary = fmt.Sprintf("整批调用被拒（%v）已逐单元降级分析：%d/%d 单元完成。%s",
			batchErr, len(summaries), len(mats), strings.Join(summaries, " "))
	} else {
		out.BatchSummary = fmt.Sprintf("整批调用被拒且逐单元降级后全部失败（%d 个单元），通常为内容安全审查拦截，建议按下方单元明细回审计日志人工复核。", len(mats))
	}
	return out, nil
}

// unitLabel 单元的人类可读标识（notes 展示用，锚点可回审计日志）。
func unitLabel(m materializedUnit) string {
	if m.su.Kind == UnitTypeSession {
		return fmt.Sprintf("会话，锚点日志 %d", m.su.AnchorLogID)
	}
	return fmt.Sprintf("独立请求，日志 %d", m.su.AnchorLogID)
}

// buildFinalSummary 把各批摘要合并为整体评估（小 token 单次调用）。
func buildFinalSummary(ctx context.Context, client *AIClient, cfg *Config, batchNotes []string, riskLevel string) (string, error) {
	if len(batchNotes) == 0 {
		return "未产生任何批次结果（窗口内日志可能均为空正文或全部处理失败）。", nil
	}
	if len(batchNotes) == 1 {
		return batchNotes[0], nil
	}

	notes := strings.Join(batchNotes, "\n- ")
	prompt := fmt.Sprintf(`以下是某租户 AI 使用行为分批审计的各批摘要，整体风险等级判定为 %s。
请合并为一段面向管理员的总体评估（中文，3-5 句话：主要风险点、涉及维度、建议关注对象），
不要输出 JSON 或其他内容。

- %s`, riskLevelLabel(riskLevel), notes)

	res, err := client.Chat(ctx, []ChatMessage{
		{Role: "system", Content: "你是审计报告摘要撰写助手，输出精炼的中文总结。"},
		{Role: "user", Content: prompt},
	}, ChatOptions{MaxTokens: 512, Temperature: 0.2})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Content), nil
}

func riskLevelLabel(level string) string {
	switch level {
	case "high":
		return "高风险"
	case "medium":
		return "中风险"
	case "low":
		return "低风险"
	case "clean":
		return "未发现风险"
	}
	return "未知"
}

// reportPayload 报告落库载荷。
type reportPayload struct {
	riskLevel        string
	auditLevel       string
	logCount         int
	sessionCount     int
	standaloneCount  int
	standaloneCapped bool
	batchErrors      int
	tokensUsed       int
	summary          string
	findings         []findingRecord
}

// persistReport 写报告主表 + 发现明细，返回报告 ID。
// 事务内统一走 ctx 传播式写法（g.DB().Model(...).Ctx(ctx)，事务由 ctx 自动挂载）。
func persistReport(ctx context.Context, job *JobEntity, cfg *Config, p *reportPayload) (int64, error) {
	// 摘要与说明也过脱敏（batch_notes 可能引用了原文错误信息）
	summary := lcommon.MaskSensitiveData(p.summary)

	var reportID int64
	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		v, err := g.DB().Model(tableReports).Ctx(ctx).Data(ReportDO{
			TenantId:        job.TenantId,
			TargetUserId:    job.TargetUserId,
			Source:          job.Source,
			TimeStart:       job.TimeStart,
			TimeEnd:         job.TimeEnd,
			RiskLevel:       p.riskLevel,
			AuditLevel:      p.auditLevel,
			LogCount:        p.logCount,
			SessionCount:    p.sessionCount,
			StandaloneCount: p.standaloneCount,
			FindingCount:    len(p.findings),
			Summary:         summary,
			AiModel:         cfg.AIModel,
			AiTokensUsed:    p.tokensUsed,
			CreatedBy:       job.CreatedBy,
		}).InsertAndGetId()
		if err != nil {
			return err
		}
		reportID = v

		if len(p.findings) > 0 {
			rows := make([]FindingDO, len(p.findings))
			for i, f := range p.findings {
				rows[i] = FindingDO{
					ReportId:    reportID,
					UnitType:    f.UnitType,
					SessionKey:  f.SessionKey,
					RequestId:   f.RequestID,
					AnchorLogId: f.AnchorLogID,
					Dimension:   f.Dimension,
					Severity:    f.Severity,
					Category:    f.Category,
					Evidence:    f.Evidence,
					Explanation: f.Explanation,
				}
			}
			if _, err := g.DB().Model(tableFindings).Ctx(ctx).Data(rows).Insert(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return reportID, nil
}

// updateJobProgress 更新任务进度（best-effort，失败不影响分析）。
func updateJobProgress(ctx context.Context, jobID int64, total, done int) {
	pct := 0
	if total > 0 {
		pct = done * 100 / total
	}
	if _, err := g.DB().Model(tableJobs).Ctx(ctx).
		Where("id", jobID).
		Data(JobDO{Progress: pct}).
		Update(); err != nil {
		glog.Warningf(ctx, "ai-req-analyze: 更新任务进度失败 job=%d: %v", jobID, err)
	}
}

// truncateRunes 按字符（非字节）截断，避免中文被拦腰截断产生乱码。
func truncateRunes(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

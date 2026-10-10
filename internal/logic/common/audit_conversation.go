package common

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
)

// ============================================================
// 会话重组（设计见 docs/会话关联与完整会话提取方案.md §6）
//
// 从锚点记录出发，把同一会话的审计记录分组串联并还原完整消息序列：
//   1. 候选收集（三层降级：会话键 → 指纹+连通性 → 身份+时间窗）
//   2. 衔接图与分段（断点=压缩/改写/超窗，段内上下文逐字连续）
//   3. 分支标记（regenerate/编辑重发产生多后继，主链外标 branch）
//   4. 渲染（尾部对齐合并：以各轮消息为材料，自动恢复被客户端
//      裁剪的早期历史；响应消息缺失时明示原因、不伪造）
//
// 纯算法（RebuildConversation）不碰 DB，单测直接覆盖；
// 数据访问（LoadConversationView）走审计库路由，集成测试覆盖。
// ============================================================

// 关联模式
const (
	CorrelationModeSessionKey   = "session_key"       // 显式会话键（精确）
	CorrelationModeContentChain = "content_chain"     // 指纹分组 + 衔接连通性校验
	CorrelationModeIdentity     = "identity_inferred" // 身份 + 时间窗（推断）
)

// 段间断点原因
const (
	ResetReasonCompression = "context_compression" // 摘要压缩/续接（写时标记或消息量骤降）
	ResetReasonRewrite     = "context_rewrite"     // 会话键连续但衔接断裂且非压缩（历史被改写/话题新建）
	ResetReasonInferredGap = "inferred_gap"        // 推断模式下时间间隔超窗
)

// 重组规模上限（查询侧 LIMIT 与渲染截断共用）
const (
	ConversationMaxRecords  = 200
	ConversationMaxMessages = 500
	// 会话键/指纹候选的时间界：锚点前后各 7 天，防极端长会话拖垮查询
	ConversationTimeBound = 7 * 24 * time.Hour
	// 身份推断模式的相邻记录间隔阈值：超过则切段
	ConversationInferredGap = 30 * time.Minute
)

// BodyColumns 会话视图按调用方选择的正文副本列（系统/租户双副本渲染隔离）
type BodyColumns struct {
	ReqBodyCol  string // request_body | tenant_request_body
	RespBodyCol string // response_body | tenant_response_body
}

// SysBodyColumns 管理后台渲染系统副本
func SysBodyColumns() BodyColumns {
	return BodyColumns{ReqBodyCol: "request_body", RespBodyCol: "response_body"}
}

// TenantBodyColumns 租户控制台渲染租户副本
func TenantBodyColumns() BodyColumns {
	return BodyColumns{ReqBodyCol: "tenant_request_body", RespBodyCol: "tenant_response_body"}
}

// ConvRecord 会话重组的记录视图（哈希列由写路径落库，正文按调用方副本填充）
type ConvRecord struct {
	ID              int64
	TenantID        int64
	RequestID       string
	CreatedAt       time.Time
	Path            string
	ModelName       string
	StatusCode      int
	LatencyMs       int
	UserID          int64
	ApiKeyID        int64
	SessionKey      string
	SessionSource   string
	ConvFingerprint string
	RespMsgHash     string
	LastAsstMsgHash string
	IsContextReset  bool
	ReqBody         string
	RespBody        string
}

// ConversationToolCall 渲染消息中的工具调用（canonTool 的导出镜像）
type ConversationToolCall struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Args string `json:"args,omitempty"`
}

// ConversationMessage 渲染后的消息（逐条可溯源）
type ConversationMessage struct {
	Role           string                 `json:"role"`
	Text           string                 `json:"text"`
	ToolCalls      []ConversationToolCall `json:"tool_calls,omitempty"`
	ToolCallID     string                 `json:"tool_call_id,omitempty"` // 工具结果消息对应的调用 ID
	SourceRecordID int64                  `json:"source_record_id"`       // 该消息首次出现的审计记录
	SegmentIndex   int                    `json:"segment_index"`
}

// ConversationRecordInfo 段内记录元信息
type ConversationRecordInfo struct {
	ID         int64  `json:"id"`
	RequestID  string `json:"request_id"`
	CreatedAt  string `json:"created_at"`
	ModelName  string `json:"model_name"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int    `json:"latency_ms"`
	IsStream   bool   `json:"is_stream"`
	IsBranch   bool   `json:"is_branch"` // 分支记录（regenerate 等产生的非主链后继）
	IsAnchor   bool   `json:"is_anchor"` // 用户入口锚点记录
}

// ConversationSegment 上下文逐字连续的记录区间
type ConversationSegment struct {
	SegmentIndex int                      `json:"segment_index"`
	RecordIDs    []int64                  `json:"record_ids"`
	StartTime    string                   `json:"start_time"`
	EndTime      string                   `json:"end_time"`
	ResetReason  string                   `json:"reset_reason"` // 本段起点相对上段的断点原因；首段为空
	SummaryText  string                   `json:"summary_text"` // 压缩断点的摘要提示（段首记录首条 user 消息）
	Records      []ConversationRecordInfo `json:"records"`
}

// ConversationMissing 渲染缺失说明（不伪造内容，明示原因）
type ConversationMissing struct {
	RecordID int64  `json:"record_id"`
	Reason   string `json:"reason"` // response_not_recorded | body_truncated
}

// ConversationView 会话重组结果
type ConversationView struct {
	CorrelationMode  string                `json:"correlation_mode"`
	SessionKeyMasked string                `json:"session_key_masked,omitempty"`
	SessionSource    string                `json:"session_source,omitempty"`
	RecordCount      int                   `json:"record_count"`
	AnchorRecordID   int64                 `json:"anchor_record_id"`
	Truncated        bool                  `json:"truncated"`
	Segments         []ConversationSegment `json:"segments"`
	Messages         []ConversationMessage `json:"messages"`
	Missing          []ConversationMissing `json:"missing,omitempty"`
}

const conversationTimeLayout = "2006-01-02 15:04:05"

// RebuildConversation 纯算法重组：records 为查询侧按锚点特征收集的候选
// （时间升序，锚点必须包含在内），mode 指示收集策略。
// 函数不修改传入切片，输出视图可安全序列化给 API 层。
func RebuildConversation(anchor *ConvRecord, records []*ConvRecord, mode string) *ConversationView {
	v := &ConversationView{
		CorrelationMode: mode,
		AnchorRecordID:  anchor.ID,
	}
	if anchor.SessionKey != "" {
		v.SessionKeyMasked = maskSessionKey(anchor.SessionKey)
		v.SessionSource = anchor.SessionSource
	}

	// 排序副本（候选基本有序，防御性插入排序；稳定保时间同序记录的相对顺序）
	sorted := make([]*ConvRecord, len(records))
	copy(sorted, records)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].CreatedAt.Before(sorted[j-1].CreatedAt); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	// 1. 衔接图：B.parent = 时间早于 B 且 resp 匹配 B.last_asst 的最近记录。
	// 同一前驱多条后继 = 分支；同内容多前驱取最近（重试生成相同回复的罕见情形）
	idx := make(map[int64]int, len(sorted))
	for i, r := range sorted {
		idx[r.ID] = i
	}
	parent := make(map[int64]int64, len(sorted))
	children := make(map[int64][]int64, len(sorted))
	for i, b := range sorted {
		if b.LastAsstMsgHash == "" {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			if sorted[j].RespMsgHash == b.LastAsstMsgHash {
				parent[b.ID] = sorted[j].ID
				children[sorted[j].ID] = append(children[sorted[j].ID], b.ID)
				break
			}
		}
	}

	// 2. content 模式：从锚点 BFS 连通筛选（指纹碰撞的无关会话在图上不连通，天然剔除）
	keep := make(map[int64]bool, len(sorted))
	if mode == CorrelationModeContentChain {
		queue := []int64{anchor.ID}
		keep[anchor.ID] = true
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			if p, ok := parent[cur]; ok && !keep[p] {
				keep[p] = true
				queue = append(queue, p)
			}
			for _, c := range children[cur] {
				if !keep[c] {
					keep[c] = true
					queue = append(queue, c)
				}
			}
		}
	} else {
		for _, r := range sorted {
			keep[r.ID] = true
		}
	}

	// 3. 主链：锚点向上沿 parent 到根，向下每层取最新后继；
	// 其余后继子树标记 branch（保留在段记录中但不参与渲染）
	// 3. 主干链：从每个保留集根节点（无前驱）向下走「最新后继」链；
	// 保留集中不在任何主干上的记录即分支（regenerate 产生的旁支），
	// 不参与渲染但保留在段记录中。多根并存即压缩/改写断段——两段都渲染
	trunk := make(map[int64]bool, len(keep))
	var roots []int64
	for _, r := range sorted {
		if !keep[r.ID] {
			continue
		}
		if _, hasParent := parent[r.ID]; !hasParent {
			roots = append(roots, r.ID)
		}
	}
	for _, root := range roots {
		for cur := root; ; {
			trunk[cur] = true
			var keptChildren []int64
			for _, c := range children[cur] {
				if keep[c] {
					keptChildren = append(keptChildren, c)
				}
			}
			if len(keptChildren) == 0 {
				break
			}
			latest := keptChildren[0]
			for _, c := range keptChildren[1:] {
				if idx[c] > idx[latest] {
					latest = c
				}
			}
			cur = latest
		}
	}

	// 4. 分段 + 记录元信息
	v.Segments = buildSegments(sorted, keep, parent, mode)
	segIndexOf := make(map[int64]int, len(sorted))
	v.RecordCount = 0
	for si := range v.Segments {
		v.RecordCount += len(v.Segments[si].RecordIDs)
		for ri := range v.Segments[si].Records {
			rec := &v.Segments[si].Records[ri]
			rec.IsBranch = keep[rec.ID] && !trunk[rec.ID]
			rec.IsAnchor = rec.ID == anchor.ID
			segIndexOf[rec.ID] = v.Segments[si].SegmentIndex
		}
	}

	// 5. 渲染：主干记录从旧到新做尾部对齐合并 + 响应追加
	v.Messages, v.Missing = renderMessages(sorted, keep, trunk, segIndexOf, mode)

	// 消息上限：保留最近部分（会话尾部价值最高）
	if len(v.Messages) > ConversationMaxMessages {
		v.Messages = v.Messages[len(v.Messages)-ConversationMaxMessages:]
		v.Truncated = true
	}
	return v
}

// buildSegments 分段扫描：identity 模式无衔接但间隔未超窗仍视为同段
// （推断本质是启发式），其余模式按衔接断裂切分；断点原因细分见 classifyBreak
func buildSegments(sorted []*ConvRecord, keep map[int64]bool, parent map[int64]int64, mode string) []ConversationSegment {
	var segs []ConversationSegment
	cur := ConversationSegment{SegmentIndex: 1}
	var prev *ConvRecord
	for _, r := range sorted {
		if !keep[r.ID] {
			continue
		}
		linked := false
		if prev != nil {
			linked = parent[r.ID] == prev.ID
			if !linked && mode == CorrelationModeIdentity && r.CreatedAt.Sub(prev.CreatedAt) <= ConversationInferredGap {
				linked = true
			}
		}
		if prev != nil && !linked {
			cur.EndTime = formatConvTime(prev.CreatedAt)
			segs = append(segs, cur)
			// 断点原因挂在**新段**上（本段起点相对上段的断点语义）
			reason := classifyBreak(prev, r)
			if mode == CorrelationModeIdentity {
				reason = ResetReasonInferredGap
			}
			cur = ConversationSegment{SegmentIndex: len(segs) + 1, ResetReason: reason}
		}
		cur.RecordIDs = append(cur.RecordIDs, r.ID)
		cur.Records = append(cur.Records, ConversationRecordInfo{
			ID:         r.ID,
			RequestID:  r.RequestID,
			CreatedAt:  formatConvTime(r.CreatedAt),
			ModelName:  r.ModelName,
			StatusCode: r.StatusCode,
			LatencyMs:  r.LatencyMs,
			IsStream:   looksLikeSSE(r.RespBody),
		})
		if cur.StartTime == "" {
			cur.StartTime = formatConvTime(r.CreatedAt)
		}
		prev = r
	}
	if prev != nil {
		cur.EndTime = formatConvTime(prev.CreatedAt)
		segs = append(segs, cur)
	}
	// 压缩断点：段首记录的首条 user 消息即摘要块所在（压缩客户端把它放在历史头部）
	for i := range segs {
		if segs[i].ResetReason == ResetReasonCompression {
			segs[i].SummaryText = segmentSummary(sorted, segs[i].RecordIDs[0])
		}
	}
	return segs
}

// classifyBreak 断点原因：写时标记或消息量骤降 → 压缩；其余（会话键连续
// 但衔接断）→ 历史改写/话题新建
func classifyBreak(prev, next *ConvRecord) string {
	if next.IsContextReset {
		return ResetReasonCompression
	}
	pc, pchars := conversationBodyStats(prev)
	nc, nchars := conversationBodyStats(next)
	if pc >= 2 && nc >= 1 && nc*2 <= pc && nchars*5 < pchars {
		return ResetReasonCompression // 消息数降过半且字符量降八成
	}
	return ResetReasonRewrite
}

// conversationBodyStats 记录正文的协议内消息数与字符量（解析失败按 0 计，
// 结构判定退化为写时标记）
func conversationBodyStats(r *ConvRecord) (msgs int, chars int) {
	for _, m := range parseConversationBody(r.Path, r.ReqBody) {
		msgs++
		chars += len(m.Text)
	}
	return msgs, chars
}

// segmentSummary 段首记录首条 user 消息文本（截断 200 字符）作为摘要提示
func segmentSummary(sorted []*ConvRecord, firstID int64) string {
	for _, r := range sorted {
		if r.ID != firstID {
			continue
		}
		for _, m := range parseConversationBody(r.Path, r.ReqBody) {
			if m.Role == "user" && m.Text != "" {
				runes := []rune(m.Text)
				if len(runes) > 200 {
					return string(runes[:200]) + "…"
				}
				return m.Text
			}
		}
		return ""
	}
	return ""
}

// renderMessages 主干渲染：从旧到新对每条记录做「尾部对齐合并」再追加该轮响应。
// 尾部对齐：已渲染消息的后 k 条在最新消息序列中找到连续匹配时，把新序列
// 匹配点之后的增量追加进来——被客户端裁剪的早期历史由较早记录的前缀保留，
// 两段在中间衔接处自动拼合。完全无交集时的行为按模式区分：
//   - 键/内容链模式：新上下文段（压缩/改写），消息直接追加，segment_index 标记边界；
//   - 推断模式：可能是并发的另一会话，不硬拼进对话流，missing 标注。
func renderMessages(sorted []*ConvRecord, keep, trunk map[int64]bool, segIndexOf map[int64]int, mode string) ([]ConversationMessage, []ConversationMissing) {
	type rendered struct {
		msg    canonMsg
		source int64
	}
	var result []rendered
	var missing []ConversationMissing

	for _, r := range sorted {
		if !keep[r.ID] || !trunk[r.ID] {
			continue
		}
		ms := parseConversationBody(r.Path, r.ReqBody)
		if len(ms) == 0 && isJSONTruncated(r.ReqBody) {
			missing = append(missing, ConversationMissing{RecordID: r.ID, Reason: "body_truncated"})
		}
		if len(ms) > 0 {
			if len(result) == 0 {
				for _, m := range ms {
					result = append(result, rendered{msg: m, source: r.ID})
				}
			} else {
				// 贪心最长尾部匹配（k 从大到小，命中即停）：
				//   命中且有增量 → 追加增量；命中无增量 → 重试/重复请求，跳过；
				//   完全无命中 → 压缩/改写的新上下文段，整体追加（消息带段归属）
				overlap := false
				maxK := len(result)
				if len(ms) < maxK {
					maxK = len(ms)
				}
				for k := maxK; k >= 1 && !overlap; k-- {
					tail := result[len(result)-k:]
					for j := 0; j+k <= len(ms); j++ {
						match := true
						for t := 0; t < k; t++ {
							if !equalCanonMsg(tail[t].msg, ms[j+t]) {
								match = false
								break
							}
						}
						if match {
							overlap = true
							for _, m := range ms[j+k:] {
								result = append(result, rendered{msg: m, source: r.ID})
							}
							break
						}
					}
				}
				if !overlap {
					if mode == CorrelationModeIdentity && len(result) > 0 {
						// 推断模式：与已渲染内容无关联，可能是并发的另一会话，
						// 不硬拼进对话流（记录仍保留在段列表中）
						missing = append(missing, ConversationMissing{RecordID: r.ID, Reason: "unaligned_inferred"})
						continue
					}
					for _, m := range ms {
						result = append(result, rendered{msg: m, source: r.ID})
					}
				}
			}
		}
		// 该轮响应：下一条记录的历史带回相同 assistant 消息时被尾部对齐去重，
		// 被裁剪/未回传时由此补全（取尾法之外回溯补全的另一半）。
		// 流式形态按 SSE 特征判定（审计表无 is_stream 列）
		if m := extractRespAssistant(r.Path, r.RespBody, looksLikeSSE(r.RespBody)); m != nil && !isDegenerate(m) {
			result = append(result, rendered{msg: *m, source: r.ID})
		} else if r.RespBody == "" {
			missing = append(missing, ConversationMissing{RecordID: r.ID, Reason: "response_not_recorded"})
		}
	}

	out := make([]ConversationMessage, 0, len(result))
	for _, rr := range result {
		out = append(out, ConversationMessage{
			Role:           rr.msg.Role,
			Text:           rr.msg.Text,
			ToolCalls:      toConversationToolCalls(rr.msg.ToolCalls),
			ToolCallID:     rr.msg.ToolCallID,
			SourceRecordID: rr.source,
			SegmentIndex:   segIndexOf[rr.source],
		})
	}
	return out, missing
}

// isDegenerate 空文本且无工具调用的消息（无渲染价值）
func isDegenerate(m *canonMsg) bool {
	return m.Text == "" && len(m.ToolCalls) == 0
}

// equalCanonMsg 规范化消息结构相等（尾部对齐匹配的判等基础）
func equalCanonMsg(a, b canonMsg) bool {
	if a.Role != b.Role || a.Text != b.Text || a.ToolCallID != b.ToolCallID || len(a.ToolCalls) != len(b.ToolCalls) {
		return false
	}
	for i := range a.ToolCalls {
		if a.ToolCalls[i] != b.ToolCalls[i] {
			return false
		}
	}
	return true
}

// isJSONTruncated 截断标记（truncateBody 的固定后缀）
func isJSONTruncated(body string) bool {
	return strings.HasSuffix(body, "...[truncated]")
}

// looksLikeSSE 响应体是否为 SSE 流形态（与 truncateBody 同一启发式：
// "data:" 行前缀）。审计表不落 is_stream 列，读取侧按形态判定
func looksLikeSSE(body string) bool {
	return strings.HasPrefix(body, "data:") || strings.Contains(body, "\ndata:")
}

func toConversationToolCalls(tools []canonTool) []ConversationToolCall {
	if len(tools) == 0 {
		return nil
	}
	out := make([]ConversationToolCall, 0, len(tools))
	for _, t := range tools {
		out = append(out, ConversationToolCall{ID: t.ID, Name: t.Name, Args: t.Args})
	}
	return out
}

// maskSessionKey 会话键掩码：保留来源与前段哈希 + 末 4 位（不透出完整键）
func maskSessionKey(key string) string {
	if len(key) <= 20 {
		return key
	}
	return key[:16] + "…" + key[len(key)-4:]
}

func formatConvTime(t time.Time) string {
	return t.Format(conversationTimeLayout)
}

// ============================================================
// 数据访问层（查询薄层，集成测试覆盖）
// ============================================================

// LoadConversationView 加载锚点并按三层降级收集候选后重组。
// tenantID > 0 时为强制的租户双键条件（租户端必传）；admin 全局视角传 0。
// cols 选择系统/租户正文副本。
func LoadConversationView(ctx context.Context, anchorID, tenantID int64, cols BodyColumns) (*ConversationView, error) {
	anchor, err := loadConvRecord(ctx, anchorID, tenantID, cols)
	if err != nil {
		return nil, err
	}

	lower := anchor.CreatedAt.Add(-ConversationTimeBound)
	upper := anchor.CreatedAt.Add(ConversationTimeBound)

	var records []*ConvRecord
	var mode string
	// 候选一律按锚点租户收敛：指纹为纯内容哈希（不含租户维度），admin 全局
	// 视角下同开场白跨租户会误并；会话键虽含租户命名空间，统一收敛无副作用
	scopeTenant := anchor.TenantID
	switch {
	case anchor.SessionKey != "":
		mode = CorrelationModeSessionKey
		records, err = queryConvRecords(ctx, scopeTenant, "session_key", anchor.SessionKey, lower, upper, cols, 0)
		// 键漂移兜底：键层只聚到锚点自己时降级指纹层。存量数据里
		// previous_response_id 派生的键随轮次漂移聚不起来；真单轮会话
		// 降级无害（指纹层同样只聚到锚点自己）
		if err == nil && len(records) <= 1 && anchor.ConvFingerprint != "" {
			mode = CorrelationModeContentChain
			records, err = queryConvRecords(ctx, scopeTenant, "conv_fingerprint", anchor.ConvFingerprint, lower, upper, cols, 0)
		}
	case anchor.ConvFingerprint != "":
		mode = CorrelationModeContentChain
		records, err = queryConvRecords(ctx, scopeTenant, "conv_fingerprint", anchor.ConvFingerprint, lower, upper, cols, 0)
	default:
		// 身份三元组（不含 model：允许会话中途切模型）
		mode = CorrelationModeIdentity
		records, err = queryConvRecords(ctx, scopeTenant, "user_id", anchor.UserID, lower, upper, cols, anchor.ApiKeyID)
	}
	if err != nil {
		return nil, err
	}

	// 多取一条用于 truncated 判定；超限时丢最旧一条（锚点恰为最旧时丢最新，保锚点在内）
	if len(records) > ConversationMaxRecords {
		if records[0].ID == anchor.ID {
			records = records[:len(records)-1]
		} else {
			records = records[1:]
		}
	}
	v := RebuildConversation(anchor, records, mode)
	if len(records) >= ConversationMaxRecords {
		v.Truncated = true
	}
	return v, nil
}

// convRecordColumns 查询列（正文按副本选择，其余为重组所需元数据）。
// 审计表不落 is_stream 列——流式形态读取侧按响应体 SSE 特征判定（looksLikeSSE）
func convRecordColumns(cols BodyColumns) string {
	return "id, tenant_id, request_id, created_at, path, model_name, status_code, latency_ms, " +
		"session_key, session_source, conv_fingerprint, resp_msg_hash, last_asst_msg_hash, " +
		"is_context_reset, user_id, api_key_id, " +
		cols.ReqBodyCol + ", " + cols.RespBodyCol
}

// loadConvRecord 加载锚点记录（tenantID > 0 时双键校验：找不到即不存在或跨租户）
func loadConvRecord(ctx context.Context, anchorID, tenantID int64, cols BodyColumns) (*ConvRecord, error) {
	m := AuditModelCtx(ctx, "aud_request_logs").Where("id", anchorID)
	if tenantID > 0 {
		m = m.Where("tenant_id", tenantID)
	}
	one, err := m.Fields(convRecordColumns(cols)).One()
	if err != nil {
		return nil, err
	}
	if one.IsEmpty() {
		return nil, NewNotFoundError("审计日志")
	}
	return scanConvRecord(one, cols), nil
}

// queryConvRecords 候选收集：键列（session_key/conv_fingerprint/user_id）+
// 租户（>0 时）+ 时间界；identity 模式额外带 api_key_id。
func queryConvRecords(ctx context.Context, tenantID int64, keyCol string, keyVal any,
	lower, upper time.Time, cols BodyColumns, extraAPIKeyID int64) ([]*ConvRecord, error) {
	m := AuditModelCtx(ctx, "aud_request_logs").
		Where(keyCol, keyVal).
		WhereBetween("created_at", lower, upper)
	if tenantID > 0 {
		m = m.Where("tenant_id", tenantID)
	}
	if extraAPIKeyID > 0 {
		m = m.Where("api_key_id", extraAPIKeyID)
	}
	all, err := m.Fields(convRecordColumns(cols)).
		Order("created_at asc").
		Limit(ConversationMaxRecords + 1).
		All()
	if err != nil {
		return nil, err
	}
	out := make([]*ConvRecord, 0, len(all))
	for _, r := range all {
		out = append(out, scanConvRecord(r, cols))
	}
	return out, nil
}

// scanConvRecord 行 → ConvRecord（正文列按副本名取值）
func scanConvRecord(one gdb.Record, cols BodyColumns) *ConvRecord {
	return &ConvRecord{
		ID:              one["id"].Int64(),
		TenantID:        one["tenant_id"].Int64(),
		RequestID:       one["request_id"].String(),
		CreatedAt:       one["created_at"].Time(),
		Path:            one["path"].String(),
		ModelName:       one["model_name"].String(),
		StatusCode:      one["status_code"].Int(),
		LatencyMs:       one["latency_ms"].Int(),
		UserID:          one["user_id"].Int64(),
		ApiKeyID:        one["api_key_id"].Int64(),
		SessionKey:      one["session_key"].String(),
		SessionSource:   one["session_source"].String(),
		ConvFingerprint: one["conv_fingerprint"].String(),
		RespMsgHash:     one["resp_msg_hash"].String(),
		LastAsstMsgHash: one["last_asst_msg_hash"].String(),
		IsContextReset:  one["is_context_reset"].Bool(),
		ReqBody:         one[cols.ReqBodyCol].String(),
		RespBody:        one[cols.RespBodyCol].String(),
	}
}

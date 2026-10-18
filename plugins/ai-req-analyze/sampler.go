package aireqanalyze

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

// 会话采样：把分析窗口内的审计日志收敛为「分析单元」描述符（不含正文）。
//
// 三类单元（见设计文档 2.5 节）：
//   - 会话单元：session_key 分组，锚点 = 组内最大 id（最新请求）
//   - 内容链单元：无会话键但有 conv_fingerprint 的记录按指纹分组
//   - 独立单元：两者皆空（单发/embedding/任务提交），逐条分析
//
// 刻意不用身份推断层（第③层）——同用户同时段的无关单发会被误聚成伪会话，污染分析上下文。
// 正文的获取是惰性的：会话单元在分批处理时经 LoadConversationView 重组，
// 独立单元按批 IN 查询，内存与查询量都随批推进而非随窗口总量扩张。
//
// 全部查询经 AuditModelCtx 访问（独立审计库部署时自动路由），插件对审计库只读。

// maxStandaloneUnits 独立单元采样上限。
// 单发请求（embedding 等）可能数量巨大，超限部分不进入本次分析并在报告中标注；
// 会话单元无此限制——每会话一行分组结果，全量进入分析（内网模型算力不受限）。
const maxStandaloneUnits = 2000

// standalonePageSize 独立单元游标分页的页大小。
const standalonePageSize = 500

// SampledUnit 分析单元描述符。
type SampledUnit struct {
	Kind        string // UnitTypeSession / UnitTypeStandalone
	SessionKey  string // 会话单元的稳定键；内容链单元与独立单元为空
	AnchorLogID int64  // 会话/内容链锚点日志ID；独立单元为记录自身 ID
	Turns       int    // 分组内记录数（独立单元恒为 1）
}

// SampleResult 采样结果。
type SampleResult struct {
	SessionUnits     []*SampledUnit // 会话 + 内容链单元
	StandaloneUnits  []*SampledUnit // 独立单元
	LogCount         int            // 窗口内审计日志总条数（含未采样的）
	StandaloneCapped bool           // 独立单元达到上限被截断
}

// sampleUnits 执行采样：2 条分组查询 + 独立单元游标分页 + 1 条计数。
func sampleUnits(ctx context.Context, tenantID, userID int64, start, end time.Time) (*SampleResult, error) {
	result := &SampleResult{}

	var err error
	result.LogCount, err = countWindowLogs(ctx, tenantID, userID, start, end)
	if err != nil {
		return nil, err
	}

	// ① 会话单元：按稳定键分组，锚点取组内最新记录（重组以它为中心外扩）
	result.SessionUnits, err = sampleGroupedUnits(ctx, tenantID, userID, start, end, true)
	if err != nil {
		return nil, err
	}

	// ② 内容链单元：无稳定键但有指纹的记录按指纹分组
	chainUnits, err := sampleGroupedUnits(ctx, tenantID, userID, start, end, false)
	if err != nil {
		return nil, err
	}
	result.SessionUnits = append(result.SessionUnits, chainUnits...)

	// ③ 独立单元：键与指纹皆空，id 游标分页
	result.StandaloneUnits, result.StandaloneCapped, err = sampleStandaloneUnits(ctx, tenantID, userID, start, end)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// baseWindowQuery 窗口过滤条件的公共构造。
func baseWindowQuery(ctx context.Context, tenantID, userID int64, start, end time.Time) *gdb.Model {
	m := lcommon.AuditModelCtx(ctx, "aud_request_logs").
		Where("tenant_id", tenantID).
		WhereBetween("created_at", start, end)
	if userID > 0 {
		m = m.Where("user_id", userID)
	}
	return m
}

// countWindowLogs 窗口内日志总条数（去重率统计的分母）。
func countWindowLogs(ctx context.Context, tenantID, userID int64, start, end time.Time) (int, error) {
	v, err := baseWindowQuery(ctx, tenantID, userID, start, end).Count()
	return v, err
}

// sampleGroupedUnits 会话键/指纹分组采样。
// bySessionKey=true 按 session_key 分组；false 按无键记录的 conv_fingerprint 分组。
func sampleGroupedUnits(ctx context.Context, tenantID, userID int64, start, end time.Time, bySessionKey bool) ([]*SampledUnit, error) {
	m := baseWindowQuery(ctx, tenantID, userID, start, end)
	keyCol := "session_key"
	if bySessionKey {
		m = m.Where("session_key <> ''")
	} else {
		m = m.Where("(session_key = '' OR session_key IS NULL)").Where("conv_fingerprint <> ''")
		keyCol = "conv_fingerprint"
	}

	all, err := m.
		Fields(keyCol + " AS group_key, MAX(id) AS anchor_id, COUNT(*) AS turns").
		Group(keyCol).
		Order("anchor_id ASC").
		All()
	if err != nil {
		return nil, err
	}

	units := make([]*SampledUnit, 0, len(all))
	for _, r := range all {
		key := r["group_key"].String()
		if key == "" {
			continue
		}
		unit := &SampledUnit{
			Kind:        UnitTypeSession,
			AnchorLogID: r["anchor_id"].Int64(),
			Turns:       r["turns"].Int(),
		}
		// 内容链单元不落 session_key（指纹不是稳定键，findings 回链靠锚点）
		if bySessionKey {
			unit.SessionKey = key
		}
		units = append(units, unit)
	}
	return units, nil
}

// sampleStandaloneUnits 独立单元采样（id 游标分页），返回单元列表与是否达上限。
func sampleStandaloneUnits(ctx context.Context, tenantID, userID int64, start, end time.Time) ([]*SampledUnit, bool, error) {
	units := make([]*SampledUnit, 0, standalonePageSize)
	var lastID int64
	capped := false

	for len(units) < maxStandaloneUnits {
		pageSize := standalonePageSize
		if remaining := maxStandaloneUnits - len(units); remaining < pageSize {
			pageSize = remaining
		}

		m := baseWindowQuery(ctx, tenantID, userID, start, end).
			Where("(session_key = '' OR session_key IS NULL)").
			Where("(conv_fingerprint = '' OR conv_fingerprint IS NULL)")
		if lastID > 0 {
			m = m.Where("id > ?", lastID)
		}
		all, err := m.Fields("id").Order("id ASC").Limit(pageSize + 1).All()
		if err != nil {
			return nil, false, err
		}

		for i, r := range all {
			if i == pageSize { // 多取一条用于判断是否还有下一页
				break
			}
			id := r["id"].Int64()
			units = append(units, &SampledUnit{Kind: UnitTypeStandalone, AnchorLogID: id, Turns: 1})
			lastID = id
		}

		if len(all) <= pageSize {
			break // 没有下一页
		}
		capped = true // 还有剩余但达到采样上限（循环条件退出时）
	}

	return units, capped, nil
}

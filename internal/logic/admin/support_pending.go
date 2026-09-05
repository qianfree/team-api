package admin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
)

// ============================================================
// 顶栏铃铛：客服支持待办聚合
//
// 与工作台（workbench.go）的分工：工作台回答「哪些事异常了」，
// 刻意不报工单总数避免永久红点；铃铛回答「有多少客户在等回复」，
// 报全量 pending —— 处理完自动归零，不需要任何人点「完成」。
// 待办不落库、实时派生，整份结果进 Redis 缓存 30s（与工作台同模式），
// 菜单红点轮询与铃铛各自独立缓存键，互不挤兑。
// ============================================================

const (
	supportPendingCacheKey = "admin:support_pending:v1"
	supportPendingCacheTTL = 30 * time.Second
)

// supportPendingCollected 一轮聚合的完整结果（进缓存的就是它）。
type supportPendingCollected struct {
	Tickets        int    `json:"tickets"`
	TicketsOverdue int    `json:"tickets_overdue"`
	Feedbacks      int    `json:"feedbacks"`
	FeedbacksWait  string `json:"feedbacks_wait"`
	GeneratedAt    string `json:"generated_at"`
}

// GetSupportPendingSummary 顶栏铃铛待办计数。
func (s *sAdmin) GetSupportPendingSummary(ctx context.Context, _ *v1.AdminSupportPendingSummaryReq) (*v1.AdminSupportPendingSummaryRes, error) {
	col := s.collectSupportPendingCached(ctx)
	return &v1.AdminSupportPendingSummaryRes{
		Tickets:        col.Tickets,
		TicketsOverdue: col.TicketsOverdue,
		Feedbacks:      col.Feedbacks,
		FeedbacksWait:  col.FeedbacksWait,
		Total:          col.Tickets + col.Feedbacks,
		GeneratedAt:    col.GeneratedAt,
	}, nil
}

// collectSupportPendingCached 读缓存，未命中则重新聚合。
// 缓存读写失败一律降级为直接查询 —— 铃铛不能因为 Redis 抖动就消失。
func (s *sAdmin) collectSupportPendingCached(ctx context.Context) *supportPendingCollected {
	if v, err := g.Redis().Get(ctx, supportPendingCacheKey); err == nil && !v.IsEmpty() {
		var col supportPendingCollected
		if json.Unmarshal(v.Bytes(), &col) == nil {
			return &col
		}
	}

	col := s.collectSupportPending(ctx)

	if b, err := json.Marshal(col); err == nil {
		if _, err := g.Redis().Do(ctx, "SET", supportPendingCacheKey, b, "EX", int(supportPendingCacheTTL.Seconds())); err != nil {
			g.Log().Warningf(ctx, "support pending: 写缓存失败: %v", err)
		}
	}
	return col
}

// collectSupportPending 两个独立查询，任一失败只丢自己那部分（记 warning），
// 不拖垮整个铃铛 —— 半个铃铛也好过没有铃铛。
func (s *sAdmin) collectSupportPending(ctx context.Context) *supportPendingCollected {
	col := &supportPendingCollected{GeneratedAt: gtime.Now().Format(time.RFC3339)}

	// 工单：pending + reopened 全量。超 SLA 口径与工作台 collector 一致：
	// 仅 pending 计超时（reopened 说明已有人处理过，不算「未首响」）。
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT COUNT(*) AS cnt,
		        COUNT(*) FILTER (WHERE status = 'pending' AND created_at <= ?) AS overdue
		   FROM spt_tickets
		  WHERE status IN ('pending', 'reopened')`, gtime.Now().Add(-wbTicketSLAHours*time.Hour))
	if err != nil {
		g.Log().Warningf(ctx, "support pending: 查询待处理工单失败: %v", err)
	} else if len(rows) > 0 {
		col.Tickets = rows[0]["cnt"].Int()
		col.TicketsOverdue = rows[0]["overdue"].Int()
	}

	// 反馈：pending 全量 + 最早一条的等待时长（反馈无 SLA 概念，只展示积压程度）。
	fbRows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT COUNT(*) AS cnt, MIN(created_at) AS oldest
		   FROM spt_feedbacks WHERE status = 'pending'`)
	if err != nil {
		g.Log().Warningf(ctx, "support pending: 查询待处理反馈失败: %v", err)
	} else if len(fbRows) > 0 {
		col.Feedbacks = fbRows[0]["cnt"].Int()
		if oldest := fbRows[0]["oldest"].GTime(); oldest != nil {
			col.FeedbacksWait = humanizeSince(oldest)
		}
	}
	return col
}

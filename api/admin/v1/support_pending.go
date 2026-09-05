package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

// === 顶栏铃铛（客服支持待办聚合） ===

// AdminSupportPendingSummaryReq 顶栏铃铛的待办计数。
// 口径与工作台刻意不同：工作台只报「超 SLA / 未分配」等异常项避免永久红点；
// 铃铛报全量待处理 —— 管理员处理一条少一条，归零即「没有客户在等回复」，
// 是运营收件箱语义而非消息通知语义（无已读概念，不落消息表）。
type AdminSupportPendingSummaryReq struct {
	g.Meta `path:"/support/pending-summary" method:"get" mime:"json" tags:"管理后台-待办提醒" summary:"顶栏铃铛待办计数"`
}

type AdminSupportPendingSummaryRes struct {
	Tickets        int    `json:"tickets" dc:"待处理工单数（pending + reopened）"`
	TicketsOverdue int    `json:"tickets_overdue" dc:"其中超 SLA 未首次响应的工单数"`
	Feedbacks      int    `json:"feedbacks" dc:"待处理反馈数"`
	FeedbacksWait  string `json:"feedbacks_wait" dc:"最早一条待处理反馈已等待时长（如「3 天」；无待处理时为空）"`
	Total          int    `json:"total" dc:"tickets + feedbacks，铃铛角标总数"`
	GeneratedAt    string `json:"generated_at" dc:"数据生成时间（RFC3339），命中缓存时为缓存写入时刻"`
}

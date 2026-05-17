package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/gconv"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

// alertEventSelectFields is the shared SELECT clause for PostgreSQL array type casting.
const alertEventSelectFields = `id, rule_id, rule_name, metric_type, level, status,
	trigger_value, threshold_value, trigger_message,
	acknowledged_by, acknowledged_at,
	resolve_notes, resolved_by, resolved_at,
	COALESCE(notified_methods::text, '') as notified_methods,
	created_at, updated_at`

// buildAlertEventWhere builds shared WHERE clause and args for alert event queries.
func buildAlertEventWhere(status, level string, ruleID int64) (string, []any) {
	where := "WHERE 1=1"
	args := []any{}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if level != "" {
		where += " AND level = ?"
		args = append(args, level)
	}
	if ruleID > 0 {
		where += " AND rule_id = ?"
		args = append(args, ruleID)
	}
	return where, args
}

// ListAlertEvents returns a paginated list of alert events.
func ListAlertEvents(ctx context.Context, page, pageSize int, status, level string, ruleID int64) (map[string]any, error) {
	page, pageSize = lcommon.NormalizePagination(page, pageSize)

	// Build shared WHERE conditions
	where, whereArgs := buildAlertEventWhere(status, level, ruleID)

	// Count total using ORM model builder
	countQuery := dao.OpsAlertEvents.Ctx(ctx)
	if status != "" {
		countQuery = countQuery.Where("status", status)
	}
	if level != "" {
		countQuery = countQuery.Where("level", level)
	}
	if ruleID > 0 {
		countQuery = countQuery.Where("rule_id", ruleID)
	}
	total, err := countQuery.Count()
	if err != nil {
		return nil, err
	}

	// Fetch page using raw SQL for PostgreSQL array type casting
	sqlQuery := fmt.Sprintf(
		"SELECT %s FROM ops_alert_events %s ORDER BY created_at DESC LIMIT ? OFFSET ?",
		alertEventSelectFields, where,
	)
	sqlArgs := append(whereArgs, pageSize, (page-1)*pageSize)

	result, err := g.DB().Ctx(ctx).Raw(sqlQuery, sqlArgs...).All()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"list":  result.List(),
		"total": total,
		"page":  page,
	}, nil
}

// AcknowledgeAlert marks an alert event as acknowledged.
func AcknowledgeAlert(ctx context.Context, eventID, adminID int64) error {
	result, err := dao.OpsAlertEvents.Ctx(ctx).
		Where("id", eventID).
		Where("status", "firing").
		Data(do.OpsAlertEvents{
			Status:         "acknowledged",
			AcknowledgedBy: adminID,
			AcknowledgedAt: gtime.Now(),
		}).Update()
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return lcommon.NewBusinessError(404, "事件不存在或状态不是 firing")
	}

	return nil
}

// ResolveAlert marks an alert event as resolved with notes.
func ResolveAlert(ctx context.Context, eventID, adminID int64, notes string) error {
	result, err := dao.OpsAlertEvents.Ctx(ctx).
		Where("id", eventID).
		Where("status !=", "resolved").
		Data(do.OpsAlertEvents{
			Status:       "resolved",
			ResolveNotes: notes,
			ResolvedBy:   adminID,
			ResolvedAt:   gtime.Now(),
		}).Update()
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return lcommon.NewBusinessError(404, "事件不存在或已解决")
	}

	// Clear the firing state in Redis for this rule
	type eventRow struct {
		RuleID int64 `json:"rule_id"`
	}
	var evt eventRow
	dao.OpsAlertEvents.Ctx(ctx).
		Where("id", eventID).
		Fields("rule_id").
		Scan(&evt)
	if evt.RuleID > 0 {
		redisKey := fmt.Sprintf("ops:alert:firing:%d", evt.RuleID)
		g.Redis().Do(ctx, "DEL", redisKey)
	}

	return nil
}

// GetAlertStats returns alert statistics summary.
func GetAlertStats(ctx context.Context) (map[string]any, error) {
	// Status counts
	type statusCount struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	var statusCounts []statusCount
	err := dao.OpsAlertEvents.Ctx(ctx).
		Fields("status, COUNT(*) as count").
		Where("created_at >= ?", time.Now().Add(-24*time.Hour)).
		Group("status").
		Scan(&statusCounts)
	if err != nil {
		return nil, err
	}

	counts := map[string]int{
		"firing":       0,
		"acknowledged": 0,
		"resolved":     0,
	}
	for _, sc := range statusCounts {
		counts[sc.Status] = sc.Count
	}

	// Level counts for firing events
	type levelCount struct {
		Level string `json:"level"`
		Count int    `json:"count"`
	}
	var levelCounts []levelCount
	err = dao.OpsAlertEvents.Ctx(ctx).
		Fields("level, COUNT(*) as count").
		Where("status", "firing").
		Group("level").
		Scan(&levelCounts)
	if err != nil {
		g.Log().Warningf(ctx, "get alert level counts: %v", err)
	}

	lCounts := map[string]int{
		"info":     0,
		"warning":  0,
		"critical": 0,
	}
	for _, lc := range levelCounts {
		lCounts[lc.Level] = lc.Count
	}

	return map[string]any{
		"status_counts": counts,
		"level_counts":  lCounts,
		"total_firing":  counts["firing"],
	}, nil
}

// SendTestAlert sends a test notification for a given rule.
func SendTestAlert(ctx context.Context, ruleID int64) error {
	rule, err := GetAlertRule(ctx, ruleID)
	if err != nil {
		return err
	}

	ruleName := gconv.String(rule["name"])
	metricType := gconv.String(rule["metric_type"])
	threshold := gconv.Float64(rule["threshold"])

	testEvent := map[string]any{
		"id":              0,
		"rule_name":       ruleName,
		"metric_type":     metricType,
		"level":           gconv.String(rule["level"]),
		"trigger_value":   threshold + 1,
		"threshold_value": threshold,
		"trigger_message": fmt.Sprintf("[测试] 告警规则「%s」测试通知", ruleName),
	}

	return sendTestNotification(ctx, rule, testEvent)
}

package monitor

import (
	"context"
	"fmt"
	"github.com/qianfree/team-api/internal/dao"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"
)

var validMetricTypes = map[string]string{
	"api.error_rate":           "API错误率(%)",
	"api.p95_latency":          "API P95延迟(ms)",
	"api.p99_latency":          "API P99延迟(ms)",
	"api.qps":                  "API QPS(请求/秒)",
	"system.cpu_percent":       "CPU使用率(%)",
	"system.memory_percent":    "内存使用率(%)",
	"system.disk_percent":      "磁盘使用率(%)",
	"db.active_connections":    "DB活跃连接数",
	"redis.used_memory_mb":     "Redis内存使用(MB)",
	"channel.error_count":      "渠道错误数(5min)",
	"channel.rate_limit_count": "渠道限速错误数(5min)",
}

// Alert rule conditions
var validConditions = map[string]string{
	"gt":  "大于",
	"gte": "大于等于",
	"lt":  "小于",
	"lte": "小于等于",
	"eq":  "等于",
}

// Alert rule levels
var validLevels = map[string]string{
	"info":     "信息",
	"warning":  "警告",
	"critical": "严重",
}

// alertRuleSelectFields is the shared SELECT clause for PostgreSQL array type casting.
const alertRuleSelectFields = `id, name, metric_type, condition, threshold, duration_seconds,
	level, is_enabled, cooldown_seconds, last_triggered_at,
	notification_methods::text as notification_methods,
	COALESCE(webhook_url, '') as webhook_url,
	COALESCE(notify_user_ids::text, '') as notify_user_ids,
	created_at, updated_at`

// buildAlertRuleWhere builds shared WHERE clause and args for alert rule queries.
func buildAlertRuleWhere(metricType, level string, enabled *bool) (string, []any) {
	where := "WHERE 1=1"
	args := []any{}
	if metricType != "" {
		where += " AND metric_type = ?"
		args = append(args, metricType)
	}
	if level != "" {
		where += " AND level = ?"
		args = append(args, level)
	}
	if enabled != nil {
		where += " AND is_enabled = ?"
		args = append(args, *enabled)
	}
	return where, args
}

// CreateAlertRule creates a new alert rule.
func CreateAlertRule(ctx context.Context, data do.OpsAlertRules) (int64, error) {
	// Validate
	name := gconv.String(data.Name)
	if name == "" {
		return 0, common.NewBadRequestError("规则名称不能为空")
	}
	metricType := gconv.String(data.MetricType)
	if _, ok := validMetricTypes[metricType]; !ok {
		return 0, gerror.Newf("不支持的指标类型: %s", metricType)
	}
	condition := gconv.String(data.Condition)
	if _, ok := validConditions[condition]; !ok {
		return 0, gerror.Newf("不支持的条件: %s", condition)
	}
	level := gconv.String(data.Level)
	if level == "" {
		level = "warning"
		data.Level = level
	}
	if _, ok := validLevels[level]; !ok {
		return 0, gerror.Newf("不支持的级别: %s", level)
	}

	if len(data.NotificationMethods) == 0 {
		data.NotificationMethods = []string{"email", "in_app"}
	}

	data.IsEnabled = true

	id, err := dao.OpsAlertRules.Ctx(ctx).InsertAndGetId(data)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// UpdateAlertRule updates an existing alert rule.
func UpdateAlertRule(ctx context.Context, id int64, updates do.OpsAlertRules) error {
	// Validate metric_type if provided
	if updates.MetricType != nil {
		metricType := gconv.String(updates.MetricType)
		if _, valid := validMetricTypes[metricType]; !valid {
			return gerror.Newf("不支持的指标类型: %s", metricType)
		}
	}

	_, err := dao.OpsAlertRules.Ctx(ctx).
		Where("id", id).
		Data(updates).
		Update()
	if err != nil {
		return err
	}

	return nil
}

// DeleteAlertRule deletes an alert rule.
func DeleteAlertRule(ctx context.Context, id int64) error {
	// Check for active events
	count, err := dao.OpsAlertEvents.Ctx(ctx).
		Where("rule_id", id).
		Where("status", "firing").
		Count()
	if err != nil {
		return err
	}
	if count > 0 {
		return common.NewBadRequestError("该规则有正在触发的告警事件，请先处理后再删除")
	}

	_, err = dao.OpsAlertRules.Ctx(ctx).Where("id", id).Delete()
	return err
}

// ListAlertRules returns a paginated list of alert rules.
func ListAlertRules(ctx context.Context, page, pageSize int, metricType, level string, enabled *bool) (map[string]any, error) {
	page, pageSize = common.NormalizePagination(page, pageSize)

	// Build shared WHERE conditions
	where, whereArgs := buildAlertRuleWhere(metricType, level, enabled)

	// Count total using ORM model builder
	countQuery := dao.OpsAlertRules.Ctx(ctx)
	if metricType != "" {
		countQuery = countQuery.Where("metric_type", metricType)
	}
	if level != "" {
		countQuery = countQuery.Where("level", level)
	}
	if enabled != nil {
		countQuery = countQuery.Where("is_enabled", *enabled)
	}
	total, err := countQuery.Count()
	if err != nil {
		return nil, err
	}

	// Fetch page using raw SQL for PostgreSQL array type casting
	sqlQuery := fmt.Sprintf(
		"SELECT %s FROM ops_alert_rules %s ORDER BY created_at DESC LIMIT ? OFFSET ?",
		alertRuleSelectFields, where,
	)
	sqlArgs := append(whereArgs, pageSize, (page-1)*pageSize)

	result, err := g.DB().Ctx(ctx).Raw(sqlQuery, sqlArgs...).All()
	if err != nil {
		return nil, err
	}
	rules := result.List()

	// Post-process: parse array text fields and add display names
	for i, rule := range rules {
		parseRuleArrayFields(rules, i, rule)
		addRuleDisplayNames(rules, i, rule)
	}

	return map[string]any{
		"list":  rules,
		"total": total,
		"page":  page,
	}, nil
}

// parseRuleArrayFields parses PostgreSQL array text fields into Go slices.
func parseRuleArrayFields(rules []map[string]any, i int, rule map[string]any) {
	if nmStr := gconv.String(rule["notification_methods"]); nmStr != "" {
		nmStr = strings.Trim(nmStr, "{}")
		if nmStr == "" {
			rules[i]["notification_methods"] = []string{}
		} else {
			rules[i]["notification_methods"] = strings.Split(nmStr, ",")
		}
	}
	if nuiStr := gconv.String(rule["notify_user_ids"]); nuiStr != "" {
		nuiStr = strings.Trim(nuiStr, "{}")
		if nuiStr == "" {
			rules[i]["notify_user_ids"] = []int64{}
		} else {
			parts := strings.Split(nuiStr, ",")
			ids := make([]int64, 0, len(parts))
			for _, p := range parts {
				if idVal, e := strconv.ParseInt(p, 10, 64); e == nil {
					ids = append(ids, idVal)
				}
			}
			rules[i]["notify_user_ids"] = ids
		}
	}
}

// addRuleDisplayNames adds human-readable labels for enum fields.
func addRuleDisplayNames(rules []map[string]any, i int, rule map[string]any) {
	if mt, ok := validMetricTypes[gconv.String(rule["metric_type"])]; ok {
		rules[i]["metric_type_label"] = mt
	}
	if cond, ok := validConditions[gconv.String(rule["condition"])]; ok {
		rules[i]["condition_label"] = cond
	}
	if lvl, ok := validLevels[gconv.String(rule["level"])]; ok {
		rules[i]["level_label"] = lvl
	}
}

// GetAlertRule returns a single alert rule by ID.
func GetAlertRule(ctx context.Context, id int64) (map[string]any, error) {
	record, err := g.DB().Ctx(ctx).Raw(
		"SELECT "+alertRuleSelectFields+" FROM ops_alert_rules WHERE id = ?", id,
	).One()
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, common.NewNotFoundError("规则")
	}
	rule := record.Map()

	// Parse array text fields
	if nmStr := gconv.String(rule["notification_methods"]); nmStr != "" {
		nmStr = strings.Trim(nmStr, "{}")
		if nmStr == "" {
			rule["notification_methods"] = []string{}
		} else {
			rule["notification_methods"] = strings.Split(nmStr, ",")
		}
	}
	if nuiStr := gconv.String(rule["notify_user_ids"]); nuiStr != "" {
		nuiStr = strings.Trim(nuiStr, "{}")
		if nuiStr == "" {
			rule["notify_user_ids"] = []int64{}
		} else {
			parts := strings.Split(nuiStr, ",")
			ids := make([]int64, 0, len(parts))
			for _, p := range parts {
				if idVal, e := strconv.ParseInt(p, 10, 64); e == nil {
					ids = append(ids, idVal)
				}
			}
			rule["notify_user_ids"] = ids
		}
	}

	return rule, nil
}

// ToggleAlertRule enables or disables an alert rule.
func ToggleAlertRule(ctx context.Context, id int64) error {
	_, err := g.DB().Ctx(ctx).Exec(ctx, `
		UPDATE ops_alert_rules
		SET is_enabled = NOT is_enabled,
			updated_at = NOW()
		WHERE id = ?
	`, id)
	return err
}

// GetEnabledRules returns all enabled alert rules (used by the alert engine).
func GetEnabledRules(ctx context.Context) ([]map[string]any, error) {
	result, err := g.DB().Ctx(ctx).Raw(
		"SELECT id, name, metric_type, condition, threshold, duration_seconds, " +
			"level, cooldown_seconds, last_triggered_at, " +
			"notification_methods::text as notification_methods, " +
			"COALESCE(webhook_url, '') as webhook_url, " +
			"COALESCE(notify_user_ids::text, '') as notify_user_ids " +
			"FROM ops_alert_rules WHERE is_enabled = true",
	).All()
	if err != nil {
		return nil, err
	}
	return result.List(), nil
}

// GetMetricTypeOptions returns all available metric types.
func GetMetricTypeOptions() []map[string]string {
	options := make([]map[string]string, 0, len(validMetricTypes))
	for k, v := range validMetricTypes {
		options = append(options, map[string]string{"value": k, "label": v})
	}
	return options
}

// GetConditionOptions returns all available conditions.
func GetConditionOptions() []map[string]string {
	options := make([]map[string]string, 0, len(validConditions))
	for k, v := range validConditions {
		options = append(options, map[string]string{"value": k, "label": v})
	}
	return options
}

// GetLevelOptions returns all available levels.
func GetLevelOptions() []map[string]string {
	options := make([]map[string]string, 0, len(validLevels))
	for k, v := range validLevels {
		options = append(options, map[string]string{"value": k, "label": v})
	}
	return options
}

// GetAdminUsers returns all admin users for the notify user selector.
func GetAdminUsers(ctx context.Context) ([]map[string]any, error) {
	var users []entity.SysAdminUsers
	err := dao.SysAdminUsers.Ctx(ctx).
		Where("status", "active").
		Fields("id, username, display_name").
		Scan(&users)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(users))
	for _, u := range users {
		result = append(result, map[string]any{
			"id":           u.Id,
			"username":     u.Username,
			"display_name": u.DisplayName,
		})
	}
	return result, nil
}

package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/gconv"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

// alertFiringState is stored in Redis to track ongoing alert violations.
type alertFiringState struct {
	RuleID    int64     `json:"rule_id"`
	EventID   int64     `json:"event_id"`
	Since     time.Time `json:"since"`
	MetricVal float64   `json:"metric_val"`
}

// RunAlertDetection checks all enabled rules against current metrics and triggers/resolves alerts.
func RunAlertDetection(ctx context.Context) error {
	// Skip during warm-up period
	if !IsWarmedUp() {
		return nil
	}

	rules, err := GetEnabledRules(ctx)
	if err != nil {
		return gerror.Wrapf(err, "load enabled rules")
	}

	for _, rule := range rules {
		ruleID := gconv.Int64(rule["id"])
		ruleName := gconv.String(rule["name"])
		metricType := gconv.String(rule["metric_type"])
		condition := gconv.String(rule["condition"])
		threshold := gconv.Float64(rule["threshold"])
		durationSeconds := gconv.Int(rule["duration_seconds"])
		level := gconv.String(rule["level"])
		cooldownSeconds := gconv.Int(rule["cooldown_seconds"])

		// Check cooldown
		if lastTriggered := gconv.Time(rule["last_triggered_at"]); !lastTriggered.IsZero() {
			if time.Since(lastTriggered) < time.Duration(cooldownSeconds)*time.Second {
				continue
			}
		}

		// Get current metric value
		currentValue, err := getMetricValue(ctx, metricType)
		if err != nil {
			g.Log().Warningf(ctx, "alert engine: get metric %s: %v", metricType, err)
			continue
		}

		// Evaluate condition
		violating := evaluateCondition(condition, currentValue, threshold)

		// Check firing state in Redis
		redisKey := fmt.Sprintf("ops:alert:firing:%d", ruleID)
		firingStateJSON, err := g.Redis().Do(ctx, "GET", redisKey)
		var firingState *alertFiringState
		if err == nil && firingStateJSON != nil {
			var fs alertFiringState
			if json.Unmarshal([]byte(firingStateJSON.String()), &fs) == nil {
				firingState = &fs
			}
		}

		if violating {
			if firingState == nil {
				// New violation starts - record start time
				fs := alertFiringState{
					RuleID:    ruleID,
					Since:     time.Now(),
					MetricVal: currentValue,
				}
				data, _ := json.Marshal(fs)
				g.Redis().Do(ctx, "SET", redisKey, string(data), "EX", 3600) // 1h TTL
			} else {
				// Already violating - check if duration threshold is met and no event yet
				elapsed := time.Since(firingState.Since)
				if firingState.EventID == 0 && int(elapsed.Seconds()) >= durationSeconds {
					// Duration met - create alert event
					eventID, err := createAlertEvent(ctx, ruleID, ruleName, metricType, level, currentValue, threshold)
					if err != nil {
						g.Log().Errorf(ctx, "alert engine: create event for rule %d: %v", ruleID, err)
						continue
					}

					// Update firing state with event ID
					firingState.EventID = eventID
					firingState.MetricVal = currentValue
					data, _ := json.Marshal(firingState)
					g.Redis().Do(ctx, "SET", redisKey, string(data), "EX", 3600)

					// Update last_triggered_at
					dao.OpsAlertRules.Ctx(ctx).
						Where("id", ruleID).
						Data(do.OpsAlertRules{
							LastTriggeredAt: gtime.Now(),
						}).
						Update()

					// Dispatch notifications
					go dispatchAlertNotifications(gctx.New(), rule, eventID, currentValue, threshold)
				}
			}
		} else {
			// Not violating
			if firingState != nil && firingState.EventID > 0 {
				// Auto-resolve the event
				err := autoResolveEvent(ctx, firingState.EventID)
				if err != nil {
					g.Log().Warningf(ctx, "alert engine: auto-resolve event %d: %v", firingState.EventID, err)
				}
			}
			// Clear firing state
			g.Redis().Do(ctx, "DEL", redisKey)
		}
	}

	return nil
}

// evaluateCondition checks if value meets the condition against threshold.
func evaluateCondition(condition string, value, threshold float64) bool {
	switch condition {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	case "eq":
		return value == threshold
	default:
		return false
	}
}

// getMetricValue returns the current value for a given metric type.
func getMetricValue(ctx context.Context, metricType string) (float64, error) {
	switch metricType {
	case "system.cpu_percent":
		return GetCPUPercent(), nil
	case "system.memory_percent":
		return GetMemoryPercent(), nil
	case "system.disk_percent":
		return GetDiskPercent(), nil
	case "api.error_rate":
		return GetErrorRate(ctx)
	case "api.p99_latency":
		return GetP99Latency(ctx)
	case "api.qps":
		return GetQPS(ctx)
	case "db.active_connections":
		return GetDBActiveConnections(ctx)
	case "redis.used_memory_mb":
		return GetRedisUsedMemoryMB(ctx)
	default:
		return 0, gerror.Newf("unsupported metric type: %s", metricType)
	}
}

// createAlertEvent creates a new alert event in the database.
func createAlertEvent(ctx context.Context, ruleID int64, ruleName, metricType, level string, triggerValue, threshold float64) (int64, error) {
	message := fmt.Sprintf("告警规则「%s」触发：指标 %s 当前值 %.2f 超过阈值 %.2f", ruleName, metricType, triggerValue, threshold)

	id, err := dao.OpsAlertEvents.Ctx(ctx).InsertAndGetId(do.OpsAlertEvents{
		RuleId:         ruleID,
		RuleName:       ruleName,
		MetricType:     metricType,
		Level:          level,
		Status:         "firing",
		TriggerValue:   triggerValue,
		ThresholdValue: threshold,
		TriggerMessage: message,
	})
	return id, err
}

// autoResolveEvent resolves an alert event automatically when the metric recovers.
func autoResolveEvent(ctx context.Context, eventID int64) error {
	result, err := dao.OpsAlertEvents.Ctx(ctx).
		Where("id", eventID).
		Where("status", "firing").
		Data(do.OpsAlertEvents{
			Status:       "resolved",
			ResolveNotes: "指标恢复正常，系统自动解决",
			ResolvedAt:   gtime.Now(),
		}).Update()
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return lcommon.NewBusinessError(404, "事件不存在或已解决")
	}
	return nil
}

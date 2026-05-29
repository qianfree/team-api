package monitor

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/middleware"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/service"
)

type sMonitor struct{}

func NewMonitor() *sMonitor {
	return &sMonitor{}
}

func init() {
	service.RegisterMonitor(NewMonitor())
}

// ===================== Monitoring Dashboard =====================

func (s *sMonitor) Dashboard(ctx context.Context, req *v1.MonitorDashboardReq) (*v1.MonitorDashboardRes, error) {
	data, err := GetDashboardData(ctx, req.Minutes)
	if err != nil {
		return nil, err
	}
	return &v1.MonitorDashboardRes{Data: data}, nil
}

func (s *sMonitor) Traffic(ctx context.Context, req *v1.MonitorTrafficReq) (*v1.MonitorTrafficRes, error) {
	data, err := GetTrafficCurve(ctx, req.Minutes)
	if err != nil {
		return nil, err
	}
	return &v1.MonitorTrafficRes{Data: data}, nil
}

func (s *sMonitor) Latency(ctx context.Context, req *v1.MonitorLatencyReq) (*v1.MonitorLatencyRes, error) {
	data, err := GetLatencyHistogram(ctx, req.Minutes)
	if err != nil {
		return nil, err
	}
	return &v1.MonitorLatencyRes{Data: data}, nil
}

func (s *sMonitor) System(ctx context.Context, req *v1.MonitorSystemReq) (*v1.MonitorSystemRes, error) {
	result := make(g.Map)

	history, _ := GetSystemMetricsHistory(ctx, req.Minutes)
	if len(history) > 0 {
		result["history"] = history
		result["current"] = history[len(history)-1]
	}

	if req.Minutes > 60 {
		dbHistory, err := GetSystemMetricsFromDB(ctx, req.Minutes)
		if err == nil {
			result["db_history"] = dbHistory
		}
	}

	return &v1.MonitorSystemRes{Data: result}, nil
}

func (s *sMonitor) DBPool(ctx context.Context, _ *v1.MonitorDBPoolReq) (*v1.MonitorDBPoolRes, error) {
	data, err := GetDBPoolMetrics(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.MonitorDBPoolRes{
		ActiveConnections: data.ActiveConnections,
		IdleConnections:   data.IdleConnections,
		TotalConnections:  data.TotalConnections,
		MaxConnections:    data.MaxConnections,
		WaitingQueries:    data.WaitingQueries,
	}, nil
}

func (s *sMonitor) RedisPool(ctx context.Context, _ *v1.MonitorRedisPoolReq) (*v1.MonitorRedisPoolRes, error) {
	data, err := GetRedisPoolMetrics(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.MonitorRedisPoolRes{
		ConnectedClients:  data.ConnectedClients,
		UsedMemoryMB:      data.UsedMemoryMB,
		MaxMemoryMB:       data.MaxMemoryMB,
		UsedMemoryPercent: data.UsedMemoryPercent,
		TotalCommands:     data.TotalCommands,
		InstantaneousOps:  int64(data.InstantaneousOps),
		KeyspaceHits:      data.KeyspaceHits,
		KeyspaceMisses:    data.KeyspaceMisses,
		HitRate:           data.HitRate,
	}, nil
}

func (s *sMonitor) Realtime(ctx context.Context, _ *v1.MonitorRealtimeReq) (*v1.MonitorRealtimeRes, error) {
	return &v1.MonitorRealtimeRes{Data: GetRealtimeData()}, nil
}

// ===================== Alert Rules =====================

func (s *sMonitor) AlertRuleList(ctx context.Context, req *v1.AlertRuleListReq) (*v1.AlertRuleListRes, error) {
	var enabled *bool
	if req.Enabled == "true" {
		val := true
		enabled = &val
	} else if req.Enabled == "false" {
		val := false
		enabled = &val
	}

	data, err := ListAlertRules(ctx, req.Page, req.PageSize, req.MetricType, req.Level, enabled)
	if err != nil {
		return nil, err
	}

	list, _ := data["list"].([]map[string]any)
	total, _ := data["total"].(int)
	return &v1.AlertRuleListRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (s *sMonitor) AlertOptions(ctx context.Context, _ *v1.AlertOptionsReq) (*v1.AlertOptionsRes, error) {
	adminUsers, err := GetAdminUsers(ctx)
	if err != nil {
		adminUsers = []map[string]any{}
	}

	return &v1.AlertOptionsRes{Data: g.Map{
		"metric_types": GetMetricTypeOptions(),
		"conditions":   GetConditionOptions(),
		"levels":       GetLevelOptions(),
		"admin_users":  adminUsers,
	}}, nil
}

func (s *sMonitor) CreateAlertRule(ctx context.Context, req *v1.AlertRuleCreateReq) (*v1.AlertRuleCreateRes, error) {
	id, err := CreateAlertRule(ctx, do.OpsAlertRules{
		Name:                req.Name,
		MetricType:          req.MetricType,
		Condition:           req.Condition,
		Threshold:           req.Threshold,
		DurationSeconds:     req.DurationSeconds,
		NotificationMethods: req.NotificationMethods,
		WebhookUrl:          req.WebhookURL,
		Level:               req.Level,
		CooldownSeconds:     req.CooldownSeconds,
		NotifyUserIds:       req.NotifyUserIDs,
	})
	if err != nil {
		return nil, err
	}
	return &v1.AlertRuleCreateRes{ID: id}, nil
}

func (s *sMonitor) UpdateAlertRule(ctx context.Context, req *v1.AlertRuleUpdateReq) (*v1.AlertRuleUpdateRes, error) {
	updates := do.OpsAlertRules{}
	if req.Name != "" {
		updates.Name = req.Name
	}
	if req.MetricType != "" {
		updates.MetricType = req.MetricType
	}
	if req.Condition != "" {
		updates.Condition = req.Condition
	}
	if req.Threshold != nil {
		updates.Threshold = *req.Threshold
	}
	if req.DurationSeconds != nil {
		updates.DurationSeconds = *req.DurationSeconds
	}
	if len(req.NotificationMethods) > 0 {
		updates.NotificationMethods = req.NotificationMethods
	}
	if req.WebhookURL != "" {
		updates.WebhookUrl = req.WebhookURL
	}
	if req.Level != "" {
		updates.Level = req.Level
	}
	if req.CooldownSeconds != nil {
		updates.CooldownSeconds = *req.CooldownSeconds
	}
	if req.NotifyUserIDs != nil {
		updates.NotifyUserIds = req.NotifyUserIDs
	}

	if err := UpdateAlertRule(ctx, req.Id, updates); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *sMonitor) DeleteAlertRule(ctx context.Context, req *v1.AlertRuleDeleteReq) (*v1.AlertRuleDeleteRes, error) {
	if err := DeleteAlertRule(ctx, req.Id); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *sMonitor) ToggleAlertRule(ctx context.Context, req *v1.AlertRuleToggleReq) (*v1.AlertRuleToggleRes, error) {
	if err := ToggleAlertRule(ctx, req.Id); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *sMonitor) AlertTest(ctx context.Context, req *v1.AlertTestReq) (*v1.AlertTestRes, error) {
	if err := SendTestAlert(ctx, req.Id); err != nil {
		return nil, err
	}
	return &v1.AlertTestRes{Message: "测试通知已发送"}, nil
}

// ===================== Alert Events =====================

func (s *sMonitor) AlertEventList(ctx context.Context, req *v1.AlertEventListReq) (*v1.AlertEventListRes, error) {
	data, err := ListAlertEvents(ctx, req.Page, req.PageSize, req.Status, req.Level, req.RuleID)
	if err != nil {
		return nil, err
	}

	list, _ := data["list"].([]map[string]any)
	total, _ := data["total"].(int)
	return &v1.AlertEventListRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (s *sMonitor) AcknowledgeAlert(ctx context.Context, req *v1.AlertEventAcknowledgeReq) (*v1.AlertEventAcknowledgeRes, error) {
	adminID := middleware.GetUserID(ctx)
	if err := AcknowledgeAlert(ctx, req.Id, adminID); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *sMonitor) ResolveAlert(ctx context.Context, req *v1.AlertEventResolveReq) (*v1.AlertEventResolveRes, error) {
	adminID := middleware.GetUserID(ctx)
	if err := ResolveAlert(ctx, req.Id, adminID, req.Notes); err != nil {
		return nil, err
	}
	return nil, nil
}

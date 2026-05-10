// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
)

type (
	IMonitor interface {
		Dashboard(ctx context.Context, req *v1.MonitorDashboardReq) (*v1.MonitorDashboardRes, error)
		Traffic(ctx context.Context, req *v1.MonitorTrafficReq) (*v1.MonitorTrafficRes, error)
		Latency(ctx context.Context, req *v1.MonitorLatencyReq) (*v1.MonitorLatencyRes, error)
		System(ctx context.Context, req *v1.MonitorSystemReq) (*v1.MonitorSystemRes, error)
		DBPool(ctx context.Context, _ *v1.MonitorDBPoolReq) (*v1.MonitorDBPoolRes, error)
		RedisPool(ctx context.Context, _ *v1.MonitorRedisPoolReq) (*v1.MonitorRedisPoolRes, error)
		Realtime(ctx context.Context, _ *v1.MonitorRealtimeReq) (*v1.MonitorRealtimeRes, error)
		AlertRuleList(ctx context.Context, req *v1.AlertRuleListReq) (*v1.AlertRuleListRes, error)
		AlertOptions(ctx context.Context, _ *v1.AlertOptionsReq) (*v1.AlertOptionsRes, error)
		CreateAlertRule(ctx context.Context, req *v1.AlertRuleCreateReq) (*v1.AlertRuleCreateRes, error)
		UpdateAlertRule(ctx context.Context, req *v1.AlertRuleUpdateReq) (*v1.AlertRuleUpdateRes, error)
		DeleteAlertRule(ctx context.Context, req *v1.AlertRuleDeleteReq) (*v1.AlertRuleDeleteRes, error)
		ToggleAlertRule(ctx context.Context, req *v1.AlertRuleToggleReq) (*v1.AlertRuleToggleRes, error)
		AlertTest(ctx context.Context, req *v1.AlertTestReq) (*v1.AlertTestRes, error)
		AlertEventList(ctx context.Context, req *v1.AlertEventListReq) (*v1.AlertEventListRes, error)
		AcknowledgeAlert(ctx context.Context, req *v1.AlertEventAcknowledgeReq) (*v1.AlertEventAcknowledgeRes, error)
		ResolveAlert(ctx context.Context, req *v1.AlertEventResolveReq) (*v1.AlertEventResolveRes, error)
	}
)

var (
	localMonitor IMonitor
)

func Monitor() IMonitor {
	if localMonitor == nil {
		panic("implement not found for interface IMonitor, forgot register?")
	}
	return localMonitor
}

func RegisterMonitor(i IMonitor) {
	localMonitor = i
}

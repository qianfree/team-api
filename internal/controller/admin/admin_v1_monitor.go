package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) MonitorDashboard(ctx context.Context, req *v1.MonitorDashboardReq) (res *v1.MonitorDashboardRes, err error) {
	return service.Monitor().Dashboard(ctx, req)
}
func (c *ControllerV1) MonitorTraffic(ctx context.Context, req *v1.MonitorTrafficReq) (res *v1.MonitorTrafficRes, err error) {
	return service.Monitor().Traffic(ctx, req)
}
func (c *ControllerV1) MonitorLatency(ctx context.Context, req *v1.MonitorLatencyReq) (res *v1.MonitorLatencyRes, err error) {
	return service.Monitor().Latency(ctx, req)
}
func (c *ControllerV1) MonitorSystem(ctx context.Context, req *v1.MonitorSystemReq) (res *v1.MonitorSystemRes, err error) {
	return service.Monitor().System(ctx, req)
}
func (c *ControllerV1) MonitorDBPool(ctx context.Context, req *v1.MonitorDBPoolReq) (res *v1.MonitorDBPoolRes, err error) {
	return service.Monitor().DBPool(ctx, req)
}
func (c *ControllerV1) MonitorRedisPool(ctx context.Context, req *v1.MonitorRedisPoolReq) (res *v1.MonitorRedisPoolRes, err error) {
	return service.Monitor().RedisPool(ctx, req)
}
func (c *ControllerV1) MonitorRealtime(ctx context.Context, req *v1.MonitorRealtimeReq) (res *v1.MonitorRealtimeRes, err error) {
	return service.Monitor().Realtime(ctx, req)
}
func (c *ControllerV1) AlertRuleList(ctx context.Context, req *v1.AlertRuleListReq) (res *v1.AlertRuleListRes, err error) {
	return service.Monitor().AlertRuleList(ctx, req)
}
func (c *ControllerV1) AlertOptions(ctx context.Context, req *v1.AlertOptionsReq) (res *v1.AlertOptionsRes, err error) {
	return service.Monitor().AlertOptions(ctx, req)
}
func (c *ControllerV1) AlertRuleCreate(ctx context.Context, req *v1.AlertRuleCreateReq) (res *v1.AlertRuleCreateRes, err error) {
	return service.Monitor().CreateAlertRule(ctx, req)
}
func (c *ControllerV1) AlertRuleUpdate(ctx context.Context, req *v1.AlertRuleUpdateReq) (res *v1.AlertRuleUpdateRes, err error) {
	return service.Monitor().UpdateAlertRule(ctx, req)
}
func (c *ControllerV1) AlertRuleDelete(ctx context.Context, req *v1.AlertRuleDeleteReq) (res *v1.AlertRuleDeleteRes, err error) {
	return service.Monitor().DeleteAlertRule(ctx, req)
}
func (c *ControllerV1) AlertRuleToggle(ctx context.Context, req *v1.AlertRuleToggleReq) (res *v1.AlertRuleToggleRes, err error) {
	return service.Monitor().ToggleAlertRule(ctx, req)
}
func (c *ControllerV1) AlertTest(ctx context.Context, req *v1.AlertTestReq) (res *v1.AlertTestRes, err error) {
	return service.Monitor().AlertTest(ctx, req)
}
func (c *ControllerV1) AlertEventList(ctx context.Context, req *v1.AlertEventListReq) (res *v1.AlertEventListRes, err error) {
	return service.Monitor().AlertEventList(ctx, req)
}
func (c *ControllerV1) AlertEventAcknowledge(ctx context.Context, req *v1.AlertEventAcknowledgeReq) (res *v1.AlertEventAcknowledgeRes, err error) {
	return service.Monitor().AcknowledgeAlert(ctx, req)
}
func (c *ControllerV1) AlertEventResolve(ctx context.Context, req *v1.AlertEventResolveReq) (res *v1.AlertEventResolveRes, err error) {
	return service.Monitor().ResolveAlert(ctx, req)
}

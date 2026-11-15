package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AiAnalysisCreate(ctx context.Context, req *v1.AiAnalysisCreateReq) (res *v1.AiAnalysisCreateRes, err error) {
	return service.Admin().AiAnalysisCreate(ctx, req)
}
func (c *ControllerV1) AiAnalysisReportList(ctx context.Context, req *v1.AiAnalysisReportListReq) (res *v1.AiAnalysisReportListRes, err error) {
	return service.Admin().AiAnalysisReportList(ctx, req)
}
func (c *ControllerV1) AiAnalysisReportDetail(ctx context.Context, req *v1.AiAnalysisReportDetailReq) (res *v1.AiAnalysisReportDetailRes, err error) {
	return service.Admin().AiAnalysisReportDetail(ctx, req)
}
func (c *ControllerV1) AiAnalysisReportConversation(ctx context.Context, req *v1.AiAnalysisReportConversationReq) (res *v1.AiAnalysisReportConversationRes, err error) {
	return service.Admin().AiAnalysisReportConversation(ctx, req)
}
func (c *ControllerV1) AiAnalysisReportDelete(ctx context.Context, req *v1.AiAnalysisReportDeleteReq) (res *v1.AiAnalysisReportDeleteRes, err error) {
	return service.Admin().AiAnalysisReportDelete(ctx, req)
}
func (c *ControllerV1) AiAnalysisJobList(ctx context.Context, req *v1.AiAnalysisJobListReq) (res *v1.AiAnalysisJobListRes, err error) {
	return service.Admin().AiAnalysisJobList(ctx, req)
}
func (c *ControllerV1) AiAnalysisConfigGet(ctx context.Context, req *v1.AiAnalysisConfigGetReq) (res *v1.AiAnalysisConfigGetRes, err error) {
	return service.Admin().AiAnalysisConfigGet(ctx, req)
}
func (c *ControllerV1) AiAnalysisConfigUpdate(ctx context.Context, req *v1.AiAnalysisConfigUpdateReq) (res *v1.AiAnalysisConfigUpdateRes, err error) {
	return service.Admin().AiAnalysisConfigUpdate(ctx, req)
}
func (c *ControllerV1) AiAnalysisTest(ctx context.Context, req *v1.AiAnalysisTestReq) (res *v1.AiAnalysisTestRes, err error) {
	return service.Admin().AiAnalysisTest(ctx, req)
}

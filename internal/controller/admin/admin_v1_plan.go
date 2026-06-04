package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PlanList(ctx context.Context, req *v1.PlanListReq) (res *v1.PlanListRes, err error) {
	return service.Admin().ListPlans(ctx, req)
}
func (c *ControllerV1) PlanCreate(ctx context.Context, req *v1.PlanCreateReq) (res *v1.PlanCreateRes, err error) {
	return service.Admin().CreatePlan(ctx, req)
}
func (c *ControllerV1) PlanDetail(ctx context.Context, req *v1.PlanDetailReq) (res *v1.PlanDetailRes, err error) {
	return service.Admin().GetPlan(ctx, req)
}
func (c *ControllerV1) PlanUpdate(ctx context.Context, req *v1.PlanUpdateReq) (res *v1.PlanUpdateRes, err error) {
	return service.Admin().UpdatePlan(ctx, req)
}
func (c *ControllerV1) PlanArchive(ctx context.Context, req *v1.PlanArchiveReq) (res *v1.PlanArchiveRes, err error) {
	return service.Admin().ArchivePlan(ctx, req)
}
func (c *ControllerV1) PlanToggleRecommend(ctx context.Context, req *v1.PlanToggleRecommendReq) (res *v1.PlanToggleRecommendRes, err error) {
	return service.Admin().ToggleRecommend(ctx, req)
}
func (c *ControllerV1) PlanExport(ctx context.Context, req *v1.PlanExportReq) (res *v1.PlanExportRes, err error) {
	return service.Admin().ExportPlans(ctx, req)
}

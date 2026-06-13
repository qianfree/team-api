package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AgreementCreate(ctx context.Context, req *v1.AgreementCreateReq) (res *v1.AgreementCreateRes, err error) {
	return service.Admin().CreateAgreement(ctx, req)
}
func (c *ControllerV1) AgreementList(ctx context.Context, req *v1.AgreementListReq) (res *v1.AgreementListRes, err error) {
	return service.Admin().ListAgreements(ctx, req)
}
func (c *ControllerV1) AgreementGet(ctx context.Context, req *v1.AgreementGetReq) (res *v1.AgreementGetRes, err error) {
	return service.Admin().GetAgreement(ctx, req)
}
func (c *ControllerV1) AgreementUpdate(ctx context.Context, req *v1.AgreementUpdateReq) (res *v1.AgreementUpdateRes, err error) {
	return service.Admin().UpdateAgreement(ctx, req)
}
func (c *ControllerV1) AgreementDelete(ctx context.Context, req *v1.AgreementDeleteReq) (res *v1.AgreementDeleteRes, err error) {
	return service.Admin().DeleteAgreement(ctx, req)
}
func (c *ControllerV1) AgreementPublish(ctx context.Context, req *v1.AgreementPublishReq) (res *v1.AgreementPublishRes, err error) {
	return service.Admin().PublishAgreement(ctx, req)
}
func (c *ControllerV1) AgreementAcceptanceList(ctx context.Context, req *v1.AgreementAcceptanceListReq) (res *v1.AgreementAcceptanceListRes, err error) {
	return service.Admin().ListAgreementAcceptances(ctx, req)
}
func (c *ControllerV1) AdminAgreementPending(ctx context.Context, req *v1.AdminAgreementPendingReq) (res *v1.AdminAgreementPendingRes, err error) {
	return service.Admin().ListAdminPendingAgreements(ctx, req)
}
func (c *ControllerV1) AdminAgreementAccept(ctx context.Context, req *v1.AdminAgreementAcceptReq) (res *v1.AdminAgreementAcceptRes, err error) {
	return service.Admin().AcceptAdminAgreements(ctx, req)
}

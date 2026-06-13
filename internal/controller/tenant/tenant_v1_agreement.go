package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantAgreementPending(ctx context.Context, req *v1.TenantAgreementPendingReq) (res *v1.TenantAgreementPendingRes, err error) {
	return service.Tenant().ListTenantPendingAgreements(ctx, req)
}
func (c *ControllerV1) TenantAgreementAccept(ctx context.Context, req *v1.TenantAgreementAcceptReq) (res *v1.TenantAgreementAcceptRes, err error) {
	return service.Tenant().AcceptTenantAgreements(ctx, req)
}
func (c *ControllerV1) AgreementCurrentList(ctx context.Context, req *v1.AgreementCurrentListReq) (res *v1.AgreementCurrentListRes, err error) {
	return service.Tenant().ListCurrentAgreements(ctx, req)
}
func (c *ControllerV1) AgreementCurrentGet(ctx context.Context, req *v1.AgreementCurrentGetReq) (res *v1.AgreementCurrentGetRes, err error) {
	return service.Tenant().GetCurrentAgreementByCode(ctx, req)
}

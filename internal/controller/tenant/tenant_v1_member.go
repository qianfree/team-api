package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberList(ctx context.Context, req *v1.TenantMemberListReq) (res *v1.TenantMemberListRes, err error) {
	return service.Tenant().ListMembers(ctx, req)
}
func (c *ControllerV1) TenantMemberInvite(ctx context.Context, req *v1.TenantMemberInviteReq) (res *v1.TenantMemberInviteRes, err error) {
	return service.Tenant().InviteMember(ctx, req)
}
func (c *ControllerV1) TenantInvitationList(ctx context.Context, req *v1.TenantInvitationListReq) (res *v1.TenantInvitationListRes, err error) {
	return service.Tenant().InvitationList(ctx, req)
}
func (c *ControllerV1) TenantInvitationRevoke(ctx context.Context, req *v1.TenantInvitationRevokeReq) (res *v1.TenantInvitationRevokeRes, err error) {
	return service.Tenant().RevokeInvitation(ctx, req)
}
func (c *ControllerV1) TenantInviteInfo(ctx context.Context, req *v1.TenantInviteInfoReq) (res *v1.TenantInviteInfoRes, err error) {
	return service.Tenant().InviteInfo(ctx, req)
}
func (c *ControllerV1) TenantMemberJoin(ctx context.Context, req *v1.TenantMemberJoinReq) (res *v1.TenantMemberJoinRes, err error) {
	return service.Tenant().JoinByInvite(ctx, req)
}
func (c *ControllerV1) TenantMemberCreate(ctx context.Context, req *v1.TenantMemberCreateReq) (res *v1.TenantMemberCreateRes, err error) {
	return service.Tenant().CreateMember(ctx, req)
}
func (c *ControllerV1) TenantMemberResetPassword(ctx context.Context, req *v1.TenantMemberResetPasswordReq) (res *v1.TenantMemberResetPasswordRes, err error) {
	return service.Tenant().ResetMemberPassword(ctx, req)
}
func (c *ControllerV1) TenantMemberRemove(ctx context.Context, req *v1.TenantMemberRemoveReq) (res *v1.TenantMemberRemoveRes, err error) {
	return service.Tenant().RemoveMember(ctx, req)
}
func (c *ControllerV1) TenantMemberUpdateRole(ctx context.Context, req *v1.TenantMemberUpdateRoleReq) (res *v1.TenantMemberUpdateRoleRes, err error) {
	return service.Tenant().UpdateMemberRole(ctx, req)
}
func (c *ControllerV1) TenantMemberGet(ctx context.Context, req *v1.TenantMemberGetReq) (res *v1.TenantMemberGetRes, err error) {
	return service.Tenant().GetMember(ctx, req)
}
func (c *ControllerV1) TenantMemberUsage(ctx context.Context, req *v1.TenantMemberUsageReq) (res *v1.TenantMemberUsageRes, err error) {
	return service.Tenant().GetMemberUsage(ctx, req)
}
func (c *ControllerV1) TenantMemberApiKeys(ctx context.Context, req *v1.TenantMemberApiKeysReq) (res *v1.TenantMemberApiKeysRes, err error) {
	return service.Tenant().ListMemberApiKeys(ctx, req)
}
func (c *ControllerV1) TenantMemberExport(ctx context.Context, req *v1.TenantMemberExportReq) (res *v1.TenantMemberExportRes, err error) {
	return service.Tenant().ExportMembers(ctx, req)
}

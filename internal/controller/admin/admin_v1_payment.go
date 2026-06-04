package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminMemberCreate(ctx context.Context, req *v1.AdminMemberCreateReq) (res *v1.AdminMemberCreateRes, err error) {
	return service.Admin().CreateMember(ctx, req)
}
func (c *ControllerV1) AdminMemberDisable(ctx context.Context, req *v1.AdminMemberDisableReq) (res *v1.AdminMemberDisableRes, err error) {
	return service.Admin().DisableMember(ctx, req)
}
func (c *ControllerV1) AdminMemberEnable(ctx context.Context, req *v1.AdminMemberEnableReq) (res *v1.AdminMemberEnableRes, err error) {
	return service.Admin().EnableMember(ctx, req)
}
func (c *ControllerV1) AdminMemberResetPassword(ctx context.Context, req *v1.AdminMemberResetPasswordReq) (res *v1.AdminMemberResetPasswordRes, err error) {
	return service.Admin().ResetMemberPassword(ctx, req)
}
func (c *ControllerV1) PaymentChannelList(ctx context.Context, req *v1.PaymentChannelListReq) (res *v1.PaymentChannelListRes, err error) {
	return service.Admin().GetPaymentChannels(ctx, req)
}
func (c *ControllerV1) PaymentChannelSave(ctx context.Context, req *v1.PaymentChannelSaveReq) (res *v1.PaymentChannelSaveRes, err error) {
	return service.Admin().SavePaymentChannel(ctx, req)
}
func (c *ControllerV1) PaymentSettingsGet(ctx context.Context, req *v1.PaymentSettingsGetReq) (res *v1.PaymentSettingsGetRes, err error) {
	return service.Admin().GetPaymentSettings(ctx, req)
}
func (c *ControllerV1) PaymentSettingsUpdate(ctx context.Context, req *v1.PaymentSettingsUpdateReq) (res *v1.PaymentSettingsUpdateRes, err error) {
	return service.Admin().UpdatePaymentSettings(ctx, req)
}

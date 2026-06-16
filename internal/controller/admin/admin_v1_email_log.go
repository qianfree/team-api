package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) EmailSendLogList(ctx context.Context, req *v1.EmailSendLogListReq) (res *v1.EmailSendLogListRes, err error) {
	return service.Admin().ListEmailSendLogs(ctx, req)
}
func (c *ControllerV1) EmailSendLogDetail(ctx context.Context, req *v1.EmailSendLogDetailReq) (res *v1.EmailSendLogDetailRes, err error) {
	return service.Admin().GetEmailSendLogDetail(ctx, req)
}
func (c *ControllerV1) EmailVerifyCodeList(ctx context.Context, req *v1.EmailVerifyCodeListReq) (res *v1.EmailVerifyCodeListRes, err error) {
	return service.Admin().ListEmailVerifyCodes(ctx, req)
}

// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package captcha

import (
	"context"

	"github.com/qianfree/team-api/api/captcha/v1"
)

type ICaptchaV1 interface {
	CaptchaGenerate(ctx context.Context, req *v1.CaptchaGenerateReq) (res *v1.CaptchaGenerateRes, err error)
	CaptchaVerify(ctx context.Context, req *v1.CaptchaVerifyReq) (res *v1.CaptchaVerifyRes, err error)
}

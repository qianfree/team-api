package captcha

import (
	"context"

	"github.com/qianfree/team-api/api/captcha/v1"
	"github.com/qianfree/team-api/internal/logic/common"
)

func (c *ControllerV1) CaptchaGenerate(ctx context.Context, req *v1.CaptchaGenerateReq) (res *v1.CaptchaGenerateRes, err error) {
	result, err := common.GenerateCaptcha(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.CaptchaGenerateRes{
		CaptchaKey:  result.CaptchaKey,
		MasterImage: result.MasterImage,
		TileImage:   result.TileImage,
		TileY:       result.TileY,
	}, nil
}

package docs

import (
	"context"

	"github.com/gogf/gf/v2/net/ghttp"

	v1 "github.com/qianfree/team-api/api/docs/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenAPISpec(ctx context.Context, req *v1.OpenAPISpecReq) (res *v1.OpenAPISpecRes, err error) {
	data, err := service.Docs().OpenAPISpec(ctx, req)
	if err != nil {
		return nil, err
	}
	r := ghttp.RequestFromCtx(ctx)
	r.Response.Header().Set("Content-Type", "application/json")
	r.Response.Write(data)
	return nil, nil
}

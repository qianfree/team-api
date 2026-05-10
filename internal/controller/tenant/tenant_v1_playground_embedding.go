package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PlaygroundEmbedding(ctx context.Context, req *v1.PlaygroundEmbeddingReq) (res *v1.PlaygroundEmbeddingRes, err error) {
	return service.Tenant().PlaygroundEmbedding(ctx, req)
}

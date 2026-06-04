package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PlaygroundChat(ctx context.Context, req *v1.PlaygroundChatReq) (res *v1.PlaygroundChatRes, err error) {
	return service.Tenant().PlaygroundChat(ctx, req)
}
func (c *ControllerV1) PlaygroundImage(ctx context.Context, req *v1.PlaygroundImageReq) (res *v1.PlaygroundImageRes, err error) {
	return service.Tenant().PlaygroundImage(ctx, req)
}
func (c *ControllerV1) PlaygroundAudioTTS(ctx context.Context, req *v1.PlaygroundAudioTTSReq) (res *v1.PlaygroundAudioTTSRes, err error) {
	return service.Tenant().PlaygroundAudioTTS(ctx, req)
}
func (c *ControllerV1) PlaygroundEmbedding(ctx context.Context, req *v1.PlaygroundEmbeddingReq) (res *v1.PlaygroundEmbeddingRes, err error) {
	return service.Tenant().PlaygroundEmbedding(ctx, req)
}
func (c *ControllerV1) PlaygroundRerank(ctx context.Context, req *v1.PlaygroundRerankReq) (res *v1.PlaygroundRerankRes, err error) {
	return service.Tenant().PlaygroundRerank(ctx, req)
}
func (c *ControllerV1) SandboxChat(ctx context.Context, req *v1.SandboxChatReq) (res *v1.SandboxChatRes, err error) {
	return service.Tenant().SandboxChat(ctx, req)
}
func (c *ControllerV1) SandboxQuota(ctx context.Context, req *v1.SandboxQuotaReq) (res *v1.SandboxQuotaRes, err error) {
	return service.Tenant().SandboxQuota(ctx, req)
}

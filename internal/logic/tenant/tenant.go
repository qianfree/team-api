package tenant

import (
	"context"

	"github.com/qianfree/team-api/internal/service"
)

// sTenant is the service implementation for tenant business logic.
type sTenant struct{}

// New creates and returns a new service instance.
func New() *sTenant {
	return &sTenant{}
}

func init() {
	service.RegisterTenant(New())
}

// ctxUserID reads userId from context.
func ctxUserID(ctx context.Context) int64 {
	v := ctx.Value("userId")
	if v == nil {
		return 0
	}
	if id, ok := v.(int64); ok {
		return id
	}
	return 0
}

// ctxTenantID reads tenantId from context.
func ctxTenantID(ctx context.Context) int64 {
	v := ctx.Value("tenantId")
	if v == nil {
		return 0
	}
	if id, ok := v.(int64); ok {
		return id
	}
	return 0
}

// ctxSessionID reads sessionId from context.
func ctxSessionID(ctx context.Context) int64 {
	v := ctx.Value("sessionId")
	if v == nil {
		return 0
	}
	if id, ok := v.(int64); ok {
		return id
	}
	return 0
}

// ctxUserRole reads role from context.
func ctxUserRole(ctx context.Context) string {
	v := ctx.Value("role")
	if v == nil {
		return ""
	}
	if role, ok := v.(string); ok {
		return role
	}
	return ""
}

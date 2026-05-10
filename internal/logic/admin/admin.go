package admin

import (
	"context"

	"github.com/qianfree/team-api/internal/service"
)

// sAdmin is the service implementation for admin business logic.
type sAdmin struct{}

// New creates and returns a new service instance.
func New() *sAdmin {
	return &sAdmin{}
}

func init() {
	service.RegisterAdmin(New())
}

// getCtxUserID extracts the admin user ID from context.
// The key "userId" is set by admin_auth middleware.
func getCtxUserID(ctx context.Context) int64 {
	val := ctx.Value("userId")
	if val == nil {
		return 0
	}
	if id, ok := val.(int64); ok {
		return id
	}
	return 0
}

package tenant

import (
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

package admin

import (
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

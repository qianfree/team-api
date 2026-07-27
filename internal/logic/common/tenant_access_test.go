package common

import (
	"errors"
	"testing"

	"github.com/qianfree/team-api/internal/consts"
)

func TestValidateTenantPrincipal(t *testing.T) {
	tests := []struct {
		name      string
		principal *TenantPrincipal
		wantErr   error
	}{
		{name: "missing principal", wantErr: consts.ErrUnauthorized},
		{name: "disabled user", principal: &TenantPrincipal{UserStatus: "disabled", TenantStatus: "active"}, wantErr: consts.ErrUnauthorized},
		{name: "suspended tenant", principal: &TenantPrincipal{UserStatus: "active", TenantStatus: "suspended"}, wantErr: consts.ErrTenantSuspended},
		{name: "closed tenant", principal: &TenantPrincipal{UserStatus: "active", TenantStatus: "closed"}, wantErr: consts.ErrTenantSuspended},
		{name: "active principal", principal: &TenantPrincipal{UserStatus: "active", TenantStatus: "active"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTenantPrincipal(tt.principal)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("ValidateTenantPrincipal() error = %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateTenantPrincipal() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

//go:build integration

package tenant_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestPlanList(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/plans", nil)
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			Identifier string `json:"identifier"`
		} `json:"list"`
	}
	resp.DecodeData(t, &data)
}

func TestCurrentPlan(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/plan/current", nil)
	// New tenant may not have a plan, accept either success or error
	resp.AssertSuccess(t)

	var data struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	resp.DecodeData(t, &data)
}

func TestCancelAutoRenew(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Put("/api/tenant/plan/cancel-auto-renew", nil)
	// New tenant likely has no active plan, so this may fail gracefully
	resp.AssertSuccess(t)
}

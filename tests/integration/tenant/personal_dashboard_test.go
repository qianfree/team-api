//go:build integration

package tenant_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestPersonalDashboardOverview(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/personal-dashboard", nil)
	resp.AssertSuccess(t)

	var data struct {
		Today     map[string]any `json:"today"`
		Month     map[string]any `json:"month"`
		ErrorRate map[string]any `json:"error_rate"`
	}
	resp.DecodeData(t, &data)
}

func TestPersonalDashboardTrends(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/personal-dashboard/trends", map[string]string{
		"days": "7",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []map[string]any `json:"list"`
	}
	resp.DecodeData(t, &data)
}

func TestPersonalDashboardModels(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/personal-dashboard/models", map[string]string{
		"days": "7",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []map[string]any `json:"list"`
	}
	resp.DecodeData(t, &data)
}

func TestPersonalDashboardApiKeyUsage(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/personal-dashboard/api-key-usage", map[string]string{
		"days": "7",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []map[string]any `json:"list"`
	}
	resp.DecodeData(t, &data)
}

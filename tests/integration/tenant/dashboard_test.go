//go:build integration

package tenant_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestDashboardOverview(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/dashboard", nil)
	resp.AssertSuccess(t)

	var data struct {
		Today       map[string]any `json:"today"`
		Month       map[string]any `json:"month"`
		Wallet      map[string]any `json:"wallet"`
		ActiveKeys  int            `json:"active_keys"`
		MemberCount int            `json:"member_count"`
	}
	resp.DecodeData(t, &data)
}

func TestDashboardTokenTrends(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/dashboard/token-trends", map[string]string{
		"days": "7",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []map[string]any `json:"list"`
	}
	resp.DecodeData(t, &data)
}

func TestDashboardModelDistribution(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/dashboard/model-distribution", map[string]string{
		"days": "7",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []map[string]any `json:"list"`
	}
	resp.DecodeData(t, &data)
}

func TestDashboardBalancePrediction(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/dashboard/balance-prediction", nil)
	resp.AssertSuccess(t)

	var data struct {
		DailyAvgCost     float64 `json:"daily_avg_cost"`
		AvailableBalance float64 `json:"available_balance"`
		WillExhaust      bool    `json:"will_exhaust"`
	}
	resp.DecodeData(t, &data)
}

func TestDashboardBudgetAlerts(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/dashboard/budget-alerts", nil)
	resp.AssertSuccess(t)

	var data struct {
		Members  []map[string]any `json:"members"`
		Projects []map[string]any `json:"projects"`
	}
	resp.DecodeData(t, &data)
}

func TestDashboardMemberUsageRanking(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/dashboard/member-usage-ranking", map[string]string{
		"days":  "7",
		"limit": "10",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []map[string]any `json:"list"`
	}
	resp.DecodeData(t, &data)
}

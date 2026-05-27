//go:build integration

package admin_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestDashboard(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/dashboard", nil)
	resp.AssertSuccess(t)

	var data struct {
		Tenants        int `json:"tenants"`
		Members        int `json:"members"`
		ActiveChannels int `json:"active_channels"`
		Today          struct {
			Requests      int     `json:"requests"`
			ActiveTenants int     `json:"active_tenants"`
			InputTokens   int     `json:"input_tokens"`
			OutputTokens  int     `json:"output_tokens"`
			TotalCost     float64 `json:"total_cost"`
			SuccessRate   float64 `json:"success_rate"`
		} `json:"today"`
		Yesterday struct {
			Requests      int     `json:"requests"`
			ActiveTenants int     `json:"active_tenants"`
			InputTokens   int     `json:"input_tokens"`
			OutputTokens  int     `json:"output_tokens"`
			TotalCost     float64 `json:"total_cost"`
			SuccessRate   float64 `json:"success_rate"`
		} `json:"yesterday"`
		Month struct {
			Requests     int     `json:"requests"`
			InputTokens  int     `json:"input_tokens"`
			OutputTokens int     `json:"output_tokens"`
			TotalCost    float64 `json:"total_cost"`
			Revenue      float64 `json:"revenue"`
		} `json:"month"`
	}
	resp.DecodeData(t, &data)

	if data.Tenants < 0 {
		t.Fatalf("expected tenants >= 0, got %d", data.Tenants)
	}
	if data.Members < 0 {
		t.Fatalf("expected members >= 0, got %d", data.Members)
	}
	if data.ActiveChannels < 0 {
		t.Fatalf("expected active_channels >= 0, got %d", data.ActiveChannels)
	}
	if data.Today.SuccessRate < 0 || data.Today.SuccessRate > 100 {
		t.Fatalf("expected today success_rate between 0 and 100, got %f", data.Today.SuccessRate)
	}
	if data.Month.Revenue < 0 {
		t.Fatalf("expected month revenue >= 0, got %f", data.Month.Revenue)
	}

	t.Logf("Dashboard: tenants=%d, members=%d, active_channels=%d",
		data.Tenants, data.Members, data.ActiveChannels)
}

func TestDashboardTrends(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/dashboard/trends", map[string]string{
		"days": "7",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []map[string]any `json:"list"`
	}
	resp.DecodeData(t, &data)

	t.Logf("Trends returned %d entries for 7 days", len(data.List))
}

func TestDashboardTopTenants(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/dashboard/top-tenants", map[string]string{
		"days": "30",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []map[string]any `json:"list"`
	}
	resp.DecodeData(t, &data)

	t.Logf("Top tenants returned %d entries", len(data.List))
}

func TestDashboardModelDistribution(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/dashboard/model-distribution", map[string]string{
		"days": "30",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []map[string]any `json:"list"`
	}
	resp.DecodeData(t, &data)

	t.Logf("Model distribution returned %d entries", len(data.List))
}

func TestDashboardChannelHealth(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/dashboard/channel-health", nil)
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			ChannelId   int64   `json:"channel_id"`
			ChannelName string  `json:"channel_name"`
			Status      string  `json:"status"`
			HealthScore float64 `json:"health_score"`
			SuccessRate float64 `json:"success_rate"`
			LatencyMs   int     `json:"latency_ms"`
		} `json:"list"`
	}
	resp.DecodeData(t, &data)

	for _, item := range data.List {
		if item.HealthScore < 0 || item.HealthScore > 100 {
			t.Errorf("channel %s: health_score %f out of range [0,100]",
				item.ChannelName, item.HealthScore)
		}
		if item.LatencyMs < 0 {
			t.Errorf("channel %s: latency_ms %d should be >= 0",
				item.ChannelName, item.LatencyMs)
		}
	}

	t.Logf("Channel health returned %d channels", len(data.List))
}

func TestDashboardRecentAlerts(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/dashboard/recent-alerts", nil)
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			Id             int64  `json:"id"`
			RuleName       string `json:"rule_name"`
			Level          string `json:"level"`
			Status         string `json:"status"`
			TriggerMessage string `json:"trigger_message"`
			CreatedAt      string `json:"created_at"`
		} `json:"list"`
	}
	resp.DecodeData(t, &data)

	for _, alert := range data.List {
		validLevels := map[string]bool{"info": true, "warning": true, "critical": true}
		if !validLevels[alert.Level] && alert.Level != "" {
			t.Errorf("alert %d: unexpected level %q", alert.Id, alert.Level)
		}
	}

	t.Logf("Recent alerts returned %d entries", len(data.List))
}

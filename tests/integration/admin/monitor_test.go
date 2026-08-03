//go:build integration

package admin_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestMonitorDashboard(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/monitor/dashboard", map[string]string{
		"minutes": "30",
	})
	resp.AssertSuccess(t)

	var data map[string]any
	resp.DecodeData(t, &data)

	t.Logf("Monitor dashboard returned %d keys", len(data))
}

func TestMonitorTraffic(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/monitor/traffic", map[string]string{
		"minutes": "60",
	})
	resp.AssertSuccess(t)

	t.Logf("Monitor traffic response received")
}

func TestMonitorTrafficFlow(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	for _, metric := range []string{"cost", "tokens", "requests"} {
		resp := client.Get("/api/admin/monitor/traffic-flow", map[string]string{
			"metric": metric,
		})
		resp.AssertSuccess(t)

		var data struct {
			Metric string `json:"metric"`
			Nodes  []any  `json:"nodes"`
			Links  []any  `json:"links"`
		}
		resp.DecodeData(t, &data)
		if data.Metric != metric {
			t.Fatalf("expected metric=%s, got %s", metric, data.Metric)
		}
		t.Logf("Traffic flow metric=%s nodes=%d links=%d", data.Metric, len(data.Nodes), len(data.Links))
	}
}

func TestMonitorModelPerformance(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/monitor/model-performance", map[string]string{
		"start_date": "2026-01-01",
		"end_date":   "2026-12-31",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			ModelName    string  `json:"model_name"`
			RequestCount int64   `json:"request_count"`
			SuccessRate  float64 `json:"success_rate"`
			Grade        string  `json:"grade"`
			AvgLatencyMs float64 `json:"avg_latency_ms"`
		} `json:"list"`
	}
	resp.DecodeData(t, &data)
	t.Logf("Model performance returned %d models", len(data.List))
}

func TestMonitorLatency(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/monitor/latency", map[string]string{
		"minutes": "30",
	})
	resp.AssertSuccess(t)

	var data map[string]any
	resp.DecodeData(t, &data)

	t.Logf("Monitor latency returned %d keys", len(data))
}

func TestMonitorSystem(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/monitor/system", map[string]string{
		"minutes": "30",
	})
	resp.AssertSuccess(t)

	var data map[string]any
	resp.DecodeData(t, &data)

	t.Logf("Monitor system returned %d keys", len(data))
}

func TestMonitorDBPool(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/monitor/db-pool", nil)
	resp.AssertSuccess(t)

	var data map[string]any
	resp.DecodeData(t, &data)

	t.Logf("DB pool monitor returned %d keys", len(data))
}

func TestMonitorRedisPool(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/monitor/redis-pool", nil)
	resp.AssertSuccess(t)

	var data map[string]any
	resp.DecodeData(t, &data)

	t.Logf("Redis pool monitor returned %d keys", len(data))
}

func TestMonitorRealtime(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/monitor/realtime", nil)
	resp.AssertSuccess(t)

	t.Logf("Realtime monitor response received")
}

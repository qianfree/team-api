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

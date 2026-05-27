//go:build integration

package tenant_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestLoginHistory(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/security/login-history", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			ID          int64  `json:"id"`
			Username    string `json:"username"`
			LoginMethod string `json:"login_method"`
			IPAddress   string `json:"ip_address"`
			Success     bool   `json:"success"`
		} `json:"list"`
		Total    int `json:"total"`
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}
	resp.DecodeData(t, &data)
}

func TestLoginHistoryWithFilters(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	// Filter by IP
	resp := client.Get("/api/tenant/security/login-history", map[string]string{
		"page":       "1",
		"page_size":  "10",
		"ip_address": "127.0.0.1",
	})
	resp.AssertSuccess(t)

	// Filter by success status
	resp = client.Get("/api/tenant/security/login-history", map[string]string{
		"page":      "1",
		"page_size": "10",
		"success":   "true",
	})
	resp.AssertSuccess(t)

	// Filter by date range
	resp = client.Get("/api/tenant/security/login-history", map[string]string{
		"page":       "1",
		"page_size":  "10",
		"start_time": "2025-01-01",
		"end_time":   "2026-12-31",
	})
	resp.AssertSuccess(t)
}

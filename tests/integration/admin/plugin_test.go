//go:build integration

package admin_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestPluginList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/plugins", nil)
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			Name        string `json:"name"`
			Label       string `json:"label"`
			Description string `json:"description"`
			Version     string `json:"version"`
			Category    string `json:"category"`
			Author      string `json:"author"`
			Status      string `json:"status"`
			Installed   bool   `json:"installed"`
			Config      any    `json:"config"`
		} `json:"list"`
	}
	resp.DecodeData(t, &data)

	t.Logf("Plugin list returned %d entries", len(data.List))
}

func TestPluginListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by category
	resp := client.Get("/api/admin/plugins", map[string]string{
		"category": "payment",
	})
	resp.AssertSuccess(t)

	// Filter by status
	resp = client.Get("/api/admin/plugins", map[string]string{
		"status": "installed",
	})
	resp.AssertSuccess(t)

	t.Logf("Plugin list filters applied successfully")
}

//go:build integration

package admin_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestRedemptionList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/redemptions", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestRedemptionListWithStatusFilter(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by status
	resp := client.Get("/api/admin/redemptions", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "active",
	})
	resp.AssertSuccess(t)

	// Filter by unused status
	resp = client.Get("/api/admin/redemptions", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "used",
	})
	resp.AssertSuccess(t)

	t.Logf("Redemption list filters applied successfully")
}

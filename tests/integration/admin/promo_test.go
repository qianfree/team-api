//go:build integration

package admin_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestPromoCodeList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/promo-codes", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

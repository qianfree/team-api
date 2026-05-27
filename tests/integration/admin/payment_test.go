//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestPaymentChannelList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/payment-channels", nil)
	resp.AssertSuccess(t)

	var data struct {
		List []map[string]any `json:"list"`
	}
	resp.DecodeData(t, &data)

	t.Logf("Payment channels returned %d entries", len(data.List))
}

func TestPaymentSettingsGet(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/payment-settings", nil)
	resp.AssertSuccess(t)

	var data struct {
		AmountOptions   []int           `json:"amount_options"`
		AmountDiscount  map[int]float64 `json:"amount_discount"`
		MinTopUp        float64         `json:"min_topup"`
		Currency        string          `json:"currency"`
		CallbackBaseURL string          `json:"callback_base_url"`
	}
	resp.DecodeData(t, &data)

	t.Logf("Payment settings: currency=%s, min_topup=%f, %d amount_options",
		data.Currency, data.MinTopUp, len(data.AmountOptions))
}

func TestPaymentSettingsUpdate(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// First get current settings
	getResp := client.Get("/api/admin/payment-settings", nil)
	getResp.AssertSuccess(t)

	var current struct {
		AmountOptions   []int           `json:"amount_options"`
		AmountDiscount  map[int]float64 `json:"amount_discount"`
		MinTopUp        float64         `json:"min_topup"`
		Currency        string          `json:"currency"`
		CallbackBaseURL string          `json:"callback_base_url"`
	}
	getResp.DecodeData(t, &current)

	// Update with same values (safe, non-destructive)
	updateResp := client.Put("/api/admin/payment-settings", map[string]any{
		"amount_options":    current.AmountOptions,
		"amount_discount":   current.AmountDiscount,
		"min_topup":         current.MinTopUp,
		"currency":          current.Currency,
		"callback_base_url": current.CallbackBaseURL,
	})
	updateResp.AssertSuccess(t)

	t.Logf("Payment settings update succeeded")
}

func TestMemberCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create a tenant first to host the member
	tenantID, tenantCleanup := testinfra.CreateTestTenant(t, client)
	defer tenantCleanup()

	// Create member
	suffix := randomSuffix()
	createResp := client.Post("/api/admin/members", map[string]any{
		"tenant_id":    tenantID,
		"username":     fmt.Sprintf("mem%s", suffix),
		"email":        fmt.Sprintf("mem%s@test.com", suffix),
		"password":     "MemberPass123!",
		"display_name": "Test Member",
		"role":         "member",
	})
	createResp.AssertSuccess(t)

	memberID := createResp.GetID(t)
	t.Logf("Created member: id=%d, tenant_id=%d", memberID, tenantID)

	// Disable member
	disableResp := client.Put(fmt.Sprintf("/api/admin/members/%d/disable", memberID), nil)
	disableResp.AssertSuccess(t)
	t.Logf("Member %d disabled", memberID)

	// Enable member
	enableResp := client.Put(fmt.Sprintf("/api/admin/members/%d/enable", memberID), nil)
	enableResp.AssertSuccess(t)
	t.Logf("Member %d enabled", memberID)

	// Reset password
	resetResp := client.Put(fmt.Sprintf("/api/admin/members/%d/reset-password", memberID), nil)
	resetResp.AssertSuccess(t)

	var resetData struct {
		NewPassword string `json:"new_password"`
	}
	resetResp.DecodeData(t, &resetData)

	if resetData.NewPassword == "" {
		t.Fatal("expected new_password to be non-empty after reset")
	}

	t.Logf("Member %d password reset successfully", memberID)
}

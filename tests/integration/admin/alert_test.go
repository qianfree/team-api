//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestAlertRuleList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/alert/rules", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestAlertOptions(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/alert/options", nil)
	resp.AssertSuccess(t)

	var data map[string]any
	resp.DecodeData(t, &data)

	t.Logf("Alert options returned %d keys", len(data))
}

func TestAlertRuleCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	suffix := randomSuffix()

	// --- Create ---
	createResp := client.Post("/api/admin/alert/rules", map[string]any{
		"name":                 fmt.Sprintf("测试告警规则 %s", suffix),
		"metric_type":          "error_rate",
		"condition":            "gt",
		"threshold":            0.5,
		"duration_seconds":     300,
		"notification_methods": []string{"in_app"},
		"notify_user_ids":      []int64{1},
		"level":                "warning",
		"cooldown_seconds":     600,
	})
	createResp.AssertSuccess(t)
	ruleID := createResp.GetID(t)
	defer func() {
		client.Delete(fmt.Sprintf("/api/admin/alert/rules/%d", ruleID))
	}()

	t.Logf("Created alert rule: id=%d", ruleID)

	// --- List should contain the rule ---
	listResp := client.Get("/api/admin/alert/rules", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	testinfra.AssertPaginatedList(t, listResp, 1)

	// --- Update ---
	updateResp := client.Put(fmt.Sprintf("/api/admin/alert/rules/%d", ruleID), map[string]any{
		"name":                 fmt.Sprintf("更新告警规则 %s", suffix),
		"metric_type":          "error_rate",
		"condition":            "gt",
		"threshold":            0.8,
		"duration_seconds":     600,
		"notification_methods": []string{"in_app"},
		"notify_user_ids":      []int64{1},
		"level":                "critical",
		"cooldown_seconds":     900,
	})
	updateResp.AssertSuccess(t)

	t.Logf("Updated alert rule %d", ruleID)

	// --- Toggle (disable) ---
	toggleResp := client.Put(fmt.Sprintf("/api/admin/alert/rules/%d/toggle", ruleID), nil)
	toggleResp.AssertSuccess(t)

	t.Logf("Toggled alert rule %d", ruleID)

	// --- Test rule ---
	testResp := client.Post(fmt.Sprintf("/api/admin/alert/rules/%d/test", ruleID), nil)
	testResp.AssertSuccess(t)

	var testResult struct {
		Message string `json:"message"`
	}
	testResp.DecodeData(t, &testResult)

	t.Logf("Test alert rule %d: message=%s", ruleID, testResult.Message)

	// --- Delete ---
	deleteResp := client.Delete(fmt.Sprintf("/api/admin/alert/rules/%d", ruleID))
	deleteResp.AssertSuccess(t)

	t.Logf("Deleted alert rule %d", ruleID)
}

func TestAlertEventList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/alert/events", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

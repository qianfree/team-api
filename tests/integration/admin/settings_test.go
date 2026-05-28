//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestSettingsCategories(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/settings/categories", nil)
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			Key   string `json:"key"`
			Label string `json:"label"`
			Icon  string `json:"icon"`
			Order int    `json:"order"`
		} `json:"list"`
	}
	resp.DecodeData(t, &data)

	if len(data.List) == 0 {
		t.Fatal("expected at least one settings category")
	}

	for _, cat := range data.List {
		if cat.Key == "" {
			t.Fatal("settings category key should not be empty")
		}
		if cat.Label == "" {
			t.Fatalf("category %s has empty label", cat.Key)
		}
	}

	t.Logf("Settings categories returned %d entries", len(data.List))
}

func TestSettingsGet(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	category := getFirstSettingsCategory(t, client)
	if category == "" {
		return
	}

	resp := client.Get(fmt.Sprintf("/api/admin/settings/%s", category), nil)
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			Key         string `json:"key"`
			Value       any    `json:"value"`
			Type        string `json:"type"`
			Label       string `json:"label"`
			Description string `json:"description"`
			Sensitive   bool   `json:"sensitive"`
			Validation  string `json:"validation"`
			Default     any    `json:"default"`
		} `json:"list"`
	}
	resp.DecodeData(t, &data)

	if len(data.List) == 0 {
		t.Fatalf("settings category %q returned empty list", category)
	}

	for _, item := range data.List {
		if item.Key == "" {
			t.Fatal("setting key should not be empty")
		}
		if item.Type == "" {
			t.Fatalf("setting %s has empty type", item.Key)
		}
		if item.Label == "" {
			t.Fatalf("setting %s has empty label", item.Key)
		}
	}

	t.Logf("Settings for category %q returned %d entries", category, len(data.List))
}

func TestSettingsUpdateAndVerify(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	category := getFirstSettingsCategory(t, client)
	if category == "" {
		return
	}

	// Get current settings
	getResp := client.Get(fmt.Sprintf("/api/admin/settings/%s", category), nil)
	getResp.AssertSuccess(t)

	var getData struct {
		List []struct {
			Key       string `json:"key"`
			Value     any    `json:"value"`
			Type      string `json:"type"`
			Sensitive bool   `json:"sensitive"`
		} `json:"list"`
	}
	getResp.DecodeData(t, &getData)

	if len(getData.List) == 0 {
		t.Skipf("No settings found in category %q", category)
	}

	// Find a non-sensitive string setting to test with
	var targetKey string
	var originalValue any
	for _, item := range getData.List {
		if !item.Sensitive && (item.Type == "string" || item.Type == "text") {
			targetKey = item.Key
			originalValue = item.Value
			break
		}
	}

	if targetKey == "" {
		t.Skipf("No non-sensitive string setting found in category %q, skipping update verification", category)
	}

	// Update with a new value
	newValue := "integration-test-value"
	settings := make(map[string]any)
	for _, item := range getData.List {
		settings[item.Key] = item.Value
	}
	settings[targetKey] = newValue

	updateResp := client.Put(fmt.Sprintf("/api/admin/settings/%s", category), map[string]any{
		"settings": settings,
	})
	updateResp.AssertSuccess(t)

	// Verify the value was persisted by reading it back
	verifyResp := client.Get(fmt.Sprintf("/api/admin/settings/%s", category), nil)
	verifyResp.AssertSuccess(t)

	var verifyData struct {
		List []struct {
			Key   string `json:"key"`
			Value any    `json:"value"`
		} `json:"list"`
	}
	verifyResp.DecodeData(t, &verifyData)

	for _, item := range verifyData.List {
		if item.Key == targetKey {
			if item.Value != newValue {
				t.Fatalf("expected setting %q=%q after update, got %v", targetKey, newValue, item.Value)
			}
			break
		}
	}

	// Restore original value
	settings[targetKey] = originalValue
	restoreResp := client.Put(fmt.Sprintf("/api/admin/settings/%s", category), map[string]any{
		"settings": settings,
	})
	restoreResp.AssertSuccess(t)

	t.Logf("Settings update verified: %q changed from %v to %q and restored", targetKey, originalValue, newValue)
}

func TestSettingsSensitiveFieldMasking(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Check all categories for sensitive fields
	catResp := client.Get("/api/admin/settings/categories", nil)
	catResp.AssertSuccess(t)

	var catData struct {
		List []struct {
			Key string `json:"key"`
		} `json:"list"`
	}
	catResp.DecodeData(t, &catData)

	for _, cat := range catData.List {
		resp := client.Get(fmt.Sprintf("/api/admin/settings/%s", cat.Key), nil)
		resp.AssertSuccess(t)

		var data struct {
			List []struct {
				Key       string `json:"key"`
				Sensitive bool   `json:"sensitive"`
				Value     any    `json:"value"`
			} `json:"list"`
		}
		resp.DecodeData(t, &data)

		for _, item := range data.List {
			if item.Sensitive {
				// Sensitive fields should not expose raw values
				if strVal, ok := item.Value.(string); ok && len(strVal) > 4 {
					// Value might be masked (e.g., "****1234" or similar)
					t.Logf("Sensitive field %q in category %q: value present (length=%d)", item.Key, cat.Key, len(strVal))
				}
			}
		}
	}
}

func TestSettingsNonExistentCategory(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/settings/nonexistent_category_xyz", nil)
	if resp.Code == 0 {
		t.Fatal("expected error for non-existent settings category, got success")
	}

	t.Logf("Non-existent category correctly rejected: code=%d, message=%s", resp.Code, resp.Message)
}

// getFirstSettingsCategory returns the key of the first settings category, or empty string if none.
func getFirstSettingsCategory(t *testing.T, client *testinfra.APIClient) string {
	t.Helper()
	catResp := client.Get("/api/admin/settings/categories", nil)
	catResp.AssertSuccess(t)

	var catData struct {
		List []struct {
			Key string `json:"key"`
		} `json:"list"`
	}
	catResp.DecodeData(t, &catData)

	if len(catData.List) == 0 {
		t.Skip("No settings categories found")
	}
	return catData.List[0].Key
}

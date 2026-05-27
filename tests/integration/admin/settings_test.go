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

	// First get categories to find a valid one
	catResp := client.Get("/api/admin/settings/categories", nil)
	catResp.AssertSuccess(t)

	var catData struct {
		List []struct {
			Key string `json:"key"`
		} `json:"list"`
	}
	catResp.DecodeData(t, &catData)

	if len(catData.List) == 0 {
		t.Skip("No settings categories found, skipping get test")
	}

	// Get settings for the first category
	category := catData.List[0].Key
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

	t.Logf("Settings for category %q returned %d entries", category, len(data.List))
}

func TestSettingsUpdate(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// First get categories to find a valid one
	catResp := client.Get("/api/admin/settings/categories", nil)
	catResp.AssertSuccess(t)

	var catData struct {
		List []struct {
			Key string `json:"key"`
		} `json:"list"`
	}
	catResp.DecodeData(t, &catData)

	if len(catData.List) == 0 {
		t.Skip("No settings categories found, skipping update test")
	}

	category := catData.List[0].Key

	// Get current settings
	getResp := client.Get(fmt.Sprintf("/api/admin/settings/%s", category), nil)
	getResp.AssertSuccess(t)

	var getData struct {
		List []struct {
			Key   string `json:"key"`
			Value any    `json:"value"`
		} `json:"list"`
	}
	getResp.DecodeData(t, &getData)

	if len(getData.List) == 0 {
		t.Skipf("No settings found in category %q, skipping update test", category)
	}

	// Build settings map from current values (non-destructive)
	settings := make(map[string]any)
	for _, item := range getData.List {
		settings[item.Key] = item.Value
	}

	// Update with same values
	updateResp := client.Put(fmt.Sprintf("/api/admin/settings/%s", category), map[string]any{
		"settings": settings,
	})
	updateResp.AssertSuccess(t)

	t.Logf("Settings update for category %q succeeded (%d keys)", category, len(settings))
}

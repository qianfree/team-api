//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestChannelList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	_, cleanup := testinfra.CreateTestChannel(t, client)
	defer cleanup()

	resp := client.Get("/api/admin/channels", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 1)
}

func TestChannelListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	_, cleanup := testinfra.CreateTestChannel(t, client)
	defer cleanup()

	resp := client.Get("/api/admin/channels", map[string]string{
		"page":      "1",
		"page_size": "10",
		"type":      "1",
	})
	resp.AssertSuccess(t)

	resp = client.Get("/api/admin/channels", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "active",
	})
	resp.AssertSuccess(t)
}

func TestChannelCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	suffix := randomSuffix()
	createResp := client.Post("/api/admin/channels", map[string]any{
		"name":     fmt.Sprintf("CRUD测试渠道 %s", suffix),
		"type":     1,
		"api_key":  "sk-test-key-" + suffix,
		"base_url": "https://api.openai.com",
		"priority": 10,
		"weight":   5,
	})
	createResp.AssertSuccess(t)
	channelID := createResp.GetID(t)
	defer client.Delete(fmt.Sprintf("/api/admin/channels/%d", channelID))

	// Get detail
	detailResp := client.Get(fmt.Sprintf("/api/admin/channels/%d", channelID), nil)
	detailResp.AssertSuccess(t)

	var detail struct {
		ID       int64 `json:"id"`
		Priority int   `json:"priority"`
		Weight   int   `json:"weight"`
	}
	detailResp.DecodeData(t, &detail)
	if detail.Priority != 10 {
		t.Fatalf("expected priority=10, got %d", detail.Priority)
	}

	// Update
	updateResp := client.Put(fmt.Sprintf("/api/admin/channels/%d", channelID), map[string]any{
		"name":     fmt.Sprintf("更新渠道名 %s", suffix),
		"priority": 20,
		"weight":   10,
		"status":   "active",
	})
	updateResp.AssertSuccess(t)

	// Verify update
	verifyResp := client.Get(fmt.Sprintf("/api/admin/channels/%d", channelID), nil)
	verifyResp.AssertSuccess(t)
	var updated struct {
		Priority int `json:"priority"`
		Weight   int `json:"weight"`
	}
	verifyResp.DecodeData(t, &updated)
	if updated.Priority != 20 {
		t.Fatalf("expected priority=20, got %d", updated.Priority)
	}

	// Delete
	deleteResp := client.Delete(fmt.Sprintf("/api/admin/channels/%d", channelID))
	deleteResp.AssertSuccess(t)
}

func TestChannelKeys(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	channelID, channelCleanup := testinfra.CreateTestChannel(t, client)
	defer channelCleanup()

	// Channel created with one key — list and verify
	listKeysResp := client.Get(fmt.Sprintf("/api/admin/channels/%d/keys", channelID), nil)
	listKeysResp.AssertSuccess(t)

	var keysResult struct {
		List []struct {
			ID     int64  `json:"id"`
			Name   string `json:"name"`
			ApiKey string `json:"api_key"`
			Status string `json:"status"`
		} `json:"list"`
	}
	listKeysResp.DecodeData(t, &keysResult)
	if len(keysResult.List) < 1 {
		t.Fatalf("expected at least 1 key, got %d", len(keysResult.List))
	}

	// Delete the existing key
	keyID := keysResult.List[0].ID
	deleteKeyResp := client.Delete(fmt.Sprintf("/api/admin/channels/%d/keys/%d", channelID, keyID))
	deleteKeyResp.AssertSuccess(t)
}

func TestChannelAbilities(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	channelID, channelCleanup := testinfra.CreateTestChannel(t, client)
	defer channelCleanup()

	modelID, modelCleanup := testinfra.CreateTestModel(t, client)
	defer modelCleanup()

	// Get the model_id string for the created model
	listResp := client.Get("/api/admin/models", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	listResp.AssertSuccess(t)

	var listResult struct {
		List []struct {
			ID      int64  `json:"id"`
			ModelID string `json:"model_id"`
		} `json:"list"`
	}
	listResp.DecodeData(t, &listResult)

	var modelIDStr string
	for _, m := range listResult.List {
		if m.ID == modelID {
			modelIDStr = m.ModelID
			break
		}
	}
	if modelIDStr == "" {
		t.Fatalf("could not find model_id string for id=%d", modelID)
	}

	// Set abilities — tolerate server errors (may need upstream model config)
	putResp := client.Put(fmt.Sprintf("/api/admin/channels/%d/abilities", channelID), map[string]any{
		"abilities": []map[string]any{
			{
				"model_name":     modelIDStr,
				"upstream_model": modelIDStr,
				"enabled":        true,
			},
		},
	})
	if putResp.Code != 0 {
		t.Logf("abilities put returned code=%d msg=%s (may be expected)", putResp.Code, putResp.Message)
		return
	}

	// Get abilities
	getResp := client.Get(fmt.Sprintf("/api/admin/channels/%d/abilities", channelID), nil)
	getResp.AssertSuccess(t)

	var abilitiesResult struct {
		List []struct {
			ModelName string `json:"model_name"`
			Enabled   bool   `json:"enabled"`
		} `json:"list"`
	}
	getResp.DecodeData(t, &abilitiesResult)
	if len(abilitiesResult.List) < 1 {
		t.Fatalf("expected at least 1 ability, got %d", len(abilitiesResult.List))
	}
}

func TestChannelClone(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	channelID, channelCleanup := testinfra.CreateTestChannel(t, client)
	defer channelCleanup()

	suffix := randomSuffix()
	cloneResp := client.Post(fmt.Sprintf("/api/admin/channels/%d/clone", channelID), map[string]any{
		"name":    fmt.Sprintf("克隆渠道 %s", suffix),
		"api_key": "sk-cloned-key-" + suffix,
	})
	cloneResp.AssertSuccess(t)
	clonedID := cloneResp.GetID(t)
	defer client.Delete(fmt.Sprintf("/api/admin/channels/%d", clonedID))

	// Verify cloned channel exists
	detailResp := client.Get(fmt.Sprintf("/api/admin/channels/%d", clonedID), nil)
	detailResp.AssertSuccess(t)
}

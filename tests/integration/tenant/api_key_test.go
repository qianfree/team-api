//go:build integration

package tenant_test

import (
	"fmt"
	"testing"

	admintest "github.com/qianfree/team-api/tests/integration/admin/testinfra"
	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestApiKeyList(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	_, cleanup := testinfra.CreateTestApiKey(t, client)
	defer cleanup()

	resp := client.Get("/api/tenant/api-keys", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	admintest.AssertPaginatedList(t, resp, 1)
}

func TestApiKeyCRUD(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	// --- Create ---
	suffix := testinfra.RandomSuffix()
	createResp := client.Post("/api/tenant/api-keys", map[string]any{
		"name": fmt.Sprintf("crud-key-%s", suffix),
	})
	createResp.AssertSuccess(t)
	keyID := createResp.GetID(t)
	defer func() {
		client.Delete(fmt.Sprintf("/api/tenant/api-keys/%d", keyID))
	}()

	var createData struct {
		ID        int64  `json:"id"`
		Key       string `json:"key"`
		KeyPrefix string `json:"key_prefix"`
		Name      string `json:"name"`
	}
	createResp.DecodeData(t, &createData)

	if createData.Key == "" {
		t.Fatal("expected non-empty key on create")
	}
	if createData.KeyPrefix == "" {
		t.Fatal("expected non-empty key_prefix")
	}

	// --- List should contain the key ---
	listResp := client.Get("/api/tenant/api-keys", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	listResp.AssertSuccess(t)

	// --- Update ---
	updateResp := client.Put(fmt.Sprintf("/api/tenant/api-keys/%d", keyID), map[string]any{
		"name":   fmt.Sprintf("updated-key-%s", suffix),
		"status": "disabled",
	})
	updateResp.AssertSuccess(t)

	// Verify update via list
	verifyResp := client.Get("/api/tenant/api-keys", map[string]string{
		"page":      "1",
		"page_size": "50",
	})
	verifyResp.AssertSuccess(t)

	// --- Delete ---
	deleteResp := client.Delete(fmt.Sprintf("/api/tenant/api-keys/%d", keyID))
	deleteResp.AssertSuccess(t)
}

func TestApiKeyUpdateScopes(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	keyID, cleanup := testinfra.CreateTestApiKey(t, client)
	defer cleanup()

	// Update scopes
	scopesResp := client.Put(fmt.Sprintf("/api/tenant/api-keys/%d/scopes", keyID), map[string]any{
		"model_names": []string{"gpt-4", "claude-3"},
	})
	scopesResp.AssertSuccess(t)

	// Verify model scopes
	getScopesResp := client.Get(fmt.Sprintf("/api/tenant/api-keys/%d/model-scopes", keyID), nil)
	getScopesResp.AssertSuccess(t)

	var scopesData struct {
		ModelNames []string `json:"model_names"`
	}
	getScopesResp.DecodeData(t, &scopesData)

	if len(scopesData.ModelNames) != 2 {
		t.Fatalf("expected 2 model names, got %d", len(scopesData.ModelNames))
	}
}

func TestApiKeyExport(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	_, cleanup := testinfra.CreateTestApiKey(t, client)
	defer cleanup()

	resp := client.Get("/api/tenant/api-keys/export", map[string]string{
		"format": "csv",
	})
	resp.AssertSuccess(t)
}

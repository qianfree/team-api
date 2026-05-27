//go:build integration

package tenant_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestAvailableModels(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/models", nil)
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			ID        int64  `json:"id"`
			ModelID   string `json:"model_id"`
			ModelName string `json:"model_name"`
			Category  string `json:"category"`
		} `json:"list"`
	}
	resp.DecodeData(t, &data)
}

func TestAvailableModelsFilterByCategory(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/models", map[string]string{
		"category": "chat",
	})
	resp.AssertSuccess(t)
}

func TestAvailableModelsSearch(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/tenant/models", map[string]string{
		"search": "gpt",
	})
	resp.AssertSuccess(t)
}

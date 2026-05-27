//go:build integration

package admin_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestModelList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Create a model to ensure at least one exists
	_, cleanup := testinfra.CreateTestModel(t, client)
	defer cleanup()

	resp := client.Get("/api/admin/models", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 1)
}

func TestModelListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	_, cleanup := testinfra.CreateTestModel(t, client)
	defer cleanup()

	// Filter by category
	resp := client.Get("/api/admin/models", map[string]string{
		"page":      "1",
		"page_size": "10",
		"category":  "chat",
	})
	resp.AssertSuccess(t)

	// Filter by status
	resp = client.Get("/api/admin/models", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "active",
	})
	resp.AssertSuccess(t)

	// Search by keyword
	resp = client.Get("/api/admin/models", map[string]string{
		"page":      "1",
		"page_size": "10",
		"search":    "test-model",
	})
	resp.AssertSuccess(t)
}

func TestModelCRUD(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// --- Create ---
	suffix := randomSuffix()
	modelID := fmt.Sprintf("crud-model-%s", suffix)
	createResp := client.Post("/api/admin/models", map[string]any{
		"model_id":           modelID,
		"model_name":         fmt.Sprintf("CRUD测试模型 %s", suffix),
		"category":           "chat",
		"max_context_tokens": 128000,
		"max_output_tokens":  4096,
		"capabilities":       map[string]any{"vision": true, "streaming": true},
		"description":        "Integration test model",
		"tags":               []string{"test", "chat"},
	})
	createResp.AssertSuccess(t)
	id := createResp.GetID(t)
	defer func() {
		client.Delete(fmt.Sprintf("/api/admin/models/%d", id))
	}()

	// --- Get detail (using model list to find the created model) ---
	listResp := client.Get("/api/admin/models", map[string]string{
		"page":      "1",
		"page_size": "50",
		"search":    modelID,
	})
	listResp.AssertSuccess(t)

	var listResult struct {
		List []struct {
			ID               int64  `json:"id"`
			ModelID          string `json:"model_id"`
			ModelName        string `json:"model_name"`
			Category         string `json:"category"`
			Status           string `json:"status"`
			MaxContextTokens int    `json:"max_context_tokens"`
			MaxOutputTokens  int    `json:"max_output_tokens"`
		} `json:"list"`
		Total int `json:"total"`
	}
	listResp.DecodeData(t, &listResult)

	found := false
	for _, m := range listResult.List {
		if m.ModelID == modelID {
			found = true
			if m.MaxContextTokens != 128000 {
				t.Fatalf("expected max_context_tokens=128000, got %d", m.MaxContextTokens)
			}
			if m.Category != "chat" {
				t.Fatalf("expected category=chat, got %s", m.Category)
			}
			break
		}
	}
	if !found {
		t.Fatalf("created model %s not found in list", modelID)
	}

	// --- Update ---
	updateResp := client.Put(fmt.Sprintf("/api/admin/models/%d", id), map[string]any{
		"model_name":        fmt.Sprintf("更新模型名 %s", suffix),
		"category":          "chat",
		"status":            "active",
		"max_output_tokens": 8192,
		"description":       "Updated integration test model",
	})
	updateResp.AssertSuccess(t)

	// Verify update
	verifyResp := client.Get("/api/admin/models", map[string]string{
		"page":      "1",
		"page_size": "50",
		"search":    modelID,
	})
	verifyResp.AssertSuccess(t)

	var verifyResult struct {
		List []struct {
			ModelID         string `json:"model_id"`
			MaxOutputTokens int    `json:"max_output_tokens"`
		} `json:"list"`
	}
	verifyResp.DecodeData(t, &verifyResult)
	for _, m := range verifyResult.List {
		if m.ModelID == modelID {
			if m.MaxOutputTokens != 8192 {
				t.Fatalf("expected max_output_tokens=8192 after update, got %d", m.MaxOutputTokens)
			}
			break
		}
	}

	// --- Delete ---
	deleteResp := client.Delete(fmt.Sprintf("/api/admin/models/%d", id))
	deleteResp.AssertSuccess(t)
}

func TestModelPricing(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	modelID, cleanup := testinfra.CreateTestModel(t, client)
	defer cleanup()

	// Get model_id string from the created model
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

	// --- Get pricing (should be empty or default) ---
	getPricingResp := client.Get(fmt.Sprintf("/api/admin/models/%s/pricing", modelIDStr), nil)
	getPricingResp.AssertSuccess(t)

	// --- Update pricing ---
	putPricingResp := client.Put(fmt.Sprintf("/api/admin/models/%s/pricing", modelIDStr), map[string]any{
		"items": []map[string]any{
			{
				"billing_mode": "token",
				"min_tokens":   0,
				"max_tokens":   128000,
				"input_price":  0.0015,
				"output_price": 0.006,
			},
			{
				"billing_mode": "token",
				"min_tokens":   128001,
				"max_tokens":   200000,
				"input_price":  0.002,
				"output_price": 0.008,
			},
		},
	})
	putPricingResp.AssertSuccess(t)

	// --- Verify pricing was saved ---
	verifyPricingResp := client.Get(fmt.Sprintf("/api/admin/models/%s/pricing", modelIDStr), nil)
	verifyPricingResp.AssertSuccess(t)

	var pricingResult struct {
		List []struct {
			BillingMode string  `json:"billing_mode"`
			MinTokens   int     `json:"min_tokens"`
			MaxTokens   int     `json:"max_tokens"`
			InputPrice  float64 `json:"input_price"`
			OutputPrice float64 `json:"output_price"`
		} `json:"list"`
	}
	verifyPricingResp.DecodeData(t, &pricingResult)
	if len(pricingResult.List) != 2 {
		t.Fatalf("expected 2 pricing tiers, got %d", len(pricingResult.List))
	}
}

func TestModelOptions(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	_, cleanup := testinfra.CreateTestModel(t, client)
	defer cleanup()

	resp := client.Get("/api/admin/models/options", map[string]string{
		"status":   "active",
		"category": "chat",
	})
	resp.AssertSuccess(t)

	var result struct {
		List []struct {
			ID        int64  `json:"id"`
			ModelID   string `json:"model_id"`
			ModelName string `json:"model_name"`
			Category  string `json:"category"`
		} `json:"list"`
	}
	resp.DecodeData(t, &result)
	if len(result.List) < 1 {
		t.Fatal("expected at least 1 model option")
	}
}

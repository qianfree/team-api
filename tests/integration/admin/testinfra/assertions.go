//go:build integration

package testinfra

import (
	"encoding/json"
	"testing"
)

func AssertPaginatedList(t *testing.T, resp *APIResponse, minCount int) {
	t.Helper()
	resp.AssertSuccess(t)

	var result struct {
		List     json.RawMessage `json:"list"`
		Total    int             `json:"total"`
		Page     int             `json:"page"`
		PageSize int             `json:"page_size"`
	}
	resp.DecodeData(t, &result)

	if result.Total < minCount {
		t.Fatalf("expected total >= %d, got %d", minCount, result.Total)
	}
	if result.Page < 1 {
		t.Fatalf("expected page >= 1, got %d", result.Page)
	}
	if result.PageSize < 1 {
		t.Fatalf("expected page_size >= 1, got %d", result.PageSize)
	}
}

func AssertNonEmptyList(t *testing.T, resp *APIResponse) {
	t.Helper()
	resp.AssertSuccess(t)

	var result struct {
		List json.RawMessage `json:"list"`
	}
	resp.DecodeData(t, &result)

	if string(result.List) == "null" || string(result.List) == "[]" {
		t.Fatal("expected non-empty list")
	}
}

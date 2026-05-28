//go:build integration

package testinfra

import (
	"encoding/json"
	"fmt"
	"math"
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

// AssertFloatEqual asserts that two float64 values are equal within tolerance.
// Used for wallet balance and billing amount comparisons where floating point
// precision matters for business logic validation.
func AssertFloatEqual(t *testing.T, expected, actual, tolerance float64, msgAndArgs ...any) {
	t.Helper()
	if math.Abs(expected-actual) > tolerance {
		msg := fmt.Sprintf("expected %.10f, got %.10f (tolerance=%.10f)", expected, actual, tolerance)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf("%v — %s", msgAndArgs[0], msg)
		}
		t.Fatal(msg)
	}
}

// AssertListContainsID asserts that a paginated list response contains an item with the given ID.
// Decodes the "list" field and checks each item's "id" field.
func AssertListContainsID(t *testing.T, resp *APIResponse, targetID int64) {
	t.Helper()
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	resp.DecodeData(t, &data)

	for _, item := range data.List {
		if item.ID == targetID {
			return
		}
	}
	t.Fatalf("expected list to contain id=%d, but it was not found", targetID)
}

// AssertListNotContainsID asserts that a paginated list response does NOT contain an item with the given ID.
// Used to verify deletions and isolation are effective.
func AssertListNotContainsID(t *testing.T, resp *APIResponse, targetID int64) {
	t.Helper()
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	resp.DecodeData(t, &data)

	for _, item := range data.List {
		if item.ID == targetID {
			t.Fatalf("expected list NOT to contain id=%d, but it was found", targetID)
		}
	}
}

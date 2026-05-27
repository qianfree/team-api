//go:build integration

package admin_test

import (
	"testing"

	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

func TestFeedbackList(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/feedbacks", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	testinfra.AssertPaginatedList(t, resp, 0)
}

func TestFeedbackListWithFilters(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	// Filter by status
	resp := client.Get("/api/admin/feedbacks", map[string]string{
		"page":      "1",
		"page_size": "10",
		"status":    "pending",
	})
	resp.AssertSuccess(t)

	// Filter by category
	resp = client.Get("/api/admin/feedbacks", map[string]string{
		"page":      "1",
		"page_size": "10",
		"category":  "bug",
	})
	resp.AssertSuccess(t)

	// Filter by priority
	resp = client.Get("/api/admin/feedbacks", map[string]string{
		"page":      "1",
		"page_size": "10",
		"priority":  "high",
	})
	resp.AssertSuccess(t)

	t.Logf("Feedback list filters applied successfully")
}

func TestFeedbackStats(t *testing.T) {
	client := testinfra.GetAuthedClient(t)

	resp := client.Get("/api/admin/feedbacks/stats", nil)
	resp.AssertSuccess(t)

	var data struct {
		Total        int              `json:"total"`
		Pending      int              `json:"pending"`
		Acknowledged int              `json:"acknowledged"`
		InProgress   int              `json:"in_progress"`
		Resolved     int              `json:"resolved"`
		Closed       int              `json:"closed"`
		ByCategory   map[string]int   `json:"by_category"`
		RecentTrend  []map[string]any `json:"recent_trend"`
	}
	resp.DecodeData(t, &data)

	if data.Total < 0 {
		t.Fatalf("expected total >= 0, got %d", data.Total)
	}

	sum := data.Pending + data.Acknowledged + data.InProgress + data.Resolved + data.Closed
	if sum > data.Total {
		t.Fatalf("status sum (%d) exceeds total (%d)", sum, data.Total)
	}

	t.Logf("Feedback stats: total=%d, pending=%d, acknowledged=%d, in_progress=%d, resolved=%d, closed=%d",
		data.Total, data.Pending, data.Acknowledged, data.InProgress, data.Resolved, data.Closed)
}

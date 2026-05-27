//go:build integration

package tenant_test

import (
	"fmt"
	"testing"

	"github.com/qianfree/team-api/tests/integration/tenant/testinfra"
)

func TestFeedbackCreate(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	resp := client.Post("/api/tenant/feedbacks", map[string]any{
		"category":    "bug_report",
		"title":       fmt.Sprintf("测试反馈 %s", testinfra.RandomSuffix()),
		"description": "这是一个集成测试创建的反馈",
	})
	resp.AssertSuccess(t)

	var data struct {
		ID int64 `json:"id"`
	}
	resp.DecodeData(t, &data)
	if data.ID == 0 {
		t.Fatal("expected non-zero feedback id")
	}
}

func TestFeedbackList(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	// Create a feedback first
	createResp := client.Post("/api/tenant/feedbacks", map[string]any{
		"category":    "suggestion",
		"title":       fmt.Sprintf("列表测试反馈 %s", testinfra.RandomSuffix()),
		"description": "测试反馈列表功能",
	})
	createResp.AssertSuccess(t)

	// List feedbacks
	resp := client.Get("/api/tenant/feedbacks", map[string]string{
		"page":      "1",
		"page_size": "10",
	})
	resp.AssertSuccess(t)

	var data struct {
		List []struct {
			ID       int64  `json:"id"`
			Category string `json:"category"`
			Title    string `json:"title"`
			Status   string `json:"status"`
		} `json:"list"`
		Total int `json:"total"`
	}
	resp.DecodeData(t, &data)
	if data.Total < 1 {
		t.Fatalf("expected at least 1 feedback, got total=%d", data.Total)
	}
}

func TestFeedbackListWithFilters(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	// Create feedbacks with different categories
	client.Post("/api/tenant/feedbacks", map[string]any{
		"category":    "bug_report",
		"title":       "Bug反馈过滤测试",
		"description": "测试按分类过滤",
	})
	client.Post("/api/tenant/feedbacks", map[string]any{
		"category":    "feature_request",
		"title":       "功能请求过滤测试",
		"description": "测试按分类过滤",
	})

	// Filter by category
	resp := client.Get("/api/tenant/feedbacks", map[string]string{
		"page":      "1",
		"page_size": "10",
		"category":  "bug_report",
	})
	resp.AssertSuccess(t)
}

func TestFeedbackGet(t *testing.T) {
	client, _ := testinfra.GetAuthedClient(t)

	// Create a feedback
	createResp := client.Post("/api/tenant/feedbacks", map[string]any{
		"category":    "complaint",
		"title":       "详情测试反馈",
		"description": "测试获取反馈详情",
	})
	createResp.AssertSuccess(t)

	var createData struct {
		ID int64 `json:"id"`
	}
	createResp.DecodeData(t, &createData)

	// Get detail
	detailResp := client.Get(fmt.Sprintf("/api/tenant/feedbacks/%d", createData.ID), nil)
	detailResp.AssertSuccess(t)

	var detail struct {
		ID          int64  `json:"id"`
		Category    string `json:"category"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	detailResp.DecodeData(t, &detail)

	if detail.ID != createData.ID {
		t.Fatalf("expected id=%d, got %d", createData.ID, detail.ID)
	}
	if detail.Category != "complaint" {
		t.Fatalf("expected category=complaint, got %s", detail.Category)
	}
}

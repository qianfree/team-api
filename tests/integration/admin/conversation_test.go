//go:build integration

package admin_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/tests/integration/admin/testinfra"
)

// 会话视图集成测试：种子多轮对话审计记录（含会话关联列，哈希经真实提取器计算），
// 经 HTTP 会话接口断言完整对话还原。

const convItTenantID = 999998

func seedConversationTurn(t *testing.T, id int64, at time.Time, reqBody, respBody, headerSession string) {
	t.Helper()
	d := common.ExtractSessionDigest("/chat/completions", reqBody, respBody, false, headerSession, convItTenantID, "gpt-4o")
	data := g.Map{
		"id":                   id,
		"tenant_id":            convItTenantID,
		"user_id":              1,
		"api_key_id":           1,
		"request_id":           fmt.Sprintf("conv-it-%d", id),
		"method":               "POST",
		"path":                 "/chat/completions",
		"status_code":          200,
		"model_name":           "gpt-4o",
		"request_body":         reqBody,
		"response_body":        respBody,
		"audit_level":          "full",
		"tenant_request_body":  reqBody,
		"tenant_response_body": respBody,
		"tenant_audit_level":   "masked",
		"is_context_reset":     d.IsContextReset,
		"created_at":           at,
		"updated_at":           at,
	}
	if d.SessionKey != "" {
		data["session_key"] = d.SessionKey
		data["session_source"] = d.SessionSource
	}
	if d.ConvFingerprint != "" {
		data["conv_fingerprint"] = d.ConvFingerprint
	}
	if d.RespMsgHash != "" {
		data["resp_msg_hash"] = d.RespMsgHash
	}
	if d.LastAsstMsgHash != "" {
		data["last_asst_msg_hash"] = d.LastAsstMsgHash
	}
	if _, err := g.DB("audit").Model("aud_request_logs").Data(data).Insert(); err != nil {
		t.Fatalf("种子审计记录失败: %v", err)
	}
}

func cleanupConversationSeed(t *testing.T) {
	t.Helper()
	if _, err := g.DB("audit").Model("aud_request_logs").
		Where("request_id LIKE ?", "conv-it-%").Delete(); err != nil {
		t.Logf("清理会话种子数据失败: %v", err)
	}
}

func TestRequestAuditLogConversation(t *testing.T) {
	client := testinfra.GetAuthedClient(t)
	defer cleanupConversationSeed(t)

	base := time.Now().Add(-time.Minute)
	const (
		id1 = int64(9900001)
		id2 = int64(9900002)
		id3 = int64(9900003)
	)
	seedConversationTurn(t, id1, base,
		`{"model":"gpt-4o","messages":[{"role":"system","content":"S"},{"role":"user","content":"u1"}]}`,
		`{"choices":[{"message":{"role":"assistant","content":"a1"}}]}`,
		"conv-it-session")
	seedConversationTurn(t, id2, base.Add(10*time.Second),
		`{"model":"gpt-4o","messages":[{"role":"system","content":"S"},{"role":"user","content":"u1"},{"role":"assistant","content":"a1"},{"role":"user","content":"u2"}]}`,
		`{"choices":[{"message":{"role":"assistant","content":"a2"}}]}`,
		"conv-it-session")
	seedConversationTurn(t, id3, base.Add(20*time.Second),
		`{"model":"gpt-4o","messages":[{"role":"system","content":"S"},{"role":"user","content":"u1"},{"role":"assistant","content":"a1"},{"role":"user","content":"u2"},{"role":"assistant","content":"a2"},{"role":"user","content":"u3"}]}`,
		`{"choices":[{"message":{"role":"assistant","content":"a3"}}]}`,
		"conv-it-session")

	resp := client.Get(fmt.Sprintf("/api/admin/audit/request-logs/%d/conversation", id3), nil)
	resp.AssertSuccess(t)

	var view struct {
		CorrelationMode  string `json:"correlation_mode"`
		RecordCount      int    `json:"record_count"`
		AnchorRecordID   int64  `json:"anchor_record_id"`
		SessionKeyMasked string `json:"session_key_masked"`
		Segments         []struct {
			SegmentIndex int `json:"segment_index"`
		} `json:"segments"`
		Messages []struct {
			Role           string `json:"role"`
			Text           string `json:"text"`
			SourceRecordID int64  `json:"source_record_id"`
		} `json:"messages"`
	}
	resp.DecodeData(t, &view)

	if view.CorrelationMode != "session_key" {
		t.Fatalf("会话键种子应精确关联，实际 %q", view.CorrelationMode)
	}
	if view.RecordCount != 3 || view.AnchorRecordID != id3 {
		t.Fatalf("记录数/锚点不符: count=%d anchor=%d", view.RecordCount, view.AnchorRecordID)
	}
	if len(view.Segments) != 1 {
		t.Fatalf("直线链应单段，实际 %d", len(view.Segments))
	}
	if len(view.Messages) != 7 {
		t.Fatalf("应还原 7 条消息（s,u1,a1,u2,a2,u3,a3），实际 %d", len(view.Messages))
	}
	wantRoles := []string{"system", "user", "assistant", "user", "assistant", "user", "assistant"}
	for i, want := range wantRoles {
		if view.Messages[i].Role != want {
			t.Fatalf("消息 %d 角色应 %s 实际 %s", i, want, view.Messages[i].Role)
		}
	}
	if view.Messages[6].Text != "a3" || view.Messages[6].SourceRecordID != id3 {
		t.Errorf("末条应为本轮响应 a3 来源 %d: %+v", id3, view.Messages[6])
	}

	// 列表筛选：会话来源
	listResp := client.Get("/api/admin/audit/request-logs", map[string]string{
		"session_source": "header",
		"page_size":      "5",
	})
	listResp.AssertSuccess(t)
}

func TestRequestAuditLogConversationNotFound(t *testing.T) {
	client := testinfra.GetAuthedClient(t)
	resp := client.Get("/api/admin/audit/request-logs/999999999/conversation", nil)
	resp.AssertError(t, 404) // consts.CodeNotFound
}

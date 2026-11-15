package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// 端到端：/v1/videos 请求携带 extra → 任务管线（ali 适配器 + mock 上游）→
// 上游收到平铺深合并后的请求体；命中保护键时预扣前 400、不发上游。

func submitVideosWithExtra(t *testing.T, raw string, upstreamURL string) *httptest.ResponseRecorder {
	t.Helper()
	req, terr := ParseVideosCreateRequest([]byte(raw), "application/json")
	if terr != nil {
		t.Fatalf("parse: %+v", terr)
	}
	if terr := ValidateVideosCreateRequest(req); terr != nil {
		t.Fatalf("validate: %+v", terr)
	}
	taskBody, err := BuildVideosTaskBody(req)
	if err != nil {
		t.Fatalf("build task body: %v", err)
	}
	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{
		TenantID: 1, UserID: 2, ApiKeyID: 3, RequestID: "req_extra_test", Writer: rec,
		Protocol: "openai_videos", RelayMode: int(constant.RelayModeVideos),
		RequestEcho: NewVideosRequestEcho(req),
	}
	HandleTaskSubmit(context.Background(), taskBody, "/v1/videos", http.Header{}, rc,
		&stubVideosSubmitProvider{}, &stubVideosBillingProvider{}, newAliChannelMeta(upstreamURL))
	return rec
}

func TestHandleTaskSubmit_ExtraMergedIntoUpstream(t *testing.T) {
	var received map[string]any
	upstream := newAliUpstream(t, &received)
	defer upstream.Close()

	rec := submitVideosWithExtra(t, `{
		"model": "wan3.0-video", "prompt": "小猫奔跑", "seconds": "5", "size": "720P",
		"extra": {"parameters": {"shot_type": "multi", "prompt_extend": false}, "input": {"negative_prompt": "模糊"}}
	}`, upstream.URL)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	params, _ := received["parameters"].(map[string]any)
	if params == nil || params["shot_type"] != "multi" || params["prompt_extend"] != false {
		t.Fatalf("extra 须深合并进 parameters: %v", received["parameters"])
	}
	if params["duration"] != float64(5) || params["resolution"] != "720P" {
		t.Fatalf("适配器生成的参数须保留: %v", params)
	}
	input, _ := received["input"].(map[string]any)
	if input == nil || input["prompt"] != "小猫奔跑" || input["negative_prompt"] != "模糊" {
		t.Fatalf("extra 须深合并进 input: %v", received["input"])
	}
	if _, leaked := received[common.ExtraFieldName]; leaked {
		t.Fatalf("extra 原键名不得外泄到上游: %v", received)
	}
}

func TestHandleTaskSubmit_ExtraProtectedRejectedBeforeUpstream(t *testing.T) {
	called := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	for _, extra := range []string{
		`{"parameters": {"duration": 15}}`,
		`{"parameters": {"resolution": "1080P"}}`,
		`{"parameters": null}`,
		`{"model": "wan3.0-pro"}`,
		`{"callback_url": "https://evil.example.com"}`,
	} {
		rec := submitVideosWithExtra(t, `{"model":"wan3.0-video","prompt":"p","extra":`+extra+`}`, upstream.URL)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), common.ErrCodeExtraFieldProtected) {
			t.Fatalf("%s: 应 400 %s, got %d %s", extra, common.ErrCodeExtraFieldProtected, rec.Code, rec.Body.String())
		}
	}
	if called {
		t.Fatal("命中保护键不得请求上游")
	}
}

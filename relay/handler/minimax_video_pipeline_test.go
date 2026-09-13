package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// 端到端集成测试：MiniMax 官方协议（/v2）与 OpenAI Videos 协议（/v1/videos）→ 转换 →
// 任务提交管线（minimax 适配器 + mock 上游）→ {"task_id"} 响应 + minimax 平台任务落库 +
// 上游收到映射后的 v2 参数。

// newMiniMaxUpstream mock MiniMax v2 上游：记录提交请求体，返回官方提交响应
func newMiniMaxUpstream(t *testing.T) (*httptest.Server, *map[string]any) {
	t.Helper()
	received := map[string]any{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/video_generation" {
			t.Errorf("unexpected upstream path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer sk-test" {
			t.Errorf("unexpected authorization: %q", auth)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &received); err != nil {
			t.Errorf("upstream body not json: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"task_id": "424010985738629"})
	}))
	return upstream, &received
}

func miniMaxChannelMeta(baseURL string) *common.ChannelMeta {
	return &common.ChannelMeta{
		ChannelID:         21,
		ChannelType:       int(constant.ProviderMiniMax),
		ChannelName:       "mock-minimax",
		BaseURL:           baseURL,
		ApiKey:            "sk-test",
		UpstreamModelName: "MiniMax-H3",
	}
}

// TestHandleTaskSubmit_MiniMaxNativeProtocol_FullPipeline 官方协议全链路：
// 原生 content[] 透传、callback_url 剥离、{"task_id"} 响应、minimax 平台落库、请求回显持久化。
func TestHandleTaskSubmit_MiniMaxNativeProtocol_FullPipeline(t *testing.T) {
	upstream, received := newMiniMaxUpstream(t)
	defer upstream.Close()

	// 官方请求 → 归一 → 通用任务体（与胶水层 HandleMiniMaxVideoSubmit 相同链路）
	raw := `{
		"model": "MiniMax-H3",
		"content": [
			{"type": "text", "text": "海浪拍打礁石，慢镜头。"},
			{"type": "image_url", "image_url": {"url": "https://cdn.example/first.png"}, "role": "first_frame"}
		],
		"resolution": "768P",
		"duration": 6,
		"ratio": "16:9",
		"callback_url": "https://evil.example/cb"
	}`
	req, taskErr := ParseMiniMaxVideoCreateRequest([]byte(raw))
	if taskErr != nil {
		t.Fatalf("parse: %+v", taskErr)
	}
	if taskErr := ValidateMiniMaxVideoCreateRequest(req); taskErr != nil {
		t.Fatalf("validate: %+v", taskErr)
	}
	taskBody, err := BuildMiniMaxVideoTaskBody(req)
	if err != nil {
		t.Fatalf("build task body: %v", err)
	}

	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{
		TenantID:    1,
		UserID:      2,
		ApiKeyID:    3,
		RequestID:   "req_mm_native",
		Writer:      rec,
		Protocol:    minimaxVideoProtocol,
		RelayMode:   int(constant.RelayModeMiniMaxVideo),
		RequestEcho: NewMiniMaxRequestEcho(req),
	}
	data := &stubVideosSubmitProvider{}

	HandleTaskSubmit(context.Background(), taskBody, "/v2/video_generation", http.Header{}, rc, data, &stubVideosBillingProvider{}, miniMaxChannelMeta(upstream.URL))

	// 客户端响应：官方 VideoGenerationV2Resp 形态
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeVideosJSON(t, rec)
	if len(resp) != 1 {
		t.Fatalf("submit response should only carry task_id: %s", rec.Body.String())
	}
	taskID, _ := resp["task_id"].(string)
	if !strings.HasPrefix(taskID, "task_") {
		t.Fatalf("expected task_ prefixed id, got %q", taskID)
	}
	if rc.TaskID != taskID {
		t.Fatalf("rc.TaskID=%q should match response id %q", rc.TaskID, taskID)
	}

	// 落库任务：minimax 平台 + 上游任务 ID + 请求回显
	if data.created == nil {
		t.Fatal("expected task record created")
	}
	if data.created.Platform != "minimax" {
		t.Fatalf("expected platform minimax, got %q", data.created.Platform)
	}
	var pd struct {
		UpstreamTaskID string                   `json:"upstream_task_id"`
		RequestEcho    *minimaxVideoRequestEcho `json:"request_echo"`
	}
	if err := json.Unmarshal(data.created.PrivateData, &pd); err != nil {
		t.Fatalf("private data unmarshal: %v", err)
	}
	if pd.UpstreamTaskID != "424010985738629" {
		t.Fatalf("upstream task id: %q", pd.UpstreamTaskID)
	}
	if pd.RequestEcho == nil || pd.RequestEcho.Resolution != "768P" || pd.RequestEcho.Duration != 6 {
		t.Fatalf("request echo persisted: %+v", pd.RequestEcho)
	}

	// 上游收到：content 原样（2 项）、官方参数、无 callback_url
	if (*received)["model"] != "MiniMax-H3" {
		t.Fatalf("upstream model: %v", (*received)["model"])
	}
	if (*received)["resolution"] != "768P" || (*received)["duration"] != float64(6) || (*received)["ratio"] != "16:9" {
		t.Fatalf("upstream params: %v", *received)
	}
	if _, has := (*received)["callback_url"]; has {
		t.Fatal("callback_url must be stripped before forwarding upstream")
	}
	content, _ := (*received)["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("expected content passthrough with 2 items, got %v", (*received)["content"])
	}
	img, _ := content[1].(map[string]any)
	if img["role"] != "first_frame" {
		t.Fatalf("content roles preserved: %v", content[1])
	}
}

// TestHandleTaskSubmit_OpenAIVideosToMiniMax OpenAI Videos 协议 → minimax 适配器：
// size 原值透传（调用方直接传 MiniMax 原生档位词汇）、seconds → duration、multipart 链路复用。
func TestHandleTaskSubmit_OpenAIVideosToMiniMax(t *testing.T) {
	upstream, received := newMiniMaxUpstream(t)
	defer upstream.Close()

	// OpenAI multipart 创建请求：size 直传 MiniMax 原生档位 768P + 8 秒
	mBody, ct := buildMultipartVideosBody(t, map[string]string{
		"model":   "MiniMax-H3",
		"prompt":  "a cat surfing",
		"seconds": "8",
		"size":    "768P",
	}, "", "", nil)
	req, taskErr := ParseVideosCreateRequest(mBody, ct)
	if taskErr != nil {
		t.Fatalf("parse: %+v", taskErr)
	}
	taskBody, err := BuildVideosTaskBody(req)
	if err != nil {
		t.Fatalf("build task body: %v", err)
	}

	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{
		TenantID:    1,
		UserID:      2,
		ApiKeyID:    3,
		RequestID:   "req_mm_openai",
		Writer:      rec,
		Protocol:    videosProtocolOpenAI,
		RelayMode:   int(constant.RelayModeVideos),
		RequestEcho: NewVideosRequestEcho(req),
	}
	data := &stubVideosSubmitProvider{}

	HandleTaskSubmit(context.Background(), taskBody, "/v1/videos", http.Header{}, rc, data, &stubVideosBillingProvider{}, miniMaxChannelMeta(upstream.URL))

	// OpenAI 协议响应保持官方 Video 对象
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeVideosJSON(t, rec)
	if resp["object"] != "video" || resp["status"] != "queued" {
		t.Fatalf("openai video object: %s", rec.Body.String())
	}
	if data.created == nil || data.created.Platform != "minimax" {
		t.Fatalf("expected minimax task record, got %+v", data.created)
	}

	// 上游收到映射后的 v2 参数：size 原值透传、seconds→duration、t2v 默认 ratio
	if (*received)["resolution"] != "768P" {
		t.Fatalf("upstream resolution passthrough: %v", (*received)["resolution"])
	}
	if (*received)["duration"] != float64(8) {
		t.Fatalf("upstream duration: %v", (*received)["duration"])
	}
	if (*received)["ratio"] != "16:9" {
		t.Fatalf("upstream ratio: %v", (*received)["ratio"])
	}
	content, _ := (*received)["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("text-only content expected, got %v", (*received)["content"])
	}
	text, _ := content[0].(map[string]any)
	if text["text"] != "a cat surfing" {
		t.Fatalf("text item: %v", content[0])
	}
}

// TestHandleTaskSubmit_MiniMaxV1NativeProtocol_FullPipeline v1 官方协议全链路：
// 扁平字段透传、callback_url 剥离、{task_id, base_resp} 响应、minimax 平台落库。
func TestHandleTaskSubmit_MiniMaxV1NativeProtocol_FullPipeline(t *testing.T) {
	// mock MiniMax v1 上游：记录提交请求体，返回官方提交响应
	received := map[string]any{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/video_generation" {
			t.Errorf("unexpected upstream path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &received); err != nil {
			t.Errorf("upstream body not json: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"task_id":   "v1-up-777",
			"base_resp": map[string]any{"status_code": 0, "status_msg": "success"},
		})
	}))
	defer upstream.Close()

	raw := `{
		"model": "MiniMax-Hailuo-2.3",
		"prompt": "一只猫在月光下的屋顶上行走",
		"first_frame_image": "https://cdn.example/f.png",
		"duration": 10,
		"resolution": "768P",
		"callback_url": "https://evil.example/cb"
	}`
	req, taskErr := ParseMiniMaxVideoV1CreateRequest([]byte(raw))
	if taskErr != nil {
		t.Fatalf("parse: %+v", taskErr)
	}
	if taskErr := ValidateMiniMaxVideoV1CreateRequest(req); taskErr != nil {
		t.Fatalf("validate: %+v", taskErr)
	}
	taskBody, err := BuildMiniMaxVideoV1TaskBody(req)
	if err != nil {
		t.Fatalf("build task body: %v", err)
	}

	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{
		TenantID:    1,
		UserID:      2,
		ApiKeyID:    3,
		RequestID:   "req_mm_v1_native",
		Writer:      rec,
		Protocol:    minimaxVideoV1Protocol,
		RelayMode:   int(constant.RelayModeMiniMaxVideo),
		RequestEcho: NewMiniMaxV1RequestEcho(req),
	}
	data := &stubVideosSubmitProvider{}
	channelMeta := &common.ChannelMeta{
		ChannelID:         22,
		ChannelType:       int(constant.ProviderMiniMax),
		ChannelName:       "mock-minimax",
		BaseURL:           upstream.URL,
		ApiKey:            "sk-test",
		UpstreamModelName: "MiniMax-Hailuo-2.3",
	}

	HandleTaskSubmit(context.Background(), taskBody, "/v1/video_generation", http.Header{}, rc, data, &stubVideosBillingProvider{}, channelMeta)

	// 客户端响应：官方 VideoGenerationResp 形态（含 base_resp 成功信封）
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeVideosJSON(t, rec)
	taskID, _ := resp["task_id"].(string)
	if !strings.HasPrefix(taskID, "task_") {
		t.Fatalf("expected task_ prefixed id, got %q", taskID)
	}
	baseResp, _ := resp["base_resp"].(map[string]any)
	if baseResp == nil || baseResp["status_code"] != float64(0) {
		t.Fatalf("base_resp success envelope: %s", rec.Body.String())
	}

	// 落库任务：minimax 平台 + 上游任务 ID + 请求回显
	if data.created == nil || data.created.Platform != "minimax" {
		t.Fatalf("expected minimax task record, got %+v", data.created)
	}
	var pd struct {
		UpstreamTaskID string                     `json:"upstream_task_id"`
		RequestEcho    *minimaxVideoV1RequestEcho `json:"request_echo"`
	}
	if err := json.Unmarshal(data.created.PrivateData, &pd); err != nil {
		t.Fatalf("private data unmarshal: %v", err)
	}
	if pd.UpstreamTaskID != "v1-up-777" {
		t.Fatalf("upstream task id: %q", pd.UpstreamTaskID)
	}
	if pd.RequestEcho == nil || pd.RequestEcho.Duration != 10 || pd.RequestEcho.Resolution != "768P" {
		t.Fatalf("request echo persisted: %+v", pd.RequestEcho)
	}

	// 上游收到：v1 扁平字段、无 content[]、无 callback_url
	if received["model"] != "MiniMax-Hailuo-2.3" || received["prompt"] != "一只猫在月光下的屋顶上行走" {
		t.Fatalf("upstream flat fields: %v", received)
	}
	if received["first_frame_image"] != "https://cdn.example/f.png" || received["duration"] != float64(10) || received["resolution"] != "768P" {
		t.Fatalf("upstream params: %v", received)
	}
	if _, has := received["callback_url"]; has {
		t.Fatal("callback_url must be stripped before forwarding upstream")
	}
	if _, has := received["content"]; has {
		t.Fatal("v1 request must not carry content[]")
	}
}

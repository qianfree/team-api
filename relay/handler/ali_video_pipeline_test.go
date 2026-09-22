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

// 端到端集成测试：阿里 DashScope 官方协议 / OpenAI Videos 协议 → 转换 → 任务提交管线
// （ali 适配器 + mock 上游）→ task_ 前缀 ID + DashScope 官方响应形态 + 落库 platform=ali
// + 上游收到 media 数组与映射参数。

// newAliUpstream 构造 mock 阿里 DashScope 上游（校验端点/异步头，记录请求体）
func newAliUpstream(t *testing.T, received *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/v1/services/aigc/video-generation/video-synthesis") {
			t.Errorf("unexpected upstream path: %s", r.URL.Path)
		}
		if r.Header.Get("X-DashScope-Async") != "enable" {
			t.Errorf("missing X-DashScope-Async header: %v", r.Header)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, received); err != nil {
			t.Errorf("upstream body not json: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output":     map[string]any{"task_status": "PENDING", "task_id": "ali-up-1"},
			"request_id": "up-req-1",
		})
	}))
}

func newAliChannelMeta(upstreamURL string) *common.ChannelMeta {
	return &common.ChannelMeta{
		ChannelID:         21,
		ChannelType:       int(constant.ProviderAli),
		ChannelName:       "mock-ali",
		BaseURL:           upstreamURL,
		ApiKey:            "sk-test",
		UpstreamModelName: "wan3.0-video",
	}
}

func TestHandleTaskSubmit_AliNativeProtocol_FullPipeline(t *testing.T) {
	var upstreamReceived map[string]any
	upstream := newAliUpstream(t, &upstreamReceived)
	defer upstream.Close()

	// 官方请求体 → 归一 → 通用任务体（与胶水层 HandleAliVideoSubmit 相同链路）
	raw := `{
		"model": "wan3.0-video",
		"input": {
			"prompt": "小猫奔跑",
			"media": [{"type": "first_frame", "url": "https://example.com/f.png"}]
		},
		"parameters": {"resolution": "480P", "ratio": "16:9", "duration": 5, "audio": false}
	}`
	req, taskErr := ParseAliVideoCreateRequest([]byte(raw))
	if taskErr != nil {
		t.Fatalf("parse: %+v", taskErr)
	}
	taskBody, err := BuildAliVideoTaskBody(req)
	if err != nil {
		t.Fatalf("build task body: %v", err)
	}

	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{
		TenantID:  1,
		UserID:    2,
		ApiKeyID:  3,
		RequestID: "req_ali_test",
		Writer:    rec,
		Protocol:  aliVideoProtocol,
		RelayMode: int(constant.RelayModeAliVideo),
	}
	data := &stubVideosSubmitProvider{}

	HandleTaskSubmit(context.Background(), taskBody, "/api/v1/services/aigc/video-generation/video-synthesis",
		http.Header{}, rc, data, &stubVideosBillingProvider{}, newAliChannelMeta(upstream.URL))

	// 客户端响应：DashScope 官方形态
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeVideosJSON(t, rec)
	output, _ := resp["output"].(map[string]any)
	if output == nil || output["task_status"] != "PENDING" {
		t.Fatalf("unexpected dashscope output: %s", rec.Body.String())
	}
	taskID, _ := output["task_id"].(string)
	if !strings.HasPrefix(taskID, "task_") {
		t.Fatalf("expected task_ prefixed id, got %q", taskID)
	}
	if resp["request_id"] != "req_ali_test" {
		t.Fatalf("expected platform request id: %s", rec.Body.String())
	}
	if rc.TaskID != taskID {
		t.Fatalf("rc.TaskID=%q should match response id %q", rc.TaskID, taskID)
	}

	// 落库任务：ali 平台 + 上游任务 ID
	if data.created == nil || data.created.Platform != "ali" {
		t.Fatalf("expected ali task record, got %+v", data.created)
	}
	var pd struct {
		UpstreamTaskID string `json:"upstream_task_id"`
	}
	if err := json.Unmarshal(data.created.PrivateData, &pd); err != nil {
		t.Fatalf("private data unmarshal: %v", err)
	}
	if pd.UpstreamTaskID != "ali-up-1" {
		t.Fatalf("expected upstream task id captured, got %q", pd.UpstreamTaskID)
	}

	// 上游收到：media 数组透传 + parameters 映射（resolution/ratio/duration/audio）
	input, _ := upstreamReceived["input"].(map[string]any)
	if input == nil || input["prompt"] != "小猫奔跑" {
		t.Fatalf("upstream input: %v", upstreamReceived["input"])
	}
	media, _ := input["media"].([]any)
	if len(media) != 1 {
		t.Fatalf("expected media array forwarded, got %v", input["media"])
	}
	first, _ := media[0].(map[string]any)
	if first["type"] != "first_frame" || first["url"] != "https://example.com/f.png" {
		t.Fatalf("unexpected media item: %v", media[0])
	}
	params, _ := upstreamReceived["parameters"].(map[string]any)
	if params == nil || params["resolution"] != "480P" || params["ratio"] != "16:9" ||
		params["duration"] != float64(5) || params["audio"] != false {
		t.Fatalf("upstream parameters: %v", upstreamReceived["parameters"])
	}
}

func TestHandleTaskSubmit_OpenAIVideosToWan3_Pipeline(t *testing.T) {
	// OpenAI Videos 协议（multipart）→ BuildVideosTaskBody → ali 适配器：
	// input_reference → media[{type:first_frame}]、seconds → duration、size 短边 → resolution、
	// sound → audio
	var upstreamReceived map[string]any
	upstream := newAliUpstream(t, &upstreamReceived)
	defer upstream.Close()

	mBody := []byte(`{"model":"wan3.0-video","prompt":"小猫对着镜头打哈欠","seconds":"8","size":"1280x720","aspect_ratio":"9:16","sound":"on","input_reference":{"image_url":"https://example.com/ref.png"}}`)
	req, taskErr := ParseVideosCreateRequest(mBody, "application/json")
	if taskErr != nil {
		t.Fatalf("parse: %+v", taskErr)
	}
	taskBody, err := BuildVideosTaskBody(req)
	if err != nil {
		t.Fatalf("build task body: %v", err)
	}

	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{
		TenantID:  1,
		UserID:    2,
		ApiKeyID:  3,
		RequestID: "req_ali_openai_test",
		Writer:    rec,
		Protocol:  aliVideoProtocol,
		RelayMode: int(constant.RelayModeAliVideo),
	}
	data := &stubVideosSubmitProvider{}

	HandleTaskSubmit(context.Background(), taskBody, "/api/v1/services/aigc/video-generation/video-synthesis",
		http.Header{}, rc, data, &stubVideosBillingProvider{}, newAliChannelMeta(upstream.URL))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if data.created == nil || data.created.Platform != "ali" {
		t.Fatalf("expected ali task record, got %+v", data.created)
	}

	// 上游收到映射后的 wan3.0 参数
	input, _ := upstreamReceived["input"].(map[string]any)
	media, _ := input["media"].([]any)
	if len(media) != 1 {
		t.Fatalf("expected first_frame media from input_reference, got %v", input["media"])
	}
	first, _ := media[0].(map[string]any)
	if first["type"] != "first_frame" {
		t.Fatalf("input_reference should map to first_frame: %v", media[0])
	}
	params, _ := upstreamReceived["parameters"].(map[string]any)
	if params == nil || params["duration"] != float64(8) {
		t.Fatalf("seconds → duration mapping lost: %v", upstreamReceived["parameters"])
	}
	if params["resolution"] != "720P" {
		t.Fatalf("size short-edge → resolution mapping lost: %v", upstreamReceived["parameters"])
	}
	if params["ratio"] != "9:16" {
		t.Fatalf("aspect_ratio → ratio mapping lost: %v", upstreamReceived["parameters"])
	}
	if params["audio"] != true {
		t.Fatalf("sound → audio mapping lost: %v", upstreamReceived["parameters"])
	}
}

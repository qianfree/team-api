package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// 端到端集成测试：OpenAI Videos 协议（multipart）→ 转换 → 任务提交管线（kling 适配器 + mock 上游）
// → video_ 前缀 ID + 官方 Video 对象响应 + 请求回显落 PrivateData + 上游收到映射后的 kling 参数。

// stubVideosBillingProvider 提交路径的 TaskBillingProvider 桩（计费全放行，金额固定）
type stubVideosBillingProvider struct {
	common.TaskBillingProvider
}

func (s *stubVideosBillingProvider) EstimateTaskCost(context.Context, int64, string, map[string]any, []byte) (decimal.Decimal, error) {
	return decimal.NewFromFloat(0.1), nil
}

func (s *stubVideosBillingProvider) PreDeductTask(_ context.Context, _ int64, _ string, cost decimal.Decimal, _ string) (decimal.Decimal, error) {
	return cost, nil
}

func (s *stubVideosBillingProvider) CheckRateLimit(context.Context, int64, int64, int64, int) (bool, string, int, int, int64) {
	return true, "", 0, 0, 0
}

func (s *stubVideosBillingProvider) AcquireApiKeyConcurrent(context.Context, int64, int) bool {
	return true
}
func (s *stubVideosBillingProvider) ReleaseApiKeyConcurrent(context.Context, int64) {}
func (s *stubVideosBillingProvider) CheckApiKeyQuota(context.Context, int64, decimal.Decimal) error {
	return nil
}

func (s *stubVideosBillingProvider) AdjustTaskBilling(_ context.Context, _ int64, _ string, _, newCost decimal.Decimal) (decimal.Decimal, error) {
	return newCost, nil
}

// stubVideosSubmitProvider 提交路径的 TaskDataProvider 桩（记录创建的任务）
type stubVideosSubmitProvider struct {
	common.TaskDataProvider

	created *common.AsyncTask
}

func (s *stubVideosSubmitProvider) CreateTask(_ context.Context, task *common.AsyncTask) error {
	s.created = task
	return nil
}

func (s *stubVideosSubmitProvider) GetTaskByPublicIDAndUser(_ context.Context, publicTaskID string, _ int64, _ int64) (*common.AsyncTask, error) {
	if s.created != nil && s.created.PublicTaskID == publicTaskID {
		return s.created, nil
	}
	return nil, nil
}

func TestHandleTaskSubmit_OpenAIVideosProtocol_FullPipeline(t *testing.T) {
	// 1. mock kling 上游：记录收到的请求体，返回标准提交成功响应
	var upstreamReceived map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1/videos/text2video") {
			t.Errorf("unexpected upstream path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &upstreamReceived); err != nil {
			t.Errorf("upstream body not json: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "success",
			"data":    map[string]any{"task_id": "kling-up-1", "task_status": "submitted"},
		})
	}))
	defer upstream.Close()

	// 2. multipart 创建请求 → 归一 → 通用任务体（与胶水层 HandleVideoCreate 相同链路）
	mBody, ct := buildMultipartVideosBody(t, map[string]string{
		"model":   "kling-v2-master",
		"prompt":  "a cat surfing",
		"seconds": "8",
		"size":    "1280x720",
	}, "", "", nil)
	req, taskErr := ParseVideosCreateRequest(mBody, ct)
	if taskErr != nil {
		t.Fatalf("parse: %+v", taskErr)
	}
	taskBody, err := BuildVideosTaskBody(req)
	if err != nil {
		t.Fatalf("build task body: %v", err)
	}

	// 3. 跑完整提交管线（kling 适配器 + mock 上游 + 计费/存储桩）
	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{
		TenantID:    1,
		UserID:      2,
		ApiKeyID:    3,
		RequestID:   "req_videos_test",
		Writer:      rec,
		Protocol:    videosProtocolOpenAI,
		RelayMode:   int(constant.RelayModeVideos),
		RequestEcho: NewVideosRequestEcho(req),
	}
	channelMeta := &common.ChannelMeta{
		ChannelID:         11,
		ChannelType:       int(constant.ProviderKling),
		ChannelName:       "mock-kling",
		BaseURL:           upstream.URL,
		ApiKey:            "sk-test",
		UpstreamModelName: "kling-v2-master",
	}
	billing := &stubVideosBillingProvider{}
	data := &stubVideosSubmitProvider{}

	HandleTaskSubmit(context.Background(), taskBody, "/v1/videos", http.Header{}, rc, data, billing, channelMeta)

	// 4. 断言客户端响应：官方 Video 对象
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeVideosJSON(t, rec)
	if resp["object"] != "video" || resp["status"] != "queued" || resp["progress"] != float64(0) {
		t.Fatalf("unexpected video object: %s", rec.Body.String())
	}
	videoID, _ := resp["id"].(string)
	if !strings.HasPrefix(videoID, "video_") {
		t.Fatalf("expected video_ prefixed id, got %q", videoID)
	}
	if resp["model"] != "kling-v2-master" || resp["seconds"] != "8" || resp["size"] != "1280x720" {
		t.Fatalf("expected echo fields in submit response: %s", rec.Body.String())
	}
	if prompt, _ := resp["prompt"].(string); prompt != "a cat surfing" {
		t.Fatalf("expected prompt echo: %s", rec.Body.String())
	}
	if rc.TaskID != videoID {
		t.Fatalf("rc.TaskID=%q should match response id %q", rc.TaskID, videoID)
	}

	// 5. 断言落库任务：video_ ID、kling 平台、PrivateData 含 request_echo
	if data.created == nil {
		t.Fatal("expected task record created")
	}
	if data.created.PublicTaskID != videoID {
		t.Fatalf("created task id=%q", data.created.PublicTaskID)
	}
	if data.created.Platform != "kling" {
		t.Fatalf("expected platform kling, got %q", data.created.Platform)
	}
	var pd struct {
		UpstreamTaskID string             `json:"upstream_task_id"`
		RequestEcho    *videosRequestEcho `json:"request_echo"`
	}
	if err := json.Unmarshal(data.created.PrivateData, &pd); err != nil {
		t.Fatalf("private data unmarshal: %v", err)
	}
	if pd.UpstreamTaskID != "kling-up-1" {
		t.Fatalf("expected upstream task id captured, got %q", pd.UpstreamTaskID)
	}
	if pd.RequestEcho == nil || pd.RequestEcho.Seconds != "8" || pd.RequestEcho.Size != "1280x720" {
		t.Fatalf("expected request echo persisted: %+v", pd.RequestEcho)
	}

	// 6. 断言上游收到映射后的 kling 参数（seconds→duration、size→aspect_ratio）
	if upstreamReceived["model_name"] != "kling-v2-master" {
		t.Fatalf("upstream model_name: %v", upstreamReceived["model_name"])
	}
	if upstreamReceived["duration"] != "8" {
		t.Fatalf("upstream duration: %v", upstreamReceived["duration"])
	}
	if upstreamReceived["aspect_ratio"] != "16:9" {
		t.Fatalf("upstream aspect_ratio: %v", upstreamReceived["aspect_ratio"])
	}
	if upstreamReceived["prompt"] != "a cat surfing" {
		t.Fatalf("upstream prompt: %v", upstreamReceived["prompt"])
	}
}

func TestHandleTaskSubmit_OpenAIVideosProtocol_Seedance(t *testing.T) {
	// mock 火山引擎（seedance）上游：提交端点返回任务 ID
	var upstreamReceived map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/v3/contents/generations/tasks") {
			t.Errorf("unexpected upstream path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &upstreamReceived); err != nil {
			t.Errorf("upstream body not json: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "cgt-seedance-1"})
	}))
	defer upstream.Close()

	// multipart 创建请求：图生视频（input_reference 文件）+ 高清横屏档
	pngBytes := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mBody, ct := buildMultipartVideosBody(t, map[string]string{
		"model":   "doubao-seedance-2-0-260128",
		"prompt":  "小猫对着镜头打哈欠",
		"seconds": "12",
		"size":    "1792x1024",
	}, "input_reference", "ref.png", pngBytes)
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
		RequestID:   "req_seedance_test",
		Writer:      rec,
		Protocol:    videosProtocolOpenAI,
		RelayMode:   int(constant.RelayModeVideos),
		RequestEcho: NewVideosRequestEcho(req),
	}
	channelMeta := &common.ChannelMeta{
		ChannelID:         12,
		ChannelType:       int(constant.ProviderVolcengine),
		ChannelName:       "mock-volcengine",
		BaseURL:           upstream.URL,
		ApiKey:            "sk-test",
		UpstreamModelName: "doubao-seedance-2-0-260128",
	}
	data := &stubVideosSubmitProvider{}

	HandleTaskSubmit(context.Background(), taskBody, "/v1/videos", http.Header{}, rc, data, &stubVideosBillingProvider{}, channelMeta)

	// 客户端响应：官方 Video 对象
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeVideosJSON(t, rec)
	if resp["object"] != "video" || resp["status"] != "queued" {
		t.Fatalf("unexpected video object: %s", rec.Body.String())
	}
	if resp["seconds"] != "12" || resp["size"] != "1792x1024" {
		t.Fatalf("expected echo fields: %s", rec.Body.String())
	}
	if data.created == nil || data.created.Platform != "volcengine" {
		t.Fatalf("expected volcengine task record, got %+v", data.created)
	}

	// 上游收到的 seedance 参数：content 文本+首帧图、duration/ratio/resolution 全部对上
	if upstreamReceived["model"] != "doubao-seedance-2-0-260128" {
		t.Fatalf("upstream model: %v", upstreamReceived["model"])
	}
	if upstreamReceived["duration"] != float64(12) {
		t.Fatalf("upstream duration: %v (%T)", upstreamReceived["duration"], upstreamReceived["duration"])
	}
	if upstreamReceived["ratio"] != "16:9" {
		t.Fatalf("upstream ratio: %v", upstreamReceived["ratio"])
	}
	if upstreamReceived["resolution"] != "1080p" {
		t.Fatalf("upstream resolution: %v", upstreamReceived["resolution"])
	}
	content, ok := upstreamReceived["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("expected text + first_frame image content items, got %v", upstreamReceived["content"])
	}
	textItem, _ := content[0].(map[string]any)
	if textItem["type"] != "text" || textItem["text"] != "小猫对着镜头打哈欠" {
		t.Fatalf("unexpected text item: %v", content[0])
	}
	imgItem, _ := content[1].(map[string]any)
	if imgItem["type"] != "image_url" {
		t.Fatalf("expected image_url item, got %v", content[1])
	}
	imgURL, _ := imgItem["image_url"].(map[string]any)
	urlStr, _ := imgURL["url"].(string)
	if !strings.HasPrefix(urlStr, "data:image/png;base64,") {
		t.Fatalf("expected base64 data url as first frame, got %.40q", urlStr)
	}
}

func TestHandleTaskSubmit_LegacyProtocolUnchanged(t *testing.T) {
	// 回归：Protocol 为空（legacy）时提交响应保持 {id: task_*, status: SUBMITTED} 旧格式
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "success",
			"data":    map[string]any{"task_id": "kling-up-2", "task_status": "submitted"},
		})
	}))
	defer upstream.Close()

	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{
		TenantID:  1,
		UserID:    2,
		ApiKeyID:  3,
		RequestID: "req_legacy_test",
		Writer:    rec,
	}
	channelMeta := &common.ChannelMeta{
		ChannelID:         11,
		ChannelType:       int(constant.ProviderKling),
		BaseURL:           upstream.URL,
		ApiKey:            "sk-test",
		UpstreamModelName: "kling-v2-master",
	}
	body := []byte(`{"model":"kling-v2-master","prompt":"legacy","metadata":{"duration":5}}`)
	HandleTaskSubmit(context.Background(), body, "/v1/video/generations", http.Header{}, rc, &stubVideosSubmitProvider{}, &stubVideosBillingProvider{}, channelMeta)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeVideosJSON(t, rec)
	if resp["status"] != "SUBMITTED" {
		t.Fatalf("legacy submit must keep SUBMITTED status: %s", rec.Body.String())
	}
	if id, _ := resp["id"].(string); !strings.HasPrefix(id, "task_") {
		t.Fatalf("legacy submit must keep task_ prefix: %s", rec.Body.String())
	}
	if _, hasObject := resp["object"]; hasObject {
		t.Fatalf("legacy submit must not carry video object field: %s", rec.Body.String())
	}
}

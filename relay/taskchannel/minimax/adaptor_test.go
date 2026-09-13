package minimax

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

// testInfo 构造带渠道元数据的 RelayInfo
func testInfo(baseURL, upstreamModel string, mapped bool) *common.RelayInfo {
	return &common.RelayInfo{
		ChannelMeta: &common.ChannelMeta{
			ChannelID:         1,
			ChannelType:       int(constant.ProviderMiniMax),
			BaseURL:           baseURL,
			ApiKey:            "sk-test",
			UpstreamModelName: upstreamModel,
			IsModelMapped:     mapped,
		},
	}
}

func buildBody(t *testing.T, a *Adaptor, info *common.RelayInfo, body string) map[string]any {
	t.Helper()
	reader, err := a.BuildRequestBody(context.Background(), info, []byte(body))
	if err != nil {
		t.Fatalf("build request body: %v", err)
	}
	raw, _ := io.ReadAll(reader)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("upstream body not json: %v, raw=%s", err, string(raw))
	}
	return out
}

// TestNormalizeResolution 仅做两代协议官方档位的大小写规范（计费矩阵键逐字符匹配），
// 非官方值原样透传、由上游裁决——不做任何档位换算。
func TestNormalizeResolution(t *testing.T) {
	cases := map[string]string{
		"480P":        "480P",
		"480p":        "480P",
		" 480P ":      "480P",
		"720p":        "720P",
		"768P":        "768P",
		"768p":        "768P",
		"1080p":       "1080P", // v1 官方档位：仅规范大小写
		"512p":        "512P",
		"2K":          "2K",
		"2k":          "2K",
		"1280x720":    "1280x720", // 非官方档位：原样透传（上游 400 + 退款）
		"custom-tier": "custom-tier",
		"":            "",
	}
	for in, want := range cases {
		if got := normalizeResolution(in); got != want {
			t.Errorf("normalizeResolution(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateRequest(t *testing.T) {
	a := &Adaptor{}

	// 通用形态：prompt 必填
	if err := a.ValidateRequest(context.Background(), nil, []byte(`{"model":"MiniMax-H3"}`)); err == nil {
		t.Error("expected error when prompt missing in generic form")
	}
	if err := a.ValidateRequest(context.Background(), nil, []byte(`{"model":"MiniMax-H3","prompt":"hi"}`)); err != nil {
		t.Errorf("generic form with prompt should pass: %v", err)
	}

	// 原生形态：content 必含非空 text 项
	if err := a.ValidateRequest(context.Background(), nil, []byte(`{"model":"MiniMax-H3","content":[{"type":"image_url","image_url":{"url":"https://x/a.png"}}]}`)); err == nil {
		t.Error("expected error when native content has no text item")
	}
	if err := a.ValidateRequest(context.Background(), nil, []byte(`{"model":"MiniMax-H3","content":[{"type":"text","text":"  "}]}`)); err == nil {
		t.Error("expected error when native content text item is blank")
	}
	if err := a.ValidateRequest(context.Background(), nil, []byte(`{"model":"MiniMax-H3","content":[{"type":"text","text":"海浪"},{"type":"image_url","image_url":{"url":"https://x/a.png"}}]}`)); err != nil {
		t.Errorf("native form with text should pass: %v", err)
	}
}

// TestEstimateBilling_OnlySpecKeys 计费上下文只能包含 spec.* 两个键：
// ratios 里的 float64 值会被计费引擎连乘，混入非乘数键即资损向量。
func TestEstimateBilling_OnlySpecKeys(t *testing.T) {
	a := &Adaptor{}
	ratios := a.EstimateBilling(context.Background(), nil, []byte(`{"model":"MiniMax-H3","prompt":"hi","seconds":"8","metadata":{"size":"2K"}}`))

	if len(ratios) != 2 {
		t.Fatalf("expected exactly 2 ratio keys, got %d: %v", len(ratios), ratios)
	}
	if d, ok := ratios["spec.duration"].(float64); !ok || d != 8 {
		t.Fatalf("spec.duration = %v (%T), want float64(8)", ratios["spec.duration"], ratios["spec.duration"])
	}
	if r := ratios["spec.resolution"]; r != "2K" {
		t.Fatalf("spec.resolution = %v, want 2K", r)
	}
}

// TestBillingBuildConsistency EstimateBilling 与 BuildRequestBody 必须共用同一归一化：
// 同一输入下，计费 spec 值必须与上游实际收到的规格一致（矩阵键逐字符匹配）。
func TestBillingBuildConsistency(t *testing.T) {
	a := &Adaptor{}
	inputs := []string{
		`{"model":"MiniMax-H3","prompt":"hi","seconds":"8","metadata":{"resolution":"1080p"}}`,
		`{"model":"MiniMax-H3","prompt":"hi","metadata":{"resolution":"1280x720","duration":4}}`,
		`{"model":"MiniMax-H3","prompt":"hi"}`,
		`{"model":"MiniMax-H3","content":[{"type":"text","text":"hi"}],"resolution":"2K","duration":15}`,
		`{"model":"MiniMax-H3-Max","content":[{"type":"text","text":"hi"}],"resolution":"480P","duration":5}`,
		// /v1/videos 入站：size 命名档位（未经四档映射，仅 size 原值透传）走回退归一化
		`{"model":"MiniMax-H3","prompt":"hi","seconds":"6","metadata":{"size":"2K"}}`,
		`{"model":"MiniMax-H3","prompt":"hi","seconds":"6","metadata":{"size":"768P"}}`,
	}
	for _, in := range inputs {
		ratios := a.EstimateBilling(context.Background(), nil, []byte(in))
		out := buildBody(t, a, testInfo("", "MiniMax-H3", false), in)

		billDur := int(ratios["spec.duration"].(float64))
		if upDur := int(out["duration"].(float64)); billDur != upDur {
			t.Errorf("input=%s: billing duration %d != upstream duration %d", in, billDur, upDur)
		}
		if ratios["spec.resolution"] != out["resolution"] {
			t.Errorf("input=%s: billing resolution %v != upstream resolution %v", in, ratios["spec.resolution"], out["resolution"])
		}
	}
}

// TestResolveResolution_SizeFallback metadata.resolution 优先于 metadata.size；
// 仅 size 存在时作为分辨率信号（/v1/videos 的分辨率承载在 size 字段，原值透传）。
func TestResolveResolution_SizeFallback(t *testing.T) {
	cases := []struct {
		name string
		req  map[string]any
		want string
	}{
		{"resolution wins", map[string]any{"metadata": map[string]any{"resolution": "2K", "size": "768P"}}, "2K"},
		{"size named tier", map[string]any{"metadata": map[string]any{"size": "768P"}}, "768P"},
		{"size lowercase tier", map[string]any{"metadata": map[string]any{"size": "2k"}}, "2K"},
		{"size non-tier passthrough", map[string]any{"metadata": map[string]any{"size": "1536p"}}, "1536p"},
		{"top-level wins all", map[string]any{"resolution": "480P", "metadata": map[string]any{"resolution": "2K", "size": "768P"}}, "480P"},
		{"default for h3 model", map[string]any{"model": "MiniMax-H3"}, "768P"},
		{"default for hailuo model", map[string]any{"model": "MiniMax-Hailuo-2.3"}, "768P"},
		{"default for legacy model", map[string]any{"model": "T2V-01"}, "720P"},
	}
	for _, c := range cases {
		if got := resolveResolution(c.req); got != c.want {
			t.Errorf("%s: resolveResolution = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestBuildRequestBody_NativePassthrough(t *testing.T) {
	a := &Adaptor{}
	body := `{
		"model": "MiniMax-H3",
		"content": [
			{"type": "text", "text": "海浪拍打礁石"},
			{"type": "image_url", "image_url": {"url": "data:image/png;base64,xxx"}, "role": "first_frame"},
			{"type": "video_url", "video_url": {"url": "https://cdn.example/ref.mp4"}, "role": "reference_video"}
		],
		"resolution": "2K",
		"duration": 10,
		"ratio": "16:9",
		"aigc_watermark": true,
		"callback_url": "https://evil.example/cb"
	}`
	out := buildBody(t, a, testInfo("", "MiniMax-H3-Upstream", true), body)

	// 模型映射 + content 原样透传 + 顶层参数
	if out["model"] != "MiniMax-H3-Upstream" {
		t.Fatalf("upstream model: %v", out["model"])
	}
	if out["resolution"] != "2K" || out["duration"] != float64(10) || out["ratio"] != "16:9" {
		t.Fatalf("upstream params: resolution=%v duration=%v ratio=%v", out["resolution"], out["duration"], out["ratio"])
	}
	if wm, _ := out["aigc_watermark"].(bool); !wm {
		t.Fatalf("aigc_watermark should pass through: %v", out["aigc_watermark"])
	}
	// callback_url 必须被剥离
	if _, has := out["callback_url"]; has {
		t.Fatal("callback_url must be stripped from upstream request")
	}
	content, ok := out["content"].([]any)
	if !ok || len(content) != 3 {
		t.Fatalf("expected 3 content items, got %v", out["content"])
	}
	img, _ := content[1].(map[string]any)
	if img["role"] != "first_frame" {
		t.Fatalf("content roles must be preserved: %v", content[1])
	}
	vid, _ := content[2].(map[string]any)
	if vid["type"] != "video_url" {
		t.Fatalf("multimodal reference item must be preserved: %v", content[2])
	}
}

func TestBuildRequestBody_GenericForm(t *testing.T) {
	a := &Adaptor{}
	body := `{
		"model": "MiniMax-H3",
		"prompt": "小猫对着镜头打哈欠",
		"seconds": "8",
		"images": ["https://cdn.example/first.png"],
		"metadata": {"resolution": "2K", "ratio": "9:16", "last_frame": "https://cdn.example/last.png"}
	}`
	out := buildBody(t, a, testInfo("", "", false), body)

	if out["duration"] != float64(8) || out["resolution"] != "2K" || out["ratio"] != "9:16" {
		t.Fatalf("mapped params: duration=%v resolution=%v ratio=%v", out["duration"], out["resolution"], out["ratio"])
	}
	content, ok := out["content"].([]any)
	if !ok || len(content) != 3 {
		t.Fatalf("expected text + first_frame + last_frame, got %v", out["content"])
	}
	text, _ := content[0].(map[string]any)
	if text["type"] != "text" || text["text"] != "小猫对着镜头打哈欠" {
		t.Fatalf("text item: %v", content[0])
	}
	first, _ := content[1].(map[string]any)
	if first["role"] != "first_frame" {
		t.Fatalf("first frame role: %v", content[1])
	}
	last, _ := content[2].(map[string]any)
	if last["role"] != "last_frame" {
		t.Fatalf("last frame role: %v", content[2])
	}
}

// TestBuildRequestBody_GenericDefaults 纯文生视频缺省值：duration=6、resolution=768P、ratio=16:9
func TestBuildRequestBody_GenericDefaults(t *testing.T) {
	a := &Adaptor{}
	out := buildBody(t, a, testInfo("", "", false), `{"model":"MiniMax-H3","prompt":"hi"}`)

	if out["duration"] != float64(6) || out["resolution"] != "768P" || out["ratio"] != "16:9" {
		t.Fatalf("defaults: duration=%v resolution=%v ratio=%v", out["duration"], out["resolution"], out["ratio"])
	}
	content, _ := out["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("expected text-only content, got %v", out["content"])
	}
}

// TestBuildRequestBody_MetadataContentAppend metadata.content 多模态参考项透传追加
func TestBuildRequestBody_MetadataContentAppend(t *testing.T) {
	a := &Adaptor{}
	body := `{
		"model": "MiniMax-H3",
		"prompt": "hi",
		"metadata": {
			"content": [{"type": "audio_url", "audio_url": {"url": "https://cdn.example/vo.mp3"}, "role": "reference_audio"}]
		}
	}`
	out := buildBody(t, a, testInfo("", "", false), body)
	content, _ := out["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("expected text + appended reference item, got %v", out["content"])
	}
	audio, _ := content[1].(map[string]any)
	if audio["type"] != "audio_url" || audio["role"] != "reference_audio" {
		t.Fatalf("appended reference item: %v", content[1])
	}
}

// httpResp 构造带 JSON body 的 http.Response 桩
func httpResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestDoResponse(t *testing.T) {
	a := &Adaptor{}
	info := testInfo("", "MiniMax-H3", false) // v2 分派（DoResponse 按模型走 v1/v2 解析）

	// 提交成功
	taskID, _, taskErr := a.DoResponse(context.Background(), httpResp(http.StatusOK, `{"task_id":"424010985738629"}`), info)
	if taskErr != nil || taskID != "424010985738629" {
		t.Fatalf("submit response: id=%q err=%+v", taskID, taskErr)
	}

	// 上游错误：OaiError 风格 message 透传
	_, _, taskErr = a.DoResponse(context.Background(), httpResp(http.StatusBadRequest, `{"error":{"type":"invalid_request_error","message":"resolution not supported"}}`), info)
	if taskErr == nil || taskErr.Message != "resolution not supported" || taskErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("error response: %+v", taskErr)
	}

	// 空 task_id
	_, _, taskErr = a.DoResponse(context.Background(), httpResp(http.StatusOK, `{"task_id":""}`), info)
	if taskErr == nil {
		t.Fatal("expected error for empty task id")
	}
}

func TestParseTaskResult(t *testing.T) {
	a := &Adaptor{}

	succeeded := `{"task":{"id":"up-1","status":"succeeded","content":{"url":"https://video.cdn.example/out.mp4"},"usage":{"total_tokens":273890,"prompt_tokens":13500,"completion_tokens":260390},"resolution":"2K","duration":5,"ratio":"16:9"}}`
	info, err := a.ParseTaskResult([]byte(succeeded))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if info.Status != common.TaskStatusSuccess || info.ResultURL != "https://video.cdn.example/out.mp4" {
		t.Fatalf("succeeded: %+v", info)
	}
	if info.TotalTokens != 273890 || info.PromptTokens != 13500 || info.CompletionTokens != 260390 {
		t.Fatalf("usage tokens: %+v", info)
	}
	if info.ActualCost != 0 {
		t.Fatalf("ActualCost must stay 0 (per_second 预扣即终价), got %v", info.ActualCost)
	}

	statuses := map[string]struct {
		want   common.TaskStatusEnum
		reason string
	}{
		`{"task":{"status":"queued"}}`:                                        {common.TaskStatusQueued, ""},
		`{"task":{"status":"running"}}`:                                       {common.TaskStatusInProgress, ""},
		`{"task":{"status":"failed","error":{"code":"E1","message":"boom"}}}`: {common.TaskStatusFailure, "boom"},
		`{"task":{"status":"cancelled"}}`:                                     {common.TaskStatusFailure, "task cancelled"},
	}
	for body, want := range statuses {
		info, err := a.ParseTaskResult([]byte(body))
		if err != nil {
			t.Fatalf("parse %s: %v", body, err)
		}
		if info.Status != want.want {
			t.Errorf("%s: status=%v want %v", body, info.Status, want.want)
		}
		if want.reason != "" && info.FailReason != want.reason {
			t.Errorf("%s: failReason=%q want %q", body, info.FailReason, want.reason)
		}
		if !info.Status.IsTerminal() && (want.want == common.TaskStatusFailure) {
			t.Errorf("%s: cancelled/failed must map to terminal state", body)
		}
	}

	// 非 task 包装形态（防御）：未知结构降级为进行中，不报错
	info, err = a.ParseTaskResult([]byte(`{"task_id":"up-1"}`))
	if err != nil || info.Status != common.TaskStatusInProgress {
		t.Fatalf("submit-shaped data should degrade to in-progress: %+v err=%v", info, err)
	}
}

func TestFetchTask_URL(t *testing.T) {
	var gotPath, gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"task":{"id":"up-1","status":"running"}}`))
	}))
	defer upstream.Close()

	a := &Adaptor{}
	taskData, _ := json.Marshal(map[string]any{"task_id": "up-1", "use_proxy": false, "model": "MiniMax-H3"})
	resp, err := a.FetchTask(context.Background(), upstream.URL, "sk-test", taskData)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	defer resp.Body.Close()
	if gotPath != "/v2/query/video_generation/up-1" {
		t.Fatalf("fetch path: %s", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("authorization header: %q", gotAuth)
	}
}

func TestCancelTask(t *testing.T) {
	var gotMethod, gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"task_id":"424010985738629","action":"cancelled","status":"cancelled"}`))
	}))
	defer upstream.Close()

	result, taskErr := CancelTask(upstream.URL, "sk-test", "424010985738629", false)
	if taskErr != nil {
		t.Fatalf("cancel: %+v", taskErr)
	}
	if gotMethod != http.MethodDelete || gotPath != "/v2/video_generation/424010985738629" {
		t.Fatalf("cancel request: %s %s", gotMethod, gotPath)
	}
	if result.Action != "cancelled" || result.Status != "cancelled" {
		t.Fatalf("cancel result: %+v", result)
	}
}

// TestCancelTask_UpstreamError 上游拒绝（如运行中不可取消）：错误码与消息透传
func TestCancelTask_UpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"running task cannot be cancelled"}}`))
	}))
	defer upstream.Close()

	result, taskErr := CancelTask(upstream.URL, "sk-test", "424010985738629", false)
	if result != nil || taskErr == nil {
		t.Fatalf("expected error, got result=%+v err=%+v", result, taskErr)
	}
	if taskErr.StatusCode != http.StatusBadRequest || taskErr.Message != "running task cannot be cancelled" {
		t.Fatalf("upstream error relay: %+v", taskErr)
	}
}

// ==================== v1（Hailuo 系列）协议 ====================

func TestIsV2Model(t *testing.T) {
	cases := map[string]bool{
		"MiniMax-H3":          true,
		"MiniMax-H3-Max":      true,
		"minimax-h3-anything": true,
		"MiniMax-Hailuo-2.3":  false,
		"MiniMax-Hailuo-02":   false,
		"T2V-01":              false,
		"S2V-01":              false,
		"":                    false,
	}
	for model, want := range cases {
		if got := IsV2Model(model); got != want {
			t.Errorf("IsV2Model(%q) = %v, want %v", model, got, want)
		}
	}
}

func TestBuildRequestBody_V1Generic(t *testing.T) {
	a := &Adaptor{}
	body := `{
		"model": "MiniMax-Hailuo-2.3",
		"prompt": "一只猫在月光下的屋顶上行走",
		"seconds": "10",
		"images": ["https://cdn.example/first.png"],
		"metadata": {"resolution": "1080P", "last_frame": "https://cdn.example/last.png"}
	}`
	out := buildBody(t, a, testInfo("", "", false), body)

	if out["model"] != "MiniMax-Hailuo-2.3" || out["prompt"] != "一只猫在月光下的屋顶上行走" {
		t.Fatalf("v1 flat fields: %v", out)
	}
	if out["duration"] != float64(10) || out["resolution"] != "1080P" {
		t.Fatalf("v1 duration/resolution: %v/%v", out["duration"], out["resolution"])
	}
	if out["first_frame_image"] != "https://cdn.example/first.png" {
		t.Fatalf("first_frame_image from images[0]: %v", out["first_frame_image"])
	}
	if out["last_frame_image"] != "https://cdn.example/last.png" {
		t.Fatalf("last_frame_image from metadata: %v", out["last_frame_image"])
	}
	if _, has := out["callback_url"]; has {
		t.Fatal("callback_url must never reach upstream")
	}
}

// TestBuildRequestBody_V1Native v1 官方入站的扁平字段透传（含可选布尔参数）
func TestBuildRequestBody_V1Native(t *testing.T) {
	a := &Adaptor{}
	body := `{
		"model": "MiniMax-Hailuo-02",
		"prompt": "电影感运镜",
		"first_frame_image": "https://cdn.example/f.png",
		"last_frame_image": "https://cdn.example/l.png",
		"duration": 10,
		"resolution": "768P",
		"prompt_optimizer": false,
		"fast_pretreatment": true,
		"aigc_watermark": true,
		"callback_url": "https://evil.example/cb"
	}`
	out := buildBody(t, a, testInfo("", "", false), body)

	if out["first_frame_image"] != "https://cdn.example/f.png" || out["last_frame_image"] != "https://cdn.example/l.png" {
		t.Fatalf("frame images passthrough: %v", out)
	}
	if po, _ := out["prompt_optimizer"].(bool); po {
		t.Fatalf("prompt_optimizer passthrough: %v", out["prompt_optimizer"])
	}
	if fp, _ := out["fast_pretreatment"].(bool); !fp {
		t.Fatalf("fast_pretreatment passthrough: %v", out["fast_pretreatment"])
	}
	if _, has := out["callback_url"]; has {
		t.Fatal("callback_url must never reach upstream")
	}
	if _, has := out["content"]; has {
		t.Fatal("v1 request must not carry content[]")
	}
}

// TestBuildRequestBody_V1DefaultResolution 分辨率缺省按模型默认档：Hailuo 2.x → 768P，旧型号 → 720P
func TestBuildRequestBody_V1DefaultResolution(t *testing.T) {
	a := &Adaptor{}
	out := buildBody(t, a, testInfo("", "", false), `{"model":"MiniMax-Hailuo-2.3","prompt":"hi"}`)
	if out["resolution"] != "768P" {
		t.Fatalf("hailuo default resolution: %v", out["resolution"])
	}
	out = buildBody(t, a, testInfo("", "", false), `{"model":"T2V-01","prompt":"hi"}`)
	if out["resolution"] != "720P" {
		t.Fatalf("legacy model default resolution: %v", out["resolution"])
	}
}

func TestDoResponse_V1(t *testing.T) {
	a := &Adaptor{}
	info := testInfo("", "MiniMax-Hailuo-2.3", false)

	// 提交成功（含 base_resp 成功信封）
	taskID, _, taskErr := a.DoResponse(context.Background(),
		httpResp(http.StatusOK, `{"task_id":"v1-up-1","base_resp":{"status_code":0,"status_msg":"success"}}`), info)
	if taskErr != nil || taskID != "v1-up-1" {
		t.Fatalf("v1 submit: id=%q err=%+v", taskID, taskErr)
	}

	// HTTP 200 但 base_resp 业务码非 0：业务失败
	_, _, taskErr = a.DoResponse(context.Background(),
		httpResp(http.StatusOK, `{"base_resp":{"status_code":2013,"status_msg":"invalid parameters"}}`), info)
	if taskErr == nil || taskErr.Message != "invalid parameters" || taskErr.ErrCode != "2013" {
		t.Fatalf("v1 business error: %+v", taskErr)
	}
}

// TestFetchTask_V1_TwoHop v1 查询：Success 后链式二跳 files/retrieve 换 download_url 并合并进响应体
func TestFetchTask_V1_TwoHop(t *testing.T) {
	var queryPath, filePath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/query/video_generation"):
			queryPath = r.URL.Path + "?" + r.URL.RawQuery
			_, _ = w.Write([]byte(`{"task_id":"v1-up-1","status":"Success","file_id":"9001","video_width":1280,"video_height":720,"base_resp":{"status_code":0}}`))
		case strings.HasPrefix(r.URL.Path, "/v1/files/retrieve"):
			filePath = r.URL.Path + "?" + r.URL.RawQuery
			_, _ = w.Write([]byte(`{"file":{"file_id":9001,"download_url":"https://cdn.example/out.mp4"},"base_resp":{"status_code":0}}`))
		default:
			t.Errorf("unexpected upstream path: %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	a := &Adaptor{}
	taskData, _ := json.Marshal(map[string]any{"task_id": "v1-up-1", "use_proxy": false, "model": "MiniMax-Hailuo-2.3"})
	resp, err := a.FetchTask(context.Background(), upstream.URL, "sk-test", taskData)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	defer resp.Body.Close()

	if queryPath != "/v1/query/video_generation?task_id=v1-up-1" {
		t.Fatalf("v1 query path: %s", queryPath)
	}
	if filePath != "/v1/files/retrieve?file_id=9001" {
		t.Fatalf("file retrieve path: %s", filePath)
	}

	// 合并后的响应体经 ParseTaskResult 提取 ResultURL
	body, _ := io.ReadAll(resp.Body)
	info, err := a.ParseTaskResult(body)
	if err != nil {
		t.Fatalf("parse merged: %v", err)
	}
	if info.Status != common.TaskStatusSuccess || info.ResultURL != "https://cdn.example/out.mp4" {
		t.Fatalf("two-hop result: %+v", info)
	}
}

// TestFetchTask_V1_TwoHopFailure 二跳失败：任务仍按 Success 返回，仅缺 download_url
func TestFetchTask_V1_TwoHopFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/query/video_generation") {
			_, _ = w.Write([]byte(`{"task_id":"v1-up-1","status":"Success","file_id":"9001","base_resp":{"status_code":0}}`))
			return
		}
		// files/retrieve 返回业务错误
		_, _ = w.Write([]byte(`{"file":{"file_id":9001},"base_resp":{"status_code":1001,"status_msg":"request timeout"}}`))
	}))
	defer upstream.Close()

	a := &Adaptor{}
	taskData, _ := json.Marshal(map[string]any{"task_id": "v1-up-1", "use_proxy": false, "model": "MiniMax-Hailuo-2.3"})
	resp, err := a.FetchTask(context.Background(), upstream.URL, "sk-test", taskData)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	info, err := a.ParseTaskResult(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if info.Status != common.TaskStatusSuccess {
		t.Fatalf("two-hop failure must not change Success status: %+v", info)
	}
	if info.ResultURL != "" {
		t.Fatalf("download_url must be absent on two-hop failure: %q", info.ResultURL)
	}
}

func TestParseTaskResult_V1Statuses(t *testing.T) {
	a := &Adaptor{}
	cases := map[string]struct {
		status common.TaskStatusEnum
		reason string
	}{
		`{"task_id":"t","status":"Preparing","base_resp":{"status_code":0}}`:                                      {common.TaskStatusQueued, ""},
		`{"task_id":"t","status":"Queueing","base_resp":{"status_code":0}}`:                                       {common.TaskStatusQueued, ""},
		`{"task_id":"t","status":"Processing","base_resp":{"status_code":0}}`:                                     {common.TaskStatusInProgress, ""},
		`{"task_id":"t","status":"Fail","base_resp":{"status_code":1026,"status_msg":"content violates policy"}}`: {common.TaskStatusFailure, "content violates policy"},
	}
	for body, want := range cases {
		info, err := a.ParseTaskResult([]byte(body))
		if err != nil {
			t.Fatalf("parse %s: %v", body, err)
		}
		if info.Status != want.status {
			t.Errorf("%s: status=%v want %v", body, info.Status, want.status)
		}
		if want.reason != "" && info.FailReason != want.reason {
			t.Errorf("%s: failReason=%q want %q", body, info.FailReason, want.reason)
		}
	}
}

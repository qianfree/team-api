package ali

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

func newTestRelayInfo() *common.RelayInfo {
	// ChannelMeta 必须非空（BuildRequestBody 读取 IsModelMapped/UpstreamModelName）
	return &common.RelayInfo{ChannelMeta: &common.ChannelMeta{}}
}

func TestDetectVideo(t *testing.T) {
	a := &AliAdaptor{}
	video := []string{
		"wan3.0-video", "wan3.0-video-prime", "WAN3.0-VIDEO",
		"wan2.7-t2v-2026-04-25", "wan2.2-t2v-plus", "wanx2.1-t2v-turbo",
	}
	for _, m := range video {
		if !a.detectVideo(m) {
			t.Errorf("%s should be detected as video", m)
		}
	}
	image := []string{
		"wanx2.1-t2i-ediff", "wanx2.5-image-edit", "wanx-v1",
		"qwen-image-generation", "flux-dev",
	}
	for _, m := range image {
		if a.detectVideo(m) {
			t.Errorf("%s should be detected as image", m)
		}
	}
}

// 回归：EstimateBilling 不再依赖 BuildRequestBody 的 isVideo 副作用（管线中前者先于后者执行，
// 旧实现视频时长信号从未上报），此处直接对原始 body 求值。
func TestEstimateBilling_VideoSpecSignals(t *testing.T) {
	a := &AliAdaptor{}

	// wan3.0 + metadata duration/resolution → spec.* 上报
	ratios := a.EstimateBilling(context.Background(), newTestRelayInfo(), []byte(
		`{"model":"wan3.0-video","prompt":"p","metadata":{"duration":8,"resolution":"720P"}}`))
	if ratios["spec.duration"] != 8.0 || ratios["duration"] != 8.0 {
		t.Fatalf("spec.duration signal lost: %v", ratios)
	}
	if ratios["spec.resolution"] != "720P" {
		t.Fatalf("spec.resolution signal lost: %v", ratios)
	}

	// 智能时长 -1：按 wan3.0 上限 30s 冻结（结算按实际秒数多退少补）
	ratios = a.EstimateBilling(context.Background(), newTestRelayInfo(), []byte(
		`{"model":"wan3.0-video","prompt":"p","metadata":{"duration":-1,"resolution":"1080P"}}`))
	if ratios["spec.duration"] != 30.0 {
		t.Fatalf("smart duration should freeze at 30s: %v", ratios)
	}
	if ratios["spec.resolution"] != "1080P" {
		t.Fatalf("spec.resolution signal lost: %v", ratios)
	}

	// 顶层 seconds（OpenAI Videos 通用词汇）优先于 metadata
	ratios = a.EstimateBilling(context.Background(), newTestRelayInfo(), []byte(
		`{"model":"wan3.0-video","prompt":"p","seconds":"12","metadata":{"duration":5}}`))
	if ratios["spec.duration"] != 12.0 {
		t.Fatalf("seconds should override metadata duration: %v", ratios)
	}

	// OpenAI Videos 路径无 resolution 字段：wan3.0 按 size 归一档位上报（裸档位/WxH 均可）
	ratios = a.EstimateBilling(context.Background(), newTestRelayInfo(), []byte(
		`{"model":"wan3.0-video","prompt":"p","metadata":{"size":"720P","duration":5}}`))
	if ratios["spec.resolution"] != "720P" {
		t.Fatalf("size tier vocabulary should report spec.resolution: %v", ratios)
	}
	ratios = a.EstimateBilling(context.Background(), newTestRelayInfo(), []byte(
		`{"model":"wan3.0-video","prompt":"p","metadata":{"size":"1280x720","duration":5}}`))
	if ratios["spec.resolution"] != "720P" {
		t.Fatalf("size WxH should report spec.resolution: %v", ratios)
	}

	// wan2.x 的 size 是原生尺寸语义，不得折算档位（预扣按矩阵兜底档走）
	ratios = a.EstimateBilling(context.Background(), newTestRelayInfo(), []byte(
		`{"model":"wan2.2-t2v-plus","prompt":"p","metadata":{"size":"1024x576","duration":5}}`))
	if _, has := ratios["spec.resolution"]; has {
		t.Fatalf("wan2 size must not derive spec.resolution: %v", ratios)
	}

	// 未传时长：不产出 spec.duration（引擎按默认 5s 估）
	ratios = a.EstimateBilling(context.Background(), newTestRelayInfo(), []byte(
		`{"model":"wan3.0-video","prompt":"p"}`))
	if _, has := ratios["spec.duration"]; has {
		t.Fatalf("no duration signal expected: %v", ratios)
	}

	// 图片模型：n>1 产出 count，不产出时长/分辨率信号
	ratios = a.EstimateBilling(context.Background(), newTestRelayInfo(), []byte(
		`{"model":"wanx-v1","prompt":"p","n":4}`))
	if ratios["count"] != 4.0 {
		t.Fatalf("image count signal lost: %v", ratios)
	}
	if _, has := ratios["spec.duration"]; has {
		t.Fatalf("image model must not carry duration signal: %v", ratios)
	}
}

func TestBuildVideoRequest_Wan3NativeMedia(t *testing.T) {
	a := &AliAdaptor{}
	body := []byte(`{
		"model": "wan3.0-video",
		"prompt": "p",
		"media": [
			{"type": "reference_video", "url": "https://v.mp4"},
			{"type": "reference_audio", "url": "https://a.mp3"}
		],
		"metadata": {"resolution": "480P", "ratio": "16:9", "duration": 5, "sound": "off"}
	}`)
	r, err := a.BuildRequestBody(context.Background(), newTestRelayInfo(), body)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var req dashScopeVideoRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(req.Input.Media) != 2 || req.Input.Media[0].Type != "reference_video" || req.Input.Media[1].URL != "https://a.mp3" {
		t.Fatalf("media passthrough lost: %+v", req.Input.Media)
	}
	if req.Parameters == nil || req.Parameters.Resolution != "480P" || req.Parameters.Ratio != "16:9" {
		t.Fatalf("parameters lost: %+v", req.Parameters)
	}
	if req.Parameters.Audio == nil || *req.Parameters.Audio {
		t.Fatalf("sound=off should map to audio=false: %+v", req.Parameters.Audio)
	}
}

func TestBuildVideoRequest_OpenAIVocabulary(t *testing.T) {
	a := &AliAdaptor{}
	// OpenAI Videos 通用词汇：metadata.image → first_frame；size 短边 → resolution；sound → audio
	body := []byte(`{
		"model": "wan3.0-video",
		"prompt": "p",
		"seconds": "8",
		"images": ["https://x/1.png"],
		"metadata": {"image": "https://x/ref.png", "size": "1280x720", "sound": "on", "aspect_ratio": "9:16"}
	}`)
	r, err := a.BuildRequestBody(context.Background(), newTestRelayInfo(), body)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var req dashScopeVideoRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(req.Input.Media) != 1 || req.Input.Media[0].Type != "first_frame" || req.Input.Media[0].URL != "https://x/ref.png" {
		t.Fatalf("metadata.image should map to first_frame: %+v", req.Input.Media)
	}
	if req.Parameters == nil || req.Parameters.Duration == nil || *req.Parameters.Duration != 8 {
		t.Fatalf("seconds → duration lost: %+v", req.Parameters)
	}
	if req.Parameters.Resolution != "720P" {
		t.Fatalf("size short-edge → resolution lost: %+v", req.Parameters)
	}
	if req.Parameters.Size != "" {
		t.Fatalf("wan3.0 must not carry size after resolution derived: %+v", req.Parameters)
	}
	if req.Parameters.Ratio != "9:16" {
		t.Fatalf("aspect_ratio → ratio lost: %+v", req.Parameters)
	}
	if req.Parameters.Audio == nil || !*req.Parameters.Audio {
		t.Fatalf("sound=on should map to audio=true: %+v", req.Parameters.Audio)
	}
}

func TestBuildVideoRequest_MultiImagesFallback(t *testing.T) {
	a := &AliAdaptor{}
	// 无 metadata.image 时，顶层 images[] → reference_image 数组
	body := []byte(`{"model":"wan3.0-video","prompt":"p","images":["https://x/1.png","https://x/2.png"]}`)
	r, err := a.BuildRequestBody(context.Background(), newTestRelayInfo(), body)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var req dashScopeVideoRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(req.Input.Media) != 2 || req.Input.Media[0].Type != "reference_image" || req.Input.Media[1].URL != "https://x/2.png" {
		t.Fatalf("images fallback lost: %+v", req.Input.Media)
	}
}

// 回归：wan2.x 旧格式不引入 wan3.0 专属行为（无 media、无 audio、size 不派生档位）
func TestBuildVideoRequest_Wan2Unchanged(t *testing.T) {
	a := &AliAdaptor{}
	body := []byte(`{"model":"wan2.2-t2v-plus","prompt":"p","metadata":{"size":"1024x576","duration":5}}`)
	r, err := a.BuildRequestBody(context.Background(), newTestRelayInfo(), body)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var req dashScopeVideoRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(req.Input.Media) != 0 {
		t.Fatalf("wan2 request must not carry media: %+v", req.Input.Media)
	}
	if req.Parameters != nil && req.Parameters.Audio != nil {
		t.Fatalf("wan2 request must not carry audio: %+v", req.Parameters)
	}
	if req.Parameters == nil || req.Parameters.Resolution != "" {
		t.Fatalf("wan2 size must not derive resolution: %+v", req.Parameters)
	}
	if req.Parameters.Size != "1024*576" {
		t.Fatalf("wan2 size passthrough broken: %+v", req.Parameters)
	}
}

func TestParseTaskResult_Wan3MaterialUsage(t *testing.T) {
	a := &AliAdaptor{}
	body := []byte(`{
		"output": {"task_id": "t1", "task_status": "SUCCEEDED", "video_url": "https://v.mp4"},
		"usage": {"video_count": 1, "duration": 5.0, "output_video_duration": 5.13, "fps": 30, "SR": 720, "ratio": "16:9"}
	}`)
	info, err := a.ParseTaskResult(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if info.Status != common.TaskStatusSuccess || info.ResultURL != "https://v.mp4" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if info.MaterialUsage == nil {
		t.Fatal("expected material usage for per_second settlement")
	}
	if info.MaterialUsage.OutputSeconds != 5.13 {
		t.Fatalf("output seconds lost: %+v", info.MaterialUsage)
	}
	if info.MaterialUsage.Resolution != "720P" {
		t.Fatalf("SR → resolution normalization broken: %+v", info.MaterialUsage)
	}

	// usage 缺失（wan2.x 响应）：MaterialUsage 为 nil，结算回退预扣口径
	info, err = a.ParseTaskResult([]byte(`{"output":{"task_id":"t2","task_status":"SUCCEEDED","video_url":"https://v2.mp4"}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if info.MaterialUsage != nil {
		t.Fatalf("no usage → no material usage: %+v", info.MaterialUsage)
	}

	// SR=0：Resolution 留空（结算查价回退 ratios 档位）
	info, err = a.ParseTaskResult([]byte(`{"output":{"task_id":"t3","task_status":"SUCCEEDED","video_url":"u"},"usage":{"output_video_duration":5.0,"SR":0}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if info.MaterialUsage == nil || info.MaterialUsage.Resolution != "" {
		t.Fatalf("zero SR should yield empty resolution: %+v", info.MaterialUsage)
	}
}

func TestExtractUpstreamRequestID(t *testing.T) {
	a := &AliAdaptor{}
	// DashScope 提交成功响应：顶层 request_id 提取，与 output.task_id 互不干扰
	if got := a.ExtractUpstreamRequestID([]byte(
		`{"output":{"task_id":"tsk-abc","task_status":"PENDING"},"request_id":"req-123","code":null,"message":""}`)); got != "req-123" {
		t.Fatalf("request_id lost: %q", got)
	}
	// 错误响应同样带顶层 request_id（DashScope 错误时 code/message 非空）
	if got := a.ExtractUpstreamRequestID([]byte(
		`{"request_id":"req-err","code":"InvalidApiKey","message":"Invalid API-key"}`)); got != "req-err" {
		t.Fatalf("error response request_id lost: %q", got)
	}
	// 缺失/非法 body：返回空串不报错
	if got := a.ExtractUpstreamRequestID([]byte(`{"output":{"task_id":"t"}}`)); got != "" {
		t.Fatalf("missing request_id should yield empty: %q", got)
	}
	if got := a.ExtractUpstreamRequestID([]byte(`not json`)); got != "" {
		t.Fatalf("invalid body should yield empty: %q", got)
	}
}

func TestResolutionFromSize(t *testing.T) {
	cases := map[string]string{
		"1280x720":  "720P",
		"720x1280":  "720P", // 竖版按短边
		"854x480":   "480P",
		"1920x1080": "1080P",
		"1080x1920": "1080P",
		"1024x1024": "1080P", // 正方形短边 1024 → 1080P
		"invalid":   "",
		"ax720":     "",
	}
	for size, want := range cases {
		if got := resolutionFromSize(size); got != want {
			t.Errorf("resolutionFromSize(%q) = %q, want %q", size, got, want)
		}
	}
}

// 裸档位词汇（OpenAI 端点 size 直传 wan3.0 原生词汇）归一；
// WxH 尺寸走短边归档，无效值返回空串
func TestNormalizeWan3Resolution(t *testing.T) {
	cases := map[string]string{
		"720P":      "720P",
		"480p":      "480P", // 小写归一为大写档位
		" 1080p ":   "1080P",
		"1280x720":  "720P",
		"1024x576":  "720P",
		"4K":        "", // 非法键必须返回空串，防止把 "4K" 之类键透传上游
		"768P":      "", // MiniMax 档位非 wan3.0 词汇，不归一
		"not-a-tie": "",
	}
	for size, want := range cases {
		if got := normalizeWan3Resolution(size); got != want {
			t.Errorf("normalizeWan3Resolution(%q) = %q, want %q", size, got, want)
		}
	}
}

// OpenAI 端点 size 直传 wan3.0 原生档位词汇（"720P"/"480p"）：
// 直接作为 resolution 下发上游，且不再透传 wan3.0 不认识的 size 参数
func TestBuildVideoRequest_SizeTierVocabulary(t *testing.T) {
	for _, size := range []string{"720P", "480p", "1080p"} {
		a := &AliAdaptor{}
		body := []byte(`{"model":"wan3.0-video","prompt":"p","metadata":{"size":"` + size + `"}}`)
		r, err := a.BuildRequestBody(context.Background(), newTestRelayInfo(), body)
		if err != nil {
			t.Fatalf("build (%s): %v", size, err)
		}
		var req dashScopeVideoRequest
		if err := json.NewDecoder(r).Decode(&req); err != nil {
			t.Fatalf("decode (%s): %v", size, err)
		}
		want := strings.ToUpper(size)
		if req.Parameters == nil || req.Parameters.Resolution != want {
			t.Fatalf("size=%s should derive resolution=%s: %+v", size, want, req.Parameters)
		}
		if req.Parameters.Size != "" {
			t.Fatalf("wan3.0 request must not carry size param (input %s): %+v", size, req.Parameters)
		}
	}
}

// 回归：metadata.size 无法归一为档位（如 MiniMax 词汇误传给 wan3.0）时，
// resolution 留空走上游默认 1080P，原 size 值不再透传
func TestBuildVideoRequest_SizeUnrecognized(t *testing.T) {
	a := &AliAdaptor{}
	body := []byte(`{"model":"wan3.0-video","prompt":"p","metadata":{"size":"768P"}}`)
	r, err := a.BuildRequestBody(context.Background(), newTestRelayInfo(), body)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var req dashScopeVideoRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// 归一失败且无其他参数时 Parameters 整体为 nil（不下发 parameters 节点），
	// 非 nil 时 resolution/size 必须为空（上游默认 1080P）
	if req.Parameters != nil && (req.Parameters.Resolution != "" || req.Parameters.Size != "") {
		t.Fatalf("unrecognized size should leave resolution/size empty: %+v", req.Parameters)
	}
}

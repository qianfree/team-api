package jsplugin

import (
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

func teamapiSource(t *testing.T) string {
	t.Helper()
	return readBuiltin(t, "teamapi")
}

func teamapiForTest(t *testing.T) (*ActivePlugin, func(string, any) map[string]json.RawMessage) {
	t.Helper()
	cleanup, err := PublishForTest(teamapiSource(t))
	if err != nil {
		t.Fatalf("发布失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, ok := CurrentGeneration().Plugin("teamapi")
	if !ok {
		t.Fatal("teamapi 未在 generation 内")
	}
	return plugin, hookCaller(t, plugin)
}

func TestTeamapiManifest(t *testing.T) {
	compiled := mustCompile(t, teamapiSource(t))
	m := compiled.Manifest
	if m.Key != "teamapi" || m.StandardVideos != "decodeVideos" || m.ModelsFromConfig != "extra_models" {
		t.Fatalf("入口声明异常: standardVideos=%q modelsFromConfig=%q", m.StandardVideos, m.ModelsFromConfig)
	}
	if !m.SupportsUpstreamModel("MiniMax-H3") {
		t.Fatalf("基线上游模型缺失: %v", m.Models)
	}
	for _, role := range []string{"first_frame", "last_frame", "reference_image", "reference_video", "reference_audio", "file", "link"} {
		if !m.SupportsReferenceRole(role) {
			t.Fatalf("参考素材角色未声明: %s", role)
		}
	}
	// 纯标准端点插件：不声明原生路由（/pjs 无入口）
	if len(m.Routes) != 0 {
		t.Fatalf("不应声明原生路由: %+v", m.Routes)
	}
}

func TestTeamapiDecodeVideos(t *testing.T) {
	_, call := teamapiForTest(t)

	// 标准端点通用任务体：秒/尺寸/比例/参考素材/extra 归一
	decoded, err := contract.ValidateDecoded(call("decodeVideos", contract.StandardEntryContext{
		Surface: "openai_videos",
		Model:   "MiniMax-H3",
		Body: json.RawMessage(`{
			"model": "MiniMax-H3", "prompt": "city night", "seconds": "8", "ratio": "16:9",
			"references": [{"role": "reference_image", "url": "https://x/i.png"}],
			"metadata": {"duration": 8, "size": "1280x720", "resolution": "720P", "sound": "on", "aspect_ratio": "16:9"}
		}`),
	}), false)
	if err != nil || decoded.Model != "MiniMax-H3" {
		t.Fatalf("解码失败: %+v err=%v", decoded, err)
	}
	var body map[string]any
	if err := json.Unmarshal(decoded.RequestBody, &body); err != nil {
		t.Fatalf("requestBody 非法: %v", err)
	}
	if body["prompt"] != "city night" || body["seconds"] != "8" || body["size"] != "1280x720" || body["aspect_ratio"] != "16:9" {
		t.Fatalf("归一字段异常: %v", body)
	}
	refs, _ := body["references"].([]any)
	if len(refs) != 1 || refs[0].(map[string]any)["role"] != "reference_image" {
		t.Fatalf("references 透传异常: %v", body["references"])
	}
	if extra, _ := body["extra"].(map[string]any); extra["sound"] != "on" {
		t.Fatalf("sound 应并入 extra: %v", body["extra"])
	}

	// references 缺席：兼容字段组装首/尾帧；seconds 回落 metadata.duration
	decoded2, err := contract.ValidateDecoded(call("decodeVideos", contract.StandardEntryContext{
		Surface: "video_generations",
		Model:   "m2",
		Body: json.RawMessage(`{
			"model": "m2", "prompt": "p2",
			"metadata": {"duration": 6, "image": "https://x/first.png", "last_frame": "https://x/last.png"}
		}`),
	}), false)
	if err != nil {
		t.Fatalf("兼容字段解码失败: %v", err)
	}
	var body2 map[string]any
	_ = json.Unmarshal(decoded2.RequestBody, &body2)
	if body2["seconds"] != float64(6) {
		t.Fatalf("seconds 应回落 metadata.duration: %v", body2)
	}
	refs2, _ := body2["references"].([]any)
	if len(refs2) != 2 || refs2[0].(map[string]any)["role"] != "first_frame" || refs2[1].(map[string]any)["role"] != "last_frame" {
		t.Fatalf("兼容字段组装异常: %v", body2["references"])
	}
}

func TestTeamapiEstimateUsage(t *testing.T) {
	_, call := teamapiForTest(t)
	est := mustHostUsageObj(t, call("estimateUsage", contract.EstimateUsageContext{
		Body: json.RawMessage(`{
			"prompt": "p", "seconds": "8", "resolution": "1080P",
			"references": [
				{"role": "first_frame", "url": "u"},
				{"role": "reference_image", "url": "u"},
				{"role": "reference_video", "url": "u"}
			]
		}`),
	}))
	if v, _ := est.Number(contract.UsageOutputSeconds); v != 8 {
		t.Fatalf("时长估算异常: %+v", est)
	}
	if v, _ := est.Number(contract.UsageInputImageCount); v != 2 {
		t.Fatalf("图片计数应只算图片角色（视频角色不计）: %+v", est)
	}
	if est.Resolution != "1080P" {
		t.Fatalf("分辨率未上报: %+v", est)
	}
}

func TestTeamapiBuildSubmitRequest(t *testing.T) {
	_, call := teamapiForTest(t)
	built, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Model:          "MiniMax-H3",
		UpstreamModel:  "minimax-h3",
		ChannelBaseURL: "https://up.example.com",
		Body: json.RawMessage(`{
			"prompt": "city night", "seconds": "8", "size": "1280x720", "aspect_ratio": "16:9",
			"resolution": "720P",
			"references": [{"role": "first_frame", "url": "https://x/f.png"}],
			"extra": {"sound": "on"}
		}`),
	}))
	if err != nil {
		t.Fatalf("构造提交请求失败: %v", err)
	}
	if built.URL != "/v1/videos" || built.Method != "POST" {
		t.Fatalf("提交端点异常: %s %s", built.Method, built.URL)
	}
	if built.Headers["Authorization"] != "" {
		t.Fatal("不应声明 Authorization（宿主注入渠道密钥）")
	}
	var body map[string]any
	if err := json.Unmarshal(built.Body, &body); err != nil {
		t.Fatalf("提交体非法: %v", err)
	}
	if body["model"] != "minimax-h3" {
		t.Fatalf("应使用渠道映射后的上游模型名: %v", body["model"])
	}
	if _, exists := body["resolution"]; exists {
		t.Fatalf("resolution 无 /v1/videos 载体不应透传: %v", body)
	}
	if body["seconds"] != "8" || body["size"] != "1280x720" || body["aspect_ratio"] != "16:9" {
		t.Fatalf("OpenAI 词汇透传异常: %v", body)
	}
	if refs, _ := body["references"].([]any); len(refs) != 1 {
		t.Fatalf("references 透传异常: %v", body["references"])
	}
	if extra, _ := body["extra"].(map[string]any); extra["sound"] != "on" {
		t.Fatalf("extra 透传异常: %v", body["extra"])
	}

	// 最小请求：仅 model/prompt，空字段不携带
	built2, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Model: "m",
		Body:  json.RawMessage(`{"prompt": ""}`),
	}))
	if err != nil {
		t.Fatalf("最小请求构造失败: %v", err)
	}
	var body2 map[string]any
	_ = json.Unmarshal(built2.Body, &body2)
	for _, absent := range []string{"seconds", "size", "aspect_ratio", "references", "extra"} {
		if _, exists := body2[absent]; exists {
			t.Fatalf("空字段 %s 不应携带: %v", absent, body2)
		}
	}
}

func TestTeamapiParseSubmitResponse(t *testing.T) {
	plugin, call := teamapiForTest(t)

	res, err := contract.ValidateSubmitted(call("parseSubmitResponse", contract.ParseSubmitContext{
		Status:       200,
		ResponseBody: json.RawMessage(`{"id":"video_abc123","object":"video","status":"queued","progress":0}`),
	}))
	if err != nil || res.TaskID != "video_abc123" {
		t.Fatalf("提交解析异常: %+v err=%v", res, err)
	}

	if err := hookCallErr(plugin, "parseSubmitResponse", contract.ParseSubmitContext{
		Status:       200,
		ResponseBody: json.RawMessage(`{"error":{"type":"invalid_request_error","message":"model not found"}}`),
	}); err == nil {
		t.Fatal("错误信封应抛出上游消息")
	}
	if err := hookCallErr(plugin, "parseSubmitResponse", contract.ParseSubmitContext{
		Status:       200,
		ResponseBody: json.RawMessage(`{"object":"video"}`),
	}); err == nil {
		t.Fatal("缺 id 应拒绝")
	}
}

func TestTeamapiParseQueryResponse(t *testing.T) {
	_, call := teamapiForTest(t)
	refs := []contract.QueryTaskRef{{TaskID: "video_abc"}}

	// 进行中：进度透传
	q, err := contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Tasks:        refs,
		ResponseBody: json.RawMessage(`{"id":"video_abc","status":"in_progress","progress":35}`),
	}), refs)
	if err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if item := q.Items[0]; item.Done || item.ProgressPct != "35%" {
		t.Fatalf("进行中判定异常: %+v", item)
	}

	// 排队中无进度：非终态、无 progress
	q, err = contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Tasks:        refs,
		ResponseBody: json.RawMessage(`{"id":"video_abc","status":"queued"}`),
	}), refs)
	if err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if item := q.Items[0]; item.Done || item.ProgressPct != "" {
		t.Fatalf("排队态判定异常: %+v", item)
	}

	// 完成 + video_url（Team-API 扩展直链）+ 全字段用量映射
	q, err = contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Tasks: refs,
		ResponseBody: json.RawMessage(`{"id":"video_abc","status":"completed","progress":100,` +
			`"video_url":"https://oss/x.mp4",` +
			`"usage":{"output_video_seconds":8,"input_video_seconds":3.5,"input_audio_seconds":2,` +
			`"input_image_count":1,"resolution":"1080P","prompt_tokens":12,"completion_tokens":300}}`),
	}), refs)
	if err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	item := q.Items[0]
	if !item.Done || len(item.Artifacts) != 1 {
		t.Fatalf("终态判定/产物声明异常: %+v", item)
	}
	art := item.Artifacts[0]
	if art.Key != "video" || art.Source.Type != "url" || art.Source.Path != "_video_url" || art.Source.Auth != "" {
		t.Fatalf("直链产物声明异常: %+v", art)
	}
	var snap map[string]any
	_ = json.Unmarshal(item.TaskData, &snap)
	if snap["_video_url"] != "https://oss/x.mp4" {
		t.Fatalf("直链未入快照: %v", snap)
	}
	u := mustHostUsage(t, item.Usage)
	if v, _ := u.Number(contract.UsageOutputSeconds); v != 8 {
		t.Fatalf("output_seconds 映射异常: %+v", u)
	}
	if v, _ := u.Number(contract.UsageInputVideoSeconds); v != 3.5 {
		t.Fatalf("input_video_seconds 映射异常: %+v", u)
	}
	if v, _ := u.Number(contract.UsageInputAudioSeconds); v != 2 {
		t.Fatalf("input_audio_seconds 映射异常: %+v", u)
	}
	if v, _ := u.Number(contract.UsageInputImageCount); v != 1 {
		t.Fatalf("input_image_count 映射异常: %+v", u)
	}
	if v, _ := u.Number(contract.UsageInputTokens); v != 12 {
		t.Fatalf("input_tokens 映射异常: %+v", u)
	}
	if v, _ := u.Number(contract.UsageOutputTokens); v != 300 {
		t.Fatalf("output_tokens 映射异常: %+v", u)
	}
	if u.Resolution != "1080P" {
		t.Fatalf("resolution 映射异常: %+v", u)
	}

	// 完成 + 官方 Sora 形态 video:[{url}]
	q, err = contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Tasks:        refs,
		ResponseBody: json.RawMessage(`{"id":"video_abc","status":"completed","video":[{"url":"https://v/x.mp4"}]}`),
	}), refs)
	if err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if art := q.Items[0].Artifacts[0]; art.Source.Path != "_video_url" {
		t.Fatalf("官方形态直链声明异常: %+v", art)
	}

	// 完成无直链：channel-bearer 回源上游 content 端点
	q, err = contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Tasks:        refs,
		ResponseBody: json.RawMessage(`{"id":"video_abc","status":"completed"}`),
	}), refs)
	if err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	item = q.Items[0]
	if art := item.Artifacts[0]; art.Source.Path != "_content_path" || art.Source.Auth != "channel-bearer" {
		t.Fatalf("回源声明异常: %+v", art)
	}
	_ = json.Unmarshal(item.TaskData, &snap)
	if snap["_content_path"] != "/v1/videos/video_abc/content" {
		t.Fatalf("回源路径异常: %v", snap)
	}

	// 完成但用量含非法值：仅报合法键
	q, err = contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Tasks: refs,
		ResponseBody: json.RawMessage(`{"id":"video_abc","status":"completed","video_url":"https://x/1.mp4",` +
			`"usage":{"output_video_seconds":-5,"resolution":"720P"}}`),
	}), refs)
	if err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	u = mustHostUsage(t, q.Items[0].Usage)
	if _, ok := u.Number(contract.UsageOutputSeconds); ok {
		t.Fatalf("负值时长不应上报: %+v", u)
	}
	if u.Resolution != "720P" {
		t.Fatalf("合法键应保留: %+v", u)
	}

	// 失败：错误对象映射
	q, err = contract.ValidateQueried(call("parseQueryResponse", contract.ParseQueryContext{
		Tasks:        refs,
		ResponseBody: json.RawMessage(`{"id":"video_abc","status":"failed","error":{"code":"video_generation_failed","message":"boom"}}`),
	}), refs)
	if err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if item := q.Items[0]; !item.Done || item.Error == nil || item.Error.Code != "video_generation_failed" || item.Error.Message != "boom" {
		t.Fatalf("失败态映射异常: %+v", item)
	}
}

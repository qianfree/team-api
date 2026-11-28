// builtin_extension_models_test.go 内置供应商插件（alibaba/doubao/hailuo）的扩展模型
// 声明与运行时行为测试：meta.modelsFromConfig 声明、未收录上游型号的宽容校验路径
// （不校验型号档位、参数透传由上游校验）、已知型号的类型/跨族错配仍拒绝。
package jsplugin

import (
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// TestBuiltinVendorExtensionDeclaration 三个供应商插件均声明 extra_models 扩展入口：
// modelsFromConfig 指向 config 中的 text 型字段（契约校验 load 时已保证，这里锁行为）。
func TestBuiltinVendorExtensionDeclaration(t *testing.T) {
	for _, key := range []string{"alibaba", "doubao", "hailuo"} {
		compiled := mustCompile(t, readBuiltin(t, key))
		m := compiled.Manifest
		if m.ModelsFromConfig != "extra_models" {
			t.Fatalf("%s: modelsFromConfig 应声明 extra_models，得到 %q", key, m.ModelsFromConfig)
		}
		field := m.ConfigFieldByKey("extra_models")
		if field == nil || field.Type != "text" {
			t.Fatalf("%s: config 缺少 extra_models text 型字段: %+v", key, m.Config)
		}
	}
}

func extensionPluginForTest(t *testing.T, key string) (*ActivePlugin, func(string, any) map[string]json.RawMessage) {
	t.Helper()
	cleanup, err := PublishForTest(readBuiltin(t, key))
	if err != nil {
		t.Fatalf("%s 发布失败: %v", key, err)
	}
	t.Cleanup(cleanup)
	plugin, ok := CurrentGeneration().Plugin(key)
	if !ok {
		t.Fatalf("%s 未在 generation 内", key)
	}
	return plugin, hookCaller(t, plugin)
}

// TestAlibabaExtensionModelFlow 扩展型号（如新万相/千问图像）走宽容路径：
// 视频不校验分辨率档位（新型号可能有新档）、首帧按 wan3.x media 形态落位、音频透传；
// 图像路由放行未知型号、仍拒绝已知视频型号；计费预估回落默认档（30s/1080P）。
func TestAlibabaExtensionModelFlow(t *testing.T) {
	plugin, call := extensionPluginForTest(t, "alibaba")

	// 原生视频路由：未知上游型号 + 越档分辨率（2K 不在内置档位表）放行
	decoded, err := contract.ValidateDecoded(call("createTask", contract.DecodeContext{
		Path: "/api/v1/services/aigc/video-generation/video-synthesis", Method: "POST",
		Body: json.RawMessage(`{"model":"wan4.0-video","input":{"prompt":"x"},"parameters":{"resolution":"2K"}}`),
	}), false)
	if err != nil {
		t.Fatalf("视频解码失败: %v", err)
	}
	vres, err := contract.ValidateValidatedRequest(call("validateRequest", contract.ValidateRequestContext{
		Body: decoded.RequestBody, Model: decoded.Model, UpstreamModel: "wan4.0-video",
	}))
	if err != nil {
		t.Fatalf("扩展视频型号不应被拒绝: %v", err)
	}
	built, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: vres.RequestBody, Model: decoded.Model, UpstreamModel: "wan4.0-video",
	}))
	if err != nil || built.URL != "/api/v1/services/aigc/video-generation/video-synthesis" {
		t.Fatalf("扩展型号提交构造异常: %+v err=%v", built, err)
	}
	est := mustHostUsageObj(t, call("estimateUsage", contract.EstimateUsageContext{
		Body: vres.RequestBody, Model: decoded.Model, UpstreamModel: "wan4.0-video",
	}))
	// 无 duration：按默认档最长时长 30s 预扣；2K 不在默认档表 → 报默认档 1080P
	if v, _ := est.Number(contract.UsageOutputSeconds); v != 30 {
		t.Fatalf("扩展型号预估应回落默认档 30s: %+v", est)
	}
	if est.Resolution != "1080P" {
		t.Fatalf("扩展型号预估分辨率应回落默认档: %+v", est)
	}

	// 标准端点：扩展型号首帧按 wan3.x media 形态落位、音频开关透传
	sdec, err := contract.ValidateDecoded(call("decodeVideos", contract.StandardEntryContext{
		Surface: contract.SurfaceOpenAIVideos, Path: "/v1/videos", Model: "m4",
		Body: json.RawMessage(`{"model":"m4","prompt":"p","seconds":8,"metadata":{"image":"https://img.example.com/f.png","sound":"on"}}`),
	}), false)
	if err != nil {
		t.Fatalf("标准端点解码失败: %v", err)
	}
	sres, err := contract.ValidateValidatedRequest(call("validateRequest", contract.ValidateRequestContext{
		Body: sdec.RequestBody, Model: sdec.Model, UpstreamModel: "wan4.0-video",
	}))
	if err != nil {
		t.Fatalf("标准端点扩展型号不应被拒绝: %v", err)
	}
	var std struct {
		VendorBody struct {
			Input struct {
				Media []struct {
					Type string `json:"type"`
					URL  string `json:"url"`
				} `json:"media"`
			} `json:"input"`
			Parameters struct {
				Audio *bool `json:"audio"`
			} `json:"parameters"`
		} `json:"vendorBody"`
	}
	_ = json.Unmarshal(sres.RequestBody, &std)
	if len(std.VendorBody.Input.Media) != 1 || std.VendorBody.Input.Media[0].Type != "first_frame" ||
		std.VendorBody.Input.Media[0].URL != "https://img.example.com/f.png" ||
		std.VendorBody.Parameters.Audio == nil || !*std.VendorBody.Parameters.Audio {
		t.Fatalf("扩展型号首帧/音频落位异常: %s", sres.RequestBody)
	}

	// 图像路由：未知型号（可能是新 Seedream 同位）放行；已知视频型号仍拒绝
	idec, err := contract.ValidateDecoded(call("createImage", contract.DecodeContext{
		Path: "/api/v1/services/aigc/text2image/image-synthesis", Method: "POST",
		Body: json.RawMessage(`{"model":"qwen-image-next","input":{"prompt":"x"}}`),
	}), false)
	if err != nil {
		t.Fatalf("图像解码失败: %v", err)
	}
	if _, err := contract.ValidateValidatedRequest(call("validateRequest", contract.ValidateRequestContext{
		Body: idec.RequestBody, Model: idec.Model, UpstreamModel: "qwen-image-next",
	})); err != nil {
		t.Fatalf("扩展图像型号不应被拒绝: %v", err)
	}
	if err := hookCallErr(plugin, "validateRequest", contract.ValidateRequestContext{
		Body: idec.RequestBody, Model: idec.Model, UpstreamModel: "wan3.0-video",
	}); err == nil {
		t.Fatal("已知视频型号走图像路由应被拒绝")
	}
}

// TestDoubaoExtensionModelFlow 扩展型号（如新 Seedance/Seedream）：视频按默认档放行，
// 图像路由仅拒绝已知视频型号。
func TestDoubaoExtensionModelFlow(t *testing.T) {
	plugin, call := extensionPluginForTest(t, "doubao")

	// 视频：未知上游型号（越档分辨率 4k 也在默认档内）放行
	decoded, err := contract.ValidateDecoded(call("createTask", contract.DecodeContext{
		Path: "/api/v3/contents/generations/tasks", Method: "POST",
		Body: json.RawMessage(`{"model":"doubao-seedance-3-0-pro-270101","content":[{"type":"text","text":"x"}],"resolution":"4k","duration":10}`),
	}), false)
	if err != nil {
		t.Fatalf("视频解码失败: %v", err)
	}
	if _, err := contract.ValidateValidatedRequest(call("validateRequest", contract.ValidateRequestContext{
		Body: decoded.RequestBody, Model: decoded.Model, UpstreamModel: "doubao-seedance-3-0-pro-270101",
	})); err != nil {
		t.Fatalf("扩展视频型号不应被拒绝: %v", err)
	}
	// validateRequest 未替换请求体（返回 {}）：宿主沿用原解码体（与迁移测试同链路）
	built, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: decoded.RequestBody, Model: decoded.Model, UpstreamModel: "doubao-seedance-3-0-pro-270101",
	}))
	if err != nil || built.URL != "/api/v3/contents/generations/tasks" {
		t.Fatalf("扩展型号提交构造异常: %+v err=%v", built, err)
	}

	// 图像：未知型号放行（可能是新 Seedream）；已知 Seedance 视频型号仍拒绝
	idec, err := contract.ValidateDecoded(call("createImage", contract.DecodeContext{
		Path: "/api/v3/images/generations", Method: "POST",
		Body: json.RawMessage(`{"model":"doubao-seedream-6-0-super","prompt":"x"}`),
	}), false)
	if err != nil {
		t.Fatalf("图像解码失败: %v", err)
	}
	if _, err := contract.ValidateValidatedRequest(call("validateRequest", contract.ValidateRequestContext{
		Body: idec.RequestBody, Model: idec.Model, UpstreamModel: "doubao-seedream-6-0-super",
	})); err != nil {
		t.Fatalf("扩展图像型号不应被拒绝: %v", err)
	}
	if err := hookCallErr(plugin, "validateRequest", contract.ValidateRequestContext{
		Body: idec.RequestBody, Model: idec.Model, UpstreamModel: "doubao-seedance-2-0-260128",
	}); err == nil {
		t.Fatal("已知视频型号走图像路由应被拒绝")
	}
}

// TestHailuoExtensionModelFlow 扩展型号默认按 H3/v2 形态处理（MiniMax 新型号均上
// /v2）：标准端点映射 /v2 请求体、不校验分辨率/时长档位（无法归一的分辨率原样透传）；
// 原生 /v1 路由按客户端所选协议放行。
func TestHailuoExtensionModelFlow(t *testing.T) {
	_, call := extensionPluginForTest(t, "hailuo")

	// 标准端点：未知型号 → H3/v2 家族；1080P 不在 H3 归一表内原样透传；时长 20 超
	// H3 上限 15 也不拦截（档位由上游校验）
	decoded, err := contract.ValidateDecoded(call("decodeVideos", contract.StandardEntryContext{
		Surface: contract.SurfaceOpenAIVideos, Path: "/v1/videos", Model: "MiniMax-H4",
		Body: json.RawMessage(`{"model":"MiniMax-H4","prompt":"dance","seconds":20,"metadata":{"resolution":"1080P"}}`),
	}), false)
	if err != nil {
		t.Fatalf("标准端点解码失败: %v", err)
	}
	vres, err := contract.ValidateValidatedRequest(call("validateRequest", contract.ValidateRequestContext{
		Body: decoded.RequestBody, Model: decoded.Model, UpstreamModel: "MiniMax-H4",
	}))
	if err != nil {
		t.Fatalf("扩展型号不应被拒绝: %v", err)
	}
	var req struct {
		Family     string `json:"family"`
		VendorBody struct {
			Resolution any `json:"resolution"`
			Duration   any `json:"duration"`
		} `json:"vendorBody"`
	}
	_ = json.Unmarshal(vres.RequestBody, &req)
	if req.Family != "h3" || req.VendorBody.Resolution != "1080P" || req.VendorBody.Duration != float64(20) {
		t.Fatalf("扩展型号应按 H3/v2 形态定稿（分辨率透传/时长不设档）: %s", vres.RequestBody)
	}
	built, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: vres.RequestBody, Model: decoded.Model, UpstreamModel: "MiniMax-H4",
	}))
	if err != nil || built.URL != "/v2/video_generation" {
		t.Fatalf("扩展型号应提交到 /v2: %+v err=%v", built, err)
	}

	// 原生 /v1 路由：客户端显式选 v1 协议，扩展型号放行（形态以路由为准）
	native, err := contract.ValidateDecoded(call("createTask", contract.DecodeContext{
		Path: "/v1/video_generation", Method: "POST",
		Body: json.RawMessage(`{"model":"MiniMax-H4","prompt":"x","duration":20}`),
	}), false)
	if err != nil {
		t.Fatalf("原生解码失败: %v", err)
	}
	if _, err := contract.ValidateValidatedRequest(call("validateRequest", contract.ValidateRequestContext{
		Body: native.RequestBody, Model: native.Model, UpstreamModel: "MiniMax-H4",
	})); err != nil {
		t.Fatalf("原生 /v1 扩展型号不应被拒绝: %v", err)
	}
	// validateRequest 未替换请求体：宿主沿用原解码体（family=v1 决定 /v1 形态）
	nbuilt, err := contract.ValidateBuiltRequest(call("buildSubmitRequest", contract.BuildSubmitContext{
		Body: native.RequestBody, Model: native.Model, UpstreamModel: "MiniMax-H4",
	}))
	if err != nil || nbuilt.URL != "/v1/video_generation" {
		t.Fatalf("原生 /v1 扩展型号提交构造异常: %+v err=%v", nbuilt, err)
	}
}

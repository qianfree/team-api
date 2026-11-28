package jsbridge

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// validatePluginSource 带 validateRequest 钩子的最小插件：上游 bad_model 拒绝，
// fix_model 修正请求体（下游钩子看到修正值），其余原样放行。
const validatePluginSource = `
export const meta = {
  apiVersion: 1, key: "validate-demo", name: "Validate Demo", version: "1.0.0",
  models: ["good_model", "bad_model", "fix_model"],
  routes: [{ method: "POST", path: "/submit", type: "submit" }, { method: "GET", path: "/q/:task_id", type: "query" }],
};
export function decodeRequest(ctx) { return { kind: "submit", model: ctx.body.model, requestBody: ctx.body }; }
export function validateRequest(ctx) {
  if (ctx.upstreamModel === "bad_model") throw new Error("bad_model rejects duration " + ctx.body.duration);
  if (ctx.upstreamModel === "fix_model") return { requestBody: Object.assign({}, ctx.body, { duration: 5 }) };
  return {};
}
export function estimateUsage(ctx) { return { output_seconds: ctx.body.duration, resolution: "1280x720" }; }
export function buildSubmitRequest(ctx) { return { url: "/submit", body: Object.assign({ upstream: ctx.upstreamModel }, ctx.body) }; }
export function parseSubmitResponse(ctx) { return { taskId: "t1" }; }
export function buildQueryRequest(ctx) { return { url: "/q" }; }
export function parseQueryResponse(ctx) { return { items: [{ taskId: ctx.tasks[0].taskId, done: false }] }; }
`

func publishValidatePlugin(t *testing.T) *jsplugin.ActivePlugin {
	t.Helper()
	cleanup, err := jsplugin.PublishForTest(validatePluginSource)
	if err != nil {
		t.Fatalf("发布插件失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, _ := jsplugin.CurrentGeneration().Plugin("validate-demo")
	return plugin
}

// validateAdaptor 构造已 Init 的适配器 + 解码计划 ctx（平台模型名 platform，渠道映射到 upstream）。
func validateAdaptor(t *testing.T, plugin *jsplugin.ActivePlugin, platform, upstream string) (*Adaptor, context.Context) {
	t.Helper()
	ctx := jsplugin.WithSubmitPlan(context.Background(), &jsplugin.SubmitPlan{
		Plugin: plugin,
		Decoded: &contract.DecodeResult{
			Kind: "submit", Model: platform,
			RequestBody: json.RawMessage(`{"model":"` + platform + `","duration":12}`),
		},
	})
	info := relayInfo("https://up.example.com")
	info.OriginModelName = platform
	info.ChannelMeta.UpstreamModelName = upstream
	info.ChannelMeta.Settings.PluginKey = plugin.Key
	adaptor := &Adaptor{}
	adaptor.Init(info)
	return adaptor, ctx
}

// TestValidateRequestUpstreamModel 渠道上游模型须 ∈ 插件有效 models；平台名可以是任意别名。
func TestValidateRequestUpstreamModel(t *testing.T) {
	plugin := publishValidatePlugin(t)

	adaptor, ctx := validateAdaptor(t, plugin, "platform-alias", "good_model")
	if taskErr := adaptor.ValidateRequest(ctx, adaptor.info, nil); taskErr != nil {
		t.Fatalf("映射到支持的上游模型应放行: %s", taskErr.Message)
	}

	adaptor, ctx = validateAdaptor(t, plugin, "platform-alias", "sora-2")
	taskErr := adaptor.ValidateRequest(ctx, adaptor.info, nil)
	if taskErr == nil || taskErr.StatusCode != http.StatusBadRequest || !strings.Contains(taskErr.Message, "sora-2") {
		t.Fatalf("不支持的上游模型应 400，得到 %+v", taskErr)
	}

	// 渠道未绑定插件：400
	adaptor, ctx = validateAdaptor(t, plugin, "platform-alias", "good_model")
	adaptor.pluginKey = ""
	if taskErr := adaptor.ValidateRequest(ctx, adaptor.info, nil); taskErr == nil || taskErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("未绑定插件的渠道应 400，得到 %+v", taskErr)
	}
}

// TestValidateRequestHook validateRequest 钩子：抛错 → 400（未进入预扣）；返回 requestBody
// → 替换归一请求体，estimateUsage/buildSubmitRequest 看到修正值且带上游模型名。
func TestValidateRequestHook(t *testing.T) {
	plugin := publishValidatePlugin(t)

	adaptor, ctx := validateAdaptor(t, plugin, "video-pro", "bad_model")
	taskErr := adaptor.ValidateRequest(ctx, adaptor.info, nil)
	if taskErr == nil || taskErr.StatusCode != http.StatusBadRequest || !strings.Contains(taskErr.Message, "bad_model rejects duration 12") {
		t.Fatalf("钩子拒绝应 400 透出原因，得到 %+v", taskErr)
	}

	adaptor, ctx = validateAdaptor(t, plugin, "video-pro", "fix_model")
	if taskErr := adaptor.ValidateRequest(ctx, adaptor.info, nil); taskErr != nil {
		t.Fatalf("修正型校验应放行: %s", taskErr.Message)
	}
	ratios := adaptor.EstimateBilling(ctx, adaptor.info, nil)
	if ratios["spec.duration"] != float64(5) {
		t.Fatalf("estimateUsage 应看到修正后的时长: %#v", ratios)
	}
	if ratios["spec.resolution"] != "720P" {
		t.Fatalf("resolution 应经宿主归一为 720P: %#v", ratios["spec.resolution"])
	}
	built, taskErr := adaptor.ensureSubmitDesc(ctx)
	if taskErr != nil {
		t.Fatalf("buildSubmitRequest 失败: %s", taskErr.Message)
	}
	var sent map[string]any
	_ = json.Unmarshal(built.Body, &sent)
	if sent["duration"] != float64(5) || sent["upstream"] != "fix_model" {
		t.Fatalf("上游请求应带修正值与上游模型名: %v", sent)
	}
}

// task_handler_jsplugin_test.go JS 插件提交链的资金出口测试：经 /pjs 分发器同构的
// SubmitPlan 注入，跑 HandleTaskSubmit 全管线（真实 jsbridge 适配器 + mock 上游 +
// 计费/存储桩），断言成功落库、预扣失败拦截、钩子/上游失败先退款再报错、
// immediate 免轮询标志透传。
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/internal/jsplugin"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// jsBillingRecorder 记录预扣/退款的计费桩（其余方法放行，金额固定 0.1）。
type jsBillingRecorder struct {
	common.TaskBillingProvider

	failPreDeduct bool
	preDeducts    []decimal.Decimal
	refunds       []decimal.Decimal
}

var errJSPreDeduct = errors.New("insufficient balance")

func (s *jsBillingRecorder) EstimateTaskCost(context.Context, int64, string, map[string]any, []byte, http.Header) (decimal.Decimal, error) {
	return decimal.NewFromFloat(0.1), nil
}

func (s *jsBillingRecorder) PreDeductTask(_ context.Context, _ int64, _ string, cost decimal.Decimal, _ string) (decimal.Decimal, error) {
	if s.failPreDeduct {
		return decimal.Zero, errJSPreDeduct
	}
	s.preDeducts = append(s.preDeducts, cost)
	return cost, nil
}

func (s *jsBillingRecorder) SettleTaskFailed(_ context.Context, _ int64, _ string, amount decimal.Decimal) error {
	s.refunds = append(s.refunds, amount)
	return nil
}

func (s *jsBillingRecorder) CheckRateLimit(context.Context, int64, int64, int64, int) (bool, string, int, int, int64) {
	return true, "", 0, 0, 0
}

func (s *jsBillingRecorder) AcquireApiKeyConcurrent(context.Context, int64, int) bool { return true }
func (s *jsBillingRecorder) ReleaseApiKeyConcurrent(context.Context, int64)           {}
func (s *jsBillingRecorder) CheckApiKeyQuota(context.Context, int64, decimal.Decimal) error {
	return nil
}

func (s *jsBillingRecorder) AdjustTaskBilling(_ context.Context, _ int64, _ string, _, newCost decimal.Decimal) (decimal.Decimal, error) {
	return newCost, nil
}

// boomPluginSource buildSubmitRequest 抛错的插件：验证构建失败出口的退款。
const boomPluginSource = `export const meta = {
  apiVersion: 1, key: "boom", name: "Boom", version: "1.0.0",
  models: ["boom_model"], fetchMode: "per_task",
  routes: [{ method: "POST", path: "/submit", type: "submit" }],
};
export function decodeRequest(ctx) {
  return { kind: "submit", model: "boom_model", requestBody: ctx.body || {} };
}
export function buildSubmitRequest() {
  throw new Error("boom: upstream down");
}
export function parseSubmitResponse() { return { taskId: "t" }; }
export function buildQueryRequest() { return { url: "/f", method: "GET" }; }
export function parseQueryResponse(ctx) {
  return { items: [{ taskId: (ctx.tasks || [])[0].taskId, done: false }] };
}`

// instantPluginSource parseSubmitResponse 返回 immediate 的插件：验证免轮询标志
// 经解码计划透传（终态化由分发器 finalizeImmediateTask 完成）。
const instantPluginSource = `export const meta = {
  apiVersion: 1, key: "instant", name: "Instant", version: "1.0.0",
  models: ["instant_model"], fetchMode: "per_task",
  routes: [{ method: "POST", path: "/submit", type: "submit" }],
};
export function decodeRequest(ctx) {
  return { kind: "submit", model: "instant_model", requestBody: ctx.body || {} };
}
export function buildSubmitRequest() {
  return { url: "/v1/instant/submit", method: "POST", body: {} };
}
export function parseSubmitResponse() {
  return {
    taskId: "inst-1",
    taskData: { ok: 1 },
    immediate: { status: "SUCCESS", taskData: { done: true } },
  };
}
export function buildQueryRequest() { return { url: "/v1/instant/fetch", method: "GET" }; }
export function parseQueryResponse(ctx) {
  return { items: [{ taskId: (ctx.tasks || [])[0].taskId, done: false }] };
}`

func publishJSPlugins(t *testing.T, sources ...string) {
	t.Helper()
	cleanup, err := jsplugin.PublishForTest(sources...)
	if err != nil {
		t.Fatalf("发布测试插件失败: %v", err)
	}
	t.Cleanup(cleanup)
}

func goldenTestSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../internal/jsplugin/builtin/golden/plugin.js")
	if err != nil {
		t.Fatalf("读取 golden 插件失败: %v", err)
	}
	return string(data)
}

// stdVideoPluginSource 声明 standardVideos 的测试插件：标准端点通用任务体经
// decodeStd 映射为原生 requestBody（metadata.duration → duration），验证合成计划
// 打通 HandleTaskSubmit 全链（不经 /pjs 分发器、ctx 初始无解码计划）。
const stdVideoPluginSource = `export const meta = {
  apiVersion: 1, key: "stdvideo", name: "StdVideo", version: "1.0.0",
  models: ["std_model"], fetchMode: "per_task", standardVideos: "decodeStd",
  routes: [{ method: "POST", path: "/submit", type: "submit" }],
};
export function decodeStd(ctx) {
  const raw = ctx.body || {};
  const metadata = raw.metadata || {};
  if (!String(raw.prompt || "").trim()) throw new Error("prompt is required");
  return { kind: "submit", model: "std_model",
    requestBody: { prompt: String(raw.prompt), duration: metadata.duration || raw.seconds || 0 } };
}
export function decodeRequest(ctx) {
  return { kind: "submit", model: "std_model", requestBody: ctx.body || {} };
}
export function buildSubmitRequest(ctx) {
  const b = ctx.body || {};
  return { url: "/std/submit", method: "POST", body: { prompt: b.prompt, duration: b.duration } };
}
export function parseSubmitResponse(ctx) {
  if (ctx.status !== 200) throw new Error("submit_http_" + ctx.status);
  const resp = ctx.responseBody || {};
  return { taskId: String(resp.id || "std-1"), taskData: resp };
}
export function buildQueryRequest(ctx) { const t = (ctx.tasks || [])[0] || {}; return { url: "/std/" + t.taskId, method: "GET" }; }
export function parseQueryResponse(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  return { items: [{ taskId: t.taskId, done: false }] };
}`

// jsSubmitPlan 构造分发器同构的解码计划并注入 ctx（jsbridge.ValidateRequest 消费）。
func jsSubmitPlan(t *testing.T, pluginKey, model, prompt string) (context.Context, *jsplugin.SubmitPlan) {
	t.Helper()
	plugin, ok := jsplugin.CurrentGeneration().Plugin(pluginKey)
	if !ok {
		t.Fatalf("插件 %s 未进 generation", pluginKey)
	}
	body := map[string]any{}
	if prompt != "" {
		body["prompt"] = prompt
	}
	requestBody, _ := json.Marshal(body)
	plan := &jsplugin.SubmitPlan{
		Plugin:     plugin,
		Params:     map[string]string{"action": "music"},
		NativePath: "/pjs/" + pluginKey + "/submit/music",
		Decoded:    &contract.DecodeResult{Kind: "submit", Model: model, RequestBody: requestBody},
	}
	return jsplugin.WithSubmitPlan(context.Background(), plan), plan
}

func jsChannelMeta(baseURL, pluginKey, model string) *common.ChannelMeta {
	return &common.ChannelMeta{
		ChannelID:         31,
		ChannelType:       int(constant.ProviderJSTask),
		ChannelName:       "js-" + pluginKey,
		BaseURL:           baseURL,
		ApiKey:            "sk-test",
		UpstreamModelName: model,
		Settings:          common.ChannelSettings{PluginKey: pluginKey},
	}
}

// TestHandleTaskSubmit_JSPlugin_Success 提交成功：上游一次往返、落库 platform=js:{key}、
// 预扣一次、不退款、私有数据携带上游任务 ID。
func TestHandleTaskSubmit_JSPlugin_Success(t *testing.T) {
	publishJSPlugins(t, goldenTestSource(t))

	vendorHits := 0
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1/golden/submit") {
			t.Errorf("上游路径异常: %s", r.URL.Path)
		}
		vendorHits++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "success", "data": "task-42"})
	}))
	t.Cleanup(vendor.Close)

	ctx, _ := jsSubmitPlan(t, "golden", "golden_music", "hello")
	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{TenantID: 1, UserID: 2, ApiKeyID: 3, RequestID: "req_js_ok", Writer: rec}
	billing := &jsBillingRecorder{}
	data := &stubVideosSubmitProvider{}

	HandleTaskSubmit(ctx, []byte(`{"model":"golden_music","prompt":"hello"}`),
		"/pjs/golden/submit/music", http.Header{}, rc, data, billing,
		jsChannelMeta(vendor.URL, "golden", "golden_music"))

	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，得到 %d body=%s", rec.Code, rec.Body.String())
	}
	if vendorHits != 1 {
		t.Fatalf("上游应恰好一次往返，得到 %d", vendorHits)
	}
	if data.created == nil || data.created.Platform != "js:golden" {
		t.Fatalf("落库任务异常: %+v", data.created)
	}
	if rc.TaskID != data.created.PublicTaskID {
		t.Fatalf("rc.TaskID=%q 应等于落库 ID %q", rc.TaskID, data.created.PublicTaskID)
	}
	if !strings.Contains(string(data.created.PrivateData), `"upstream_task_id":"task-42"`) {
		t.Fatalf("私有数据应携带上游任务 ID: %s", data.created.PrivateData)
	}
	if len(billing.preDeducts) != 1 {
		t.Fatalf("应预扣一次，得到 %v", billing.preDeducts)
	}
	if len(billing.refunds) != 0 {
		t.Fatalf("成功提交不应退款: %v", billing.refunds)
	}
}

// TestHandleTaskSubmit_JSPlugin_PreDeductFail 预扣失败（余额不足）：402 拦截，
// 不触上游、不落库、无退款（冻结未发生）。
func TestHandleTaskSubmit_JSPlugin_PreDeductFail(t *testing.T) {
	publishJSPlugins(t, goldenTestSource(t))

	vendorHits := 0
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		vendorHits++
		_, _ = io.WriteString(w, "{}")
	}))
	t.Cleanup(vendor.Close)

	ctx, _ := jsSubmitPlan(t, "golden", "golden_music", "hello")
	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{TenantID: 1, UserID: 2, ApiKeyID: 3, RequestID: "req_js_402", Writer: rec}
	billing := &jsBillingRecorder{failPreDeduct: true}
	data := &stubVideosSubmitProvider{}

	HandleTaskSubmit(ctx, []byte(`{"model":"golden_music","prompt":"hello"}`),
		"/pjs/golden/submit/music", http.Header{}, rc, data, billing,
		jsChannelMeta(vendor.URL, "golden", "golden_music"))

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("期望 402，得到 %d body=%s", rec.Code, rec.Body.String())
	}
	if vendorHits != 0 {
		t.Fatalf("预扣失败不应触达上游，命中 %d 次", vendorHits)
	}
	if data.created != nil || len(billing.refunds) != 0 || len(billing.preDeducts) != 0 {
		t.Fatalf("预扣失败后不应有落库/退款/预扣记录: created=%v refunds=%v", data.created, billing.refunds)
	}
}

// TestHandleTaskSubmit_JSPlugin_HookBuildError_Refunds 插件 buildSubmitRequest 抛错：
// 预扣金额原额退款后报 500，资金不悬空。
func TestHandleTaskSubmit_JSPlugin_HookBuildError_Refunds(t *testing.T) {
	publishJSPlugins(t, boomPluginSource)

	ctx, _ := jsSubmitPlan(t, "boom", "boom_model", "")
	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{TenantID: 1, UserID: 2, ApiKeyID: 3, RequestID: "req_js_boom", Writer: rec}
	billing := &jsBillingRecorder{}
	data := &stubVideosSubmitProvider{}

	HandleTaskSubmit(ctx, []byte(`{"model":"boom_model"}`),
		"/pjs/boom/submit/music", http.Header{}, rc, data, billing,
		jsChannelMeta("http://unused", "boom", "boom_model"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("期望 500，得到 %d body=%s", rec.Code, rec.Body.String())
	}
	if len(billing.preDeducts) != 1 || len(billing.refunds) != 1 {
		t.Fatalf("应预扣一次并退款一次: preDeducts=%v refunds=%v", billing.preDeducts, billing.refunds)
	}
	if !billing.refunds[0].Equal(billing.preDeducts[0]) {
		t.Fatalf("退款金额应等于预扣: %v vs %v", billing.refunds[0], billing.preDeducts[0])
	}
	if data.created != nil {
		t.Fatalf("失败提交不应落库: %+v", data.created)
	}
}

// TestHandleTaskSubmit_JSPlugin_UpstreamError_Refunds 上游 5xx：上游状态码原样透出
// （jsbridge 对非 200 直传 StatusCode），退款先行，任务不落库。
func TestHandleTaskSubmit_JSPlugin_UpstreamError_Refunds(t *testing.T) {
	publishJSPlugins(t, goldenTestSource(t))

	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"err":"down"}`)
	}))
	t.Cleanup(vendor.Close)

	ctx, _ := jsSubmitPlan(t, "golden", "golden_music", "hello")
	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{TenantID: 1, UserID: 2, ApiKeyID: 3, RequestID: "req_js_5xx", Writer: rec}
	billing := &jsBillingRecorder{}
	data := &stubVideosSubmitProvider{}

	HandleTaskSubmit(ctx, []byte(`{"model":"golden_music","prompt":"hello"}`),
		"/pjs/golden/submit/music", http.Header{}, rc, data, billing,
		jsChannelMeta(vendor.URL, "golden", "golden_music"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("期望 500（上游状态码透传），得到 %d body=%s", rec.Code, rec.Body.String())
	}
	if len(billing.refunds) != 1 || !billing.refunds[0].Equal(decimal.NewFromFloat(0.1)) {
		t.Fatalf("应按预扣金额退款: %v", billing.refunds)
	}
	if data.created != nil {
		t.Fatalf("失败提交不应落库: %+v", data.created)
	}
}

// TestHandleTaskSubmit_JSPlugin_ImmediateFlagPlumbed parseSubmitResponse 返回
// immediate → 标志经解码计划透传（分发器据此免轮询直达终态）。
func TestHandleTaskSubmit_JSPlugin_ImmediateFlagPlumbed(t *testing.T) {
	publishJSPlugins(t, instantPluginSource)

	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"anything":1}`)
	}))
	t.Cleanup(vendor.Close)

	ctx, plan := jsSubmitPlan(t, "instant", "instant_model", "")
	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{TenantID: 1, UserID: 2, ApiKeyID: 3, RequestID: "req_js_inst", Writer: rec}
	billing := &jsBillingRecorder{}
	data := &stubVideosSubmitProvider{}

	HandleTaskSubmit(ctx, []byte(`{"model":"instant_model"}`),
		"/pjs/instant/submit/music", http.Header{}, rc, data, billing,
		jsChannelMeta(vendor.URL, "instant", "instant_model"))

	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，得到 %d body=%s", rec.Code, rec.Body.String())
	}
	if plan.Immediate == nil || plan.Immediate.Status != "SUCCESS" {
		t.Fatalf("immediate 标志应透传到解码计划: %+v", plan.Immediate)
	}
	if len(billing.refunds) != 0 {
		t.Fatalf("成功提交不应退款: %v", billing.refunds)
	}
}

// TestHandleTaskSubmit_JSPlugin_StandardEntry_Success 标准端点（/v1/video/generations）
// 提交 JS 插件模型：ctx 初始无解码计划，管线内合成（decodeStd 映射）→ 全链打通，
// 上游收到映射后的原生 body，落库 platform=js:{key}。
func TestHandleTaskSubmit_JSPlugin_StandardEntry_Success(t *testing.T) {
	publishJSPlugins(t, stdVideoPluginSource)

	var upstreamBody map[string]any
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/std/submit") {
			t.Errorf("上游路径异常: %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&upstreamBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "std-42"})
	}))
	t.Cleanup(vendor.Close)

	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{TenantID: 1, UserID: 2, ApiKeyID: 3, RequestID: "req_std_ok", Writer: rec}
	billing := &jsBillingRecorder{}
	data := &stubVideosSubmitProvider{}

	// 注意：plain ctx（无 SubmitPlan）——解码计划由管线内 SynthesizeStandardPlan 合成
	HandleTaskSubmit(context.Background(),
		[]byte(`{"model":"std_model","prompt":"hello","seconds":5}`),
		"/v1/video/generations", http.Header{}, rc, data, billing,
		jsChannelMeta(vendor.URL, "stdvideo", "std_model"))

	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，得到 %d body=%s", rec.Code, rec.Body.String())
	}
	if upstreamBody["prompt"] != "hello" || upstreamBody["duration"] != float64(5) {
		t.Fatalf("上游应收到映射后的原生 body（decodeStd 生效）: %v", upstreamBody)
	}
	if data.created == nil || data.created.Platform != "js:stdvideo" {
		t.Fatalf("落库任务异常: %+v", data.created)
	}
	if !strings.Contains(string(data.created.PrivateData), `"upstream_task_id":"std-42"`) {
		t.Fatalf("私有数据应携带上游任务 ID: %s", data.created.PrivateData)
	}
	if len(billing.preDeducts) != 1 || len(billing.refunds) != 0 {
		t.Fatalf("应预扣一次不退款: preDeducts=%v refunds=%v", billing.preDeducts, billing.refunds)
	}
}

// TestHandleTaskSubmit_JSPlugin_StandardEntry_OpenAIVideosBody /v1/videos 入口：
// 胶水层归一后的通用任务体（metadata.duration 形态）同样走合成链。
func TestHandleTaskSubmit_JSPlugin_StandardEntry_OpenAIVideosBody(t *testing.T) {
	publishJSPlugins(t, stdVideoPluginSource)

	var upstreamBody map[string]any
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&upstreamBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "std-43"})
	}))
	t.Cleanup(vendor.Close)

	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{TenantID: 1, UserID: 2, ApiKeyID: 3, RequestID: "req_std_oai", Writer: rec}

	HandleTaskSubmit(context.Background(),
		[]byte(`{"model":"std_model","prompt":"a cat","seconds":"8","metadata":{"duration":8,"size":"1280x720"}}`),
		"/v1/videos", http.Header{}, rc, &stubVideosSubmitProvider{}, &jsBillingRecorder{},
		jsChannelMeta(vendor.URL, "stdvideo", "std_model"))

	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，得到 %d body=%s", rec.Code, rec.Body.String())
	}
	if upstreamBody["duration"] != float64(8) {
		t.Fatalf("metadata.duration 应映射为原生 duration: %v", upstreamBody)
	}
}

// TestHandleTaskSubmit_JSPlugin_StandardEntry_NoDeclaration 插件未声明 standardVideos：
// 400 明确提示（含声明指引），不触上游、不预扣。
func TestHandleTaskSubmit_JSPlugin_StandardEntry_NoDeclaration(t *testing.T) {
	publishJSPlugins(t, goldenTestSource(t))

	vendorHits := 0
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		vendorHits++
		_, _ = io.WriteString(w, "{}")
	}))
	t.Cleanup(vendor.Close)

	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{TenantID: 1, UserID: 2, ApiKeyID: 3, RequestID: "req_std_nodecl", Writer: rec}
	billing := &jsBillingRecorder{}

	HandleTaskSubmit(context.Background(),
		[]byte(`{"model":"golden_music","prompt":"hello"}`),
		"/v1/videos", http.Header{}, rc, &stubVideosSubmitProvider{}, billing,
		jsChannelMeta(vendor.URL, "golden", "golden_music"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("期望 400，得到 %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "standardVideos") {
		t.Fatalf("错误信息应包含声明指引: %s", rec.Body.String())
	}
	if vendorHits != 0 || len(billing.preDeducts) != 0 {
		t.Fatalf("拒绝路径不应触上游/预扣: hits=%d preDeducts=%v", vendorHits, billing.preDeducts)
	}
}

// TestHandleTaskSubmit_JSPlugin_NonStandardEndpoint_Rejected 非支持端点（厂商原生
// 协议/音乐入口）即使插件声明了 standardVideos 也保持拒绝：body 不是通用任务体。
func TestHandleTaskSubmit_JSPlugin_NonStandardEndpoint_Rejected(t *testing.T) {
	publishJSPlugins(t, stdVideoPluginSource)

	vendorHits := 0
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		vendorHits++
		_, _ = io.WriteString(w, "{}")
	}))
	t.Cleanup(vendor.Close)

	rec := httptest.NewRecorder()
	rc := &TaskRelayContext{TenantID: 1, UserID: 2, ApiKeyID: 3, RequestID: "req_std_path", Writer: rec}

	HandleTaskSubmit(context.Background(),
		[]byte(`{"model":"std_model","prompt":"hello"}`),
		"/suno/submit/music", http.Header{}, rc, &stubVideosSubmitProvider{}, &jsBillingRecorder{},
		jsChannelMeta(vendor.URL, "stdvideo", "std_model"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("期望 400，得到 %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "not supported") {
		t.Fatalf("错误信息应说明端点不支持: %s", rec.Body.String())
	}
	if vendorHits != 0 {
		t.Fatalf("拒绝路径不应触上游，命中 %d 次", vendorHits)
	}
}

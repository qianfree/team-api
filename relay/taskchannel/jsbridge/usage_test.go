package jsbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
	"github.com/qianfree/team-api/relay/common"
)

// TestEstimateUsageRatios 提交链计费接线：estimateUsage（宿主固定词汇）→ ratios
// （formula_facts u() 通道 + output_count → count 信号）。
func TestEstimateUsageRatios(t *testing.T) {
	plugin := publishGolden(t)

	decoded := &contract.DecodeResult{
		Kind:        "submit",
		Model:       "golden_music",
		RequestBody: json.RawMessage(`{"prompt":"hello","action":"MUSIC","clips":3}`),
	}
	ctx := jsplugin.WithSubmitPlan(context.Background(), &jsplugin.SubmitPlan{
		Plugin:  plugin,
		Params:  map[string]string{"action": "music"},
		Decoded: decoded,
	})
	adaptor := &Adaptor{}
	info := relayInfo("http://unused")
	adaptor.Init(info)
	if taskErr := adaptor.ValidateRequest(ctx, info, nil); taskErr != nil {
		t.Fatalf("ValidateRequest 失败: %s", taskErr.Message)
	}

	ratios := adaptor.EstimateBilling(ctx, info, nil)
	facts, ok := ratios["formula_facts"].(map[string]any)
	if !ok || facts["output_count"] != float64(3) {
		t.Fatalf("formula_facts 异常: %#v", ratios["formula_facts"])
	}
	if ratios["count"] != float64(3) {
		t.Fatalf("output_count 应映射 count 信号: %#v", ratios["count"])
	}
	if base, ok := ratios["base"].(float64); !ok || base != 1.0 {
		t.Fatalf("base 乘数应保留（非 formula 模型兜底）: %#v", ratios["base"])
	}
}

// TestParseTaskResultUsageAndArtifacts 轮询终态：extractUsage → MaterialUsage（u() 通道）
// + artifacts → TaskInfo.Artifacts（宿主转存流水线的输入）。
func TestParseTaskResultUsageAndArtifacts(t *testing.T) {
	publishGolden(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/golden/fetch", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","clips":2,"tier":"pro","data":{"audio_url":"https://up.example.com/a.mp3"}}`))
	})
	vendor := httptest.NewServer(mux)
	t.Cleanup(vendor.Close)

	adaptor := &Adaptor{}
	taskData, _ := json.Marshal(map[string]any{
		"task_id": "task-42", "platform": "js:golden", "model": "golden_music",
	})
	resp, err := adaptor.FetchTask(context.Background(), vendor.URL, "sk-test", taskData)
	if err != nil {
		t.Fatalf("FetchTask 失败: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	result, err := adaptor.ParseTaskResult(body)
	if err != nil {
		t.Fatalf("ParseTaskResult 失败: %v", err)
	}
	if result.Status != common.TaskStatusSuccess {
		t.Fatalf("应为 SUCCESS: %v", result.Status)
	}
	if result.MaterialUsage == nil {
		t.Fatal("终态成功应携带 MaterialUsage")
	}
	if result.MaterialUsage.Facts["output_count"] != 2 {
		t.Fatalf("extractUsage facts 异常: %#v", result.MaterialUsage.Facts)
	}
	if result.MaterialUsage.OutputSeconds != 0 {
		t.Fatalf("output_count 不应映射 OutputSeconds: %v", result.MaterialUsage.OutputSeconds)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].Key != "audio" ||
		result.Artifacts[0].SourceType != "url" || result.Artifacts[0].SourcePath != "data.audio_url" {
		t.Fatalf("artifacts 映射异常: %#v", result.Artifacts)
	}
}

// TestEstimateBillingWithoutHook 无 plan/无钩子时回落默认乘数（Phase 1 行为兼容）。
func TestEstimateBillingWithoutHook(t *testing.T) {
	adaptor := &Adaptor{}
	ratios := adaptor.EstimateBilling(context.Background(), nil, nil)
	if ratios["base"] != 1.0 || ratios["formula_facts"] != nil {
		t.Fatalf("无钩子应回落 base=1.0: %#v", ratios)
	}
}

// estimatePluginSource 构造 estimateUsage 行为由 mode 决定的插件源码（其余钩子最小化）：
// throw = 抛错；unknown-key = 返回词汇外的用量键；non-object = 返回非对象；ok = 正常上报；
// absent = 不导出该钩子。
func estimatePluginSource(mode string) string {
	hook := ""
	if mode != "absent" {
		hook = fmt.Sprintf(`export function estimateUsage(ctx) {
  const mode = %q;
  if (mode === "throw") throw new Error("estimate boom");
  if (mode === "unknown-key") return { bogus_key: 3 };
  if (mode === "non-object") return "not-an-object";
  return { output_seconds: 8 };
}
`, mode)
	}
	return `export const meta = {
  apiVersion: 1, key: "est", name: "Estimate", version: "1.0.0",
  models: ["est_m"],
  routes: [{ method: "POST", path: "/submit", type: "submit" }],
};
` + hook + `export function buildSubmitRequest(ctx) {
  return { url: "/v1/submit", method: "POST" };
}
export function parseSubmitResponse(ctx) {
  return { taskId: "t-1" };
}
`
}

// newEstimateAdaptor 发布 estimate 插件，返回已完成 ValidateRequest 的适配器
// （解码计划已挂 ctx，适配器已持有计划）。
func newEstimateAdaptor(t *testing.T, source string) (*Adaptor, context.Context, *common.RelayInfo) {
	t.Helper()
	cleanup, err := jsplugin.PublishForTest(source)
	if err != nil {
		t.Fatalf("发布插件失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, ok := jsplugin.CurrentGeneration().Plugin("est")
	if !ok {
		t.Fatal("插件未进 generation")
	}
	ctx := jsplugin.WithSubmitPlan(context.Background(), &jsplugin.SubmitPlan{
		Plugin: plugin,
		Decoded: &contract.DecodeResult{
			Kind:        "submit",
			Model:       "est_m",
			RequestBody: json.RawMessage(`{}`),
		},
	})
	info := &common.RelayInfo{
		OriginModelName: "est_m",
		ChannelMeta: &common.ChannelMeta{
			ChannelID:         1,
			ChannelType:       43,
			BaseURL:           "http://unused",
			ApiKey:            "sk-test",
			UpstreamModelName: "est_m",
			Settings:          common.ChannelSettings{PluginKey: "est"},
		},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)
	if taskErr := adaptor.ValidateRequest(ctx, info, nil); taskErr != nil {
		t.Fatalf("ValidateRequest 失败: %s", taskErr.Message)
	}
	return adaptor, ctx, info
}

// TestEstimateBillingStrictRejectsFailure 严格估算：estimateUsage 抛错、返回词汇外的用量键、
// 返回非对象时返回 TaskError（提交管线据此拒绝提交），不再静默回落默认乘数——公式模式下
// 静默失败会让 u() 全取 0、预扣落到下限 0.01。宽松入口 EstimateBilling 保留回落语义。
func TestEstimateBillingStrictRejectsFailure(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		wantMsg string
	}{
		{"钩子抛错", "throw", "estimateUsage"},
		{"词汇外的用量键", "unknown-key", "bogus_key"},
		{"返回非对象", "non-object", "estimateUsage:"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			adaptor, ctx, info := newEstimateAdaptor(t, estimatePluginSource(c.mode))

			ratios, taskErr := adaptor.EstimateBillingStrict(ctx, info, nil)
			if taskErr == nil {
				t.Fatalf("估算失败应返回错误，got ratios=%#v", ratios)
			}
			if taskErr.StatusCode != http.StatusBadGateway {
				t.Fatalf("状态码应为 502，got %d", taskErr.StatusCode)
			}
			if !strings.Contains(taskErr.Message, c.wantMsg) {
				t.Fatalf("错误消息应含 %q，got %q", c.wantMsg, taskErr.Message)
			}

			// 宽松入口：回落默认乘数（不阻断只认 TaskAdaptor 的调用方）
			if loose := adaptor.EstimateBilling(ctx, info, nil); loose["base"] != 1.0 || loose["spec.duration"] != nil {
				t.Fatalf("宽松入口应回落 base=1.0: %#v", loose)
			}
		})
	}
}

// TestEstimateBillingStrictSuccess 正常上报与未导出钩子：用量映射进 ratios；
// 未导出 estimateUsage 不是失败，按默认乘数放行。
func TestEstimateBillingStrictSuccess(t *testing.T) {
	adaptor, ctx, info := newEstimateAdaptor(t, estimatePluginSource("ok"))

	ratios, taskErr := adaptor.EstimateBillingStrict(ctx, info, nil)
	if taskErr != nil {
		t.Fatalf("正常估算不应出错: %s", taskErr.Message)
	}
	if ratios["spec.duration"] != 8.0 || ratios["base"] != 1.0 {
		t.Fatalf("output_seconds 应映射 spec.duration: %#v", ratios)
	}
	if facts, ok := ratios["formula_facts"].(map[string]any); !ok || facts["output_seconds"] != 8.0 {
		t.Fatalf("formula_facts 异常: %#v", ratios["formula_facts"])
	}

	adaptor, ctx, info = newEstimateAdaptor(t, estimatePluginSource("absent"))
	ratios, taskErr = adaptor.EstimateBillingStrict(ctx, info, nil)
	if taskErr != nil {
		t.Fatalf("未导出钩子不应出错: %s", taskErr.Message)
	}
	if ratios["base"] != 1.0 || ratios["formula_facts"] != nil {
		t.Fatalf("未导出钩子应回落 base=1.0: %#v", ratios)
	}
}

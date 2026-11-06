package jsbridge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
	"github.com/qianfree/team-api/relay/common"
)

// TestEstimateUsageRatios 提交链计费接线：estimateUsage → ratios
// （formula_facts u() 通道 + unit=count → count 信号）。
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
	facts, ok := ratios["formula_facts"].(map[string]float64)
	if !ok || facts["clips"] != 3 {
		t.Fatalf("formula_facts 异常: %#v", ratios["formula_facts"])
	}
	if ratios["count"] != float64(3) {
		t.Fatalf("unit=count 应映射 count 信号: %#v", ratios["count"])
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
	if result.MaterialUsage.Facts["clips"] != 2 {
		t.Fatalf("extractUsage facts 异常: %#v", result.MaterialUsage.Facts)
	}
	if result.MaterialUsage.OutputSeconds != 0 {
		t.Fatalf("unit=count 不应映射 OutputSeconds: %v", result.MaterialUsage.OutputSeconds)
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

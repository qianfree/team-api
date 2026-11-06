package jsbridge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
	"github.com/qianfree/team-api/relay/common"
)

func goldenSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../../internal/jsplugin/builtin/golden/plugin.js")
	if err != nil {
		t.Fatalf("读取 golden 插件失败: %v", err)
	}
	return string(data)
}

// publishGolden 发布 golden 插件到 generation（测试辅助，绕过 DB）。
func publishGolden(t *testing.T) *jsplugin.ActivePlugin {
	t.Helper()
	cleanup, err := jsplugin.PublishForTest(goldenSource(t))
	if err != nil {
		t.Fatalf("发布 golden 插件失败: %v", err)
	}
	t.Cleanup(cleanup)
	plugin, ok := jsplugin.CurrentGeneration().Plugin("golden")
	if !ok {
		t.Fatal("golden 插件未进 generation")
	}
	return plugin
}

// mockVendor 模拟 golden 上游：提交返回任务 ID，查询首轮 processing、次轮 success。
func mockVendor(t *testing.T, pollCount *int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/golden/submit", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["prompt"] == nil || body["action"] != "MUSIC" {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "invalid", "message": "bad body"})
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "unauthorized"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "success", "data": "task-42"})
	})
	mux.HandleFunc("/v1/golden/fetch", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("task_id") != "task-42" {
			w.WriteHeader(404)
			return
		}
		*pollCount++
		if *pollCount < 2 {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "processing"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   map[string]any{"audio_url": "https://oss.example.com/x.mp3"},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func relayInfo(baseURL string) *common.RelayInfo {
	return &common.RelayInfo{
		OriginModelName: "golden_music",
		ChannelMeta: &common.ChannelMeta{
			ChannelID:         1,
			ChannelType:       43,
			BaseURL:           baseURL,
			ApiKey:            "sk-test",
			UpstreamModelName: "golden_music",
			Settings:          common.ChannelSettings{PluginKey: "golden"},
		},
	}
}

// TestSubmitFlow 提交链：plan 注入 → build → DoRequest（mock 上游）→ parse → 任务 ID。
func TestSubmitFlow(t *testing.T) {
	plugin := publishGolden(t)
	pollCount := 0
	vendor := mockVendor(t, &pollCount)

	// 分发器产出的解码计划（正常由 decodeRequest 钩子产生，此处直接构造）
	decoded := &contract.DecodeResult{
		Kind:        "submit",
		Model:       "golden_music",
		RequestBody: json.RawMessage(`{"prompt":"hello","action":"MUSIC"}`),
	}
	ctx := jsplugin.WithSubmitPlan(context.Background(), &jsplugin.SubmitPlan{
		Plugin:     plugin,
		Params:     map[string]string{"action": "music"},
		NativePath: "/submit/music",
		Decoded:    decoded,
	})

	adaptor := &Adaptor{}
	info := relayInfo(vendor.URL)
	adaptor.Init(info)

	if taskErr := adaptor.ValidateRequest(ctx, info, nil); taskErr != nil {
		t.Fatalf("ValidateRequest 失败: %s", taskErr.Message)
	}
	body, err := adaptor.BuildRequestBody(ctx, info, nil)
	if err != nil {
		t.Fatalf("BuildRequestBody 失败: %v", err)
	}
	resp, err := adaptor.DoRequest(ctx, info, body)
	if err != nil {
		t.Fatalf("DoRequest 失败: %v", err)
	}
	taskID, taskData, taskErr := adaptor.DoResponse(ctx, resp, info)
	if taskErr != nil {
		t.Fatalf("DoResponse 失败: %s (code=%s)", taskErr.Message, taskErr.ErrCode)
	}
	if taskID != "task-42" {
		t.Fatalf("上游任务 ID 异常: %s", taskID)
	}
	var data map[string]any
	if err := json.Unmarshal(taskData, &data); err != nil || data["code"] != "success" {
		t.Fatalf("taskData 异常: %s", taskData)
	}
}

// TestPollFlow 轮询链：FetchTask（per_task）→ ParseTaskResult → IN_PROGRESS→SUCCESS。
func TestPollFlow(t *testing.T) {
	publishGolden(t)
	pollCount := 0
	vendor := mockVendor(t, &pollCount)

	adaptor := &Adaptor{}
	taskData, _ := json.Marshal(map[string]any{
		"task_id":  "task-42",
		"platform": "js:golden",
		"model":    "golden_music",
	})

	// 第一轮：processing
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
	if result.Status != common.TaskStatusInProgress {
		t.Fatalf("首轮应 IN_PROGRESS，得到 %v", result.Status)
	}

	// 第二轮：success（mock 第二次起返回终态）
	resp2, err := adaptor.FetchTask(context.Background(), vendor.URL, "sk-test", taskData)
	if err != nil {
		t.Fatalf("FetchTask(2) 失败: %v", err)
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	result2, err := adaptor.ParseTaskResult(body2)
	if err != nil {
		t.Fatalf("ParseTaskResult(2) 失败: %v", err)
	}
	if result2.Status != common.TaskStatusSuccess {
		t.Fatalf("次轮应 SUCCESS，得到 %v", result2.Status)
	}
	if !jsonContains(string(result2.Data), "audio_url") {
		t.Fatalf("终态 taskData 缺少 audio_url: %s", result2.Data)
	}
}

// TestPollFailure 上游 failed → FAILURE + FailReason。
func TestPollFailure(t *testing.T) {
	publishGolden(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/golden/fetch", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"failed","error":"upstream boom"}`))
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
	if result.Status != common.TaskStatusFailure || result.FailReason != "upstream boom" {
		t.Fatalf("失败映射异常: status=%v reason=%s", result.Status, result.FailReason)
	}
}

// TestValidateRequestWithoutPlan 未经分发器进入（无 plan）→ 拒绝。
func TestValidateRequestWithoutPlan(t *testing.T) {
	publishGolden(t)
	adaptor := &Adaptor{}
	info := relayInfo("http://unused")
	adaptor.Init(info)
	taskErr := adaptor.ValidateRequest(context.Background(), info, nil)
	if taskErr == nil {
		t.Fatal("无解码计划应当拒绝")
	}
}

// TestValidateRequestPluginMismatch 渠道绑定的插件与 plan 插件不一致 → 拒绝。
func TestValidateRequestPluginMismatch(t *testing.T) {
	plugin := publishGolden(t)
	adaptor := &Adaptor{}
	info := relayInfo("http://unused")
	info.ChannelMeta.Settings.PluginKey = "other-plugin"
	adaptor.Init(info)
	ctx := jsplugin.WithSubmitPlan(context.Background(), &jsplugin.SubmitPlan{Plugin: plugin})
	if taskErr := adaptor.ValidateRequest(ctx, info, nil); taskErr == nil {
		t.Fatal("插件不一致应当拒绝")
	}
}

func jsonContains(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

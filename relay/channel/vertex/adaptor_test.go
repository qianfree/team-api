package vertex

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

func vertexTestInfo(model string) *common.RelayInfo {
	return &common.RelayInfo{
		OriginModelName: model,
		InboundFormat:   constant.RelayFormatClaude,
		ClientFormat:    constant.RelayFormatClaude,
		RelayMode:       int(constant.RelayModeClaudeMessages),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderVertex),
			BaseURL:           "https://us-central1-aiplatform.googleapis.com",
			ApiKey:            "test-project",
			UpstreamModelName: model,
		},
	}
}

// TestInit_ModelDrivenProtocolDetection 模型名决定协议分流；判据与 helper 的
// 权威实现共用，本测试同时钉住「URL 端点」与「协议判定」两者一致。
func TestInit_ModelDrivenProtocolDetection(t *testing.T) {
	tests := []struct {
		model        string
		wantType     modelType
		wantURLParts []string
	}{
		{"gemini-2.5-pro", modelTypeGemini, []string{"publishers/google/models/gemini-2.5-pro", ":generateContent"}},
		{"claude-sonnet-4-5", modelTypeClaude, []string{"publishers/anthropic/models/claude-sonnet-4-5", ":rawPredict"}},
		{"CLAUDE-OPUS-4", modelTypeClaude, []string{"publishers/anthropic/", ":rawPredict"}},
		{"text-embedding-004", modelTypeGemini, []string{"publishers/google/", ":generateContent"}},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			a := &Adaptor{}
			info := vertexTestInfo(tt.model)
			a.Init(info)

			if a.detectedType != tt.wantType {
				t.Errorf("detectedType = %v, want %v", a.detectedType, tt.wantType)
			}

			url, err := a.GetRequestURL(info)
			if err != nil {
				t.Fatalf("GetRequestURL 失败: %v", err)
			}
			for _, part := range tt.wantURLParts {
				if !strings.Contains(url, part) {
					t.Errorf("URL = %q，期望包含 %q", url, part)
				}
			}
		})
	}
}

// TestAdaptClaudeBodyForVertex Vertex Anthropic 端点的体改写：
// 注入 anthropic_version（必填）、删除 model（模型由 URL 指定）。
func TestAdaptClaudeBodyForVertex(t *testing.T) {
	t.Run("注入版本并删除 model", func(t *testing.T) {
		out := adaptClaudeBodyForVertex([]byte(`{"model":"claude-sonnet-4-5","max_tokens":1024,"messages":[{"role":"user","content":"hi"}]}`))

		var obj map[string]any
		if err := json.Unmarshal(out, &obj); err != nil {
			t.Fatalf("输出不是合法 JSON: %v", err)
		}
		if _, exists := obj["model"]; exists {
			t.Error("model 字段未被删除（Vertex rawPredict 端点不接受体内 model）")
		}
		if obj["anthropic_version"] != vertexAnthropicVersion {
			t.Errorf("anthropic_version = %v, want %q", obj["anthropic_version"], vertexAnthropicVersion)
		}
		// 其余字段必须原样保留
		if obj["max_tokens"] != float64(1024) {
			t.Errorf("max_tokens 丢失或变形: %v", obj["max_tokens"])
		}
		if _, ok := obj["messages"]; !ok {
			t.Error("messages 丢失")
		}
	})

	t.Run("幂等", func(t *testing.T) {
		in := []byte(`{"model":"claude-x","max_tokens":1,"messages":[]}`)
		once := adaptClaudeBodyForVertex(in)
		twice := adaptClaudeBodyForVertex(once)
		if string(once) != string(twice) {
			t.Errorf("重复执行结果不同（RequestPostProcessor 契约要求幂等）:\n once=%s\ntwice=%s", once, twice)
		}
	})

	t.Run("尊重客户端显式指定的版本", func(t *testing.T) {
		out := adaptClaudeBodyForVertex([]byte(`{"anthropic_version":"vertex-2099-01-01","messages":[]}`))
		var obj map[string]any
		if err := json.Unmarshal(out, &obj); err != nil {
			t.Fatalf("输出不是合法 JSON: %v", err)
		}
		if obj["anthropic_version"] != "vertex-2099-01-01" {
			t.Errorf("覆盖了客户端指定的版本: %v", obj["anthropic_version"])
		}
	})

	t.Run("非 JSON 体原样返回", func(t *testing.T) {
		in := []byte("not json at all")
		if got := adaptClaudeBodyForVertex(in); string(got) != string(in) {
			t.Errorf("非 JSON 体被改动: %s", got)
		}
	})
}

// TestPostProcessConvertedRequest relaykit 接管路径下的私有后处理：
// Claude 模型改写体，Gemini 模型原样返回。
func TestPostProcessConvertedRequest(t *testing.T) {
	claudeBody := []byte(`{"model":"claude-sonnet-4-5","max_tokens":1,"messages":[]}`)

	a := &Adaptor{}
	a.Init(vertexTestInfo("claude-sonnet-4-5"))
	out, err := a.PostProcessConvertedRequest(context.Background(), nil, claudeBody)
	if err != nil {
		t.Fatalf("后处理失败: %v", err)
	}
	if strings.Contains(string(out), `"model"`) {
		t.Errorf("Claude 模型的体仍含 model 字段: %s", out)
	}
	if !strings.Contains(string(out), vertexAnthropicVersion) {
		t.Errorf("Claude 模型的体缺少 anthropic_version: %s", out)
	}

	geminiBody := []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`)
	g := &Adaptor{}
	g.Init(vertexTestInfo("gemini-2.5-pro"))
	out, err = g.PostProcessConvertedRequest(context.Background(), nil, geminiBody)
	if err != nil {
		t.Fatalf("后处理失败: %v", err)
	}
	if string(out) != string(geminiBody) {
		t.Errorf("Gemini 模型的体不应被改动:\n got=%s\nwant=%s", out, geminiBody)
	}
}

// TestConvertRequest_ClaudeAppliesVertexShape legacy 路径（adaptor.ConvertRequest）
// 必须与 relaykit 路径（PostProcessConvertedRequest）得到同样的体形态——
// 两条路径行为漂移正是 RequestPostProcessor 接口契约要防的事。
func TestConvertRequest_ClaudeAppliesVertexShape(t *testing.T) {
	a := &Adaptor{}
	info := vertexTestInfo("claude-sonnet-4-5")
	a.Init(info)

	r, err := a.ConvertRequest(context.Background(), info,
		[]byte(`{"model":"origin-model","max_tokens":1024,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("ConvertRequest 失败: %v", err)
	}
	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("读取转换结果失败: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		t.Fatalf("结果不是合法 JSON: %v", err)
	}
	if _, exists := obj["model"]; exists {
		t.Errorf("legacy 路径未删除 model 字段: %s", body)
	}
	if obj["anthropic_version"] != vertexAnthropicVersion {
		t.Errorf("legacy 路径未注入 anthropic_version: %s", body)
	}
}

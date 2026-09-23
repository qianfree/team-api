package handler

import (
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

type modelMappingStub struct {
	models map[string]bool
	calls  []string
}

func (s *modelMappingStub) GetModelMapping(_ context.Context, modelName string) (string, string, error) {
	s.calls = append(s.calls, modelName)
	if !s.models[modelName] {
		return "", "", common.ErrModelNotFound
	}
	return modelName, "chat", nil
}

func TestResolveRelayModel(t *testing.T) {
	// 后缀语法已移除：目录名即请求名，不再有「字面模型优先 / 剥后缀回退」两段查找
	t.Run("catalog model resolves literally", func(t *testing.T) {
		provider := &modelMappingStub{models: map[string]bool{"gpt-4o": true}}
		lookup, err := resolveRelayModel(t.Context(), provider, "gpt-4o")
		if err != nil {
			t.Fatalf("resolveRelayModel() error = %v", err)
		}
		if lookup != "gpt-4o" {
			t.Errorf("lookup model = %q, want %q", lookup, "gpt-4o")
		}
	})

	// 名字带 -max 等字样的真实目录模型按字面命中，不做任何后缀解释
	t.Run("literal model ending in effort-like word", func(t *testing.T) {
		provider := &modelMappingStub{models: map[string]bool{"qwen3.8-max": true}}
		lookup, err := resolveRelayModel(t.Context(), provider, "qwen3.8-max")
		if err != nil {
			t.Fatalf("resolveRelayModel() error = %v", err)
		}
		if lookup != "qwen3.8-max" {
			t.Errorf("lookup model = %q, want %q", lookup, "qwen3.8-max")
		}
		if len(provider.calls) != 1 || provider.calls[0] != "qwen3.8-max" {
			t.Errorf("lookup calls = %v, want exactly one literal lookup", provider.calls)
		}
	})

	// 目录中不存在即报错，不再尝试剥后缀回退（"o3-max" 不会命中 "o3"）
	t.Run("missing model returns error without suffix fallback", func(t *testing.T) {
		provider := &modelMappingStub{models: map[string]bool{"o3": true}}
		_, err := resolveRelayModel(t.Context(), provider, "o3-max")
		if err == nil {
			t.Fatal("want error for model absent from catalog")
		}
		if len(provider.calls) != 1 {
			t.Errorf("lookup calls = %v, want single literal lookup", provider.calls)
		}
	})
}

// TestEstimateInputTokens 多模态请求体的预扣估算：base64 内联媒体必须按固定成本计入，
// 不能按字节折算——一张 2MB 的图按 len/4 会被估成约 70 万 token，预扣直接冻穿余额。
func TestEstimateInputTokens(t *testing.T) {
	// 模拟 2MB 的 base64 图片载荷
	payload := strings.Repeat("A", 2<<20)

	cases := []struct {
		name string
		body string
		want int
	}{
		{
			"纯文本按字节折算",
			strings.Repeat("x", 400),
			100,
		},
		{
			"空体",
			"",
			0,
		},
		{
			// 文本骨架 40 字节 + 1 个媒体
			"单图不按字节折算",
			`{"url":"data:image/png;base64,` + payload + `"}`,
			(len(`{"url":"data:image/png;base64,`+`"}`))/4 + inlineMediaTokenCost,
		},
		{
			"双图按个数累加",
			`{"a":"data:image/png;base64,` + payload + `","b":"data:image/png;base64,` + payload + `"}`,
			(len(`{"a":"data:image/png;base64,`+`","b":"data:image/png;base64,`+`"}`))/4 + 2*inlineMediaTokenCost,
		},
		{
			// 畸形 data URL（无 base64 标记）退化为纯文本路径
			"畸形 data URL",
			`{"url":"data:image/png,` + strings.Repeat("A", 400) + `"}`,
			len(`{"url":"data:image/png,`+`"}`)/4 + 100,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := estimateInputTokens([]byte(c.body)); got != c.want {
				t.Errorf("estimateInputTokens() = %d, want %d", got, c.want)
			}
		})
	}

	// 回归保护：2MB 图片的估算必须远低于按字节折算的量级
	naive := len(`{"url":"data:image/png;base64,`+payload+`"}`) / 4
	got := estimateInputTokens([]byte(`{"url":"data:image/png;base64,` + payload + `"}`))
	if got >= naive/10 {
		t.Errorf("2MB 图片估算 %d tokens，与按字节折算的 %d 处于同一量级，预扣仍会冻穿余额", got, naive)
	}
}

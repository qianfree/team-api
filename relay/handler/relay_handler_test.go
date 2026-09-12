package handler

import (
	"context"
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

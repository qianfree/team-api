package handler

import (
	"testing"

	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/relay/constant"
)

// TestTaskPipelineScopeGate 任务管线的 scope 收口：任务类模式必须能被类型化 scope 正确拦下。
// 此前任务管线只拦 read_only，chat_only/rerank_only 等密钥可越权提交按次计费的生成任务。
func TestTaskPipelineScopeGate(t *testing.T) {
	taskModes := map[string]constant.RelayMode{
		"video_generations":  constant.RelayModeVideoGenerations,
		"videos":             constant.RelayModeVideos,
		"suno_submit":        constant.RelayModeSunoSubmit,
		"mj_submit":          constant.RelayModeMjSubmit,
		"minimax_video":      constant.RelayModeMiniMaxVideo,
		"ali_video":          constant.RelayModeAliVideo,
		"images_generations": constant.RelayModeImagesGenerations,
	}

	for modeStr, mode := range taskModes {
		if got := relayModeString(mode); got != modeStr {
			t.Errorf("relayModeString(%v) = %q, want %q", mode, got, modeStr)
		}

		// read_only 在任务提交模式下必须被拒（模式串为空串时 CheckScope 会误放行，
		// 这里同时守护 relayModeString 不退化为 ""）
		if billing.CheckScope(billing.ScopeReadOnly, modeStr) {
			t.Errorf("read_only scope 应被 %q 模式拒绝", modeStr)
		}
		// 对话/嵌入/重排/音频类密钥不得提交生成任务
		for _, scope := range []string{billing.ScopeChatOnly, billing.ScopeEmbeddingsOnly, billing.ScopeRerankOnly, billing.ScopeAudioOnly} {
			if billing.CheckScope(scope, modeStr) {
				t.Errorf("%s scope 不应允许 %q 模式", scope, modeStr)
			}
		}
		// 空 scope 与 full 不受影响
		if !billing.CheckScope("", modeStr) || !billing.CheckScope(billing.ScopeFull, modeStr) {
			t.Errorf("full/空 scope 应允许 %q 模式", modeStr)
		}
	}

	// images_only 允许异步图片（同步图片异步化管线按 images_generations 判定）
	if !billing.CheckScope(billing.ScopeImagesOnly, "images_generations") {
		t.Error("images_only scope 应允许 images_generations 模式")
	}
	if billing.CheckScope(billing.ScopeImagesOnly, "video_generations") {
		t.Error("images_only scope 不应允许 video_generations 模式")
	}
}

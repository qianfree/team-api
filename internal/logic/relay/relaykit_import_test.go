package relay

import (
	"testing"

	"github.com/qianfree/team-api/relaykit/types"
)

// TestRelaykitImport 是阶段 1 验收冒烟测试：证明主项目可通过 replace 指令
// 依赖本地 relaykit 模块，且核心类型可用。后续阶段的 handler 集成建立在此依赖之上。
func TestRelaykitImport(t *testing.T) {
	if string(types.RelayFormatOpenAI) != "openai" {
		t.Fatalf("expected RelayFormatOpenAI='openai', got %q", types.RelayFormatOpenAI)
	}
	if string(types.RelayFormatClaude) != "claude" {
		t.Fatalf("expected RelayFormatClaude='claude', got %q", types.RelayFormatClaude)
	}
	if string(types.RelayFormatGemini) != "gemini" {
		t.Fatalf("expected RelayFormatGemini='gemini', got %q", types.RelayFormatGemini)
	}
	if types.FinishReasonStop != "stop" {
		t.Fatalf("expected FinishReasonStop='stop', got %q", types.FinishReasonStop)
	}
}

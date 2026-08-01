package reasoning

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrimEffortSuffix(t *testing.T) {
	cases := []struct {
		model        string
		wantBase     string
		wantEffort   string
		wantFound    bool
	}{
		{"gpt-low", "gpt", "low", true},
		{"gpt-medium", "gpt", "medium", true},
		{"gpt-high", "gpt", "high", true},
		{"gpt-xhigh", "gpt", "xhigh", true},
		{"gpt-max", "gpt", "max", true},
		{"gpt-minimal", "gpt", "minimal", true},
		{"gpt", "gpt", "", false},
	}
	for _, c := range cases {
		base, effort, ok := TrimEffortSuffix(c.model)
		assert.Equal(t, c.wantBase, base, "model=%q base", c.model)
		assert.Equal(t, c.wantEffort, effort, "model=%q effort", c.model)
		assert.Equal(t, c.wantFound, ok, "model=%q found", c.model)
	}
}

func TestTrimEffortSuffixWithSuffixes(t *testing.T) {
	base, effort, ok := TrimEffortSuffixWithSuffixes("m-custom", []string{"-custom"})
	assert.True(t, ok)
	assert.Equal(t, "m", base)
	assert.Equal(t, "custom", effort)

	// 无匹配
	base, _, ok = TrimEffortSuffixWithSuffixes("m", []string{"-custom"})
	assert.False(t, ok)
	assert.Equal(t, "m", base)
}

func TestParseOpenAIReasoningEffortFromModelSuffix(t *testing.T) {
	effort, base := ParseOpenAIReasoningEffortFromModelSuffix("o3-high")
	assert.Equal(t, "high", effort)
	assert.Equal(t, "o3", base)

	effort, base = ParseOpenAIReasoningEffortFromModelSuffix("o3-none")
	assert.Equal(t, "none", effort)
	assert.Equal(t, "o3", base)

	// 无后缀：effort 为空，baseModel 为原名
	effort, base = ParseOpenAIReasoningEffortFromModelSuffix("o3")
	assert.Equal(t, "", effort)
	assert.Equal(t, "o3", base)
}

func TestParseDeepSeekV4ThinkingSuffix(t *testing.T) {
	// 合法：deepseek-v4-<seg>-max / -none
	base, thinking, effort, ok := ParseDeepSeekV4ThinkingSuffix("deepseek-v4-chat-max")
	assert.True(t, ok)
	assert.Equal(t, "deepseek-v4-chat", base)
	assert.Equal(t, "enabled", thinking)
	assert.Equal(t, "max", effort)

	base, thinking, effort, ok = ParseDeepSeekV4ThinkingSuffix("deepseek-v4-chat-none")
	assert.True(t, ok)
	assert.Equal(t, "deepseek-v4-chat", base)
	assert.Equal(t, "disabled", thinking)
	assert.Equal(t, "", effort)

	// 无第二段（deepseek-v4-max）：baseModel 缺少 "deepseek-v4-" 前缀 → false
	_, _, _, ok = ParseDeepSeekV4ThinkingSuffix("deepseek-v4-max")
	assert.False(t, ok)

	// 非 deepseek 前缀
	_, _, _, ok = ParseDeepSeekV4ThinkingSuffix("other-model-max")
	assert.False(t, ok)

	// 无后缀
	_, _, _, ok = ParseDeepSeekV4ThinkingSuffix("deepseek-v4-chat")
	assert.False(t, ok)
}

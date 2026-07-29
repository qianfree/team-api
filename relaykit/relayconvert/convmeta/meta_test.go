package convmeta

import (
	"testing"

	"github.com/qianfree/team-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValuesTypedNilMetaIsSafe(t *testing.T) {
	var values *Values
	var meta Meta = values

	assert.Empty(t, meta.GetOriginModelName())
	assert.Empty(t, meta.GetUpstreamModelName())
	assert.False(t, meta.HasChannelMeta())
	assert.Zero(t, meta.GetChannelID())
	assert.Zero(t, meta.GetChannelType())
	assert.False(t, meta.GetIsStream())
	assert.Empty(t, meta.GetReasoningEffort())
	assert.Zero(t, meta.GetEstimatePromptTokens())
	assert.Zero(t, meta.GetSendResponseCount())

	require.NotPanics(t, func() {
		meta.SetReasoningEffort("high")
		meta.IncrSendResponseCount()
		meta.AppendRequestConversion(types.RelayFormatClaude)
	})

	convertInfo := meta.EnsureClaudeConvertInfo()
	require.NotNil(t, convertInfo)
	assert.Equal(t, LastMessageTypeNone, convertInfo.LastMessagesType)
	require.NotNil(t, meta.ConvOptions())
	require.NotNil(t, OptionsOf(meta))
	assert.Empty(t, UpstreamModelName(meta))
	assert.Zero(t, ChannelTypeOf(meta))
}

// --- Options nil-safe 辅助方法 ---

func TestClaudeOptions_DefaultMaxTokensFor(t *testing.T) {
	var nilOpts *ClaudeOptions
	v, ok := nilOpts.DefaultMaxTokensFor("m")
	assert.False(t, ok)
	assert.Zero(t, v)

	opts := &ClaudeOptions{} // hook 为 nil
	v, ok = opts.DefaultMaxTokensFor("m")
	assert.False(t, ok)
	assert.Zero(t, v)

	opts.DefaultMaxTokens = func(model string) int { return 999 }
	v, ok = opts.DefaultMaxTokensFor("m")
	assert.True(t, ok)
	assert.Equal(t, 999, v)
}

func TestGeminiOptions_SupportsImagineModel(t *testing.T) {
	var nilOpts *GeminiOptions
	assert.False(t, nilOpts.SupportsImagineModel("m"))

	opts := &GeminiOptions{}
	assert.False(t, opts.SupportsImagineModel("m"))

	opts.SupportsImagine = func(model string) bool { return model == "imagine-1" }
	assert.True(t, opts.SupportsImagineModel("imagine-1"))
	assert.False(t, opts.SupportsImagineModel("other"))
}

func TestGeminiOptions_SafetySettingFor(t *testing.T) {
	var nilOpts *GeminiOptions
	assert.Empty(t, nilOpts.SafetySettingFor("cat"))

	opts := &GeminiOptions{SafetySetting: func(category string) string {
		if category == "HARM" {
			return "BLOCK"
		}
		return ""
	}}
	assert.Equal(t, "BLOCK", opts.SafetySettingFor("HARM"))
	assert.Empty(t, opts.SafetySettingFor("other"))
}

func TestOptions_ShouldPreserveThinkingSuffix(t *testing.T) {
	var nilOpts *Options
	assert.False(t, nilOpts.ShouldPreserveThinkingSuffix("m"))

	opts := &Options{}
	assert.False(t, opts.ShouldPreserveThinkingSuffix("m"))

	opts.PreserveThinkingSuffix = func(model string) bool { return model == "keep-me" }
	assert.True(t, opts.ShouldPreserveThinkingSuffix("keep-me"))
	assert.False(t, opts.ShouldPreserveThinkingSuffix("other"))
}

// --- Values 行为 ---

func TestValues_AppendRequestConversion_Dedup(t *testing.T) {
	v := &Values{}
	v.AppendRequestConversion(types.RelayFormatClaude)
	v.AppendRequestConversion(types.RelayFormatClaude) // 重复 → 去重
	v.AppendRequestConversion(types.RelayFormatGemini)
	v.AppendRequestConversion("") // 空格式 → no-op
	assert.Equal(t, []types.RelayFormat{types.RelayFormatClaude, types.RelayFormatGemini}, v.ConversionChain)
}

func TestValues_EnsureClaudeConvertInfo_Lazy(t *testing.T) {
	v := &Values{}
	require.Nil(t, v.ClaudeConvertInfo)
	ci := v.EnsureClaudeConvertInfo()
	require.NotNil(t, ci)
	assert.Equal(t, LastMessageTypeNone, ci.LastMessagesType)
	// 二次调用返回同一实例
	assert.Same(t, ci, v.EnsureClaudeConvertInfo())
}

func TestValues_ConvOptions_Lazy(t *testing.T) {
	v := &Values{}
	require.Nil(t, v.Options)
	opts := v.ConvOptions()
	require.NotNil(t, opts)
	assert.Same(t, opts, v.ConvOptions())
}

func TestValues_SetReasoningEffortAndIncr(t *testing.T) {
	v := &Values{}
	v.SetReasoningEffort("high")
	assert.Equal(t, "high", v.GetReasoningEffort())
	assert.Zero(t, v.GetSendResponseCount())
	v.IncrSendResponseCount()
	v.IncrSendResponseCount()
	assert.Equal(t, 2, v.GetSendResponseCount())
}

func TestPackageAccessors_WithChannelMeta(t *testing.T) {
	v := &Values{
		ChannelMetaAttached: true,
		UpstreamModelName:   "claude-x",
		ChannelType:         7,
		Options:             &Options{OpenRouterDialect: true},
	}
	var m Meta = v
	assert.Equal(t, "claude-x", UpstreamModelName(m))
	assert.Equal(t, 7, ChannelTypeOf(m))
	assert.True(t, OptionsOf(m).OpenRouterDialect)

	// 未挂载 channel meta → 取不到上游信息
	v.ChannelMetaAttached = false
	assert.Empty(t, UpstreamModelName(v))
	assert.Zero(t, ChannelTypeOf(v))
}

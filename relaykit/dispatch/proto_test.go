package dispatch

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProtoFactor_ResponsesPreference protoFactor 在 responses 入站下的软偏好：
// 匹配渠道因子为 1，不匹配渠道按策略 ResponsesMismatch 降权；ProtoAny 不降权。
func TestProtoFactor_ResponsesPreference(t *testing.T) {
	pol := DefaultRoutingPolicy()
	match := healthyChannel(1, TierPrimary, 10)
	match.SupportsResponses = true
	mismatch := healthyChannel(2, TierPrimary, 10)

	assert.Equal(t, 1.0, protoFactor(match, ProtoResponses, pol))
	assert.Equal(t, pol.Proto.ResponsesMismatch, protoFactor(mismatch, ProtoResponses, pol))
	assert.Equal(t, 1.0, protoFactor(mismatch, ProtoAny, pol))
	// chat 入站对 responses 支持与否不敏感
	assert.Equal(t, 1.0, protoFactor(match, ProtoChat, pol))
}

// TestProtoFactor_ChatBridgePreference protoFactor 在 chat 入站下的软偏好：
// 原生 chat 渠道因子为 1，chat_via_responses 桥接渠道按 ChatBridgeMismatch 降权。
func TestProtoFactor_ChatBridgePreference(t *testing.T) {
	pol := DefaultRoutingPolicy()
	native := healthyChannel(1, TierPrimary, 10)
	bridge := healthyChannel(2, TierPrimary, 10)
	bridge.ChatViaResponses = true

	assert.Equal(t, 1.0, protoFactor(native, ProtoChat, pol))
	assert.Equal(t, pol.Proto.ChatBridgeMismatch, protoFactor(bridge, ProtoChat, pol))
	assert.Equal(t, 1.0, protoFactor(bridge, ProtoResponses, pol)) // 未声明 supports_responses
}

// TestEffectiveWeight_ProtoInProduct proto 因子参与权重乘积且记录在分解明细中。
func TestEffectiveWeight_ProtoInProduct(t *testing.T) {
	pol := DefaultRoutingPolicy()
	c := healthyChannel(1, TierPrimary, 10)
	wNoPref, _ := EffectiveWeight(c, pol)
	wPref, bd := EffectiveWeight(c, pol, ProtoResponses)
	require.Equal(t, pol.Proto.ResponsesMismatch, bd.Proto)
	assert.InDelta(t, wNoPref*pol.Proto.ResponsesMismatch, wPref, 1e-9)
}

// TestCoordinator_ProtoResponsesPreference 统计意义验证：两个同权重同健康渠道，
// responses 入站时绝大多数会话键选中声明 SupportsResponses 的渠道（软偏好非硬过滤，
// HRW 哈希仍可能命中另一渠道——降权只是让匹配渠道的选中概率占优）。
func TestCoordinator_ProtoResponsesPreference(t *testing.T) {
	ctx := context.Background()
	match := healthyChannel(1, TierPrimary, 10)
	match.SupportsResponses = true
	other := healthyChannel(2, TierPrimary, 10)

	matchWins, otherWins := 0, 0
	for i := range 200 {
		state := newFakeState()
		co, _ := newTestCoordinator(state, match, other)
		p := testProfile()
		p.RequestID = fmt.Sprintf("req-%d", i)
		p.Signals = SessionSignals{HeaderSessionID: fmt.Sprintf("sess-%d", i)}
		p.Proto = ProtoResponses
		d := co.Route(ctx, p).Next(ctx)
		require.NotNil(t, d)
		if d.Channel.ID == 1 {
			matchWins++
		} else {
			otherWins++
		}
	}
	assert.Greater(t, matchWins, otherWins*3, "responses-capable channel should dominate: match=%d other=%d", matchWins, otherWins)
}

// TestCoordinator_ProtoAnyNoPreference 对照组：无偏好时两同权重渠道选中次数同量级。
func TestCoordinator_ProtoAnyNoPreference(t *testing.T) {
	ctx := context.Background()
	a := healthyChannel(1, TierPrimary, 10)
	a.SupportsResponses = true
	b := healthyChannel(2, TierPrimary, 10)

	aWins, bWins := 0, 0
	for i := range 200 {
		state := newFakeState()
		co, _ := newTestCoordinator(state, a, b)
		p := testProfile()
		p.RequestID = fmt.Sprintf("req-%d", i)
		p.Signals = SessionSignals{HeaderSessionID: fmt.Sprintf("sess-%d", i)}
		d := co.Route(ctx, p).Next(ctx)
		require.NotNil(t, d)
		if d.Channel.ID == 1 {
			aWins++
		} else {
			bWins++
		}
	}
	// HRW 均匀性：无偏好时两渠道选中次数应为同量级（放宽到 4 倍避免哈希偶然性导致的 flake）
	assert.Less(t, aWins, bWins*4)
	assert.Less(t, bWins, aWins*4)
}

// TestPolicyValidate_ProtoFactors proto 因子越界（>1 或 <=0）必须被拒绝。
func TestPolicyValidate_ProtoFactors(t *testing.T) {
	pol := DefaultRoutingPolicy()
	require.NoError(t, pol.Validate())

	pol.Proto.ResponsesMismatch = 0
	assert.Error(t, pol.Validate())
	pol = DefaultRoutingPolicy()
	pol.Proto.ResponsesMismatch = 1.5
	assert.Error(t, pol.Validate())
	pol = DefaultRoutingPolicy()
	pol.Proto.ChatBridgeMismatch = 0
	assert.Error(t, pol.Validate())
}

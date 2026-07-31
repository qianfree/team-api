package dispatchadapter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/qianfree/team-api/relaykit/dispatch"
)

func testCatalog(rows []catalogRow, keys map[int64][]int64, runtime runtimeReader) *Catalog {
	pol := dispatch.DefaultRoutingPolicy()
	c := NewCatalog(
		func() *dispatch.RoutingPolicy { return pol },
		func(_ context.Context) (*catalogData, error) {
			return &catalogData{rows: rows, keysByChannel: keys}, nil
		},
		runtime,
	)
	c.Rebuild(context.Background())
	return c
}

func TestCatalog_快照构建与范围过滤(t *testing.T) {
	rows := []catalogRow{
		{ChannelID: 1, ChannelName: "A", ModelName: "gpt-4o", Weight: 10, Tier: "primary", CostRatio: 0.8},
		{ChannelID: 2, ChannelName: "B", ModelName: "gpt-4o", Weight: 5, Tier: "secondary", CostRatio: 1.0},
		{ChannelID: 2, ChannelName: "B", ModelName: "claude-sonnet", UpstreamModel: "claude-sonnet-4", Weight: 5, Tier: "secondary"},
	}
	keys := map[int64][]int64{1: {11, 12}, 2: {21}}
	c := testCatalog(rows, keys, nil)

	// 按模型取候选
	got := c.Snapshot(context.Background(), 0, "gpt-4o", nil)
	require.Len(t, got, 2)
	assert.Equal(t, dispatch.TierPrimary, got[0].Tier)
	assert.Equal(t, []int64{11, 12}, got[0].KeyIDs)
	assert.Equal(t, 0.8, got[0].CostRatio)
	assert.Equal(t, float64(10), got[0].BaseWeight)

	// 租户渠道范围过滤
	got = c.Snapshot(context.Background(), 0, "gpt-4o", []int64{2})
	require.Len(t, got, 1)
	assert.Equal(t, int64(2), got[0].ID)

	// 未知模型返回空
	assert.Empty(t, c.Snapshot(context.Background(), 0, "unknown", nil))
}

func TestCatalog_转发元数据(t *testing.T) {
	rows := []catalogRow{
		{ChannelID: 2, ChannelName: "B", ChannelType: 3, BaseURL: "https://up.example.com",
			ModelName: "claude-sonnet", UpstreamModel: "claude-sonnet-4", Weight: 5, Tier: "secondary",
			MaxConcurrency: 20, Settings: `{"x":1}`},
	}
	c := testCatalog(rows, nil, nil)

	m, ok := c.ForwardMeta(2, "claude-sonnet")
	require.True(t, ok)
	assert.Equal(t, "claude-sonnet-4", m.UpstreamModel)
	assert.True(t, m.IsModelMapped)
	assert.Equal(t, "https://up.example.com", m.BaseURL)
	assert.Equal(t, 20, m.MaxConcurrency)

	// 未映射模型：上游模型名回退为原名
	rows[0].UpstreamModel = ""
	c = testCatalog(rows, nil, nil)
	m, _ = c.ForwardMeta(2, "claude-sonnet")
	assert.Equal(t, "claude-sonnet", m.UpstreamModel)
	assert.False(t, m.IsModelMapped)

	_, ok = c.ForwardMeta(99, "claude-sonnet")
	assert.False(t, ok)
}

func TestCatalog_运行时读值合并(t *testing.T) {
	rows := []catalogRow{
		{ChannelID: 1, ModelName: "m", Weight: 10, Tier: "primary", MaxConcurrency: 50},
	}
	c := testCatalog(rows, nil, func(_ context.Context, channelID int64, model string) RuntimeReadout {
		return RuntimeReadout{
			SuccEwma: 0.85, LatEwmaMs: 1200, Inflight: 7,
			Breaker: dispatch.BreakerHalfOpen, ModelBreaker: dispatch.BreakerClosed,
		}
	})

	got := c.Snapshot(context.Background(), 0, "m", nil)
	require.Len(t, got, 1)
	assert.Equal(t, 0.85, got[0].SuccEwma)
	assert.Equal(t, float64(1200), got[0].LatEwmaMs)
	assert.Equal(t, 7, got[0].Inflight)
	assert.Equal(t, 50, got[0].SoftLimit)
	assert.Equal(t, dispatch.BreakerHalfOpen, got[0].Breaker)
}

func TestCatalog_严格容量查询(t *testing.T) {
	rows := []catalogRow{
		{ChannelID: 1, ModelName: "m", Weight: 10, Tier: "primary", StrictCapacity: true, MaxConcurrency: 30},
		{ChannelID: 2, ModelName: "m", Weight: 10, Tier: "primary"},
	}
	c := testCatalog(rows, nil, nil)

	strict, maxConc := c.StrictLookup(1)
	assert.True(t, strict)
	assert.Equal(t, 30, maxConc)

	strict, _ = c.StrictLookup(2)
	assert.False(t, strict)
}

func TestCatalog_加载失败保留上一份快照(t *testing.T) {
	pol := dispatch.DefaultRoutingPolicy()
	calls := 0
	c := NewCatalog(
		func() *dispatch.RoutingPolicy { return pol },
		func(_ context.Context) (*catalogData, error) {
			calls++
			if calls == 1 {
				return &catalogData{rows: []catalogRow{{ChannelID: 1, ModelName: "m", Weight: 10, Tier: "primary"}}}, nil
			}
			return nil, assert.AnError
		},
		nil,
	)
	ctx := context.Background()
	c.Rebuild(ctx)
	require.Len(t, c.Snapshot(ctx, 0, "m", nil), 1)

	c.Rebuild(ctx) // 加载失败
	assert.Len(t, c.Snapshot(ctx, 0, "m", nil), 1, "last-known 快照必须保留")
}

func TestRampElapsed(t *testing.T) {
	now := time.Now().UnixMilli()
	window := int64(120_000)

	// 熔断刚恢复 60s：爬坡中
	assert.Equal(t, int64(60_000), rampElapsed(now, now-60_000, 0, window))
	// 恢复已超窗口：不爬坡
	assert.Equal(t, int64(-1), rampElapsed(now, now-130_000, 0, window))
	// 新建渠道 30s：爬坡中
	assert.Equal(t, int64(30_000), rampElapsed(now, 0, now-30_000, window))
	// 老渠道：不爬坡
	assert.Equal(t, int64(-1), rampElapsed(now, 0, now-time.Hour.Milliseconds(), window))
	// 窗口未配置：不爬坡
	assert.Equal(t, int64(-1), rampElapsed(now, now-10, 0, 0))
}

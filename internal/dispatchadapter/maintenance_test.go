package dispatchadapter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readoutOf 构造一份"有真实上报"的读值。
func readoutOf(succ, lat float64) RuntimeReadout {
	return RuntimeReadout{SuccEwma: succ, LatEwmaMs: lat, HasHealth: true}
}

// staticReader 按模型名返回预置读值；未预置的模型返回"无数据"默认满分。
func staticReader(m map[string]RuntimeReadout) func(string) RuntimeReadout {
	return func(model string) RuntimeReadout {
		if rt, ok := m[model]; ok {
			return rt
		}
		return RuntimeReadout{SuccEwma: 1} // HasHealth=false，模拟从未被真实流量访问过
	}
}

func TestAggregateChannelHealth_只统计有真实上报的模型(t *testing.T) {
	// 20 个模型的渠道死掉 1 个：旧实现把 19 个无数据模型按满分计入均值，
	// avgSucc≈0.95 → 91 分绿灯，完全掩盖故障。
	models := make([]string, 0, 20)
	for i := range 20 {
		models = append(models, string(rune('a'+i)))
	}
	agg := aggregateChannelHealth(models, staticReader(map[string]RuntimeReadout{
		"a": readoutOf(0.05, 8000),
	}))

	assert.False(t, agg.Degraded)
	assert.Equal(t, 1, agg.Counted, "只有 1 个模型有真实上报")
	assert.InDelta(t, 0.05, agg.AvgSucc, 1e-9, "无数据模型不得稀释故障模型的分数")
	assert.InDelta(t, 8000, agg.AvgLat, 1e-9)
	assert.Contains(t, agg.Detail, "a=0.050")
	assert.Contains(t, agg.Detail, "b=无数据")
}

func TestAggregateChannelHealth_多模型均值(t *testing.T) {
	agg := aggregateChannelHealth([]string{"m1", "m2", "m3"}, staticReader(map[string]RuntimeReadout{
		"m1": readoutOf(1.0, 100),
		"m2": readoutOf(0.6, 300),
		"m3": readoutOf(0.8, 200),
	}))

	assert.Equal(t, 3, agg.Counted)
	assert.InDelta(t, 0.8, agg.AvgSucc, 1e-9)
	assert.InDelta(t, 200, agg.AvgLat, 1e-9)
}

func TestAggregateChannelHealth_全部无数据不产生均值(t *testing.T) {
	agg := aggregateChannelHealth([]string{"m1", "m2"}, staticReader(nil))

	assert.False(t, agg.Degraded)
	assert.Zero(t, agg.Counted, "Counted=0 时调用方保留库中旧值，不落盘")
	assert.Zero(t, agg.AvgSucc, "不得回落成默认满分 1.0 造成 100 分落盘")
}

func TestAggregateChannelHealth_读失败整体降级(t *testing.T) {
	// Redis 故障时读值既非实测也非"确实无数据"，必须整体作废而非按满分聚合
	agg := aggregateChannelHealth([]string{"m1", "m2", "m3"}, staticReader(map[string]RuntimeReadout{
		"m1": readoutOf(0.9, 100),
		"m2": {SuccEwma: 1, Degraded: true},
		"m3": readoutOf(0.9, 100),
	}))

	assert.True(t, agg.Degraded)
	assert.Zero(t, agg.Counted, "降级时不得给出任何聚合结论")
}

func TestAggregateChannelHealth_首模型读值供自动禁用判定(t *testing.T) {
	first := readoutOf(0.5, 100)
	first.FirstOpenedMs = 12345
	agg := aggregateChannelHealth([]string{"m1", "m2"}, staticReader(map[string]RuntimeReadout{
		"m1": first,
		"m2": readoutOf(0.5, 100),
	}))

	assert.Equal(t, int64(12345), agg.ChannelRt.FirstOpenedMs, "渠道级熔断读值取首个模型")
}

// TestRequestHealthSnapshot_不阻塞且满时丢弃 调用方是管理后台请求线程（渠道测试 /
// 重置健康度），队列满时必须丢弃而非阻塞——下一轮定时维护会兜底重算。
func TestRequestHealthSnapshot_不阻塞且满时丢弃(t *testing.T) {
	drain := func() {
		for {
			select {
			case <-healthRefresh:
			default:
				return
			}
		}
	}
	drain()
	defer drain()

	for i := range cap(healthRefresh) {
		RequestHealthSnapshot(int64(i))
	}
	require.Len(t, healthRefresh, cap(healthRefresh))

	done := make(chan struct{})
	go func() {
		RequestHealthSnapshot(9999)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("队列满时 RequestHealthSnapshot 必须立即丢弃，不得阻塞调用方")
	}
	assert.Len(t, healthRefresh, cap(healthRefresh), "满队列不应再增长")
}

// TestRefreshChannelHealth_未组装时静默返回 管理后台可能在首次 relay 请求前就触发重算，
// 此时调度引擎尚未组装（catalog / redisState 为 nil），不得 panic。
func TestRefreshChannelHealth_未组装时静默返回(t *testing.T) {
	require.Nil(t, catalog, "前置条件：本包测试不组装调度引擎")
	assert.NotPanics(t, func() { refreshChannelHealth(context.Background(), 1) })
}

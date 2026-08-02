package dispatch

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scoredSet(weights map[int64]float64) []ScoredChannel {
	out := make([]ScoredChannel, 0, len(weights))
	for id, w := range weights {
		out = append(out, ScoredChannel{Channel: Channel{ID: id}, Weight: w})
	}
	return out
}

func TestPickHRW_粘性稳定(t *testing.T) {
	cands := scoredSet(map[int64]float64{1: 5, 2: 3, 3: 2})
	first := PickHRW(cands, "session-abc")
	require.NotNil(t, first)
	for range 10_000 {
		got := PickHRW(cands, "session-abc")
		require.Equal(t, first.ID, got.ID, "相同 sessionKey + 相同候选必须永远同一结果")
	}
}

func TestPickHRW_候选顺序无关(t *testing.T) {
	a := []ScoredChannel{
		{Channel: Channel{ID: 1}, Weight: 5},
		{Channel: Channel{ID: 2}, Weight: 3},
		{Channel: Channel{ID: 3}, Weight: 2},
	}
	b := []ScoredChannel{a[2], a[0], a[1]} // 打乱顺序
	for i := range 1000 {
		key := fmt.Sprintf("sess-%d", i)
		pa, pb := PickHRW(a, key), PickHRW(b, key)
		require.Equal(t, pa.ID, pb.ID, "候选顺序不得影响选择结果")
	}
}

func TestPickHRW_权重比例分布(t *testing.T) {
	// 权重 5:3:2 → 期望占比 50%/30%/20%，10 万采样误差 < 2 个百分点
	cands := scoredSet(map[int64]float64{1: 5, 2: 3, 3: 2})
	const n = 100_000
	counts := map[int64]int{}
	for i := range n {
		p := PickHRW(cands, fmt.Sprintf("session-%d", i))
		counts[p.ID]++
	}
	expect := map[int64]float64{1: 0.5, 2: 0.3, 3: 0.2}
	for id, want := range expect {
		got := float64(counts[id]) / n
		assert.InDelta(t, want, got, 0.02, "渠道 %d 占比 got=%v want=%v", id, got, want)
	}
}

func TestPickHRW_权重非正不参与(t *testing.T) {
	cands := scoredSet(map[int64]float64{1: 0, 2: -1, 3: 2})
	for i := range 1000 {
		p := PickHRW(cands, fmt.Sprintf("s-%d", i))
		require.Equal(t, int64(3), p.ID)
	}

	assert.Nil(t, PickHRW(scoredSet(map[int64]float64{1: 0}), "s"), "无正权重候选返回 nil")
	assert.Nil(t, PickHRW(nil, "s"))
}

func TestPickHRW_单调性(t *testing.T) {
	// 增加一个渠道只迁移有限比例会话：不含新渠道时选中 A 的会话，
	// 加入新渠道后要么仍选 A，要么选新渠道，绝不迁到其它旧渠道。
	oldSet := scoredSet(map[int64]float64{1: 5, 2: 3, 3: 2})
	newSet := scoredSet(map[int64]float64{1: 5, 2: 3, 3: 2, 4: 3})
	moved := 0
	const n = 20_000
	for i := range n {
		key := fmt.Sprintf("mono-%d", i)
		before, after := PickHRW(oldSet, key), PickHRW(newSet, key)
		if before.ID != after.ID {
			require.Equal(t, int64(4), after.ID, "迁移只能流向新渠道")
			moved++
		}
	}
	// 新渠道权重占比 3/13 ≈ 23%，迁移比例应接近
	assert.InDelta(t, 3.0/13.0, float64(moved)/n, 0.02)
}

func TestUniformHash_值域(t *testing.T) {
	for i := range 10_000 {
		u := uniformHash(fmt.Sprintf("k-%d", i), int64(i))
		require.Greater(t, u, 0.0)
		require.Less(t, u, 1.0)
		require.False(t, math.IsInf(-math.Log(u), 1), "不得出现 log(0)")
	}
}

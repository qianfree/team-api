package scheduler

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"sort"
	"strconv"

	"github.com/qianfree/team-api/relay/common"
)

// ChannelCandidate 调度候选渠道
type ChannelCandidate struct {
	ChannelID      int64
	ChannelType    int
	ChannelName    string
	BaseURL        string
	Priority       int
	Weight         int
	HealthScore    float64
	UpstreamModel  string
	IsModelMapped  bool
	Settings       common.ChannelSettings
	MaxConcurrency int
}

// SchedulerResult 调度结果
type SchedulerResult struct {
	ChannelID         int64
	ChannelType       int
	ChannelName       string
	BaseURL           string
	Priority          int
	Weight            int
	HealthScore       float64
	UpstreamModelName string
	IsModelMapped     bool
	Settings          common.ChannelSettings
	MaxConcurrency    int
}

// Select 从候选渠道中选择最佳渠道
// 算法：
// 1. 排除健康度 < 20 的渠道
// 2. 按优先级分组，取最高优先级组
// 3. 组内按权重加权随机选择
func Select(candidates []ChannelCandidate) *SchedulerResult {
	highestPriority := highestPriorityCandidates(candidates)
	if len(highestPriority) == 0 {
		return nil
	}
	if len(highestPriority) == 1 {
		return candidateToResult(highestPriority[0])
	}

	// 按权重随机选择
	selected := weightedRandomSelect(highestPriority)
	return candidateToResult(selected)
}

// SelectStable 在最高可用优先级组内优先使用亲和渠道，否则按权重做稳定选择。
// 相同 seed 和候选集在不同进程中会得到相同结果，适合并发冷请求和多实例部署。
func SelectStable(candidates []ChannelCandidate, preferredChannelID int64, seed string) *SchedulerResult {
	highestPriority := highestPriorityCandidates(candidates)
	if len(highestPriority) == 0 {
		return nil
	}

	if preferredChannelID > 0 {
		for _, candidate := range highestPriority {
			if candidate.ChannelID == preferredChannelID {
				return candidateToResult(candidate)
			}
		}
	}

	selected := weightedRendezvousSelect(highestPriority, seed)
	return candidateToResult(selected)
}

func highestPriorityCandidates(candidates []ChannelCandidate) []ChannelCandidate {
	selectable := make([]ChannelCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Weight > 0 {
			selectable = append(selectable, candidate)
		}
	}
	if len(selectable) == 0 {
		return nil
	}

	healthy := make([]ChannelCandidate, 0, len(selectable))
	for _, candidate := range selectable {
		if candidate.HealthScore >= 20 {
			healthy = append(healthy, candidate)
		}
	}
	if len(healthy) == 0 {
		healthy = selectable
	}

	groups := groupByPriority(healthy)
	return groups[len(groups)-1]
}

// groupByPriority 按优先级分组，返回按优先级升序排列的组
func groupByPriority(candidates []ChannelCandidate) [][]ChannelCandidate {
	if len(candidates) == 0 {
		return nil
	}

	// 按优先级排序
	sorted := make([]ChannelCandidate, len(candidates))
	copy(sorted, candidates)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	var groups [][]ChannelCandidate
	currentPriority := sorted[0].Priority
	currentGroup := []ChannelCandidate{sorted[0]}

	for i := 1; i < len(sorted); i++ {
		if sorted[i].Priority == currentPriority {
			currentGroup = append(currentGroup, sorted[i])
		} else {
			groups = append(groups, currentGroup)
			currentPriority = sorted[i].Priority
			currentGroup = []ChannelCandidate{sorted[i]}
		}
	}
	groups = append(groups, currentGroup)

	return groups
}

// weightedRandomSelect 在同优先级组内按权重加权随机选择
func weightedRandomSelect(candidates []ChannelCandidate) ChannelCandidate {
	totalWeight := 0
	for _, c := range candidates {
		w := effectiveWeight(c)
		totalWeight += w
	}

	if totalWeight <= 0 {
		return candidates[0]
	}

	r := rand.IntN(totalWeight)
	cumWeight := 0
	for _, c := range candidates {
		w := effectiveWeight(c)
		cumWeight += w
		if r < cumWeight {
			return c
		}
	}

	return candidates[len(candidates)-1]
}

func weightedRendezvousSelect(candidates []ChannelCandidate, seed string) ChannelCandidate {
	selected := candidates[0]
	bestScore := math.Inf(1)
	for _, candidate := range candidates {
		h := sha256.New()
		_, _ = h.Write([]byte(seed))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(strconv.FormatInt(candidate.ChannelID, 10)))
		sum := h.Sum(nil)
		raw := binary.BigEndian.Uint64(sum[:8])
		// 使用 53 位有效精度构造 (0,1) 区间，避免 log(0)。
		u := (float64(raw>>11) + 1) / (float64(uint64(1)<<53) + 1)
		score := -math.Log(u) / float64(effectiveWeight(candidate))
		if score < bestScore || (score == bestScore && candidate.ChannelID < selected.ChannelID) {
			bestScore = score
			selected = candidate
		}
	}
	return selected
}

func effectiveWeight(c ChannelCandidate) int {
	w := c.Weight
	if w <= 0 {
		return 0
	}

	// 健康度降权：50-79 权重减半，20-49 权重降到 1/4
	if c.HealthScore < 80 {
		if c.HealthScore >= 50 {
			w = w / 2
		} else {
			w = w / 4
		}
	}
	if w <= 0 {
		return 1
	}
	return w
}

func candidateToResult(c ChannelCandidate) *SchedulerResult {
	return &SchedulerResult{
		ChannelID:         c.ChannelID,
		ChannelType:       c.ChannelType,
		ChannelName:       c.ChannelName,
		BaseURL:           c.BaseURL,
		Priority:          c.Priority,
		Weight:            c.Weight,
		HealthScore:       c.HealthScore,
		UpstreamModelName: c.UpstreamModel,
		IsModelMapped:     c.IsModelMapped,
		Settings:          c.Settings,
		MaxConcurrency:    c.MaxConcurrency,
	}
}

package scheduler

import (
	"math/rand/v2"
	"sort"

	"github.com/qianfree/team-api/relay/common"
)

// ChannelCandidate 调度候选渠道
type ChannelCandidate struct {
	ChannelID     int64
	ChannelType   int
	ChannelName   string
	BaseURL       string
	Priority      int
	Weight        int
	HealthScore   float64
	UpstreamModel string
	IsModelMapped bool
	Settings      common.ChannelSettings
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
}

// Select 从候选渠道中选择最佳渠道
// 算法：
// 1. 排除健康度 < 20 的渠道
// 2. 按优先级分组，取最高优先级组
// 3. 组内按权重加权随机选择
func Select(candidates []ChannelCandidate) *SchedulerResult {
	if len(candidates) == 0 {
		return nil
	}

	// 排除健康度过低的渠道
	healthy := make([]ChannelCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.HealthScore >= 20 {
			healthy = append(healthy, c)
		}
	}

	// 如果全部不健康，降级使用所有渠道
	if len(healthy) == 0 {
		healthy = candidates
	}

	// 按优先级分组
	groups := groupByPriority(healthy)

	// 取最高优先级组
	highestPriority := groups[len(groups)-1]
	if len(highestPriority) == 1 {
		return candidateToResult(highestPriority[0])
	}

	// 按权重随机选择
	selected := weightedRandomSelect(highestPriority)
	return candidateToResult(selected)
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
		w := c.Weight
		if w <= 0 {
			w = 1
		}

		// 健康度降权：50-79 权重减半，20-49 权重降到 1/4
		if c.HealthScore < 80 {
			if c.HealthScore >= 50 {
				w = w / 2
			} else {
				w = w / 4
			}
		}
		totalWeight += w
	}

	if totalWeight <= 0 {
		return candidates[0]
	}

	r := rand.IntN(totalWeight)
	cumWeight := 0
	for _, c := range candidates {
		w := c.Weight
		if w <= 0 {
			w = 1
		}
		if c.HealthScore < 80 {
			if c.HealthScore >= 50 {
				w = w / 2
			} else {
				w = w / 4
			}
		}
		cumWeight += w
		if r < cumWeight {
			return c
		}
	}

	return candidates[len(candidates)-1]
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
	}
}

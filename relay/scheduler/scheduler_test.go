package scheduler

import (
	"testing"
)

func TestSelect_Empty(t *testing.T) {
	result := Select(nil)
	if result != nil {
		t.Error("expected nil for empty candidates")
	}
}

func TestSelect_SingleCandidate(t *testing.T) {
	candidates := []ChannelCandidate{
		{ChannelID: 1, Priority: 1, Weight: 100, HealthScore: 80},
	}
	result := Select(candidates)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ChannelID != 1 {
		t.Errorf("expected ChannelID 1, got %d", result.ChannelID)
	}
}

func TestSelect_PriorityGrouping(t *testing.T) {
	candidates := []ChannelCandidate{
		{ChannelID: 1, Priority: 1, Weight: 100, HealthScore: 80},
		{ChannelID: 2, Priority: 10, Weight: 100, HealthScore: 80},
		{ChannelID: 3, Priority: 10, Weight: 100, HealthScore: 80},
	}

	// Should always select from priority 10 group
	for i := 0; i < 20; i++ {
		result := Select(candidates)
		if result.ChannelID != 2 && result.ChannelID != 3 {
			t.Errorf("expected ChannelID 2 or 3, got %d", result.ChannelID)
		}
	}
}

func TestSelect_HealthFiltering(t *testing.T) {
	candidates := []ChannelCandidate{
		{ChannelID: 1, Priority: 10, Weight: 100, HealthScore: 10}, // below 20
		{ChannelID: 2, Priority: 1, Weight: 100, HealthScore: 80},
	}

	// Channel 1 has health < 20 but is only candidate in highest priority
	// Should fall back to all candidates since only one healthy candidate exists
	// Actually channel 2 is healthy (80 >= 20), so it should be selected
	result := Select(candidates)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ChannelID != 2 {
		t.Errorf("expected ChannelID 2 (healthy), got %d", result.ChannelID)
	}
}

func TestSelect_AllUnhealthy_Fallback(t *testing.T) {
	candidates := []ChannelCandidate{
		{ChannelID: 1, Priority: 10, Weight: 100, HealthScore: 10},
		{ChannelID: 2, Priority: 1, Weight: 100, HealthScore: 5},
	}

	// All unhealthy, should still return the highest priority
	result := Select(candidates)
	if result == nil {
		t.Fatal("expected non-nil result (fallback to all)")
	}
	if result.ChannelID != 1 {
		t.Errorf("expected ChannelID 1 (highest priority fallback), got %d", result.ChannelID)
	}
}

func TestSelect_WeightedRandom(t *testing.T) {
	// Channel 1 has weight 100, Channel 2 has weight 1
	// Channel 1 should be selected much more often
	candidates := []ChannelCandidate{
		{ChannelID: 1, Priority: 10, Weight: 100, HealthScore: 80},
		{ChannelID: 2, Priority: 10, Weight: 1, HealthScore: 80},
	}

	counts := map[int64]int{}
	for i := 0; i < 100; i++ {
		result := Select(candidates)
		counts[result.ChannelID]++
	}

	// Channel 1 should be selected significantly more often
	if counts[1] <= counts[2] {
		t.Errorf("expected channel 1 to be selected more often: ch1=%d, ch2=%d", counts[1], counts[2])
	}
}

func TestSelect_HealthDegradation(t *testing.T) {
	tests := []struct {
		name   string
		input  ChannelCandidate
		expect int
	}{
		{
			name:   "healthy full weight",
			input:  ChannelCandidate{Weight: 100, HealthScore: 80},
			expect: 100,
		},
		{
			name:   "degraded half weight",
			input:  ChannelCandidate{Weight: 100, HealthScore: 60},
			expect: 50,
		},
		{
			name:   "poor quarter weight",
			input:  ChannelCandidate{Weight: 100, HealthScore: 40},
			expect: 25,
		},
		{
			name:   "zero weight defaults to selectable",
			input:  ChannelCandidate{Weight: 0, HealthScore: 80},
			expect: 1,
		},
		{
			name:   "degraded low weight remains selectable",
			input:  ChannelCandidate{Weight: 1, HealthScore: 40},
			expect: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := effectiveWeight(tt.input); got != tt.expect {
				t.Fatalf("effectiveWeight() = %d, want %d", got, tt.expect)
			}
		})
	}
}

func TestGroupByPriority(t *testing.T) {
	candidates := []ChannelCandidate{
		{Priority: 1, ChannelID: 1},
		{Priority: 3, ChannelID: 3},
		{Priority: 2, ChannelID: 2},
		{Priority: 1, ChannelID: 4},
		{Priority: 3, ChannelID: 5},
	}

	groups := groupByPriority(candidates)

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	// Groups should be in ascending priority order
	if groups[0][0].Priority != 1 {
		t.Errorf("expected first group priority 1, got %d", groups[0][0].Priority)
	}
	if groups[1][0].Priority != 2 {
		t.Errorf("expected second group priority 2, got %d", groups[1][0].Priority)
	}
	if groups[2][0].Priority != 3 {
		t.Errorf("expected third group priority 3, got %d", groups[2][0].Priority)
	}

	if len(groups[0]) != 2 {
		t.Errorf("expected priority 1 group to have 2 members, got %d", len(groups[0]))
	}
	if len(groups[2]) != 2 {
		t.Errorf("expected priority 3 group to have 2 members, got %d", len(groups[2]))
	}
}

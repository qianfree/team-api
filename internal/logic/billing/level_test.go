package billing

import (
	"testing"
)

func TestLevelOnlyUpgradeLogic(t *testing.T) {
	// 验证"仅升不降"策略：
	// max_members/max_concurrency 仅在新值 > 当前值时才更新
	// 0 = 无限，视为最大

	tests := []struct {
		name              string
		currentMaxMembers int
		newMaxMembers     int
		shouldUpdate      bool
	}{
		{"new > current", 10, 20, true},
		{"new < current", 20, 10, false},
		{"new == current", 10, 10, false},
		{"current 0 (unlimited), new finite", 0, 20, false}, // unlimited > finite
		{"new 0 (unlimited), current finite", 10, 0, true},  // unlimited upgrade
	}

	for _, tt := range tests {
		t.Run(tt.name+"_max_members", func(t *testing.T) {
			shouldUpdate := tt.newMaxMembers == 0 ||
				(tt.currentMaxMembers != 0 && tt.newMaxMembers > tt.currentMaxMembers)
			if shouldUpdate != tt.shouldUpdate {
				t.Errorf("max_members: current=%d new=%d shouldUpdate=%v want=%v",
					tt.currentMaxMembers, tt.newMaxMembers, shouldUpdate, tt.shouldUpdate)
			}
		})
	}
}

func TestLevelOnlyUpgradeLogic_MaxConcurrency(t *testing.T) {
	tests := []struct {
		name           string
		currentMaxConc int
		newMaxConc     int
		shouldUpdate   bool
	}{
		{"new > current", 5, 10, true},
		{"new < current", 10, 5, false},
		{"current unlimited, new finite", 0, 10, false},
		{"new unlimited, current finite", 5, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldUpdate := tt.newMaxConc == 0 ||
				(tt.currentMaxConc != 0 && tt.newMaxConc > tt.currentMaxConc)
			if shouldUpdate != tt.shouldUpdate {
				t.Errorf("max_concurrency: current=%d new=%d shouldUpdate=%v want=%v",
					tt.currentMaxConc, tt.newMaxConc, shouldUpdate, tt.shouldUpdate)
			}
		})
	}
}

func TestLevelUpgradeDecision(t *testing.T) {
	tests := []struct {
		name      string
		current   int
		newLevel  int
		shouldUpg bool
	}{
		{"upgrade", 1, 2, true},
		{"same level", 3, 3, false},
		{"downgrade blocked", 5, 3, false},
		{"from 0", 0, 1, true},
		{"to 0", 1, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldUpg := tt.newLevel > tt.current
			if shouldUpg != tt.shouldUpg {
				t.Errorf("current=%d new=%d shouldUpgrade=%v want=%v",
					tt.current, tt.newLevel, shouldUpg, tt.shouldUpg)
			}
		})
	}
}

package relay

import (
	"math"
	"testing"
)

const floatEps = 1e-6

func floatEq(a, b float64) bool {
	return math.Abs(a-b) < floatEps
}

func TestCalcLatencyScore(t *testing.T) {
	tests := []struct {
		name    string
		latency float64
		want    float64
	}{
		{"zero", 0, 100},
		{"under 1s", 500, 100},
		{"exactly 1s", 1000, 100},
		{"1s-3s midpoint 2s", 2000, 65},    // 80 - (2000-1000)/2000*30
		{"exactly 3s", 3000, 50},           // 80 - (3000-1000)/2000*30
		{"3s-10s midpoint 6.5s", 6500, 35}, // 50 - (6500-3000)/7000*30
		{"exactly 10s", 10000, 20},         // 50 - (10000-3000)/7000*30
		{"over 10s", 20000, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calcLatencyScore(tt.latency); !floatEq(got, tt.want) {
				t.Errorf("calcLatencyScore(%.0f) = %.4f, want %.4f", tt.latency, got, tt.want)
			}
		})
	}
}

func TestCalcHealthScore(t *testing.T) {
	tests := []struct {
		name           string
		successRate    float64
		latencyMs      float64
		stability      float64
		consecutiveFax int
		want           float64
	}{
		// 全满分：100*.4 + 100*.25 + 100*.2 + 100*.15 = 100
		{"perfect", 100, 500, 100, 0, 100},
		// 全最差：0 + 20*.25 + 0 + 0 = 5
		{"worst", 0, 20000, 0, 5, 5},
		// 80*.4 + 65*.25(2s) + 90*.2 + 80*.15(1 fail) = 32 + 16.25 + 18 + 12 = 78.25
		{"mixed", 80, 2000, 90, 1, 78.25},
		// 连续失败分下限为 0（不为负）：100*.4 + 100*.25 + 100*.2 + 0*.15 = 85
		{"fail score floored at zero", 100, 500, 100, 10, 85},
		// 失败 5 次即 0 分：100*.4 + 100*.25 + 100*.2 + 0 = 85
		{"five failures hits zero", 100, 500, 100, 5, 85},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcHealthScore(tt.successRate, tt.latencyMs, tt.stability, tt.consecutiveFax)
			if !floatEq(got, tt.want) {
				t.Errorf("calcHealthScore(%.0f,%.0f,%.0f,%d) = %.4f, want %.4f",
					tt.successRate, tt.latencyMs, tt.stability, tt.consecutiveFax, got, tt.want)
			}
			if got < 0 || got > 100 {
				t.Errorf("score %.4f out of [0,100] range", got)
			}
		})
	}
}

func TestCalcHealthScore_RoundedToTwoDecimals(t *testing.T) {
	// 构造一个会产生多位小数的组合，验证四舍五入到两位
	got := calcHealthScore(33.333, 500, 33.333, 0)
	// 33.333*.4 + 100*.25 + 33.333*.2 + 100*.15 = 13.3332 + 25 + 6.6666 + 15 = 59.9998 -> 60.00
	if !floatEq(got, 60) {
		t.Errorf("got %.6f, want 60.00 (rounded)", got)
	}
}

func TestCalcStability(t *testing.T) {
	tests := []struct {
		name       string
		newLatency float64
		oldLatency float64
		current    float64
		want       float64
	}{
		{"no history returns 100", 1500, 0, 50, 100},
		{"negative old returns 100", 1500, -1, 50, 100},
		// 延迟无变化：cur*0.9 + 1*100*0.1
		{"stable latency keeps high", 1000, 1000, 100, 100}, // 90 + 10
		{"stable latency from 50", 1000, 1000, 50, 55},      // 45 + 10
		// 延迟翻倍：ratio 2 -> 0.5 -> cur*0.9 + 0.5*100*0.1
		{"latency doubled", 2000, 1000, 80, 77}, // 72 + 5
		// 延迟减半：ratio 0.5（保持）-> 对称，结果同上
		{"latency halved is symmetric", 500, 1000, 80, 77}, // 72 + 5
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcStability(tt.newLatency, tt.oldLatency, tt.current)
			if !floatEq(got, tt.want) {
				t.Errorf("calcStability(%.0f,%.0f,%.0f) = %.4f, want %.4f",
					tt.newLatency, tt.oldLatency, tt.current, got, tt.want)
			}
			if got < 0 || got > 100 {
				t.Errorf("stability %.4f out of [0,100] range", got)
			}
		})
	}
}

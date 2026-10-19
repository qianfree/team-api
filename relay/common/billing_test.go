package common

import (
	"errors"
	"testing"
)

// TestNormalizeImageCount 图片张数 n 的统一校验规则：1~10 的整数，
// n=1 归一为 0（不放大），n>1 返回 n，其余一律 ErrInvalidImageCount。
func TestNormalizeImageCount(t *testing.T) {
	okCases := []struct {
		name string
		n    float64
		want int
	}{
		{"n=1 no amplification", 1, 0},
		{"n=2", 2, 2},
		{"n=10 upper bound", 10, 10},
		{"integral float 3.0 equals 3", 3.0, 3},
	}
	for _, tt := range okCases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeImageCount(tt.n)
			if err != nil {
				t.Fatalf("NormalizeImageCount(%v) error = %v", tt.n, err)
			}
			if got != tt.want {
				t.Errorf("NormalizeImageCount(%v) = %d, want %d", tt.n, got, tt.want)
			}
		})
	}

	errCases := []struct {
		name string
		n    float64
	}{
		{"zero", 0},
		{"negative", -1},
		{"non-integer 2.5", 2.5},
		{"over limit 11", 11},
		{"way over limit", 10000},
	}
	for _, tt := range errCases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NormalizeImageCount(tt.n); !errors.Is(err, ErrInvalidImageCount) {
				t.Errorf("NormalizeImageCount(%v) error = %v, want ErrInvalidImageCount", tt.n, err)
			}
		})
	}
}

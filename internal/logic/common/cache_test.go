package common

import (
	"testing"
	"time"
)

func TestJitterTTLRange(t *testing.T) {
	base := 10 * time.Minute
	minTTL := base - base/10
	maxTTL := base + base/10
	for range 1000 {
		got := jitterTTL(base)
		if got < minTTL || got > maxTTL {
			t.Fatalf("jitterTTL(%v) = %v, want range [%v, %v]", base, got, minTTL, maxTTL)
		}
	}
	if got := jitterTTL(0); got != 0 {
		t.Fatalf("jitterTTL(0) = %v, want 0", got)
	}
}

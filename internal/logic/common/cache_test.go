package common

import (
	"encoding/json"
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

// TestUnmarshalCopyIndependent 副本必须与 target 完全独立：改写副本不得波及 target，
// 反之亦然（GetJSON L2 回填后调用方改写 target 不得产生缓存别名竞争）。
func TestUnmarshalCopyIndependent(t *testing.T) {
	type seg struct {
		Name string  `json:"name"`
		Mult float64 `json:"mult"`
	}
	type payload struct {
		TimeMultiplier float64 `json:"time_multiplier"`
		TimeRule       string  `json:"time_rule"`
		Segs           []seg   `json:"segs"`
	}

	target := payload{}
	jsonStr := `{"time_multiplier":1.5,"time_rule":"忙时","segs":[{"name":"a","mult":2}]}`
	if err := json.Unmarshal([]byte(jsonStr), &target); err != nil {
		t.Fatalf("unmarshal target: %v", err)
	}

	cpAny, err := unmarshalCopy(jsonStr, &target)
	if err != nil {
		t.Fatalf("unmarshalCopy: %v", err)
	}
	cp, ok := cpAny.(*payload)
	if !ok {
		t.Fatalf("unmarshalCopy 返回类型 %T, want *payload", cpAny)
	}

	// 初始值一致
	if cp.TimeRule != target.TimeRule || cp.TimeMultiplier != target.TimeMultiplier || len(cp.Segs) != len(target.Segs) {
		t.Fatalf("副本初始值与 target 不一致: %+v vs %+v", cp, &target)
	}

	// 改写副本不波及 target
	cp.TimeMultiplier = 0.5
	cp.TimeRule = "闲时"
	if target.TimeMultiplier != 1.5 || target.TimeRule != "忙时" {
		t.Fatalf("改写副本波及了 target: %+v", &target)
	}

	// 改写 target 不波及副本
	target.TimeMultiplier = 9.9
	if cp.TimeMultiplier != 0.5 {
		t.Fatalf("改写 target 波及了副本: %+v", cp)
	}
}

// TestUnmarshalCopyRejectsNonPointer 非 pointer target 必须报错而非 panic。
func TestUnmarshalCopyRejectsNonPointer(t *testing.T) {
	if _, err := unmarshalCopy(`{}`, map[string]any{}); err == nil {
		t.Fatal("non-pointer target 应返回错误")
	}
}

package relayconvert

import (
	"testing"
)

// validReqSpec 构造一个最小合法 RequestConverterSpec（直转，Convert 非 nil）。
func validReqSpec(id string) RequestConverterSpec {
	from, to := makeRoute(id, "req")
	return RequestConverterSpec{
		ID:      id,
		From:    from,
		To:      to,
		Quality: RequestConverterQualityGood,
		Convert: noopReqConvert,
	}
}

func TestLookupRequestConverter_NotFound(t *testing.T) {
	if _, ok := LookupRequestConverter("definitely_not_registered_req"); ok {
		t.Fatal("expected not-found on unknown id")
	}
}

func TestRegisterBuiltinRequestConverter_Valid(t *testing.T) {
	spec := validReqSpec("req_basic")
	registerBuiltinRequestConverter(spec)

	got, ok := LookupRequestConverter("req_basic")
	if !ok {
		t.Fatal("expected converter registered")
	}
	if got.From != spec.From || got.To != spec.To {
		t.Errorf("From/To mismatch: %q/%q", got.From, got.To)
	}
	if got.Quality != RequestConverterQualityGood {
		t.Errorf("Quality = %q, want good", got.Quality)
	}
	if got.Convert == nil {
		t.Error("expected non-nil Convert")
	}

	// 路由查找：direct route 命中
	if _, ok := lookupRequestRoute(spec.From, spec.To); !ok {
		t.Error("lookupRequestRoute expected hit")
	}
	if _, ok := lookupRequestDirectRoute(spec.From, spec.To); !ok {
		t.Error("lookupRequestDirectRoute expected hit for direct converter")
	}

	// 带 ID 空格被 trim
	registerBuiltinRequestConverter(validReqSpec("  req_trim  "))
	if _, ok := LookupRequestConverter("req_trim"); !ok {
		t.Error("expected trimmed id to be lookup-able")
	}
}

func TestRegisterBuiltinRequestConverter_Panics(t *testing.T) {
	// 空 ID
	assertPanic(t, func() {
		s := validReqSpec("req_empty_id")
		s.ID = " "
		registerBuiltinRequestConverter(s)
	})
	// 空 From
	assertPanic(t, func() {
		s := validReqSpec("req_empty_from")
		s.From = ""
		registerBuiltinRequestConverter(s)
	})
	// 空 Quality
	assertPanic(t, func() {
		s := validReqSpec("req_empty_quality")
		s.Quality = ""
		registerBuiltinRequestConverter(s)
	})
	// 既无 Convert 也无 StepConverters
	assertPanic(t, func() {
		s := validReqSpec("req_no_convert")
		s.Convert = nil
		registerBuiltinRequestConverter(s)
	})
	// 同时声明 Convert 和 StepConverters
	assertPanic(t, func() {
		s := validReqSpec("req_convert_and_steps")
		s.StepConverters = []string{"anything"}
		registerBuiltinRequestConverter(s)
	})
	// 重复 ID
	assertPanic(t, func() {
		registerBuiltinRequestConverter(validReqSpec("req_dup"))
		registerBuiltinRequestConverter(validReqSpec("req_dup"))
	})
	// 重复路由（不同 ID）
	assertPanic(t, func() {
		from, to := makeRoute("req_route_collision", "req")
		a := RequestConverterSpec{ID: "req_route_a", From: from, To: to, Quality: RequestConverterQualityGood, Convert: noopReqConvert}
		b := RequestConverterSpec{ID: "req_route_b", From: from, To: to, Quality: RequestConverterQualityGood, Convert: noopReqConvert}
		registerBuiltinRequestConverter(a)
		registerBuiltinRequestConverter(b) // 同路由 → panic
	})
}

// 注册步骤转换器的合法路径，并校验路由/direct 查找行为。
func TestRegisterBuiltinRequestConverter_StepConverters(t *testing.T) {
	registerBuiltinRequestConverter(RequestConverterSpec{
		ID: "req_step_ab", From: "req_step_fmt_a", To: "req_step_fmt_b",
		Quality: RequestConverterQualityGood, Convert: noopReqConvert,
	})
	registerBuiltinRequestConverter(RequestConverterSpec{
		ID: "req_step_bc", From: "req_step_fmt_b", To: "req_step_fmt_c",
		Quality: RequestConverterQualityGood, Convert: noopReqConvert,
	})
	// 合法步骤转换器 A→C（经过 A→B、B→C）
	registerBuiltinRequestConverter(RequestConverterSpec{
		ID: "req_step_ac", From: "req_step_fmt_a", To: "req_step_fmt_c",
		Quality: RequestConverterQualityGood, StepConverters: []string{"req_step_ab", "req_step_bc"},
	})

	got, ok := LookupRequestConverter("req_step_ac")
	if !ok {
		t.Fatal("expected step converter registered")
	}
	if len(got.StepConverters) != 2 || got.StepConverters[0] != "req_step_ab" || got.StepConverters[1] != "req_step_bc" {
		t.Errorf("StepConverters = %v", got.StepConverters)
	}
	// 步骤转换器不出现在 direct 路由表
	if _, ok := lookupRequestDirectRoute("req_step_fmt_a", "req_step_fmt_c"); ok {
		t.Error("step converter should not appear in direct routes")
	}
	// 但普通路由查找命中
	if _, ok := lookupRequestRoute("req_step_fmt_a", "req_step_fmt_c"); !ok {
		t.Error("lookupRequestRoute expected hit for step converter")
	}
}

// 覆盖步骤转换器校验的各个 panic 分支。每个分支用独立链路名，互不冲突。
func TestRegisterBuiltinRequestConverter_StepPanics(t *testing.T) {
	// 1) 引用未知 step
	assertPanic(t, func() {
		registerBuiltinRequestConverter(RequestConverterSpec{
			ID: "req_step_unknown", From: "req_su_from", To: "req_su_to",
			Quality: RequestConverterQualityGood, StepConverters: []string{"req_step_nonexistent"},
		})
	})

	// 2) “step 必须是 direct 转换器”：先合法注册一个步骤转换器，再用它当别人的 step。
	registerBuiltinRequestConverter(RequestConverterSpec{
		ID: "req_chain_direct_pq", From: "req_chain_p", To: "req_chain_q",
		Quality: RequestConverterQualityGood, Convert: noopReqConvert,
	})
	registerBuiltinRequestConverter(RequestConverterSpec{
		ID: "req_chain_direct_qr", From: "req_chain_q", To: "req_chain_r",
		Quality: RequestConverterQualityGood, Convert: noopReqConvert,
	})
	registerBuiltinRequestConverter(RequestConverterSpec{
		ID: "req_chain_step_pr", From: "req_chain_p", To: "req_chain_r",
		Quality: RequestConverterQualityGood, StepConverters: []string{"req_chain_direct_pq", "req_chain_direct_qr"},
	})
	// req_chain_step_pr 自身带 StepConverters → 作为 step 引用会触发 “must be a direct converter”
	assertPanic(t, func() {
		registerBuiltinRequestConverter(RequestConverterSpec{
			ID: "req_chain_step_nondirect", From: "req_chain_p", To: "req_chain_x",
			Quality: RequestConverterQualityGood, StepConverters: []string{"req_chain_step_pr"},
		})
	})

	// 3) From 不衔接：step 的 From 与当前链路起点不符
	assertPanic(t, func() {
		registerBuiltinRequestConverter(RequestConverterSpec{
			ID: "req_step_frommismatch", From: "req_mm_z", To: "req_chain_q",
			Quality: RequestConverterQualityGood, StepConverters: []string{"req_chain_direct_pq"},
		})
	})

	// 4) 终点不匹配：链路终点与 spec.To 不符
	assertPanic(t, func() {
		registerBuiltinRequestConverter(RequestConverterSpec{
			ID: "req_step_endmismatch", From: "req_chain_p", To: "req_chain_wrong",
			Quality: RequestConverterQualityGood, StepConverters: []string{"req_chain_direct_pq"},
		})
	})
}

func TestCloneRequestConverterSpec(t *testing.T) {
	// 构造一条合法的两段链 A→B→C，步骤转换器 A→C 持有 StepConverters。
	registerBuiltinRequestConverter(RequestConverterSpec{
		ID: "req_clone_ab", From: "req_clone_a", To: "req_clone_b",
		Quality: RequestConverterQualityGood, Convert: noopReqConvert,
	})
	registerBuiltinRequestConverter(RequestConverterSpec{
		ID: "req_clone_bc", From: "req_clone_b", To: "req_clone_c",
		Quality: RequestConverterQualityGood, Convert: noopReqConvert,
	})
	registerBuiltinRequestConverter(RequestConverterSpec{
		ID: "req_clone_iso", From: "req_clone_a", To: "req_clone_c",
		Quality: RequestConverterQualityGood, StepConverters: []string{"req_clone_ab", "req_clone_bc"},
	})
	got, _ := LookupRequestConverter("req_clone_iso")
	got.StepConverters = append(got.StepConverters, "extra")

	again, _ := LookupRequestConverter("req_clone_iso")
	if len(again.StepConverters) != 2 {
		t.Errorf("registry StepConverters corrupted after clone mutation: %v", again.StepConverters)
	}
}

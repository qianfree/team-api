package relayconvert

import (
	"testing"
)

// validRespSpec 构造一个最小合法 ResponseConverterSpec（Convert 非 nil）。
func validRespSpec(id string) ResponseConverterSpec {
	from, to := makeRoute(id, "resp")
	return ResponseConverterSpec{
		ID:      id,
		From:    from,
		To:      to,
		Quality: ResponseConverterQualityGood,
		Convert: noopRespConvert,
	}
}

func TestLookupResponseConverter_NotFound(t *testing.T) {
	if _, ok := LookupResponseConverter("definitely_not_registered_resp"); ok {
		t.Fatal("expected not-found on unknown id")
	}
}

func TestRegisterBuiltinResponseConverter_Valid(t *testing.T) {
	spec := validRespSpec("resp_basic")
	registerBuiltinResponseConverter(spec)

	got, ok := LookupResponseConverter("resp_basic")
	if !ok {
		t.Fatal("expected converter registered")
	}
	if got.From != spec.From || got.To != spec.To {
		t.Errorf("From/To mismatch: %q/%q", got.From, got.To)
	}
	if got.Convert == nil {
		t.Error("expected non-nil Convert")
	}
}

func TestRegisterBuiltinResponseConverter_Panics(t *testing.T) {
	// 空 ID
	assertPanic(t, func() {
		s := validRespSpec("resp_empty_id")
		s.ID = " "
		registerBuiltinResponseConverter(s)
	})
	// 空 From
	assertPanic(t, func() {
		s := validRespSpec("resp_empty_from")
		s.From = ""
		registerBuiltinResponseConverter(s)
	})
	// 空 Quality
	assertPanic(t, func() {
		s := validRespSpec("resp_empty_quality")
		s.Quality = ""
		registerBuiltinResponseConverter(s)
	})
	// 无任何实现且无 step
	assertPanic(t, func() {
		s := validRespSpec("resp_no_impl")
		s.Convert = nil
		registerBuiltinResponseConverter(s)
	})
	// direct 与 step 共存
	assertPanic(t, func() {
		s := validRespSpec("resp_direct_and_steps")
		s.StepConverters = []string{"anything"}
		registerBuiltinResponseConverter(s)
	})
	// 重复 ID
	assertPanic(t, func() {
		registerBuiltinResponseConverter(validRespSpec("resp_dup"))
		registerBuiltinResponseConverter(validRespSpec("resp_dup"))
	})
	// 重复路由
	assertPanic(t, func() {
		from, to := makeRoute("resp_route_collision", "resp")
		a := ResponseConverterSpec{ID: "resp_route_a", From: from, To: to, Quality: ResponseConverterQualityGood, Convert: noopRespConvert}
		b := ResponseConverterSpec{ID: "resp_route_b", From: from, To: to, Quality: ResponseConverterQualityGood, Convert: noopRespConvert}
		registerBuiltinResponseConverter(a)
		registerBuiltinResponseConverter(b)
	})
}

func TestRegisterBuiltinResponseConverter_StepConverters(t *testing.T) {
	registerBuiltinResponseConverter(ResponseConverterSpec{
		ID: "resp_step_ab", From: "resp_step_fmt_a", To: "resp_step_fmt_b",
		Quality: ResponseConverterQualityGood, Convert: noopRespConvert,
	})
	registerBuiltinResponseConverter(ResponseConverterSpec{
		ID: "resp_step_bc", From: "resp_step_fmt_b", To: "resp_step_fmt_c",
		Quality: ResponseConverterQualityGood, Convert: noopRespConvert,
	})
	// 合法步骤转换器 A→C
	registerBuiltinResponseConverter(ResponseConverterSpec{
		ID: "resp_step_ac", From: "resp_step_fmt_a", To: "resp_step_fmt_c",
		Quality: ResponseConverterQualityGood, StepConverters: []string{"resp_step_ab", "resp_step_bc"},
	})
	got, ok := LookupResponseConverter("resp_step_ac")
	if !ok {
		t.Fatal("expected step response converter registered")
	}
	if len(got.StepConverters) != 2 {
		t.Errorf("StepConverters = %v", got.StepConverters)
	}

	// panic：引用未知 step
	assertPanic(t, func() {
		registerBuiltinResponseConverter(ResponseConverterSpec{
			ID: "resp_step_unknown", From: "resp_su_from", To: "resp_su_to",
			Quality: ResponseConverterQualityGood, StepConverters: []string{"resp_step_nonexistent"},
		})
	})
	// panic：step 非 direct（引用了步骤转换器 resp_step_ac）
	assertPanic(t, func() {
		registerBuiltinResponseConverter(ResponseConverterSpec{
			ID: "resp_step_nondirect", From: "resp_step_fmt_a", To: "resp_step_fmt_z",
			Quality: ResponseConverterQualityGood, StepConverters: []string{"resp_step_ac"},
		})
	})
	// panic：From 不衔接
	assertPanic(t, func() {
		registerBuiltinResponseConverter(ResponseConverterSpec{
			ID: "resp_step_frommismatch", From: "resp_mm_z", To: "resp_step_fmt_b",
			Quality: ResponseConverterQualityGood, StepConverters: []string{"resp_step_ab"},
		})
	})
	// panic：终点不匹配
	assertPanic(t, func() {
		registerBuiltinResponseConverter(ResponseConverterSpec{
			ID: "resp_step_endmismatch", From: "resp_step_fmt_a", To: "resp_step_fmt_wrong",
			Quality: ResponseConverterQualityGood, StepConverters: []string{"resp_step_ab"},
		})
	})
}

func TestRegisterResponseConverterAlias(t *testing.T) {
	registerBuiltinResponseConverter(validRespSpec("resp_alias_target"))

	// 合法别名：经别名可查
	registerResponseConverterAlias("resp_alias_canonical", "resp_alias_target")
	if _, ok := LookupResponseConverter("resp_alias_canonical"); !ok {
		t.Error("expected alias to resolve to canonical converter")
	}
	// alias == converter 视为无操作
	registerResponseConverterAlias("resp_alias_target", "resp_alias_target")

	// panic 分支
	assertPanic(t, func() { registerResponseConverterAlias("  ", "resp_alias_target") })
	assertPanic(t, func() { registerResponseConverterAlias("resp_alias_bad_target", " ") })
	assertPanic(t, func() { registerResponseConverterAlias("resp_alias_target", "resp_alias_canonical") }) // 别名与已注册 converter 冲突
	assertPanic(t, func() { registerResponseConverterAlias("resp_alias_unknown", "resp_does_not_exist") }) // 引用未知 converter
}

func TestCloneResponseConverterSpec(t *testing.T) {
	// 构造一条合法的两段链 A→B→C，步骤转换器 A→C 持有 StepConverters。
	registerBuiltinResponseConverter(ResponseConverterSpec{
		ID: "resp_clone_ab", From: "resp_clone_a", To: "resp_clone_b",
		Quality: ResponseConverterQualityGood, Convert: noopRespConvert,
	})
	registerBuiltinResponseConverter(ResponseConverterSpec{
		ID: "resp_clone_bc", From: "resp_clone_b", To: "resp_clone_c",
		Quality: ResponseConverterQualityGood, Convert: noopRespConvert,
	})
	registerBuiltinResponseConverter(ResponseConverterSpec{
		ID: "resp_clone_iso", From: "resp_clone_a", To: "resp_clone_c",
		Quality: ResponseConverterQualityGood, StepConverters: []string{"resp_clone_ab", "resp_clone_bc"},
	})
	got, _ := LookupResponseConverter("resp_clone_iso")
	got.StepConverters = append(got.StepConverters, "extra")

	again, _ := LookupResponseConverter("resp_clone_iso")
	if len(again.StepConverters) != 2 {
		t.Errorf("registry StepConverters corrupted after clone mutation: %v", again.StepConverters)
	}
}

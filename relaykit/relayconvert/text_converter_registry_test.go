package relayconvert

import (
	"testing"

	"github.com/qianfree/team-api/relaykit/types"
)

// validTextSpec 构造一个最小合法 TextConverterSpec（From/To 由 id 派生保证唯一）。
func validTextSpec(id string) TextConverterSpec {
	from, to := makeRoute(id, "text")
	return TextConverterSpec{
		ID:      id,
		From:    from,
		To:      to,
		Quality: TextConverterQualityGood,
		Req:     TextRequestSide{Convert: noopReqConvert},
		Resp:    TextResponseSide{Convert: noopRespConvert},
	}
}

func TestLookupTextConverter_NotFound(t *testing.T) {
	if _, ok := LookupTextConverter("definitely_not_registered_text"); ok {
		t.Fatal("expected not-found on empty/unknown id")
	}
}

func TestRegisterAndLookupTextConverter(t *testing.T) {
	spec := validTextSpec("text_basic")
	RegisterTextConverter(spec)

	got, ok := LookupTextConverter("text_basic")
	if !ok {
		t.Fatal("expected converter to be registered")
	}
	if got.ID != "text_basic" {
		t.Errorf("ID = %q, want text_basic", got.ID)
	}
	if got.From != spec.From || got.To != spec.To {
		t.Errorf("From/To = %q/%q, want %q/%q", got.From, got.To, spec.From, spec.To)
	}
	if got.Quality != TextConverterQualityGood {
		t.Errorf("Quality = %q, want good", got.Quality)
	}
	if got.Req.Convert == nil || got.Resp.Convert == nil {
		t.Error("expected non-nil Req.Convert and Resp.Convert")
	}

	// 带空格的 ID 应被 trim
	RegisterTextConverter(validTextSpec("  text_trim  "))
	if _, ok := LookupTextConverter("text_trim"); !ok {
		t.Error("expected trimmed id to be lookup-able")
	}
}

func TestRegisterTextConverter_Panics(t *testing.T) {
	// 空 ID
	assertPanic(t, func() {
		s := validTextSpec("text_empty_id")
		s.ID = " "
		RegisterTextConverter(s)
	})
	// 空 From
	assertPanic(t, func() {
		s := validTextSpec("text_empty_from")
		s.From = ""
		RegisterTextConverter(s)
	})
	// 空 To
	assertPanic(t, func() {
		s := validTextSpec("text_empty_to")
		s.To = ""
		RegisterTextConverter(s)
	})
	// 空 Quality
	assertPanic(t, func() {
		s := validTextSpec("text_empty_quality")
		s.Quality = ""
		RegisterTextConverter(s)
	})
	// 双侧全空（无请求转换也无响应转换）
	assertPanic(t, func() {
		s := validTextSpec("text_no_both")
		s.Req = TextRequestSide{}
		s.Resp = TextResponseSide{}
		RegisterTextConverter(s)
	})
	// 单侧 spec 合法：仅请求方向（Req-only，如 Responses→OpenAI chat 请求侧转换）
	{
		s := validTextSpec("text_req_only")
		s.Resp = TextResponseSide{}
		RegisterTextConverter(s)
		if _, ok := LookupTextConverter("text_req_only"); !ok {
			t.Error("req-only text converter should register and be lookup-able")
		}
		if _, ok := LookupRequestConverter("text_req_only"); !ok {
			t.Error("req-only text converter should be registered in request registry")
		}
	}
	// 单侧 spec 合法：仅响应方向（Resp-only）
	{
		s := validTextSpec("text_resp_only")
		s.Req = TextRequestSide{}
		RegisterTextConverter(s)
		if _, ok := LookupTextConverter("text_resp_only"); !ok {
			t.Error("resp-only text converter should register and be lookup-able")
		}
	}
	// 重复 ID（不同路由）
	assertPanic(t, func() {
		RegisterTextConverter(validTextSpec("text_dup"))
		RegisterTextConverter(validTextSpec("text_dup")) // 同 ID，makeRoute 派生相同路由
	})
}

func TestRegisterTextConverterAlias(t *testing.T) {
	RegisterTextConverter(validTextSpec("text_alias_target"))

	// 合法别名：可经别名查找
	registerTextConverterAlias("text_alias_canonical", "text_alias_target")
	if _, ok := LookupTextConverter("text_alias_canonical"); !ok {
		t.Error("expected alias to resolve to canonical converter")
	}
	// alias == converter 视为无操作（不 panic）
	registerTextConverterAlias("text_alias_target", "text_alias_target")

	// 空 alias
	assertPanic(t, func() { registerTextConverterAlias("  ", "text_alias_target") })
	// 空 target
	assertPanic(t, func() { registerTextConverterAlias("text_alias_bad_target", " ") })
	// 别名与已注册 converter 冲突
	assertPanic(t, func() { registerTextConverterAlias("text_alias_target", "text_alias_canonical") })
	// 引用未知 converter
	assertPanic(t, func() { registerTextConverterAlias("text_alias_unknown", "text_does_not_exist") })
}

func TestTextRequestResponseSideConfigured(t *testing.T) {
	// 仅 StepConverters 也算请求侧已配置
	if !textRequestSideConfigured(TextRequestSide{StepConverters: []string{"a"}}) {
		t.Error("StepConverters-only request side should be configured")
	}
	if textRequestSideConfigured(TextRequestSide{}) {
		t.Error("empty request side should not be configured")
	}
	// 响应侧：单个流式字段非 nil 即视为已配置
	if !textResponseSideConfigured(TextResponseSide{ConvertStream: noopRespConvert}) {
		t.Error("ConvertStream-only response side should be configured")
	}
	if textResponseSideConfigured(TextResponseSide{}) {
		t.Error("empty response side should not be configured")
	}
}

func TestCloneTextConverterStrings(t *testing.T) {
	if got := cloneTextConverterStrings(nil); got != nil {
		t.Errorf("cloneTextConverterStrings(nil) = %v, want nil", got)
	}
	src := []string{"a", "b"}
	clone := cloneTextConverterStrings(src)
	if len(clone) != len(src) || clone[0] != "a" || clone[1] != "b" {
		t.Errorf("clone = %v, want %v", clone, src)
	}
	// 修改 clone 不影响源
	clone = append(clone, "c")
	if len(src) != 2 {
		t.Errorf("source slice mutated after clone append: %v", src)
	}
}

func TestLookupTextConverter_CloneIsolation(t *testing.T) {
	// 注意：Req/Resp 侧不能同时声明 Convert 与 StepConverters（请求/响应注册表会拒绝），
	// 故此处仅用 Resp.Aliases 验证 clone 隔离（cloneTextConverterSpec 会克隆 Aliases 切片）。
	RegisterTextConverter(TextConverterSpec{
		ID:      "text_clone_iso",
		From:    types.RelayFormat("text_clone_iso_from"),
		To:      types.RelayFormat("text_clone_iso_to"),
		Quality: TextConverterQualityGood,
		Req:     TextRequestSide{Convert: noopReqConvert},
		Resp:    TextResponseSide{Convert: noopRespConvert, Aliases: []string{"text_clone_alias"}},
	})

	s1, _ := LookupTextConverter("text_clone_iso")
	// 篡改返回的 clone 切片
	s1.Resp.Aliases = append(s1.Resp.Aliases, "z")

	// 再次查找，注册表内的 Aliases 应不受影响
	s2, _ := LookupTextConverter("text_clone_iso")
	if len(s2.Resp.Aliases) != 1 || s2.Resp.Aliases[0] != "text_clone_alias" {
		t.Errorf("registry Aliases corrupted: %v", s2.Resp.Aliases)
	}
}

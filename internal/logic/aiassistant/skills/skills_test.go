package skills

import (
	"strings"
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin"
)

// TestBillingFormulaReferenceDynamic 计费表达式参考动态生成：
// 变量表（KnownVarDocs）与 u() 注册表（KnownFactKeys）都来自代码事实。
func TestBillingFormulaReferenceDynamic(t *testing.T) {
	ref := BillingFormulaSkill{}.Reference()
	for _, want := range []string{"p：", "c：", "image_count", "audio_seconds", "fixed(", "每 1M token"} {
		if !strings.Contains(ref, want) {
			t.Fatalf("参考缺少 %q", want)
		}
	}
}

// TestBillingFormulaValidate 合法/非法公式的校验闭环。
func TestBillingFormulaValidate(t *testing.T) {
	s := BillingFormulaSkill{}
	rep := s.Validate("p*2+c*6")
	if !rep.OK {
		t.Fatalf("合法公式应通过: %+v", rep)
	}
	if len(rep.Preview) == 0 {
		t.Fatal("应返回烟测试算结果")
	}

	rep = s.Validate(`p*2+u("no_such_key")`)
	if rep.OK || !strings.Contains(rep.Error, "no_such_key") {
		t.Fatalf("未注册 u() key 应拒绝: %+v", rep)
	}

	rep = s.Validate("((p*")
	if rep.OK {
		t.Fatal("语法错误应拒绝")
	}

	rep = s.Validate("   ")
	if rep.OK {
		t.Fatal("空表达式应拒绝")
	}
}

// TestJSPluginReferenceTemplate js_plugin 参考注入含完整 golden 模板。
func TestJSPluginReferenceTemplate(t *testing.T) {
	ref := JSPluginSkill{}.Reference()
	if !strings.Contains(ref, "export const meta") {
		t.Fatal("应包含 golden 模板全文")
	}
	if !strings.Contains(ref, jsplugin.TemplateSource()) {
		t.Fatal("golden 模板应全量注入")
	}
}

// TestJSPluginValidateTemplate golden 模板自身应通过 dry-run 校验（引擎与契约的基准夹具）。
func TestJSPluginValidateTemplate(t *testing.T) {
	rep := JSPluginSkill{}.Validate(jsplugin.TemplateSource())
	if !rep.OK {
		t.Fatalf("golden 模板应通过校验: %+v", rep)
	}
	if len(rep.Checks) == 0 {
		t.Fatal("应返回检查项清单")
	}
}

// TestJSPluginValidateBadSource 坏源码应 fail。
func TestJSPluginValidateBadSource(t *testing.T) {
	rep := JSPluginSkill{}.Validate("export const notMeta = 1")
	if rep.OK {
		t.Fatalf("缺 meta 的源码应不通过: %+v", rep)
	}
	rep = JSPluginSkill{}.Validate("")
	if rep.OK {
		t.Fatal("空源码应不通过")
	}
}

// TestPromptFor 协议拼装：多技能按序、未知忽略、空入参空串。
func TestPromptFor(t *testing.T) {
	if PromptFor(nil) != "" {
		t.Fatal("空入参应返回空串")
	}
	p := PromptFor([]string{"js_plugin", "billing_formula", "unknown"})
	if !strings.Contains(p, "````formula") || !strings.Contains(p, "````js") {
		t.Fatal("协议应包含两个技能的围栏标记")
	}
	// billing_formula 在前（字母序）
	if strings.Index(p, "【技能：计费表达式生成】") > strings.Index(p, "【技能：JS 插件代码生成】") {
		t.Fatal("技能参考应按字母序拼接")
	}
}

// TestRegistryGetAndGetKeys 注册表可用性。
func TestRegistryGetAndGetKeys(t *testing.T) {
	if _, ok := Get("billing_formula"); !ok {
		t.Fatal("billing_formula 应已注册")
	}
	if _, ok := Get("nope"); ok {
		t.Fatal("未注册 key 应返回 false")
	}
	if keys := Keys(); len(keys) != 2 {
		t.Fatalf("应有 2 个技能，得到 %v", keys)
	}
}

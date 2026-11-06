package jsplugin

import (
	"testing"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// 掩码/空值保持语义是加密配置的生命线：判断必须发生在加密之前。
// 历史缺陷：configValueFromAny 先返回密文，UpdateConfig 再拿密文比对掩码，
// 恒不相等 → 整表单保存把真实密钥静默替换成 "********" 的密文。
func TestKeepCurrentValue(t *testing.T) {
	enc := contract.ConfigField{Key: "secret", Type: "string", Encrypted: true}
	plain := contract.ConfigField{Key: "name", Type: "string"}

	cases := []struct {
		name  string
		field contract.ConfigField
		value any
		want  bool
	}{
		{"加密字段掩码保持", enc, "********", true},
		{"加密字段空串保持", enc, "", true},
		{"加密字段空白串保持", enc, "   ", true},
		{"加密字段新值覆盖", enc, "new-secret", false},
		{"加密字段非字符串不保持", enc, 123, false},
		{"明文字段掩码保持", plain, "********", true},
		{"明文字段空串清空", plain, "", false},
		{"明文字段新值覆盖", plain, "hello", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := keepCurrentValue(c.field, c.value); got != c.want {
				t.Fatalf("keepCurrentValue(%v, %v) = %v, want %v", c.field.Key, c.value, got, c.want)
			}
		})
	}
}

// int 型配置拒绝非数字字符串：原样入库会让 parseConfigValue 静默归 0。
func TestConfigValueFromAnyIntValidation(t *testing.T) {
	ctx := t.Context()
	field := contract.ConfigField{Key: "limit", Type: "int"}

	if str, err := configValueFromAny(ctx, field, float64(42)); err != nil || str != "42" {
		t.Fatalf("float64 输入应转为 42，得到 %q err=%v", str, err)
	}
	if str, err := configValueFromAny(ctx, field, " 7 "); err != nil || str != "7" {
		t.Fatalf("数字字符串应归一为 7，得到 %q err=%v", str, err)
	}
	if _, err := configValueFromAny(ctx, field, "abc"); err == nil {
		t.Fatal("非数字字符串应报错，而不是原样入库")
	}
	if _, err := configValueFromAny(ctx, field, true); err == nil {
		t.Fatal("布尔值应报错")
	}
}

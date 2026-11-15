package jsplugin

import (
	"context"
	"errors"
	"testing"

	"github.com/gogf/gf/v2/errors/gerror"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// TestPluginOpErrorCarriesBusinessCode 插件管理校验错误必须带业务码（>=10000）：
// response.Error 只透出 code>=10000 的消息（HTTP 422），不带码会被脱敏成
// 「服务器内部错误」，管理员在前端看不到真实原因（版本冲突等文案因此失效）。
func TestPluginOpErrorCarriesBusinessCode(t *testing.T) {
	err := pluginOpError("插件 %s@%s 已存在且源码不同", "myapi", "1.4.0")
	var gerr *gerror.Error
	if !errors.As(err, &gerr) {
		t.Fatalf("pluginOpError 应返回 gerror（供 errors.As 提取 code），got %T", err)
	}
	if code := gerr.Code().Code(); code != consts.CodeJSPluginOperation {
		t.Fatalf("code = %d, want %d", code, consts.CodeJSPluginOperation)
	}
	if want := "插件 myapi@1.4.0 已存在且源码不同"; gerr.Error() != want {
		t.Fatalf("message = %q, want %q", gerr.Error(), want)
	}

	// WrapCode 保留原文案：编译链错误透传时消息不丢失
	inner := gerror.New("esbuild 编译失败：syntax error")
	wrapped := gerror.WrapCode(pluginOpCode(), inner)
	var wg *gerror.Error
	if !errors.As(wrapped, &wg) || wg.Code().Code() != consts.CodeJSPluginOperation {
		t.Fatalf("WrapCode 未携带业务码")
	}
	if wrapped.Error() != inner.Error() {
		t.Fatalf("WrapCode 改写了原文案: %q", wrapped.Error())
	}
}

// TestUpdateConfigValidationCarriesBusinessCode 插件配置保存的校验错误
// （空值/未声明配置项/类型不符/模型冲突）必须带业务码透出给前端。
func TestUpdateConfigValidationCarriesBusinessCode(t *testing.T) {
	ctx := context.Background()
	m := &contract.Manifest{Key: "t", Config: []contract.ConfigField{
		{Key: "api_key", Type: "string"},
		{Key: "limit", Type: "int"},
	}}

	assertBizCode := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: 期望错误", name)
		}
		var gerr *gerror.Error
		if !errors.As(err, &gerr) || gerr.Code().Code() != consts.CodeJSPluginOperation {
			t.Fatalf("%s: 错误未携带业务码 %d, err=%v", name, consts.CodeJSPluginOperation, err)
		}
	}

	assertBizCode("空值", UpdateConfig(ctx, "t", m, nil))
	assertBizCode("未声明配置项", UpdateConfig(ctx, "t", m, map[string]any{"unknown": "x"}))
	assertBizCode("类型不符", UpdateConfig(ctx, "t", m, map[string]any{"limit": "abc"}))
	assertBizCode("加密类型限制", validateEncryptedConfigTypes(&contract.Manifest{
		Config: []contract.ConfigField{{Key: "k", Type: "int", Encrypted: true}},
	}))
}

// TestInheritEnabled 激活时 enabled 继承语义（表驱动）：
//   - 有同 key 旧 active 行 → 继承其 enabled（热切换不掉线 / 禁用不被激活复活）
//   - 无同 key active 行（首次上线）→ 保持目标行原值（仍需显式 Enable）
//
// 背景：Upload 写死 enabled=false，Activate 若不继承，同 key 换版本会使插件
// 掉出 generation，执行中的任务被轮询链判死退款（上游已真实扣费 → 平台资损）。
func TestInheritEnabled(t *testing.T) {
	cases := []struct {
		name          string
		key           string
		targetEnabled bool
		others        []PluginVersionRow
		want          bool
	}{
		{
			name:          "旧激活版本启用中_继承启用",
			key:           "kling",
			targetEnabled: false, // Upload 写死的默认值
			others: []PluginVersionRow{
				{PluginKey: "other", Enabled: true},
				{PluginKey: "kling", Enabled: true},
			},
			want: true,
		},
		{
			name:          "旧激活版本已禁用_保持禁用",
			key:           "kling",
			targetEnabled: false,
			others: []PluginVersionRow{
				{PluginKey: "kling", Enabled: false},
			},
			want: false,
		},
		{
			name:          "目标行原本启用_同key旧行禁用_仍继承禁用",
			key:           "kling",
			targetEnabled: true,
			others: []PluginVersionRow{
				{PluginKey: "kling", Enabled: false},
			},
			want: false,
		},
		{
			name:          "无同key激活行_保持目标行原值_禁用",
			key:           "kling",
			targetEnabled: false,
			others: []PluginVersionRow{
				{PluginKey: "other", Enabled: true},
			},
			want: false,
		},
		{
			name:          "无任何激活行_保持目标行原值",
			key:           "kling",
			targetEnabled: true,
			others:        nil,
			want:          true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := inheritEnabled(c.key, c.targetEnabled, c.others); got != c.want {
				t.Fatalf("inheritEnabled(%q, %v, others) = %v, want %v", c.key, c.targetEnabled, got, c.want)
			}
		})
	}
}

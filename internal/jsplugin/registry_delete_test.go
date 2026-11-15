package jsplugin

import (
	"strings"
	"testing"
)

// TestCheckDeletePluginRows 整删预检纯逻辑：不存在 / 含内置版本 / 启用中拒绝，
// 纯上传且未启用放行并给出正确版本数。
func TestCheckDeletePluginRows(t *testing.T) {
	cases := []struct {
		name          string
		rows          []PluginVersionRow
		wantAllowed   bool
		wantVersions  int
		wantReasonHit string // 不允许时原因应包含的关键词
	}{
		{
			name:          "插件不存在",
			rows:          nil,
			wantAllowed:   false,
			wantReasonHit: "不存在",
		},
		{
			name: "含内置版本的平台基座不可整删",
			rows: []PluginVersionRow{
				{Version: "1.0.0", Origin: "upload", Active: true},
				{Version: "0.9.0", Origin: "builtin"},
			},
			wantAllowed:   false,
			wantReasonHit: "内置版本",
		},
		{
			name: "激活且启用中拒绝",
			rows: []PluginVersionRow{
				{Version: "1.0.0", Origin: "upload", Active: true, Enabled: true},
			},
			wantAllowed:   false,
			wantReasonHit: "先禁用",
		},
		{
			name: "激活但已禁用允许整删",
			rows: []PluginVersionRow{
				{Version: "1.0.0", Origin: "upload", Active: true, Enabled: false},
			},
			wantAllowed:  true,
			wantVersions: 1,
		},
		{
			name: "多个上传版本均未启用允许整删",
			rows: []PluginVersionRow{
				{Version: "1.2.0", Origin: "upload", Active: true, Enabled: false},
				{Version: "1.1.0", Origin: "upload"},
				{Version: "1.0.0", Origin: "upload"},
			},
			wantAllowed:  true,
			wantVersions: 3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkDeletePluginRows(tc.rows)
			if got.Allowed != tc.wantAllowed {
				t.Fatalf("Allowed = %v, want %v (reason: %q)", got.Allowed, tc.wantAllowed, got.Reason)
			}
			if !tc.wantAllowed {
				if got.Reason == "" {
					t.Fatalf("拒绝时必须给出中文原因（给前端确认弹窗展示）")
				}
				if tc.wantReasonHit != "" && !strings.Contains(got.Reason, tc.wantReasonHit) {
					t.Fatalf("原因 %q 未包含关键词 %q", got.Reason, tc.wantReasonHit)
				}
				return
			}
			if got.VersionCount != tc.wantVersions {
				t.Fatalf("VersionCount = %d, want %d", got.VersionCount, tc.wantVersions)
			}
			if got.Reason != "" {
				t.Fatalf("放行时不应携带原因, got %q", got.Reason)
			}
		})
	}
}

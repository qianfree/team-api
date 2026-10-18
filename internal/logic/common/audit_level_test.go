package common

import "testing"

// TestAuditResponseContentDropped 响应类内容（对话响应、异步任务结果）的级别判定：
// none/meta_only/question_only 一律不落库，其余保留（masked 由调用方脱敏）。
// task_result 等库内单列的共享列：落库按全局级别判定，租户视角在租户详情接口
// 按租户级别做同样的读取过滤。
func TestAuditResponseContentDropped(t *testing.T) {
	cases := map[string]bool{
		"none":          true,
		"meta_only":     true,
		"question_only": true,
		"masked":        false,
		"full_text":     false,
		"full":          false,
	}
	for level, want := range cases {
		if got := AuditResponseContentDropped(level); got != want {
			t.Errorf("AuditResponseContentDropped(%q) = %v, want %v", level, got, want)
		}
	}
}

// TestAuditRequestContentDropped 请求侧内容（含 user_field 等请求元数据）的级别判定：
// 仅 none/meta_only 丢弃；question_only 保留请求侧，照常落库。
func TestAuditRequestContentDropped(t *testing.T) {
	cases := map[string]bool{
		"none":          true,
		"meta_only":     true,
		"question_only": false,
		"masked":        false,
		"full_text":     false,
		"full":          false,
	}
	for level, want := range cases {
		if got := AuditRequestContentDropped(level); got != want {
			t.Errorf("AuditRequestContentDropped(%q) = %v, want %v", level, got, want)
		}
	}
}

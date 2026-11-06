package jsplugin

import (
	"context"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// SubmitPlan 分发器在提交链上完成的解码计划：decodeRequest 的产物 + 路由匹配信息。
// 分发器解码一次后经 ctx 传给任务桥（taskchannel/jsbridge），桥直接消费，
// 避免对同一请求调用两次 JS decode（契约要求 decode 可重入，但省一次是一次）。
type SubmitPlan struct {
	Plugin     *ActivePlugin
	Route      contract.Route
	Params     map[string]string
	NativePath string
	Decoded    *contract.DecodeResult
	// Immediate 同步即完成（parseSubmitResponse 返回 immediate 时由任务桥回填）：
	// 提交成功后分发器立即终态化任务（结算/产物与轮询终态同口径）。
	Immediate *contract.ImmediateResult
	// Files multipart 上传的文件载荷（ref → 内容），任务桥发送前内联占位符。
	Files map[string]FilePayload
}

// FilePayload 请求级文件载荷（字节仅在宿主侧流转）。
type FilePayload struct {
	Filename string
	MimeType string
	Data     []byte
}

type submitPlanKey struct{}

// WithSubmitPlan 把解码计划挂入 ctx（分发器 → 任务桥）。
func WithSubmitPlan(ctx context.Context, plan *SubmitPlan) context.Context {
	return context.WithValue(ctx, submitPlanKey{}, plan)
}

// SubmitPlanFromContext 取出解码计划（任务桥侧；无则 nil）。
func SubmitPlanFromContext(ctx context.Context) *SubmitPlan {
	if ctx == nil {
		return nil
	}
	if plan, ok := ctx.Value(submitPlanKey{}).(*SubmitPlan); ok {
		return plan
	}
	return nil
}

// Package relayconvert — 请求转换执行引擎。
//
// 注册表支持两类请求转换器：直接转换器（Convert 非空）与步骤链转换器
// （StepConverters 非空，注册时已校验 From/To 连续性）。本文件提供统一的
// 执行入口：宿主桥接层只需持有转换器 ID，无需关心其为直接实现还是
// 「经 OpenAI 中枢两跳」的链式组合。
package relayconvert

import (
	"context"
	"fmt"

	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// ConvertRequestByID 按转换器 ID 执行请求转换（直接或步骤链）。
// request 为入站格式的已解析 DTO；返回目标格式 DTO。
func ConvertRequestByID(ctx context.Context, info convmeta.Meta, converterID string, request any) (any, error) {
	spec, ok := LookupRequestConverter(converterID)
	if !ok {
		return nil, fmt.Errorf("request converter %q is not registered", converterID)
	}
	return ExecuteRequestConversion(ctx, info, spec, request)
}

// ExecuteRequestConversion 执行一个请求转换器 spec：直接转换器调用其 Convert；
// 步骤链按注册顺序依次执行各步骤，中间值逐步传递（各步骤的 DTO 出入参
// 由注册时的 From/To 连续性校验保证兼容）。
func ExecuteRequestConversion(ctx context.Context, info convmeta.Meta, spec RequestConverterSpec, request any) (any, error) {
	if spec.Convert != nil {
		return spec.Convert(ctx, info, request)
	}
	if len(spec.StepConverters) == 0 {
		return nil, fmt.Errorf("request converter %q has no convert implementation", spec.ID)
	}
	current := request
	for _, stepID := range spec.StepConverters {
		step, ok := LookupRequestConverter(stepID)
		if !ok || step.Convert == nil {
			return nil, fmt.Errorf("request converter %q step %q is not a registered direct converter", spec.ID, stepID)
		}
		next, err := step.Convert(ctx, info, current)
		if err != nil {
			return nil, fmt.Errorf("request converter %q step %q: %w", spec.ID, stepID, err)
		}
		current = next
	}
	return current, nil
}

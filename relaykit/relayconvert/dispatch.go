package relayconvert

import (
	"context"
	"fmt"

	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// ExecuteRequestConverter 执行请求转换器 spec，是宿主桥接层执行转换的统一入口：
//   - 直接转换器（Convert != nil）直接调用 Convert；
//   - 链式 spec（StepConverters 非空）按注册时声明的顺序逐跳执行，
//     上一跳输出原样传入下一跳。步骤间类型契约由各步骤转换器的入参断言保证，
//     From/To 连续性已在注册时校验（registerBuiltinRequestConverter）。
//
// 注册表本身只做注册与查找，本文件补齐链式 spec 的执行引擎。
func ExecuteRequestConverter(ctx context.Context, spec RequestConverterSpec, info convmeta.Meta, input any) (any, error) {
	if spec.Convert != nil {
		return spec.Convert(ctx, info, input)
	}
	if len(spec.StepConverters) == 0 {
		return nil, fmt.Errorf("converter %q has neither convert nor step converters", spec.ID)
	}

	current := input
	for _, stepID := range spec.StepConverters {
		step, ok := LookupRequestConverter(stepID)
		if !ok {
			return nil, fmt.Errorf("converter %q references unknown step converter %q", spec.ID, stepID)
		}
		// 注册时已禁止步骤本身再带步骤（must be a direct converter），此处防御性复查
		if step.Convert == nil || len(step.StepConverters) > 0 {
			return nil, fmt.Errorf("converter %q step %q must be a direct converter", spec.ID, stepID)
		}
		result, err := step.Convert(ctx, info, current)
		if err != nil {
			return nil, fmt.Errorf("converter %q step %q failed: %w", spec.ID, stepID, err)
		}
		current = result
	}
	return current, nil
}

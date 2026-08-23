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
		converted, err := spec.Convert(ctx, info, input)
		if err != nil {
			return converted, err
		}
		recordRequestConversionChain(info, spec)
		return converted, nil
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
	recordRequestConversionChain(info, spec)
	return current, nil
}

// recordRequestConversionChain 记录请求侧协议转换轨迹（宿主渠道调试日志经
// ConversionChain() 消费）：链首记 spec.From，其后逐跳记各步骤的 To——链式组合
// （如 claude→responses→chat）的中间格式由此留痕。仅在整条转换成功后提交：
// 中途失败直接报错，半程轨迹会挤掉调试日志"未覆盖时兜底记录两端"的分支。
func recordRequestConversionChain(info convmeta.Meta, spec RequestConverterSpec) {
	if info == nil {
		return
	}
	info.AppendRequestConversion(spec.From)
	for _, stepID := range spec.StepConverters {
		if step, ok := LookupRequestConverter(stepID); ok {
			info.AppendRequestConversion(step.To)
		}
	}
	if spec.Convert != nil {
		info.AppendRequestConversion(spec.To)
	}
}

package relayconvert

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// registerExecTestChain 注册一条 A→B→C 两跳链供执行器测试使用（幂等，重复调用跳过）。
// 转换函数做字符串拼接，可验证数据是否按顺序流经每一跳。
func registerExecTestChain(t *testing.T) {
	t.Helper()
	if _, ok := LookupRequestConverter("exec_chain_ac"); ok {
		return
	}

	aToB := func(_ context.Context, _ convmeta.Meta, request any) (any, error) {
		return request.(string) + "|ab", nil
	}
	bToC := func(_ context.Context, _ convmeta.Meta, request any) (any, error) {
		return request.(string) + "|bc", nil
	}

	registerBuiltinRequestConverter(RequestConverterSpec{
		ID: "exec_direct_ab", From: "exec_fmt_a", To: "exec_fmt_b",
		Quality: RequestConverterQualityGood, Convert: aToB,
	})
	registerBuiltinRequestConverter(RequestConverterSpec{
		ID: "exec_direct_bc", From: "exec_fmt_b", To: "exec_fmt_c",
		Quality: RequestConverterQualityGood, Convert: bToC,
	})
	registerBuiltinRequestConverter(RequestConverterSpec{
		ID: "exec_chain_ac", From: "exec_fmt_a", To: "exec_fmt_c",
		Quality: RequestConverterQualityGood, StepConverters: []string{"exec_direct_ab", "exec_direct_bc"},
	})
}

// 直接转换器走 Convert 单次调用。
func TestExecuteRequestConverter_Direct(t *testing.T) {
	registerExecTestChain(t)

	spec, ok := LookupRequestConverter("exec_direct_ab")
	if !ok {
		t.Fatal("expected direct converter registered")
	}
	got, err := ExecuteRequestConverter(context.Background(), spec, nil, "in")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "in|ab" {
		t.Errorf("direct result = %v, want in|ab", got)
	}
}

// 链式 spec 按顺序逐跳执行，上一跳输出传入下一跳。
func TestExecuteRequestConverter_Chain(t *testing.T) {
	registerExecTestChain(t)

	spec, ok := LookupRequestConverter("exec_chain_ac")
	if !ok {
		t.Fatal("expected chain converter registered")
	}
	got, err := ExecuteRequestConverter(context.Background(), spec, nil, "in")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "in|ab|bc" {
		t.Errorf("chain result = %v, want in|ab|bc", got)
	}
}

// 中间步骤失败时错误向上传播并携带步骤 ID。
func TestExecuteRequestConverter_StepError(t *testing.T) {
	sentinel := errors.New("boom")
	registerBuiltinRequestConverter(RequestConverterSpec{
		ID: "exec_err_ab", From: "exec_fmt_e", To: "exec_fmt_f",
		Quality: RequestConverterQualityGood,
		Convert: func(_ context.Context, _ convmeta.Meta, _ any) (any, error) { return nil, sentinel },
	})
	registerBuiltinRequestConverter(RequestConverterSpec{
		ID: "exec_err_bc", From: "exec_fmt_f", To: "exec_fmt_g",
		Quality: RequestConverterQualityGood, Convert: noopReqConvert,
	})
	registerBuiltinRequestConverter(RequestConverterSpec{
		ID: "exec_chain_err", From: "exec_fmt_e", To: "exec_fmt_g",
		Quality: RequestConverterQualityGood, StepConverters: []string{"exec_err_ab", "exec_err_bc"},
	})

	spec, _ := LookupRequestConverter("exec_chain_err")
	_, err := ExecuteRequestConverter(context.Background(), spec, nil, "in")
	if err == nil {
		t.Fatal("expected error from failing step")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error should wrap sentinel, got: %v", err)
	}
}

// 既无 Convert 也无 StepConverters 的 spec 返回明确错误（防御性，注册表校验下不可达）。
func TestExecuteRequestConverter_EmptySpec(t *testing.T) {
	_, err := ExecuteRequestConverter(context.Background(), RequestConverterSpec{ID: "exec_empty"}, nil, "in")
	if err == nil {
		t.Fatal("expected error for empty spec")
	}
}

// 直接转换器成功后记录轨迹 [From, To]。
func TestExecuteRequestConverter_RecordsChain_Direct(t *testing.T) {
	registerExecTestChain(t)

	spec, _ := LookupRequestConverter("exec_direct_ab")
	meta := &convmeta.Values{}
	if _, err := ExecuteRequestConverter(context.Background(), spec, meta, "in"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []types.RelayFormat{"exec_fmt_a", "exec_fmt_b"}
	if !reflect.DeepEqual(meta.ConversionChain, want) {
		t.Errorf("chain = %v, want %v", meta.ConversionChain, want)
	}
}

// 链式 spec 成功后记录轨迹 [From, 各跳 To...]，中间格式留痕。
func TestExecuteRequestConverter_RecordsChain_Chained(t *testing.T) {
	registerExecTestChain(t)

	spec, _ := LookupRequestConverter("exec_chain_ac")
	meta := &convmeta.Values{}
	if _, err := ExecuteRequestConverter(context.Background(), spec, meta, "in"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []types.RelayFormat{"exec_fmt_a", "exec_fmt_b", "exec_fmt_c"}
	if !reflect.DeepEqual(meta.ConversionChain, want) {
		t.Errorf("chain = %v, want %v", meta.ConversionChain, want)
	}
}

// 转换失败时不提交半程轨迹（调用方回退旧路径，轨迹留给兜底两端记录）。
func TestExecuteRequestConverter_RecordsChain_NotOnFailure(t *testing.T) {
	if _, ok := LookupRequestConverter("exec_chain_failrec"); !ok {
		registerBuiltinRequestConverter(RequestConverterSpec{
			ID: "exec_err_first_failrec", From: "exec_fmt_h", To: "exec_fmt_i",
			Quality: RequestConverterQualityGood,
			Convert: func(_ context.Context, _ convmeta.Meta, _ any) (any, error) {
				return nil, errors.New("boom")
			},
		})
		registerBuiltinRequestConverter(RequestConverterSpec{
			ID: "exec_err_second_failrec", From: "exec_fmt_i", To: "exec_fmt_j",
			Quality: RequestConverterQualityGood, Convert: noopReqConvert,
		})
		registerBuiltinRequestConverter(RequestConverterSpec{
			ID: "exec_chain_failrec", From: "exec_fmt_h", To: "exec_fmt_j",
			Quality:        RequestConverterQualityGood,
			StepConverters: []string{"exec_err_first_failrec", "exec_err_second_failrec"},
		})
	}

	spec, _ := LookupRequestConverter("exec_chain_failrec")
	meta := &convmeta.Values{}
	if _, err := ExecuteRequestConverter(context.Background(), spec, meta, "in"); err == nil {
		t.Fatal("expected error from failing step")
	}
	if len(meta.ConversionChain) != 0 {
		t.Errorf("failed conversion should not record chain, got %v", meta.ConversionChain)
	}
}

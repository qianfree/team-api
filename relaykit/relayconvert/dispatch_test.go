package relayconvert

import (
	"context"
	"errors"
	"testing"

	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
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

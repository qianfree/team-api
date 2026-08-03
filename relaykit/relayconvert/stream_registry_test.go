package relayconvert

import (
	"context"
	"io"
	"testing"

	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// assertPanic 断言 fn 触发 panic。
func assertPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic, got none")
		}
	}()
	fn()
}

func TestLookupStreamConverter_NotFound(t *testing.T) {
	// 空注册表（relayconvert 包测试不 import register，内置转换器未注册）
	fn, id, ok := LookupStreamConverter(types.RelayFormatOpenAI, types.RelayFormatClaude)
	if ok {
		t.Fatal("expected not-found on empty registry")
	}
	if fn != nil || id != "" {
		t.Fatalf("expected nil fn and empty id, got fn=%v id=%q", fn, id)
	}
}

func TestRegisterAndLookupStreamConverter(t *testing.T) {
	called := false
	fn := func(ctx context.Context, _ convmeta.Meta, _ io.Reader, _ func(any) error) error {
		called = true
		return nil
	}

	RegisterStreamConverter(types.RelayFormatOpenAI, types.RelayFormatEmbedding, "test_stream_oai_to_emb", fn)

	got, id, ok := LookupStreamConverter(types.RelayFormatOpenAI, types.RelayFormatEmbedding)
	if !ok {
		t.Fatal("expected converter to be registered")
	}
	if got == nil {
		t.Fatal("expected non-nil converter fn")
	}
	if id != "test_stream_oai_to_emb" {
		t.Errorf("ID = %q, want %q", id, "test_stream_oai_to_emb")
	}
	// 反向不应命中
	if _, _, ok := LookupStreamConverter(types.RelayFormatEmbedding, types.RelayFormatOpenAI); ok {
		t.Error("reverse route should not be registered")
	}
	// 调用返回的函数确认是同一个
	if err := got(context.Background(), nil, nil, nil); err != nil {
		t.Fatalf("fn returned error: %v", err)
	}
	if !called {
		t.Error("registered fn was not invoked")
	}
}

func TestRegisterStreamConverter_DuplicateRoutePanics(t *testing.T) {
	RegisterStreamConverter(types.RelayFormatOpenAI, types.RelayFormatRerank, "test_stream_dup_route_1",
		func(context.Context, convmeta.Meta, io.Reader, func(any) error) error { return nil })

	// 同路由不同 ID → panic
	assertPanic(t, func() {
		RegisterStreamConverter(types.RelayFormatOpenAI, types.RelayFormatRerank, "test_stream_dup_route_2",
			func(context.Context, convmeta.Meta, io.Reader, func(any) error) error { return nil })
	})
}

func TestRegisterStreamConverter_DuplicateIDPanics(t *testing.T) {
	RegisterStreamConverter(types.RelayFormatGemini, types.RelayFormatTask, "test_stream_dup_id_1",
		func(context.Context, convmeta.Meta, io.Reader, func(any) error) error { return nil })

	// 同 ID 不同路由 → panic
	assertPanic(t, func() {
		RegisterStreamConverter(types.RelayFormatClaude, types.RelayFormatEmbedding, "test_stream_dup_id_1",
			func(context.Context, convmeta.Meta, io.Reader, func(any) error) error { return nil })
	})
}

func TestRegisterStreamConverter_InvalidArgsPanic(t *testing.T) {
	validFn := func(context.Context, convmeta.Meta, io.Reader, func(any) error) error { return nil }

	assertPanic(t, func() {
		RegisterStreamConverter(types.RelayFormatOpenAI, types.RelayFormatClaude, "", validFn)
	})
	assertPanic(t, func() {
		RegisterStreamConverter("", types.RelayFormatClaude, "test_invalid_1", validFn)
	})
	assertPanic(t, func() {
		RegisterStreamConverter(types.RelayFormatOpenAI, types.RelayFormatClaude, "test_invalid_2", nil)
	})
}

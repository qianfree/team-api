package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestErrProtocolMismatch_Wrapping(t *testing.T) {
	wrapped := fmt.Errorf("%w: 3 chunks parsed, none contained choices", ErrProtocolMismatch)
	if !errors.Is(wrapped, ErrProtocolMismatch) {
		t.Error("wrapped error should match sentinel via errors.Is")
	}
	if !errors.Is(fmt.Errorf("outer: %w", wrapped), ErrProtocolMismatch) {
		t.Error("double-wrapped error should still match sentinel")
	}
}

func TestEmbeddedUpstreamError(t *testing.T) {
	err := &EmbeddedUpstreamError{Body: json.RawMessage(`{"error":{"message":"quota exceeded"}}`)}
	var target *EmbeddedUpstreamError
	if !errors.As(error(err), &target) {
		t.Error("errors.As should match *EmbeddedUpstreamError")
	}
	if string(target.Body) != `{"error":{"message":"quota exceeded"}}` {
		t.Errorf("Body = %s", target.Body)
	}
	if got := err.Error(); got == "" {
		t.Error("Error() should be non-empty")
	}
}

// 超长 Body 截断不 panic 且信息可读。
func TestEmbeddedUpstreamError_LongBodyTruncated(t *testing.T) {
	long := make([]byte, 2000)
	for i := range long {
		long[i] = 'a'
	}
	err := &EmbeddedUpstreamError{Body: long}
	msg := err.Error()
	if len(msg) > 600 {
		t.Errorf("Error() should truncate long body, got %d chars", len(msg))
	}
}

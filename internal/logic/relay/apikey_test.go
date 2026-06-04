package relay

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestApiKeyHash(t *testing.T) {
	const key = "sk-abcdef123456"

	got := apiKeyHash(key)

	// 契约：apiKeyHash 等价于 hex(sha256(key))
	sum := sha256.Sum256([]byte(key))
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Errorf("apiKeyHash(%q) = %s, want %s", key, got, want)
	}
	if len(got) != 64 {
		t.Errorf("hash length = %d, want 64 hex chars", len(got))
	}
	// 确定性
	if apiKeyHash(key) != got {
		t.Error("apiKeyHash is not deterministic")
	}
	// 不同输入 -> 不同输出
	if apiKeyHash("sk-different-key") == got {
		t.Error("different inputs produced the same hash")
	}
}

func TestDefaultChannelSettings(t *testing.T) {
	s := DefaultChannelSettings()
	if s.TimeoutSeconds != 60 {
		t.Errorf("default TimeoutSeconds = %d, want 60", s.TimeoutSeconds)
	}
	if s.RetryCount != 1 {
		t.Errorf("default RetryCount = %d, want 1", s.RetryCount)
	}
}

func TestParseChannelSettings_Defaults(t *testing.T) {
	for _, in := range []string{"", "{}", "null"} {
		s := ParseChannelSettings(in)
		if s.TimeoutSeconds != 60 || s.RetryCount != 1 {
			t.Errorf("ParseChannelSettings(%q) = %+v, want defaults (60,1)", in, s)
		}
	}
}

func TestParseChannelSettings_InvalidJSONFallsBackToDefaults(t *testing.T) {
	s := ParseChannelSettings("{not valid json")
	if s.TimeoutSeconds != 60 || s.RetryCount != 1 {
		t.Errorf("invalid JSON should fall back to defaults, got %+v", s)
	}
}

func TestParseChannelSettings_PartialOverrideKeepsDefaults(t *testing.T) {
	// 只覆盖 timeout，retry 缺省应回填为 1
	s := ParseChannelSettings(`{"timeout_seconds":30}`)
	if s.TimeoutSeconds != 30 {
		t.Errorf("TimeoutSeconds = %d, want 30", s.TimeoutSeconds)
	}
	if s.RetryCount != 1 {
		t.Errorf("missing RetryCount should default to 1, got %d", s.RetryCount)
	}
}

func TestParseChannelSettings_FullOverride(t *testing.T) {
	s := ParseChannelSettings(`{"timeout_seconds":120,"retry_count":3}`)
	if s.TimeoutSeconds != 120 {
		t.Errorf("TimeoutSeconds = %d, want 120", s.TimeoutSeconds)
	}
	if s.RetryCount != 3 {
		t.Errorf("RetryCount = %d, want 3", s.RetryCount)
	}
}

package billing

import (
	"testing"
)

func TestRateLimitConfig(t *testing.T) {
	cfg := DefaultRateLimitConfig

	if cfg.SystemQPS != 10000 {
		t.Errorf("expected SystemQPS 10000, got %d", cfg.SystemQPS)
	}
	if cfg.TenantQPS != 1000 {
		t.Errorf("expected TenantQPS 1000, got %d", cfg.TenantQPS)
	}
	if cfg.UserQPS != 100 {
		t.Errorf("expected UserQPS 100, got %d", cfg.UserQPS)
	}
	if cfg.KeyQPS != 60 {
		t.Errorf("expected KeyQPS 60, got %d", cfg.KeyQPS)
	}
	if cfg.TenantConc != 50 {
		t.Errorf("expected TenantConc 50, got %d", cfg.TenantConc)
	}
	if cfg.UserConc != 10 {
		t.Errorf("expected UserConc 10, got %d", cfg.UserConc)
	}
	if cfg.KeyConc != 5 {
		t.Errorf("expected KeyConc 5, got %d", cfg.KeyConc)
	}
}

func TestRateLimitResult(t *testing.T) {
	result := &RateLimitResult{
		Allowed:    true,
		LimitLevel: "tenant",
		Limit:      1000,
		Remaining:  999,
		ResetAt:    1234567890,
	}

	if !result.Allowed {
		t.Error("expected allowed")
	}
	if result.LimitLevel != "tenant" {
		t.Errorf("expected level tenant, got %s", result.LimitLevel)
	}
	if result.Remaining != 999 {
		t.Errorf("expected remaining 999, got %d", result.Remaining)
	}
}

func TestRateLimitHeaders(t *testing.T) {
	result := &RateLimitResult{
		Limit:     60,
		Remaining: 45,
		ResetAt:   1234567890,
	}

	headers := RateLimitHeaders(result)
	if headers == nil {
		t.Fatal("expected non-nil headers")
	}
	if headers["X-RateLimit-Limit"] != "60" {
		t.Errorf("expected X-RateLimit-Limit 60, got %s", headers["X-RateLimit-Limit"])
	}
	if headers["X-RateLimit-Remaining"] != "45" {
		t.Errorf("expected X-RateLimit-Remaining 45, got %s", headers["X-RateLimit-Remaining"])
	}
	if headers["X-RateLimit-Reset"] != "1234567890" {
		t.Errorf("expected X-RateLimit-Reset 1234567890, got %s", headers["X-RateLimit-Reset"])
	}
}

func TestRateLimitHeaders_Nil(t *testing.T) {
	headers := RateLimitHeaders(nil)
	if headers != nil {
		t.Error("expected nil headers for nil result")
	}
}

func TestBillingProviderImpl_NilPreDeduct(t *testing.T) {
	// BillingProviderImpl.SettleFailed with 0 amount should not panic
	provider := &BillingProviderImpl{}
	err := provider.SettleFailed(nil, 1, "test-request", 0)
	if err != nil {
		t.Errorf("SettleFailed with 0 amount: %v", err)
	}
}

func TestBillingProviderImpl_ScopeCheck(t *testing.T) {
	provider := &BillingProviderImpl{}

	tests := []struct {
		scope     string
		relayMode string
		want      bool
	}{
		{"full", "chat_completions", true},
		{"chat_only", "embeddings", false},
		{"embeddings_only", "embeddings", true},
	}

	for _, tt := range tests {
		got := provider.CheckScope(tt.scope, tt.relayMode)
		if got != tt.want {
			t.Errorf("CheckScope(%q, %q) = %v, want %v", tt.scope, tt.relayMode, got, tt.want)
		}
	}
}

func TestBillingProviderImpl_IPWhitelist(t *testing.T) {
	provider := &BillingProviderImpl{}

	if !provider.CheckIPWhitelist("", "1.2.3.4") {
		t.Error("empty whitelist should allow all")
	}
	if !provider.CheckIPWhitelist("1.2.3.4", "1.2.3.4") {
		t.Error("exact match should allow")
	}
	if provider.CheckIPWhitelist("1.2.3.4", "5.6.7.8") {
		t.Error("no match should deny")
	}
}

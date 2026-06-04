package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
)

func TestComputeHMACSignature(t *testing.T) {
	const (
		secret = "top-secret"
		ts     = "1700000000"
		nonce  = "abc123"
		method = "POST"
		path   = "/open/v1/messages"
	)
	body := []byte(`{"hello":"world"}`)

	got := computeHMACSignature(secret, ts, nonce, method, path, body)

	// 契约：HMAC-SHA256(secret, "ts\nnonce\nmethod\npath\nhex(sha256(body))")
	bodyHash := sha256.Sum256(body)
	msg := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", ts, nonce, method, path, hex.EncodeToString(bodyHash[:]))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	want := hex.EncodeToString(mac.Sum(nil))

	if got != want {
		t.Errorf("signature = %s, want %s", got, want)
	}

	// 确定性
	if computeHMACSignature(secret, ts, nonce, method, path, body) != got {
		t.Error("signature is not deterministic")
	}
	// body 改变 -> 签名改变（请求完整性）
	if computeHMACSignature(secret, ts, nonce, method, path, []byte(`{"hello":"WORLD"}`)) == got {
		t.Error("changing the body must change the signature")
	}
	// secret 改变 -> 签名改变
	if computeHMACSignature("other-secret", ts, nonce, method, path, body) == got {
		t.Error("changing the secret must change the signature")
	}
	// 空 body 与有 body 的签名必须不同
	if computeHMACSignature(secret, ts, nonce, method, path, nil) == got {
		t.Error("empty body and non-empty body must differ")
	}
}

func TestParseResourceFromPath(t *testing.T) {
	tests := []struct {
		path     string
		wantType string
		wantID   int64
	}{
		{"/api/admin/channels", "channel", 0},
		{"/api/admin/channels/123", "channel", 123},
		{"/api/tenant/users/5", "user", 5},
		{"/api/admin/models", "model", 0},
		{"/api/admin/orders/42", "order", 42},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			gotType, gotID := parseResourceFromPath(tt.path)
			if gotType != tt.wantType || gotID != tt.wantID {
				t.Errorf("parseResourceFromPath(%q) = (%q,%d), want (%q,%d)",
					tt.path, gotType, gotID, tt.wantType, tt.wantID)
			}
		})
	}
}

func TestBuildAction(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		resourceType string
		want         string
	}{
		{"create", "POST", "/api/admin/channels", "channel", "create_channel"},
		{"update", "PUT", "/api/admin/channels/123", "channel", "update_channel"},
		{"patch is update", "PATCH", "/api/admin/channels/123", "channel", "update_channel"},
		{"delete", "DELETE", "/api/admin/channels/123", "channel", "delete_channel"},
		{"custom action on resource", "POST", "/api/admin/channels/123/test", "channel", "test_channel"},
		{"no resource type falls back to method", "GET", "/api/admin/dashboard", "", "get"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildAction(tt.method, tt.path, tt.resourceType); got != tt.want {
				t.Errorf("buildAction(%q,%q,%q) = %q, want %q",
					tt.method, tt.path, tt.resourceType, got, tt.want)
			}
		})
	}
}

func TestMaskSensitiveFields(t *testing.T) {
	t.Run("masks top-level sensitive fields", func(t *testing.T) {
		out := maskSensitiveFields(`{"password":"secret","name":"bob"}`)
		var m map[string]any
		if err := json.Unmarshal([]byte(out), &m); err != nil {
			t.Fatalf("output not valid JSON: %v", err)
		}
		if m["password"] != "******" {
			t.Errorf("password not masked: %v", m["password"])
		}
		if m["name"] != "bob" {
			t.Errorf("non-sensitive field altered: %v", m["name"])
		}
	})

	t.Run("masks nested sensitive fields", func(t *testing.T) {
		out := maskSensitiveFields(`{"user":{"api_key":"k","email":"a@b.c"}}`)
		var m map[string]any
		_ = json.Unmarshal([]byte(out), &m)
		user := m["user"].(map[string]any)
		if user["api_key"] != "******" {
			t.Errorf("nested api_key not masked: %v", user["api_key"])
		}
		if user["email"] != "a@b.c" {
			t.Errorf("nested non-sensitive field altered: %v", user["email"])
		}
	})

	t.Run("case-insensitive key match", func(t *testing.T) {
		out := maskSensitiveFields(`{"Password":"x","Refresh_Token":"y"}`)
		var m map[string]any
		_ = json.Unmarshal([]byte(out), &m)
		if m["Password"] != "******" || m["Refresh_Token"] != "******" {
			t.Errorf("case-insensitive masking failed: %v", m)
		}
	})

	t.Run("non-JSON returned as-is", func(t *testing.T) {
		if got := maskSensitiveFields("not json"); got != "not json" {
			t.Errorf("non-JSON should pass through, got %q", got)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		if got := maskSensitiveFields(""); got != "" {
			t.Errorf("empty input should return empty, got %q", got)
		}
	})
}

func TestIsSensitive(t *testing.T) {
	sensitive := []string{"password", "PASSWORD", "api_key", "Secret", "refresh_token", "encrypted_key"}
	for _, f := range sensitive {
		if !isSensitive(f) {
			t.Errorf("%q should be sensitive", f)
		}
	}
	for _, f := range []string{"name", "email", "id", "username"} {
		if isSensitive(f) {
			t.Errorf("%q should not be sensitive", f)
		}
	}
}

func TestParseMaintenanceDuration(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		in   string
		want int
	}{
		{"", 300},
		{"invalid", 300},
		{"0s", 300},
		{"-5s", 300},
		{"30s", 30},
		{"2m", 120},
		{"1h", 3600},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := parseMaintenanceDuration(ctx, tt.in); got != tt.want {
				t.Errorf("parseMaintenanceDuration(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

package totp

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	lib "github.com/pquerna/otp/totp"
)

func TestGenerateSecret(t *testing.T) {
	secret, uri, err := GenerateSecret("alice@example.com")
	if err != nil {
		t.Fatalf("GenerateSecret error: %v", err)
	}
	if secret == "" {
		t.Error("secret is empty")
	}
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Errorf("uri should be an otpauth TOTP URI, got %q", uri)
	}
	if !strings.Contains(uri, "Team-API") {
		t.Errorf("uri should embed issuer, got %q", uri)
	}
}

func TestValidateCode(t *testing.T) {
	secret, _, err := GenerateSecret("bob@example.com")
	if err != nil {
		t.Fatal(err)
	}

	// 用同一算法为当前时间生成正确验证码，应通过
	valid, err := lib.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !ValidateCode(valid, secret) {
		t.Errorf("freshly generated code %q should validate", valid)
	}

	// 明显非法的验证码应被拒绝（非数字 / 长度错误）
	for _, bad := range []string{"", "abcdef", "12", "1234567"} {
		if ValidateCode(bad, secret) {
			t.Errorf("invalid code %q should not validate", bad)
		}
	}

	// 错误 secret 下，正确格式的码也应失败
	otherSecret, _, _ := GenerateSecret("carol@example.com")
	if ValidateCode(valid, otherSecret) && otherSecret != secret {
		t.Error("code generated for one secret should not validate against another")
	}
}

func TestGenerateBackupCodes(t *testing.T) {
	const count = 10
	codes, err := GenerateBackupCodes(count)
	if err != nil {
		t.Fatalf("GenerateBackupCodes error: %v", err)
	}
	if len(codes) != count {
		t.Fatalf("got %d codes, want %d", len(codes), count)
	}

	seen := make(map[string]bool)
	for _, c := range codes {
		if len(c) != 12 { // 6 bytes -> 12 hex chars
			t.Errorf("code %q length = %d, want 12 hex chars", c, len(c))
		}
		if _, err := hex.DecodeString(c); err != nil {
			t.Errorf("code %q is not valid hex: %v", c, err)
		}
		if seen[c] {
			t.Errorf("duplicate backup code: %q", c)
		}
		seen[c] = true
	}
}

func TestGenerateBackupCodes_Zero(t *testing.T) {
	codes, err := GenerateBackupCodes(0)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(codes) != 0 {
		t.Errorf("expected empty slice, got %d", len(codes))
	}
}

package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// key32 returns a deterministic 32-byte AES-256 key filled with b.
func key32(b byte) []byte {
	return bytes.Repeat([]byte{b}, 32)
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := key32('A')
	cases := []string{
		"",
		"hello",
		"sk-proj-1234567890abcdef",
		strings.Repeat("长文本测试", 100), // 多字节 + 较长
	}
	for _, plain := range cases {
		enc, err := EncryptString(key, plain)
		if err != nil {
			t.Fatalf("EncryptString(%q) error: %v", plain, err)
		}
		if enc == plain && plain != "" {
			t.Errorf("ciphertext equals plaintext for %q", plain)
		}
		got, err := DecryptString(key, enc)
		if err != nil {
			t.Fatalf("DecryptString error: %v", err)
		}
		if got != plain {
			t.Errorf("round-trip mismatch: got %q, want %q", got, plain)
		}
	}
}

func TestEncrypt_NonceIsRandom(t *testing.T) {
	key := key32('A')
	a, _ := EncryptString(key, "same-plaintext")
	b, _ := EncryptString(key, "same-plaintext")
	if a == b {
		t.Error("two encryptions of the same plaintext produced identical ciphertext (nonce not random)")
	}
	// 两者都应能正确解密
	for _, c := range []string{a, b} {
		got, err := DecryptString(key, c)
		if err != nil || got != "same-plaintext" {
			t.Errorf("decrypt failed: got %q err %v", got, err)
		}
	}
}

func TestEncryptDecrypt_InvalidKeyLength(t *testing.T) {
	for _, badKey := range [][]byte{nil, make([]byte, 16), make([]byte, 31), make([]byte, 33)} {
		if _, err := Encrypt(badKey, []byte("x")); !errors.Is(err, ErrInvalidKeyLength) {
			t.Errorf("Encrypt with key len %d: got %v, want ErrInvalidKeyLength", len(badKey), err)
		}
		if _, err := Decrypt(badKey, "AAAA"); !errors.Is(err, ErrInvalidKeyLength) {
			t.Errorf("Decrypt with key len %d: got %v, want ErrInvalidKeyLength", len(badKey), err)
		}
	}
}

func TestDecrypt_WrongKeyFails(t *testing.T) {
	enc, err := EncryptString(key32('A'), "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptString(key32('B'), enc); err == nil {
		t.Error("decryption with a different key must fail (GCM auth), got nil error")
	}
}

func TestDecrypt_TamperedCiphertextFails(t *testing.T) {
	key := key32('A')
	enc, err := EncryptString(key, "secret-payload")
	if err != nil {
		t.Fatal(err)
	}
	// 剥离 v1: 版本前缀后取 base64 密文
	payload := strings.TrimPrefix(enc, "v1:")
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatal(err)
	}
	// 翻转最后一个字节（密文/认证标签部分）
	raw[len(raw)-1] ^= 0xFF
	tampered := "v1:" + base64.StdEncoding.EncodeToString(raw)

	if _, err := DecryptString(key, tampered); err == nil {
		t.Error("tampered ciphertext must fail authentication, got nil error")
	}
}

func TestEncrypt_HasVersionPrefix(t *testing.T) {
	key := key32('A')
	enc, err := EncryptString(key, "payload")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "v1:") {
		t.Errorf("encrypted value should carry the v1: version prefix, got %q", enc)
	}
	// 旧格式（无前缀）密文必须仍能解密（向后兼容存量数据）
	legacy := base64.StdEncoding.EncodeToString(rawGcmCiphertext(key, []byte("legacy")))
	got, err := DecryptString(key, legacy)
	if err != nil || string(got) != "legacy" {
		t.Errorf("legacy unprefixed ciphertext must still decrypt: got %q err %v", got, err)
	}
}

// rawGcmCiphertext 用与 Encrypt 相同的算法生成无前缀的密文，用于模拟历史数据。
func rawGcmCiphertext(key, plaintext []byte) []byte {
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	_, _ = rand.Read(nonce)
	return gcm.Seal(nonce, nonce, plaintext, nil)
}

func TestDecrypt_TooShortAndInvalidBase64(t *testing.T) {
	key := key32('A')
	// 合法 base64 但长度不足 nonce
	short := base64.StdEncoding.EncodeToString([]byte("tiny"))
	if _, err := Decrypt(key, short); !errors.Is(err, ErrInvalidCiphertext) {
		t.Errorf("too-short ciphertext: got %v, want ErrInvalidCiphertext", err)
	}
	// 非法 base64
	if _, err := Decrypt(key, "!!!not-base64!!!"); err == nil {
		t.Error("invalid base64 must error, got nil")
	}
}

func TestMustGetEncryptionKey(t *testing.T) {
	// 合法：64 个 hex 字符 = 32 字节
	key := MustGetEncryptionKey(strings.Repeat("ab", 32))
	if len(key) != 32 {
		t.Errorf("key length = %d, want 32", len(key))
	}

	// 非法 hex 应 panic
	assertPanics(t, "invalid hex", func() { MustGetEncryptionKey("zzzz") })
	// 长度不足应 panic（32 个 hex = 16 字节）
	assertPanics(t, "wrong length", func() { MustGetEncryptionKey(strings.Repeat("ab", 16)) })
}

func TestHashAndVerifyPassword(t *testing.T) {
	const pw = "S3cretP@ssw0rd"
	hash, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	if hash == pw {
		t.Error("hash must not equal the plaintext password")
	}
	if !VerifyPassword(pw, hash) {
		t.Error("correct password failed verification")
	}
	if VerifyPassword("wrong-password", hash) {
		t.Error("wrong password passed verification")
	}
	// 加盐：同一密码两次哈希应不同，但都能验证
	hash2, _ := HashPassword(pw)
	if hash2 == hash {
		t.Error("two hashes of the same password are identical (no salt)")
	}
	if !VerifyPassword(pw, hash2) {
		t.Error("second hash failed verification")
	}
}

func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("%s: expected panic, got none", name)
		}
	}()
	fn()
}

package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	ErrInvalidKeyLength  = errors.New("key must be 32 bytes for AES-256")
)

// versionPrefix 是当前加密格式的版本前缀。写入密文以便后续密钥/算法轮换：
// 旧数据（无前缀）按 v1 解密，新数据带 "v1:" 前缀，未来引入 v2 时可平滑迁移。
const versionPrefix = "v1:"

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns "v1:" + base64-encoded ciphertext (nonce prepended).
func Encrypt(key, plaintext []byte) (string, error) {
	if len(key) != 32 {
		return "", ErrInvalidKeyLength
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return versionPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded AES-256-GCM ciphertext.
// 兼容无前缀的历史密文（按 v1 处理）与带 "v1:" 前缀的新密文。
func Decrypt(key []byte, encoded string) ([]byte, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKeyLength
	}

	// 剥离版本前缀（若有）。无前缀的历史密文按 v1 处理，保证存量数据可解。
	payload := encoded
	if strings.HasPrefix(encoded, versionPrefix) {
		payload = strings.TrimPrefix(encoded, versionPrefix)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// EncryptString is a convenience wrapper for string input/output.
func EncryptString(key []byte, plaintext string) (string, error) {
	return Encrypt(key, []byte(plaintext))
}

// DecryptString is a convenience wrapper for string input/output.
func DecryptString(key []byte, encoded string) (string, error) {
	plain, err := Decrypt(key, encoded)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// GetEncryptionKey decodes and validates a hex-encoded AES-256 key.
// Returns an error instead of panicking, so callers can validate at startup.
func GetEncryptionKey(hexKey string) ([]byte, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key hex: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidKeyLength, len(key))
	}
	return key, nil
}

// MustGetEncryptionKey returns the encryption key from a hex-encoded string.
// Panics if the key is invalid. Prefer validating the key at startup (see
// GetEncryptionKey) so this never panics on a live request path.
func MustGetEncryptionKey(hexKey string) []byte {
	key, err := GetEncryptionKey(hexKey)
	if err != nil {
		panic(err.Error())
	}
	return key
}

// HashPassword hashes a password using bcrypt.
// This is a convenience wrapper to keep crypto utilities together.
func HashPassword(password string) (string, error) {
	// Use Go's standard library bcrypt
	return hashPasswordBcrypt(password)
}

// VerifyPassword verifies a password against a bcrypt hash.
func VerifyPassword(password, hash string) bool {
	return verifyPasswordBcrypt(password, hash)
}

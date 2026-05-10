package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// GenerateState 生成 OAuth state 参数（32 字节 → base64url，约 43 字符）
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64URLEncode(b), nil
}

// GenerateSessionID 生成 OAuth 会话 ID（16 字节 → hex，32 字符）
func GenerateSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GenerateCodeVerifier 生成 PKCE code_verifier（32 字节 → base64url，43 字符）
func GenerateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64URLEncode(b), nil
}

// GenerateOpenAICodeVerifier 生成 OpenAI 专用 PKCE code_verifier（64 字节 → hex，128 字符）
func GenerateOpenAICodeVerifier() (string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GenerateCodeChallenge 从 code_verifier 生成 PKCE code_challenge（S256 方法）
func GenerateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64URLEncode(h[:])
}

// base64URLEncode 实现 base64url 编码（无 padding）
func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

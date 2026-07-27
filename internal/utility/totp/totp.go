package totp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	issuerName    = "Team-API"
	accountPrefix = "team-api"
)

// GenerateSecret generates a new TOTP secret for a user.
// Returns the secret string and the otpauth:// URI for QR code scanning.
//
// 算法保留 SHA-1（而非升级 SHA-256），原因：当前存储链路只持久化 base32 secret，
// 不保存每条 secret 的算法元信息，而 totp.Validate(code, secret) 始终按 SHA-1 校验
// （忽略 otpauth URI 中携带的 algorithm）。若生成用 SHA-256、校验用 SHA-1，会导致
// 验证器 App 生成的 SHA-256 码在服务端永远校验失败。要安全切换到 SHA-256 必须先：
//  1. 在用户表持久化每条 secret 对应的算法；
//  2. ValidateCode 改用 totp.ValidateCustom 并按存储的算法校验。
// 这属于涉及 schema 与 2FA 安全路径的较大改动，超出 P3 细节优化范围，故暂保留 SHA-1。
func GenerateSecret(accountName string) (secret string, uri string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuerName,
		AccountName: accountName,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", fmt.Errorf("generate totp key: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// ValidateCode validates a TOTP code against a secret.
func ValidateCode(code, secret string) bool {
	return totp.Validate(code, secret)
}

// GenerateBackupCodes generates a set of one-time backup recovery codes.
// Returns plain text codes (to show to user once) and their SHA256 hashes (to store).
func GenerateBackupCodes(count int) (plainCodes []string, err error) {
	plainCodes = make([]string, count)
	for i := 0; i < count; i++ {
		bytes := make([]byte, 8) // 64-bit 熵 → 16 hex chars
		if _, err = rand.Read(bytes); err != nil {
			return nil, fmt.Errorf("generate backup code: %w", err)
		}
		plainCodes[i] = hex.EncodeToString(bytes)
	}
	return plainCodes, nil
}

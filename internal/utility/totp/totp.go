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
		bytes := make([]byte, 6) // 48-bit entropy → 12 hex chars
		if _, err = rand.Read(bytes); err != nil {
			return nil, fmt.Errorf("generate backup code: %w", err)
		}
		plainCodes[i] = hex.EncodeToString(bytes)
	}
	return plainCodes, nil
}

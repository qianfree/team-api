package kling

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// buildKlingJWT 从 accessKey|secretKey 格式的 API Key 生成 Kling JWT token
func buildKlingJWT(apiKey string) string {
	parts := strings.SplitN(apiKey, "|", 2)
	if len(parts) != 2 {
		return apiKey
	}
	accessKey, secretKey := parts[0], parts[1]

	now := time.Now()
	claims := jwt.MapClaims{
		"iss": accessKey,
		"exp": now.Add(1800 * time.Second).Unix(),
		"nbf": now.Add(-5 * time.Second).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secretKey))
	if err != nil {
		fmt.Printf("kling jwt sign error: %v\n", err)
		return apiKey
	}
	return signed
}

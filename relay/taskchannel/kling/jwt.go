package kling

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
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
		// 调试残留清理：stdout 打印改为结构化日志，回退明文 key 继续请求（由上游报错暴露问题）
		g.Log().Warningf(context.Background(), "kling jwt sign error: %v", err)
		return apiKey
	}
	return signed
}

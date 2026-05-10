package tencent

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// sign 使用 TC3-HMAC-SHA256 算法计算腾讯云 API 签名，返回完整的 Authorization 头值。
//
// 签名流程：
//  1. 拼接规范请求串（CanonicalRequest）
//  2. 拼接待签名字符串（StringToSign）
//  3. 逐层 HMAC 派生签名密钥（SecretDate → SecretService → SecretSigning）
//  4. 计算签名并组装 Authorization 头
func sign(secretID, secretKey, service, host, contentType string, payload []byte, timestamp int64) string {
	// 日期：UTC 格式 YYYY-MM-DD
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")

	// ---- 步骤 1：拼接规范请求 ----
	httpRequestMethod := "POST"
	canonicalURI := "/"
	canonicalQueryString := ""

	// 签名头必须包含 content-type 和 host，且按字典序排列
	signedHeaders := "content-type;host"
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\n", contentType, host)

	hashedPayload := sha256Hex(payload)

	canonicalRequest := strings.Join([]string{
		httpRequestMethod,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedPayload,
	}, "\n")

	// ---- 步骤 2：拼接待签名字符串 ----
	algorithm := "TC3-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)

	stringToSign := strings.Join([]string{
		algorithm,
		strconv.FormatInt(timestamp, 10),
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	// ---- 步骤 3：计算签名密钥 ----
	secretDate := hmacSHA256([]byte("TC3"+secretKey), []byte(date))
	secretService := hmacSHA256(secretDate, []byte(service))
	secretSigning := hmacSHA256(secretService, []byte("tc3_request"))

	// ---- 步骤 4：计算签名 ----
	signature := hex.EncodeToString(hmacSHA256(secretSigning, []byte(stringToSign)))

	// ---- 组装 Authorization ----
	return fmt.Sprintf(
		"%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, secretID, credentialScope, signedHeaders, signature,
	)
}

// hmacSHA256 计算 HMAC-SHA256
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// sha256Hex 计算 SHA-256 哈希并返回十六进制字符串
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

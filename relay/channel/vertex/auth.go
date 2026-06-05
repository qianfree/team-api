package vertex

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// serviceAccountKey Google 服务账号 JSON 密钥结构
type serviceAccountKey struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
}

// tokenEntry 缓存的访问令牌
type tokenEntry struct {
	accessToken string
	expiry      time.Time
}

// tokenCache 全局令牌缓存（key 为 client_email）
var tokenCache sync.Map

// sfGroup 合并并发令牌刷新请求
var sfGroup singleflight.Group

// getVertexAccessToken 获取 Vertex AI 访问令牌。
// 通过服务账号 JSON 密钥签发 JWT，然后通过 Google OAuth2 端点交换访问令牌。
// 令牌缓存在 sync.Map 中，未过期时复用。并发刷新通过 singleflight 合并。
func getVertexAccessToken(serviceAccountJSON string) (string, error) {
	var key serviceAccountKey
	if err := json.Unmarshal([]byte(serviceAccountJSON), &key); err != nil {
		return "", fmt.Errorf("parse service account JSON failed: %w", err)
	}

	// 检查缓存
	if cached, ok := tokenCache.Load(key.ClientEmail); ok {
		entry := cached.(*tokenEntry)
		if time.Now().Before(entry.expiry.Add(-60 * time.Second)) {
			return entry.accessToken, nil
		}
	}

	// 使用 singleflight 合并并发刷新
	val, err, _ := sfGroup.Do(key.ClientEmail, func() (any, error) {
		// 双重检查：进入 singleflight 后再次检查缓存
		if cached, ok := tokenCache.Load(key.ClientEmail); ok {
			entry := cached.(*tokenEntry)
			if time.Now().Before(entry.expiry.Add(-60 * time.Second)) {
				return entry.accessToken, nil
			}
		}

		rsaKey, err := parseRSAPrivateKey(key.PrivateKey)
		if err != nil {
			return "", fmt.Errorf("parse private key failed: %w", err)
		}

		now := time.Now()
		jwtToken, err := createSignedJWT(key.ClientEmail, rsaKey, now)
		if err != nil {
			return "", fmt.Errorf("create JWT failed: %w", err)
		}

		accessToken, expiresIn, err := exchangeJWTForToken(jwtToken)
		if err != nil {
			return "", fmt.Errorf("exchange token failed: %w", err)
		}

		tokenCache.Store(key.ClientEmail, &tokenEntry{
			accessToken: accessToken,
			expiry:      now.Add(time.Duration(expiresIn) * time.Second),
		})

		return accessToken, nil
	})
	if err != nil {
		return "", err
	}

	return val.(string), nil
}

// parseServiceAccountProjectID 从服务账号 JSON 中提取 project_id
func parseServiceAccountProjectID(serviceAccountJSON string) (string, error) {
	var key serviceAccountKey
	if err := json.Unmarshal([]byte(serviceAccountJSON), &key); err != nil {
		return "", fmt.Errorf("parse service account JSON failed: %w", err)
	}
	if key.ProjectID == "" {
		return "", fmt.Errorf("project_id not found in service account JSON")
	}
	return key.ProjectID, nil
}

// isServiceAccountJSON 判断给定字符串是否为服务账号 JSON
func isServiceAccountJSON(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") {
		return false
	}
	var key struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(s), &key); err != nil {
		return false
	}
	return key.Type == "service_account"
}

// parseRSAPrivateKey 从 PEM 编码的私钥字符串解析 RSA 私钥
func parseRSAPrivateKey(pemKey string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// 尝试 PKCS8 格式
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("key is not RSA")
		}
		return rsaKey, nil
	}

	// 尝试 PKCS1 格式
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key failed: %w", err)
	}
	return rsaKey, nil
}

// createSignedJWT 创建并签名 JWT
func createSignedJWT(clientEmail string, privateKey *rsa.PrivateKey, now time.Time) (string, error) {
	// JWT Header
	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	// JWT Claims
	claims := map[string]interface{}{
		"iss":   clientEmail,
		"scope": "https://www.googleapis.com/auth/cloud-platform",
		"aud":   "https://www.googleapis.com/oauth2/v4/token",
		"iat":   now.Unix(),
		"exp":   now.Add(3600 * time.Second).Unix(),
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	// 编码 Header 和 Claims
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := headerB64 + "." + claimsB64

	// RS256 签名
	hash := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign JWT failed: %w", err)
	}

	signatureB64 := base64.RawURLEncoding.EncodeToString(signature)
	return signingInput + "." + signatureB64, nil
}

// exchangeJWTForToken 通过 JWT 交换 Google OAuth2 访问令牌
func exchangeJWTForToken(jwtToken string) (string, int64, error) {
	tokenURL := "https://www.googleapis.com/oauth2/v4/token"

	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	data.Set("assertion", jwtToken)

	resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("read token response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("token exchange failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", 0, fmt.Errorf("parse token response failed: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", 0, fmt.Errorf("empty access token in response")
	}

	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}

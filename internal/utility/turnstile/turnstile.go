package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const verifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

var httpClient = &http.Client{Timeout: 10 * time.Second}

// VerifyResult represents the Cloudflare Turnstile verification response.
type VerifyResult struct {
	Success     bool     `json:"success"`
	ErrorCodes  []string `json:"error-codes"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	Action      string   `json:"action"`
	CData       string   `json:"cdata"`
}

// Verify verifies a Turnstile token with Cloudflare's siteverify API.
func Verify(ctx context.Context, secretKey, token, clientIP string) (*VerifyResult, error) {
	if token == "" {
		return nil, fmt.Errorf("turnstile token is empty")
	}

	data := url.Values{}
	data.Set("secret", secretKey)
	data.Set("response", token)
	if clientIP != "" {
		data.Set("remoteip", clientIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifyURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create verify request: %w", err)
	}
	req.URL.RawQuery = data.Encode()

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("turnstile verify request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read verify response: %w", err)
	}

	var result VerifyResult
	if err = json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse verify response: %w", err)
	}

	return &result, nil
}

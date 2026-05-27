//go:build integration

package testinfra

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type APIResponse struct {
	Code       int             `json:"code"`
	Message    string          `json:"message"`
	Data       json.RawMessage `json:"data"`
	RequestID  string          `json:"request_id"`
	HTTPStatus int
}

type APIClient struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string
}

func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *APIClient) WithToken(token string) *APIClient {
	clone := *c
	clone.Token = token
	return &clone
}

func (c *APIClient) Get(path string, params map[string]string) *APIResponse {
	return c.doRequest("GET", path, params, nil)
}

func (c *APIClient) Post(path string, body any) *APIResponse {
	return c.doRequest("POST", path, nil, body)
}

func (c *APIClient) Put(path string, body any) *APIResponse {
	return c.doRequest("PUT", path, nil, body)
}

func (c *APIClient) Delete(path string) *APIResponse {
	return c.doRequest("DELETE", path, nil, nil)
}

func (c *APIClient) doRequest(method, path string, params map[string]string, body any) *APIResponse {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return &APIResponse{HTTPStatus: 0, Message: fmt.Sprintf("marshal error: %v", err)}
		}
		bodyReader = bytes.NewReader(b)
	}

	fullURL := c.BaseURL + path
	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			q.Set(k, v)
		}
		if strings.Contains(fullURL, "?") {
			fullURL += "&" + q.Encode()
		} else {
			fullURL += "?" + q.Encode()
		}
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return &APIResponse{HTTPStatus: 0, Message: fmt.Sprintf("create request error: %v", err)}
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return &APIResponse{HTTPStatus: 0, Message: fmt.Sprintf("request error: %v", err)}
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return &APIResponse{HTTPStatus: resp.StatusCode, Message: fmt.Sprintf("read body error: %v", err)}
	}

	apiResp := &APIResponse{HTTPStatus: resp.StatusCode}
	_ = json.Unmarshal(data, apiResp)
	return apiResp
}

func (r *APIResponse) AssertSuccess(t *testing.T) {
	t.Helper()
	if r.HTTPStatus == 0 {
		t.Fatalf("request failed: %s", r.Message)
	}
	if r.Code != 0 {
		t.Fatalf("expected code=0, got code=%d message=%q request_id=%s", r.Code, r.Message, r.RequestID)
	}
}

func (r *APIResponse) AssertError(t *testing.T, expectedCode int) {
	t.Helper()
	if r.Code != expectedCode {
		t.Fatalf("expected code=%d, got code=%d message=%q", expectedCode, r.Code, r.Message)
	}
}

func (r *APIResponse) AssertHTTPStatus(t *testing.T, expected int) {
	t.Helper()
	if r.HTTPStatus != expected {
		t.Fatalf("expected HTTP %d, got %d", expected, r.HTTPStatus)
	}
}

func (r *APIResponse) DecodeData(t *testing.T, target any) {
	t.Helper()
	if err := json.Unmarshal(r.Data, target); err != nil {
		t.Fatalf("decode data error: %v, raw: %s", err, string(r.Data))
	}
}

func (r *APIResponse) GetID(t *testing.T) int64 {
	t.Helper()
	var result struct {
		ID int64 `json:"id"`
	}
	r.DecodeData(t, &result)
	return result.ID
}

func (r *APIResponse) GetTotal(t *testing.T) int {
	t.Helper()
	var result struct {
		Total int `json:"total"`
	}
	r.DecodeData(t, &result)
	return result.Total
}

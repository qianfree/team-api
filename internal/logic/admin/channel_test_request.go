package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	relaycommon "github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// sendTestRequest 发送最小测试请求到上游供应商
func sendTestRequest(ctx context.Context, channelType int, baseURL, apiKey, modelName string, useProxy bool) testResult {
	baseURL = strings.TrimSuffix(baseURL, "/")

	switch constant.ProviderType(channelType) {
	case constant.ProviderOpenAI:
		return testOpenAI(ctx, baseURL, apiKey, modelName, useProxy)
	case constant.ProviderClaude:
		return testClaude(ctx, baseURL, apiKey, modelName, useProxy)
	case constant.ProviderGemini:
		return testGemini(ctx, baseURL, apiKey, modelName, useProxy)
	case constant.ProviderZhipu:
		return testZhipu(ctx, baseURL, apiKey, modelName, useProxy)
	default:
		// 尝试 OpenAI 兼容格式
		return testOpenAI(ctx, baseURL, apiKey, modelName, useProxy)
	}
}

// doTestHTTPRequest 发送测试HTTP请求，根据渠道设置决定是否使用代理
func doTestHTTPRequest(ctx context.Context, method, reqURL string, headers map[string]string, body []byte, useProxy bool) (*http.Response, error) {
	client := relaycommon.NewPooledClient(30, useProxy)

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return client.Do(req)
}

// executeTest 封装通用的测试请求执行和结果处理逻辑
func executeTest(ctx context.Context, provider, method, reqURL string, headers map[string]string, body []byte, useProxy bool) testResult {
	maskedHdrs := maskHeaders(headers)
	bodyStr := string(body)

	g.Log().Infof(ctx, "[ChannelTest] %s | 发送请求 | %s %s | 代理: %v", provider, method, reqURL, useProxy)

	resp, err := doTestHTTPRequest(ctx, method, reqURL, headers, body, useProxy)
	if err != nil {
		g.Log().Warningf(ctx, "[ChannelTest] %s | 请求失败 | %s | 错误: %v", provider, reqURL, err)
		return testResult{
			Error:   fmt.Sprintf("请求失败: %v", err),
			Request: buildReqDetail(method, reqURL, maskedHdrs, bodyStr),
		}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	respStr := string(respBody)

	if resp.StatusCode != 200 {
		g.Log().Warningf(ctx, "[ChannelTest] %s | 测试失败 | HTTP %d | 响应: %s", provider, resp.StatusCode, truncateStr(respStr, 200))
		return testResult{
			Success:  false,
			Error:    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncateStr(respStr, 500)),
			Response: truncateStr(respStr, 500),
			Request:  buildReqDetail(method, reqURL, maskedHdrs, bodyStr),
		}
	}

	g.Log().Infof(ctx, "[ChannelTest] %s | 测试成功 | HTTP %d | 响应: %s", provider, resp.StatusCode, truncateStr(respStr, 200))
	return testResult{
		Success:  true,
		Response: truncateStr(respStr, 500),
		Request:  buildReqDetail(method, reqURL, maskedHdrs, bodyStr),
	}
}

func testOpenAI(ctx context.Context, baseURL, apiKey, modelName string, useProxy bool) testResult {
	reqBody := map[string]any{
		"model":      modelName,
		"max_tokens": 5,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	}
	bodyJSON, _ := json.Marshal(reqBody)
	reqURL := baseURL + "/v1/chat/completions"
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Content-Type":  "application/json",
	}
	return executeTest(ctx, "OpenAI", "POST", reqURL, headers, bodyJSON, useProxy)
}

func testClaude(ctx context.Context, baseURL, apiKey, modelName string, useProxy bool) testResult {
	reqBody := map[string]any{
		"model":      modelName,
		"max_tokens": 5,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	}
	bodyJSON, _ := json.Marshal(reqBody)
	reqURL := baseURL + "/v1/messages"
	headers := map[string]string{
		"x-api-key":         apiKey,
		"anthropic-version": "2023-06-01",
		"Content-Type":      "application/json",
	}
	return executeTest(ctx, "Claude", "POST", reqURL, headers, bodyJSON, useProxy)
}

func testGemini(ctx context.Context, baseURL, apiKey, modelName string, useProxy bool) testResult {
	reqBody := map[string]any{
		"contents": []map[string]any{
			{"role": "user", "parts": []map[string]string{{"text": "hi"}}},
		},
		"generationConfig": map[string]any{
			"maxOutputTokens": 5,
		},
	}
	bodyJSON, _ := json.Marshal(reqBody)
	reqURL := fmt.Sprintf("%s/v1beta/models/%s:generateContent", baseURL, modelName)
	headers := map[string]string{
		"x-goog-api-key": apiKey,
		"Content-Type":   "application/json",
	}
	return executeTest(ctx, "Gemini", "POST", reqURL, headers, bodyJSON, useProxy)
}

func testZhipu(ctx context.Context, baseURL, apiKey, modelName string, useProxy bool) testResult {
	reqBody := map[string]any{
		"model":      modelName,
		"max_tokens": 5,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	}
	bodyJSON, _ := json.Marshal(reqBody)
	reqURL := baseURL + "/api/paas/v4/chat/completions"
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Content-Type":  "application/json",
	}
	return executeTest(ctx, "Zhipu", "POST", reqURL, headers, bodyJSON, useProxy)
}

func buildReqDetail(method, url string, headers map[string]string, body string) *v1.ChannelTestReqDetail {
	return &v1.ChannelTestReqDetail{
		Method:  method,
		URL:     url,
		Headers: headers,
		Body:    body,
	}
}

// maskHeaders 对请求头中的敏感信息进行脱敏
func maskHeaders(headers map[string]string) map[string]string {
	masked := make(map[string]string, len(headers))
	for k, v := range headers {
		lower := strings.ToLower(k)
		if strings.Contains(lower, "key") || strings.Contains(lower, "auth") {
			// 保留 "Bearer " 前缀
			if strings.HasPrefix(v, "Bearer ") {
				masked[k] = "Bearer " + maskKey(strings.TrimPrefix(v, "Bearer "))
			} else {
				masked[k] = maskKey(v)
			}
		} else {
			masked[k] = v
		}
	}
	return masked
}

// maskKey 对 API Key 进行脱敏处理
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// truncateStr 截断字符串
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

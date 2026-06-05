package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/relay/constant"
)

// sendTestRequest 发送最小测试请求到上游供应商
func sendTestRequest(ctx context.Context, channelType int, baseURL, apiKey, modelName string) testResult {
	baseURL = strings.TrimSuffix(baseURL, "/")

	switch constant.ProviderType(channelType) {
	case constant.ProviderOpenAI:
		return testOpenAI(ctx, baseURL, apiKey, modelName)
	case constant.ProviderClaude:
		return testClaude(ctx, baseURL, apiKey, modelName)
	case constant.ProviderGemini:
		return testGemini(ctx, baseURL, apiKey, modelName)
	case constant.ProviderZhipu:
		return testZhipu(ctx, baseURL, apiKey, modelName)
	default:
		// 尝试 OpenAI 兼容格式
		return testOpenAI(ctx, baseURL, apiKey, modelName)
	}
}

func testOpenAI(ctx context.Context, baseURL, apiKey, modelName string) testResult {
	reqBody := map[string]any{
		"model":      modelName,
		"max_tokens": 5,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	}
	bodyJSON, _ := json.Marshal(reqBody)

	reqURL := baseURL + "/v1/chat/completions"

	client := g.Client().SetTimeout(30 * time.Second)
	client.SetHeader("Authorization", "Bearer "+apiKey)
	client.SetHeader("Content-Type", "application/json")

	resp, err := client.DoRequest(ctx, "POST", reqURL, reqBody)
	if err != nil {
		return testResult{
			Error:   fmt.Sprintf("请求失败: %v", err),
			Request: buildReqDetail("POST", reqURL, map[string]string{"Authorization": maskKey(apiKey), "Content-Type": "application/json"}, string(bodyJSON)),
		}
	}
	defer resp.Close()

	respBody := string(resp.ReadAll())

	if resp.StatusCode != 200 {
		return testResult{
			Success:  false,
			Error:    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncateStr(respBody, 500)),
			Response: truncateStr(respBody, 500),
			Request:  buildReqDetail("POST", reqURL, map[string]string{"Authorization": maskKey(apiKey), "Content-Type": "application/json"}, string(bodyJSON)),
		}
	}

	return testResult{
		Success:  true,
		Response: truncateStr(respBody, 500),
		Request:  buildReqDetail("POST", reqURL, map[string]string{"Authorization": maskKey(apiKey), "Content-Type": "application/json"}, string(bodyJSON)),
	}
}

func testClaude(ctx context.Context, baseURL, apiKey, modelName string) testResult {
	reqBody := map[string]any{
		"model":      modelName,
		"max_tokens": 5,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	}
	bodyJSON, _ := json.Marshal(reqBody)

	reqURL := baseURL + "/v1/messages"

	client := g.Client().SetTimeout(30 * time.Second)
	client.SetHeader("x-api-key", apiKey)
	client.SetHeader("anthropic-version", "2023-06-01")
	client.SetHeader("Content-Type", "application/json")

	resp, err := client.DoRequest(ctx, "POST", reqURL, reqBody)
	if err != nil {
		return testResult{
			Error:   fmt.Sprintf("请求失败: %v", err),
			Request: buildReqDetail("POST", reqURL, map[string]string{"x-api-key": maskKey(apiKey), "anthropic-version": "2023-06-01", "Content-Type": "application/json"}, string(bodyJSON)),
		}
	}
	defer resp.Close()

	respBody := string(resp.ReadAll())

	if resp.StatusCode != 200 {
		return testResult{
			Success:  false,
			Error:    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncateStr(respBody, 500)),
			Response: truncateStr(respBody, 500),
			Request:  buildReqDetail("POST", reqURL, map[string]string{"x-api-key": maskKey(apiKey), "anthropic-version": "2023-06-01", "Content-Type": "application/json"}, string(bodyJSON)),
		}
	}

	return testResult{
		Success:  true,
		Response: truncateStr(respBody, 500),
		Request:  buildReqDetail("POST", reqURL, map[string]string{"x-api-key": maskKey(apiKey), "anthropic-version": "2023-06-01", "Content-Type": "application/json"}, string(bodyJSON)),
	}
}

func testGemini(ctx context.Context, baseURL, apiKey, modelName string) testResult {
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

	client := g.Client().SetTimeout(30 * time.Second)
	client.SetHeader("x-goog-api-key", apiKey)
	client.SetHeader("Content-Type", "application/json")

	resp, err := client.DoRequest(ctx, "POST", reqURL, reqBody)
	if err != nil {
		return testResult{
			Error:   fmt.Sprintf("请求失败: %v", err),
			Request: buildReqDetail("POST", reqURL, map[string]string{"x-goog-api-key": maskKey(apiKey), "Content-Type": "application/json"}, string(bodyJSON)),
		}
	}
	defer resp.Close()

	respBody := string(resp.ReadAll())

	if resp.StatusCode != 200 {
		return testResult{
			Success:  false,
			Error:    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncateStr(respBody, 500)),
			Response: truncateStr(respBody, 500),
			Request:  buildReqDetail("POST", reqURL, map[string]string{"x-goog-api-key": maskKey(apiKey), "Content-Type": "application/json"}, string(bodyJSON)),
		}
	}

	return testResult{
		Success:  true,
		Response: truncateStr(respBody, 500),
		Request:  buildReqDetail("POST", reqURL, map[string]string{"x-goog-api-key": maskKey(apiKey), "Content-Type": "application/json"}, string(bodyJSON)),
	}
}

func testZhipu(ctx context.Context, baseURL, apiKey, modelName string) testResult {
	reqBody := map[string]any{
		"model":      modelName,
		"max_tokens": 5,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	}
	bodyJSON, _ := json.Marshal(reqBody)

	reqURL := baseURL + "/api/paas/v4/chat/completions"

	client := g.Client().SetTimeout(30 * time.Second)
	client.SetHeader("Authorization", "Bearer "+apiKey)
	client.SetHeader("Content-Type", "application/json")

	resp, err := client.DoRequest(ctx, "POST", reqURL, reqBody)
	if err != nil {
		return testResult{
			Error:   fmt.Sprintf("请求失败: %v", err),
			Request: buildReqDetail("POST", reqURL, map[string]string{"Authorization": maskKey(apiKey), "Content-Type": "application/json"}, string(bodyJSON)),
		}
	}
	defer resp.Close()

	respBody := string(resp.ReadAll())

	if resp.StatusCode != 200 {
		return testResult{
			Success:  false,
			Error:    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncateStr(respBody, 500)),
			Response: truncateStr(respBody, 500),
			Request:  buildReqDetail("POST", reqURL, map[string]string{"Authorization": maskKey(apiKey), "Content-Type": "application/json"}, string(bodyJSON)),
		}
	}

	return testResult{
		Success:  true,
		Response: truncateStr(respBody, 500),
		Request:  buildReqDetail("POST", reqURL, map[string]string{"Authorization": maskKey(apiKey), "Content-Type": "application/json"}, string(bodyJSON)),
	}
}

func buildReqDetail(method, url string, headers map[string]string, body string) *v1.ChannelTestReqDetail {
	return &v1.ChannelTestReqDetail{
		Method:  method,
		URL:     url,
		Headers: headers,
		Body:    body,
	}
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

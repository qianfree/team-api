package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"

	// blank import 触发内置转换器注册（与生产桥接路径一致）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
)

// TestOpenAIToClaudeInbound_Parity openai→claude 非流式对拍：经 handler 全路径
//（已接 relaykit 桥接）与 legacy openAIToClaudeResponse 直调比较。
// 能力接口经 common.RelayInfo 生效（msg_<RequestID> 合成 ID、映射渠道模型名两侧一致）。
func TestOpenAIToClaudeInbound_Parity(t *testing.T) {
	const body = `{
		"id":"c1","model":"gpt-4o-2024","choices":[{
			"index":0,
			"message":{"role":"assistant","content":"答案","reasoning_content":"思考",
				"tool_calls":[{"id":"t1","type":"function","function":{"name":"f","arguments":"{\"a\":1}"}}]},
			"finish_reason":"tool_calls"}],
		"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120,
			"prompt_tokens_details":{"cached_tokens":30}}
	}`

	legacyInfo := &common.RelayInfo{
		ClientFormat:    constant.RelayFormatClaude,
		InboundFormat:   constant.RelayFormatClaude,
		RequestID:       "req-9",
		OriginModelName: "gpt-4o",
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			IsModelMapped:     true,
			UpstreamModelName: "gpt-4o-2024",
		},
	}
	var openaiResp dto.ChatCompletionResponse
	_ = json.Unmarshal([]byte(body), &openaiResp)
	legacyOut, _ := json.Marshal(openAIToClaudeResponse(&openaiResp, legacyInfo))

	// relaykit：经注册表公共 API（spec A Resp 侧，info 即 RelayInfo——能力接口生效）
	spec, ok := relayconvert.LookupTextConverter(relayconvert.ConverterClaudeMessagesToOpenAIChat)
	if !ok {
		t.Fatal("expected spec A registered")
	}
	kitAny, _, err := spec.Resp.Convert(context.Background(), legacyInfo, &openaiResp)
	if err != nil {
		t.Fatalf("relaykit convert: %v", err)
	}
	kitOut, _ := json.Marshal(kitAny)

	var legacyMap, kitMap map[string]any
	_ = json.Unmarshal(legacyOut, &legacyMap)
	_ = json.Unmarshal(kitOut, &kitMap)
	if !reflect.DeepEqual(legacyMap, kitMap) {
		t.Errorf("parity mismatch\nlegacy:  %s\nrelaykit: %s", legacyOut, kitOut)
	}

	// 接线验证：handler 全路径产物 == 桥接产物（relaykit 接管）
	resp := &http.Response{StatusCode: 200, Body: http.NoBody}
	resp.Body = io.NopCloser(strings.NewReader(body))
	rec := httptest.NewRecorder()
	usage, err := handleClaudeInboundNonStream(context.Background(), resp, legacyInfo, rec)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !reflect.DeepEqual(rec.Body.Bytes(), kitOut) {
		t.Errorf("handler body 应为 relaykit 产物\nhandler:  %s\nrelaykit: %s", rec.Body.String(), kitOut)
	}
	if usage.PromptTokens != 100 || usage.CompletionTokens != 20 {
		t.Errorf("usage = %+v（legacy 口径提取）", usage)
	}
}

// TestOpenAIToGeminiInbound_Parity openai→gemini 非流式对拍。
func TestOpenAIToGeminiInbound_Parity(t *testing.T) {
	const body = `{
		"id":"c2","model":"glm-4.6","choices":[{
			"index":0,
			"message":{"role":"assistant","content":"答案","reasoning_content":"思考",
				"tool_calls":[{"id":"t1","type":"function","function":{"name":"f","arguments":"{\"a\":1}"}}]},
			"finish_reason":"tool_calls"}],
		"usage":{"prompt_tokens":50,"completion_tokens":30,"total_tokens":80,
			"prompt_tokens_details":{"cached_tokens":10},
			"completion_tokens_details":{"reasoning_tokens":12}}
	}`

	info := &common.RelayInfo{
		ClientFormat:    constant.RelayFormatGemini,
		InboundFormat:   constant.RelayFormatGemini,
		OriginModelName: "glm-4.6",
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			IsModelMapped:     false,
			UpstreamModelName: "glm-4.6",
		},
	}
	var openaiResp dto.ChatCompletionResponse
	_ = json.Unmarshal([]byte(body), &openaiResp)
	legacyOut, _ := json.Marshal(openAIToGeminiResponse(&openaiResp, info))

	spec, ok := relayconvert.LookupTextConverter(relayconvert.ConverterGeminiContentToOpenAIChat)
	if !ok {
		t.Fatal("expected spec B registered")
	}
	kitAny, _, err := spec.Resp.Convert(context.Background(), info, &openaiResp)
	if err != nil {
		t.Fatalf("relaykit convert: %v", err)
	}
	kitOut, _ := json.Marshal(kitAny)

	var legacyMap, kitMap map[string]any
	_ = json.Unmarshal(legacyOut, &legacyMap)
	_ = json.Unmarshal(kitOut, &kitMap)
	if !reflect.DeepEqual(legacyMap, kitMap) {
		t.Errorf("parity mismatch\nlegacy:  %s\nrelaykit: %s", legacyOut, kitOut)
	}

	// 接线验证
	resp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}
	rec := httptest.NewRecorder()
	if _, err := handleGeminiInboundNonStream(context.Background(), resp, info, rec); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !reflect.DeepEqual(rec.Body.Bytes(), kitOut) {
		t.Errorf("handler body 应为 relaykit 产物\nhandler:  %s\nrelaykit: %s", rec.Body.String(), kitOut)
	}
}

package openai

import (
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
)

// TestApplyResponsesToolCalls Responses output 项的服务端工具计数：
// web_search_call / file_search_call 计数；function_call（客户端自定义函数）不计
func TestApplyResponsesToolCalls(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		Output: []dto.ResponsesOutput{
			{Type: "message", Role: "assistant"},
			{Type: "web_search_call", ID: "ws_1", Status: "completed"},
			{Type: "web_search_call", ID: "ws_2", Status: "completed"},
			{Type: "file_search_call", ID: "fs_1", Status: "completed"},
			{Type: "function_call", ID: "fc_1", Name: "get_weather", Status: "completed"},
			{Type: "reasoning"},
		},
	}
	usage := &common.Usage{}
	applyResponsesToolCalls(usage, resp)
	if got := usage.ToolCalls["web_search"]; got != 2 {
		t.Errorf("ToolCalls[web_search] = %d, want 2", got)
	}
	if got := usage.ToolCalls["file_search"]; got != 1 {
		t.Errorf("ToolCalls[file_search] = %d, want 1", got)
	}
	if _, has := usage.ToolCalls["function_call"]; has {
		t.Errorf("function_call must not be counted (client-side execution)")
	}
	if len(usage.ToolCalls) != 2 {
		t.Errorf("ToolCalls = %v, want exactly 2 keys", usage.ToolCalls)
	}

	// 空防护
	empty := &common.Usage{}
	applyResponsesToolCalls(empty, nil)
	applyResponsesToolCalls(nil, resp)
	if len(empty.ToolCalls) != 0 {
		t.Errorf("nil response must not produce counts")
	}
}

// TestResponsesResponseToChatCompletions_ToolCalls chat-via-responses 非流式桥：
// 计费用量附带服务端工具计数
func TestResponsesResponseToChatCompletions_ToolCalls(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		Output: []dto.ResponsesOutput{
			{Type: "message", Role: "assistant", Content: []dto.ResponsesOutputContent{
				{Type: "output_text", Text: "答案"},
			}},
			{Type: "web_search_call", ID: "ws_1", Status: "completed"},
		},
	}
	_, usage, err := ResponsesResponseToChatCompletions(resp, "chatcmpl-1", "gpt-4.1")
	if err != nil {
		t.Fatalf("ResponsesResponseToChatCompletions: %v", err)
	}
	if got := usage.ToolCalls["web_search"]; got != 1 {
		t.Errorf("ToolCalls[web_search] = %d, want 1", got)
	}
}

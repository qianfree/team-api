package oai_responses

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// TestR2C_WebSearchToolExtraction web_search 工具提取为 chat 请求级 web_search_options，
// 不再按内置工具丢弃；filters 等无 chat 对应物的键被剔除。
func TestR2C_WebSearchToolExtraction(t *testing.T) {
	req := mustParseResponsesRequest(t, `{
		"model": "gpt-5.6-sol",
		"input": "search something",
		"tools": [
			{"type":"function","name":"f1","parameters":{"type":"object"}},
			{"type":"web_search","search_context_size":"high","user_location":{"type":"approximate"},"filters":{"allowed_domains":["example.com"]}}
		]
	}`)
	info := &recordingReporter{Values: convmeta.Values{
		ChannelMetaAttached: true,
		UpstreamModelName:   "gpt-5.6-sol",
	}}

	result, err := (&ResponsesToOpenAIChatRequestConverter{}).ConvertRequest(context.Background(), info, req)
	if err != nil {
		t.Fatalf("ConvertRequest: %v", err)
	}
	chat := mustCastChatRequest(t, result)

	if len(chat.Tools) != 1 || chat.Tools[0].Function.Name != "f1" {
		t.Errorf("tools = %+v, want only f1", chat.Tools)
	}
	if len(chat.WebSearchOptions) == 0 {
		t.Fatal("WebSearchOptions missing")
	}
	var opts map[string]any
	if err := json.Unmarshal(chat.WebSearchOptions, &opts); err != nil {
		t.Fatalf("parse web_search_options: %v", err)
	}
	if opts["search_context_size"] != "high" {
		t.Errorf("search_context_size = %v, want high", opts["search_context_size"])
	}
	if _, ok := opts["user_location"]; !ok {
		t.Error("user_location missing")
	}
	if _, ok := opts["filters"]; ok {
		t.Error("filters 无 chat 对应物，不应透出")
	}
	// web_search 已提取，不应再有 tool:web_search 降级上报
	for _, r := range info.reports {
		if r.reason == "tool:web_search" {
			t.Errorf("web_search 不应再上报丢弃: %v", info.reports)
		}
	}
}

// TestClaudeToResponsesNonStream_WebSearchCall Claude 托管搜索块（server_tool_use +
// web_search_tool_result）还原为 web_search_call 项（query + sources）。
func TestClaudeToResponsesNonStream_WebSearchCall(t *testing.T) {
	info := &convmeta.Values{ChannelMetaAttached: true, UpstreamModelName: "claude-sonnet-4"}
	claudeResp := &dto.ClaudeResponse{
		ID: "msg_ws",
		Content: []dto.ClaudeContentBlock{
			{
				Type: "server_tool_use", ID: "srvtoolu_1", Name: "web_search",
				Input: map[string]any{"query": "golang generics"},
			},
			{
				Type:      "web_search_tool_result",
				ToolUseID: "srvtoolu_1",
				Content: []map[string]any{
					{"type": "web_search_result", "url": "https://go.dev/doc/generics", "title": "Generics"},
					{"type": "web_search_result", "url": "", "title": "no-url skipped"},
				},
			},
		},
	}
	result, _, err := (&ClaudeToResponsesResponseConverter{}).ConvertResponse(context.Background(), info, claudeResp)
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	resp := result.(*dto.OpenAIResponsesResponse)
	if len(resp.Output) != 1 {
		t.Fatalf("output = %+v, want single web_search_call", resp.Output)
	}
	item := resp.Output[0]
	if item.Type != "web_search_call" || item.ID != "srvtoolu_1" || item.Status != "completed" {
		t.Fatalf("item = %+v", item)
	}
	var action dto.ResponsesWebSearchAction
	mustJSON(t, item.Action, &action)
	if action.Type != "search" || action.Query != "golang generics" {
		t.Errorf("action = %+v", action)
	}
	if len(action.Sources) != 1 || action.Sources[0].URL != "https://go.dev/doc/generics" {
		t.Errorf("sources = %+v", action.Sources)
	}
}

// TestClaudeToResponsesStream_WebSearchCall 流式：server_tool_use 块开 web_search_call 项，
// query 由 input_json_delta 累积，sources 由 web_search_tool_result 块提供，
// done 与 completed output 携带完整 action。
func TestClaudeToResponsesStream_WebSearchCall(t *testing.T) {
	info := &convmeta.Values{ChannelMetaAttached: true, UpstreamModelName: "claude-sonnet-4"}
	sse := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_ws","model":"claude-sonnet-4","usage":{"input_tokens":10,"output_tokens":5}}}`,
		``,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"query\":\"go"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":" generics\"}"}}`,
		``,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"web_search_tool_result","tool_use_id":"srvtoolu_1","content":[{"type":"web_search_result","url":"https://go.dev","title":"Go"}]}}`,
		``,
		`data: {"type":"content_block_stop","index":1}`,
		``,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	var events []*dto.ResponsesStreamEvent
	err := (&ClaudeToResponsesStreamConverter{}).ConvertStreamResponse(
		context.Background(), info, strings.NewReader(sse),
		func(chunk any) error {
			if ev, ok := chunk.(*dto.ResponsesStreamEvent); ok {
				events = append(events, ev)
			}
			return nil
		})
	if err != nil {
		t.Fatalf("ConvertStreamResponse: %v", err)
	}

	var sawAdded bool
	var doneAction map[string]any
	var completedOutput []dto.ResponsesOutput
	for _, ev := range events {
		data, _ := ev.Data.(map[string]any)
		switch ev.Type {
		case "response.output_item.added":
			if item, ok := data["item"].(map[string]any); ok && item["type"] == "web_search_call" {
				sawAdded = true
			}
		case "response.output_item.done":
			if item, ok := data["item"].(map[string]any); ok && item["type"] == "web_search_call" {
				doneAction, _ = item["action"].(map[string]any)
			}
		case "response.completed":
			if resp, ok := data["response"].(*dto.OpenAIResponsesResponse); ok {
				completedOutput = resp.Output
			}
		}
	}
	if !sawAdded {
		t.Error("missing web_search_call output_item.added")
	}
	if doneAction == nil || doneAction["query"] != "go generics" {
		t.Fatalf("done action = %v, want query=go generics", doneAction)
	}
	sources, _ := doneAction["sources"].([]any)
	if len(sources) != 1 {
		t.Errorf("done action sources = %v, want 1", doneAction["sources"])
	}
	if len(completedOutput) != 1 || completedOutput[0].Type != "web_search_call" {
		t.Fatalf("completed output = %+v", completedOutput)
	}
	var action dto.ResponsesWebSearchAction
	mustJSON(t, completedOutput[0].Action, &action)
	if action.Query != "go generics" || len(action.Sources) != 1 || action.Sources[0].URL != "https://go.dev" {
		t.Errorf("completed action = %+v", action)
	}
}

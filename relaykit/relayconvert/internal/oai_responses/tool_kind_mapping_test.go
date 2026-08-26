package oai_responses

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// stashMeta 组合 convmeta.Values 与工具类型 stash 能力（响应侧转换器测试用）。
type stashMeta struct {
	convmeta.Values
	stash *toolKindStashFake
}

func newStashMeta(kinds map[string]string) *stashMeta {
	return &stashMeta{stash: &toolKindStashFake{kinds: kinds}}
}

func (m *stashMeta) StashResponsesToolKind(name, kind string) { m.stash.StashResponsesToolKind(name, kind) }
func (m *stashMeta) ResponsesToolKind(name string) string     { return m.stash.ResponsesToolKind(name) }

// TestBuildToolCallDoneItem 响应侧按 stash 还原输出项类型。
func TestBuildToolCallDoneItem(t *testing.T) {
	info := newStashMeta(map[string]string{
		"exec":        ToolKindCustom,
		"shell":       ToolKindLocalShell,
		"apply_patch": ToolKindApplyPatch,
	})

	// custom：{"input":...} 解包为 freeform 字符串
	item := buildToolCallDoneItem(info, "call_1", "exec", `{"input":"ls -la"}`)
	if item.Type != "custom_tool_call" || item.Input != "ls -la" || item.Name != "exec" || item.CallID != "call_1" {
		t.Errorf("custom item = %+v", item)
	}

	// local_shell：arguments 还原为 action（type 恒 exec）
	item = buildToolCallDoneItem(info, "call_2", "shell", `{"command":["npm","test"],"timeout":60000}`)
	if item.Type != "local_shell_call" {
		t.Fatalf("shell item type = %s", item.Type)
	}
	var action map[string]any
	mustJSON(t, item.Action, &action)
	if action["type"] != "exec" || action["timeout"].(float64) != 60000 {
		t.Errorf("shell action = %v", action)
	}
	cmd, _ := action["command"].([]any)
	if len(cmd) != 2 || cmd[0] != "npm" {
		t.Errorf("shell action.command = %v", action["command"])
	}

	// apply_patch：operation/path/patch → action
	item = buildToolCallDoneItem(info, "call_3", "apply_patch", `{"operation":"create_file","path":"a.go","patch":"@@"}`)
	if item.Type != "apply_patch_call" {
		t.Fatalf("patch item type = %s", item.Type)
	}
	mustJSON(t, item.Action, &action)
	if action["type"] != "create_file" || action["path"] != "a.go" || action["patch"] != "@@" {
		t.Errorf("patch action = %v", action)
	}

	// 未 stash：function_call 原样
	item = buildToolCallDoneItem(info, "call_4", "get_weather", `{"city":"sh"}`)
	if item.Type != "function_call" || item.Arguments != `{"city":"sh"}` {
		t.Errorf("function item = %+v", item)
	}

	// nil Meta / 未实现 stash：function_call（行为与映射引入前一致）
	item = buildToolCallDoneItem(nil, "call_5", "shell", `{}`)
	if item.Type != "function_call" {
		t.Errorf("nil meta item type = %s, want function_call", item.Type)
	}
}

// TestChatToResponsesNonStream_ToolKindRestore 非流式 chat→responses 按 stash 还原输出项。
func TestChatToResponsesNonStream_ToolKindRestore(t *testing.T) {
	info := newStashMeta(map[string]string{"shell": ToolKindLocalShell})
	chatResp := &dto.ChatCompletionResponse{
		ID: "chatcmpl_1",
		Choices: []dto.Choice{{
			Message: dto.Message{
				ToolCalls: []dto.ToolCall{{
					ID:   "call_s1",
					Type: "function",
					Function: dto.FunctionCall{
						Name:      "shell",
						Arguments: `{"command":["ls"]}`,
					},
				}},
			},
		}},
	}
	result, _, err := (&OpenAIChatToResponsesResponseConverter{}).ConvertResponse(context.Background(), info, chatResp)
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	resp := result.(*dto.OpenAIResponsesResponse)
	if len(resp.Output) != 1 || resp.Output[0].Type != "local_shell_call" {
		t.Fatalf("output = %+v, want single local_shell_call", resp.Output)
	}
	var action map[string]any
	mustJSON(t, resp.Output[0].Action, &action)
	if action["type"] != "exec" {
		t.Errorf("action = %v", action)
	}
}

// TestChatToResponsesStream_ToolKindRestore 流式 chat→responses：custom 工具的
// added/done 事件类型与 completed output 还原。
func TestChatToResponsesStream_ToolKindRestore(t *testing.T) {
	info := newStashMeta(map[string]string{"exec": ToolKindCustom})
	sse := strings.Join([]string{
		`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"exec","arguments":""}}]}}]}`,
		``,
		`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"input\":\"ls"}}]}}]}`,
		``,
		`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":" -la\"}"}}]}}]}`,
		``,
		`data: {"id":"chatcmpl_1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	var events []*dto.ResponsesStreamEvent
	err := (&OpenAIChatToResponsesStreamConverter{}).ConvertStreamResponse(
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

	var addedItem, doneItem map[string]any
	var sawArgsDelta, sawInputDone bool
	var completedOutput []dto.ResponsesOutput
	for _, ev := range events {
		data, _ := ev.Data.(map[string]any)
		switch ev.Type {
		case "response.output_item.added":
			if item, ok := data["item"].(map[string]any); ok && item["type"] == "custom_tool_call" {
				addedItem = item
			}
		case "response.function_call_arguments.delta":
			sawArgsDelta = true
		case "response.custom_tool_call_input.done":
			sawInputDone = true
			if data["input"] != "ls -la" {
				t.Errorf("input.done input = %v, want %q", data["input"], "ls -la")
			}
		case "response.output_item.done":
			if item, ok := data["item"].(map[string]any); ok && item["type"] == "custom_tool_call" {
				doneItem = item
			}
		case "response.completed":
			if resp, ok := data["response"].(*dto.OpenAIResponsesResponse); ok {
				completedOutput = resp.Output
			}
		}
	}
	if addedItem == nil {
		t.Error("missing custom_tool_call output_item.added")
	}
	if sawArgsDelta {
		t.Error("custom 工具的 JSON 包装 arguments 不应透出 function_call_arguments.delta")
	}
	if !sawInputDone {
		t.Error("missing response.custom_tool_call_input.done")
	}
	if doneItem == nil || doneItem["input"] != "ls -la" {
		t.Errorf("done item = %v", doneItem)
	}
	if len(completedOutput) != 1 || completedOutput[0].Type != "custom_tool_call" || completedOutput[0].Input != "ls -la" {
		t.Errorf("completed output = %+v", completedOutput)
	}
}

// TestClaudeToResponsesNonStream_ToolKindRestore claude tool_use 按 stash 还原。
func TestClaudeToResponsesNonStream_ToolKindRestore(t *testing.T) {
	info := newStashMeta(map[string]string{"exec": ToolKindCustom})
	input := map[string]any{"input": "pwd"}
	claudeResp := &dto.ClaudeResponse{
		ID: "msg_1",
		Content: []dto.ClaudeContentBlock{{
			Type: "tool_use", ID: "toolu_1", Name: "exec", Input: input,
		}},
	}
	result, _, err := (&ClaudeToResponsesResponseConverter{}).ConvertResponse(context.Background(), info, claudeResp)
	if err != nil {
		t.Fatalf("ConvertResponse: %v", err)
	}
	resp := result.(*dto.OpenAIResponsesResponse)
	if len(resp.Output) != 1 || resp.Output[0].Type != "custom_tool_call" || resp.Output[0].Input != "pwd" {
		t.Fatalf("output = %+v, want custom_tool_call input=pwd", resp.Output)
	}
}

func mustJSON(t *testing.T, raw []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
}

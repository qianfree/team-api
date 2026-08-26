package oai_responses

import (
	"encoding/json"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// codex 等 Responses 客户端的非 function 工具（local_shell / custom / apply_patch /
// namespace 子工具）在 chat 协议中无原生对应物。本文件实现它们与 chat function 工具的
// 双向映射：
//   - 请求侧：映射为 function 工具（匿名工具分配固定映射名），并按映射名 stash 原始
//     工具类型（经 Meta 可选能力接口，宿主 RelayInfo 实现）；
//   - 响应侧：上游返回的 function_call 按映射名还原为 codex 期望的输出项类型
//     （local_shell_call / custom_tool_call / apply_patch_call）。
//
// 历史项（custom_tool_call / local_shell_call / apply_patch_call 及其 output）在请求侧
// 还原为 assistant tool_calls + tool 消息，使多轮 agent 循环在 chat 上游不断裂。

const (
	// ToolKindCustom Responses custom（freeform/grammar）工具：映射为带 {"input": string}
	// 包裹 schema 的 function 工具，响应侧还原为 custom_tool_call
	ToolKindCustom = "custom"
	// ToolKindLocalShell Responses local_shell 工具：映射为名为 shell 的 function 工具，
	// 响应侧还原为 local_shell_call（action.type=exec）
	ToolKindLocalShell = "local_shell"
	// ToolKindApplyPatch Responses apply_patch 工具：映射为同名 function 工具，
	// 响应侧还原为 apply_patch_call
	ToolKindApplyPatch = "apply_patch"
)

// 匿名 Responses 工具的固定映射名（local_shell/apply_patch 工具定义本身无 name 字段）
const (
	localShellMappedName = "shell"
	applyPatchMappedName = "apply_patch"
)

// responsesToolKindStash 宿主可选实现的 Meta 扩展接口：请求侧记录工具映射名 → 原始
// Responses 工具类型，响应侧据此把上游 function_call 还原为对应输出项类型。
// 宿主 relay/common.RelayInfo 实现本接口；未实现时（单测/非宿主调用）所有工具调用
// 按普通 function_call 处理（行为与映射引入前一致）。
type responsesToolKindStash interface {
	StashResponsesToolKind(name, kind string)
	ResponsesToolKind(name string) string
}

// stashToolKind 记录工具映射名 → 原始类型（接口未实现时 no-op）。
func stashToolKind(info convmeta.Meta, name, kind string) {
	if stash, ok := info.(responsesToolKindStash); ok && stash != nil {
		stash.StashResponsesToolKind(name, kind)
	}
}

// toolKindOf 查询工具名的原始 Responses 工具类型（未 stash 返回空串=function 工具）。
func toolKindOf(info convmeta.Meta, name string) string {
	if stash, ok := info.(responsesToolKindStash); ok && stash != nil {
		return stash.ResponsesToolKind(name)
	}
	return ""
}

// customToolChatSchema custom（freeform/grammar）工具的 chat function 包裹 schema：
// 模型产出 {"input": "<freeform text>"}，响应侧解包还原为 custom_tool_call 的 input 字符串。
func customToolChatSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"input": map[string]any{
				"type":        "string",
				"description": "Freeform input passed to the tool verbatim.",
			},
		},
		"required":             []string{"input"},
		"additionalProperties": false,
	}
}

// localShellChatSchema local_shell 工具的 chat function schema（对齐 ResponsesShellAction
// 的 exec 字段；command 必填，其余可选）。
func localShellChatSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "The command to execute, as an argv array (e.g. [\"ls\", \"-la\"]).",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Maximum runtime in milliseconds.",
			},
			"working_directory": map[string]any{
				"type":        "string",
				"description": "Working directory for the command.",
			},
			"env": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "string"},
				"description":          "Environment variables to set.",
			},
			"user": map[string]any{
				"type":        "string",
				"description": "User to run the command as.",
			},
			"max_output_length": map[string]any{
				"type":        "integer",
				"description": "Maximum output characters (UTF-8).",
			},
		},
		"required":             []string{"command"},
		"additionalProperties": false,
	}
}

// applyPatchChatSchema apply_patch 工具的 chat function schema（对齐 ResponsesPatchAction）。
func applyPatchChatSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"enum":        []string{"create_file", "update_file", "delete_file"},
				"description": "Patch operation type.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Target file path.",
			},
			"patch": map[string]any{
				"type":        "string",
				"description": "Unified diff content (not required for delete_file).",
			},
		},
		"required":             []string{"operation", "path"},
		"additionalProperties": false,
	}
}

// wrapCustomToolInput 把 custom 工具的 freeform input 字符串包装为 chat function 的
// arguments JSON（{"input": "..."}）；与响应侧的解包互逆。
func wrapCustomToolInput(input string) string {
	b, err := json.Marshal(map[string]string{"input": input})
	if err != nil {
		return `{"input":""}`
	}
	return string(b)
}

// ==================== 响应侧：function_call → codex 输出项还原 ====================

// buildToolCallAddedItem 构建 output_item.added 事件的 item 载荷（in_progress）。
// 按 stash 的工具原始类型产出对应项类型；未 stash 时为 function_call（行为与映射引入前一致）。
func buildToolCallAddedItem(info convmeta.Meta, id, name string) map[string]any {
	switch toolKindOf(info, name) {
	case ToolKindCustom:
		return map[string]any{
			"type": "custom_tool_call", "id": id, "call_id": id,
			"name": name, "input": "", "status": "in_progress",
		}
	case ToolKindLocalShell:
		return map[string]any{
			"type": "local_shell_call", "id": id, "call_id": id,
			"status": "in_progress", "action": map[string]any{"type": "exec"},
		}
	case ToolKindApplyPatch:
		return map[string]any{
			"type": "apply_patch_call", "id": id, "call_id": id, "status": "in_progress",
		}
	default:
		return map[string]any{
			"type": "function_call", "id": id, "call_id": id,
			"name": name,
			// codex 等严格客户端的 FunctionCall.arguments 为必填键，缺失解析失败（真实 OpenAI 恒带空串）
			"arguments": "", "status": "in_progress",
		}
	}
}

// unwrapCustomToolInput 解包 custom 工具的 {"input":...} arguments 为 freeform 字符串
// （与请求侧 wrapCustomToolInput 互逆）；解包失败时回退原始 arguments。
func unwrapCustomToolInput(argsJSON string) string {
	var v struct {
		Input string `json:"input"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &v); err == nil && v.Input != "" {
		return v.Input
	}
	return argsJSON
}

// buildShellAction 由 function arguments 构建 local_shell_call 的 action（type 恒 exec）。
func buildShellAction(argsJSON string) json.RawMessage {
	action := map[string]any{"type": "exec"}
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err == nil {
		for k, v := range args {
			if k != "type" {
				action[k] = v
			}
		}
	}
	b, _ := json.Marshal(action)
	return b
}

// buildPatchAction 由 function arguments（operation/path/patch）构建 apply_patch_call 的 action。
func buildPatchAction(argsJSON string) json.RawMessage {
	var args struct {
		Operation string `json:"operation"`
		Path      string `json:"path"`
		Patch     string `json:"patch"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &args)
	op := args.Operation
	if op == "" {
		op = "update_file"
	}
	action := map[string]any{"type": op}
	if args.Path != "" {
		action["path"] = args.Path
	}
	if args.Patch != "" {
		action["patch"] = args.Patch
	}
	b, _ := json.Marshal(action)
	return b
}

// buildToolCallDoneItem 构建 completed 状态的工具调用输出项（非流式 output 数组与
// 流式 output_item.done / response.completed 共用）。
func buildToolCallDoneItem(info convmeta.Meta, id, name, argsJSON string) dto.ResponsesOutput {
	switch toolKindOf(info, name) {
	case ToolKindCustom:
		return dto.ResponsesOutput{
			Type: "custom_tool_call", ID: id, CallID: id, Name: name,
			Input: unwrapCustomToolInput(argsJSON), Status: "completed",
		}
	case ToolKindLocalShell:
		return dto.ResponsesOutput{
			Type: "local_shell_call", ID: id, CallID: id,
			Status: "completed", Action: buildShellAction(argsJSON),
		}
	case ToolKindApplyPatch:
		return dto.ResponsesOutput{
			Type: "apply_patch_call", ID: id, CallID: id,
			Status: "completed", Action: buildPatchAction(argsJSON),
		}
	default:
		return dto.ResponsesOutput{
			Type: "function_call", ID: id, CallID: id, Name: name,
			Arguments: argsJSON, Status: "completed",
		}
	}
}

// toolCallArgsDoneEvent 返回工具参数收尾事件的类型与载荷键；ok=false 表示该工具类型
// 无参数收尾事件（local_shell/apply_patch 的参数非流式文本，仅在 output_item.done 携带）。
// custom 工具的参数收尾事件为 response.custom_tool_call_input.done，载荷键 input
// （解包后的 freeform 字符串）；function 工具为 response.function_call_arguments.done /
// arguments（原始 JSON 字符串）。
func toolCallArgsDoneEvent(info convmeta.Meta, name string) (eventType, payloadKey string, ok bool) {
	switch toolKindOf(info, name) {
	case ToolKindCustom:
		return "response.custom_tool_call_input.done", "input", true
	case ToolKindLocalShell, ToolKindApplyPatch:
		return "", "", false
	default:
		return "response.function_call_arguments.done", "arguments", true
	}
}

// toolCallArgsDeltaEvent 返回工具参数增量事件类型；ok=false 表示抑制增量
// （非 function 工具的 arguments 是 JSON 包装形态，增量透出会破坏客户端解析，
// 缓冲至收尾事件一次性给出）。
func toolCallArgsDeltaEvent(info convmeta.Meta, name string) (string, bool) {
	if toolKindOf(info, name) == "" {
		return "response.function_call_arguments.delta", true
	}
	return "", false
}

// toolCallDoneItemPayload 将 done 状态的输出项转为事件载荷 map（output_item.done 的 item 键）。
func toolCallDoneItemPayload(info convmeta.Meta, id, name, argsJSON string) map[string]any {
	item := buildToolCallDoneItem(info, id, name, argsJSON)
	b, _ := json.Marshal(item)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

// mapNonFunctionToolToChat 将单个非 function 的 Responses 工具定义映射为 chat function
// 工具。返回 false 表示无法映射（调用方走丢弃上报）。seen 为已占用的工具名集合
// （跨顶层 tools 与 additional_tools 共享），映射名冲突时放弃映射。
// namespace 工具不是叶子，由调用方递归展开，本函数不处理。
func mapNonFunctionToolToChat(info convmeta.Meta, tool map[string]any, seen map[string]bool) (dto.Tool, bool) {
	toolType, _ := tool["type"].(string)
	switch toolType {
	case ToolKindLocalShell:
		if seen[localShellMappedName] {
			return dto.Tool{}, false
		}
		seen[localShellMappedName] = true
		stashToolKind(info, localShellMappedName, ToolKindLocalShell)
		return dto.Tool{Type: "function", Function: dto.FunctionDef{
			Name:        localShellMappedName,
			Description: "Execute a command in the local shell environment.",
			Parameters:  localShellChatSchema(),
		}}, true
	case ToolKindApplyPatch:
		if seen[applyPatchMappedName] {
			return dto.Tool{}, false
		}
		seen[applyPatchMappedName] = true
		stashToolKind(info, applyPatchMappedName, ToolKindApplyPatch)
		return dto.Tool{Type: "function", Function: dto.FunctionDef{
			Name:        applyPatchMappedName,
			Description: "Create, update or delete a file using a unified diff patch.",
			Parameters:  applyPatchChatSchema(),
		}}, true
	case ToolKindCustom:
		name, _ := tool["name"].(string)
		if name == "" || seen[name] {
			return dto.Tool{}, false
		}
		seen[name] = true
		stashToolKind(info, name, ToolKindCustom)
		fn := dto.FunctionDef{
			Name:       name,
			Parameters: customToolChatSchema(),
		}
		if desc, ok := tool["description"].(string); ok {
			fn.Description = desc
		}
		return dto.Tool{Type: "function", Function: fn}, true
	default:
		return dto.Tool{}, false
	}
}

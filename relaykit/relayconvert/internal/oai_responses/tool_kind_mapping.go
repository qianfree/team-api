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

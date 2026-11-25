package aiassistant

import "encoding/json"

// PluginCodeTools 插件生成工作台的模型工具集（Claude Code 式 agent loop）：
// 模型通过调用工具读写代码，宿主执行（前端状态机维护代码缓冲）并把结果以
// role=tool 消息回传续轮。工具即协议——取代此前的四反引号围栏文本协议，
// 结构化参数杜绝"模型输出一堆反引号"的解析歧义。
//
// 单文件（plugin.js）场景的四个原语：
//   - write_code   全量写入（新版本；整体重写）
//   - append_code  末尾追加一段（长文件分次写，done 标记收尾）
//   - edit_code    精确替换（修复轮定点修改，old_text 须唯一匹配）
//   - validate_code 跑上传同款校验并取回报告（模型自我验证）
var PluginCodeTools = []ToolDef{
	{
		Type: "function",
		Function: ToolDefFunc{
			Name:        "write_code",
			Description: "全量写入 plugin.js 源码（创建或整体重写，产生新版本）。code 必须是完整文件内容，不要省略任何部分。",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"code": {"type": "string", "description": "完整的 plugin.js 源码"}
				},
				"required": ["code"]
			}`),
		},
	},
	{
		Type: "function",
		Function: ToolDefFunc{
			Name:        "append_code",
			Description: "向当前源码末尾追加一段（源码过长时分次写，避免单次输出超限）。每段是文件的连续切片，末尾不带解释文字。",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"code": {"type": "string", "description": "追加的源码片段"},
					"done": {"type": "boolean", "description": "本段之后文件是否已完整（默认 false）"}
				},
				"required": ["code"]
			}`),
		},
	},
	{
		Type: "function",
		Function: ToolDefFunc{
			Name:        "edit_code",
			Description: "精确修改现有源码：old_text 必须与当前源码中的片段完全一致且唯一出现，将被 new_text 原样替换。修复校验错误时优先用本工具，不要重写整个文件。",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"old_text": {"type": "string", "description": "要替换的原文片段（须在当前源码中唯一匹配）"},
					"new_text": {"type": "string", "description": "替换后的内容"}
				},
				"required": ["old_text", "new_text"]
			}`),
		},
	},
	{
		Type: "function",
		Function: ToolDefFunc{
			Name:        "read_code",
			Description: "读取当前完整源码（升级/修改已有插件时必须先调用本工具查看现状，再按需修改）。返回纯源码原文与行数。",
			Parameters:  json.RawMessage(`{"type": "object", "properties": {}, "required": []}`),
		},
	},
	{
		Type: "function",
		Function: ToolDefFunc{
			Name:        "validate_code",
			Description: "对当前完整源码运行与插件上传同款的校验（编译/契约/试运行，不落库）。写入或修改完成后都应调用查看报告，有 fail 项就继续修复。",
			Parameters:  json.RawMessage(`{"type": "object", "properties": {}, "required": []}`),
		},
	},
}

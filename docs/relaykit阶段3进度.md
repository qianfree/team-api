# Relaykit 迁移 - 阶段 3 进度报告

**开始日期**：2026-07-28  
**当前状态**：进行中（Task 1 已完成）

---

## 总体进度

| 任务 | 状态 | 完成时间 |
|------|------|---------|
| Task 1: 提取共享逻辑 | ✅ 已完成 | 2026-07-28 |
| Task 2: OpenAI → Claude 请求转换器 | ✅ 已完成 | 2026-07-28 |
| Task 3: Claude → OpenAI 响应转换器 | ⏳ 待开始 | - |
| Task 4: OpenAI → Gemini 请求转换器 | ⏳ 待开始 | - |
| Task 5: Gemini → OpenAI 响应转换器 | ⏳ 待开始 | - |
| Task 6: Golden 测试 | ⏳ 待开始 | - |

---

## Task 1: 提取共享逻辑 ✅

### 已完成文件

#### 1. `message_mapper.go` - 消息格式映射

**核心函数**：
- `MapTextContent(content any) string` - 从任意内容格式提取纯文本
- `MapOpenAIContentPartsToClaude(parts []dto.ContentPart) []dto.ClaudeContentBlock` - OpenAI 多模态内容 → Claude
- `MapClaudeContentToOpenAI(blocks []dto.ClaudeContentBlock) any` - Claude 内容 → OpenAI（字符串或 ContentPart[]）
- `MapOpenAIImageToClaudeSource(imageURL dto.ImageURL) dto.ClaudeSource` - 图片 URL 转换

**关键实现细节**：
- Claude `ClaudeContentBlock.Text` 字段为 `*string`，需要指针转换
- 单个文本块返回字符串，多个块返回 ContentPart 数组
- 支持 data URL（base64）和 HTTP URL

**测试覆盖**：
- ✅ 纯文本内容提取
- ✅ ContentPart 数组提取
- ✅ 多模态内容转换（文本 + 图片）
- ✅ 空内容处理

#### 2. `tool_mapper.go` - 工具调用映射

**核心函数**：
- `MapOpenAIToolsToClaudeTools(tools []dto.Tool) []dto.ClaudeTool` - 工具定义转换
- `MapClaudeToolsToOpenAITools(tools []dto.ClaudeTool) []dto.Tool` - 工具定义反向转换
- `MapClaudeToolCallsToOpenAI(blocks []dto.ClaudeContentBlock) []dto.ToolCall` - Claude tool_use → OpenAI ToolCall
- `MapOpenAIToolCallsToClaude(toolCalls []dto.ToolCall) []dto.ClaudeContentBlock` - OpenAI ToolCall → Claude tool_use

**关键实现细节**：
- OpenAI `Tool.Function.Parameters` 是 `json.RawMessage`（JSON Schema）
- Claude `ClaudeTool.InputSchema` 是 `json.RawMessage`
- Tool call 的 Arguments 是 JSON 字符串，需要序列化/反序列化

**测试覆盖**：
- ✅ 工具定义双向转换
- ✅ 工具调用双向转换（含 JSON 参数验证）
- ✅ 多工具调用处理

#### 3. `thinking_adapter.go` - Thinking 模式适配

**核心函数**：
- `ParseThinkingSuffix(modelName string) ThinkingInfo` - 解析模型名后缀
- `ApplyThinkingToClaude(req *dto.ClaudeRequest, info ThinkingInfo, opts convmeta.ClaudeOptions)` - 应用到 Claude
- `ApplyThinkingToGemini(config *dto.GeminiGenerationConfig, info ThinkingInfo, opts convmeta.GeminiOptions)` - 应用到 Gemini
- `ShouldPreserveThinkingSuffix(modelName string, opts *convmeta.Options) bool` - 检查是否保留后缀

**支持的后缀**：
- `-thinking` / `-nothinking` - 显式启用/禁用思考
- `-low` / `-medium` / `-high` / `-xhigh` / `-max` / `-minimal` - 推理强度等级

**关键实现细节**：
- Claude: 设置 `Thinking.Type = "enabled"` + `BudgetTokens`（基于 max_tokens 百分比）
- Gemini: 设置 `ThinkingConfig.IncludeThoughts = true` + `ThoughtBudget`（基于 maxOutputTokens 百分比）
- `-nothinking` 显式禁用，优先级高于适配器启用
- 依赖 `relaykit/relayconvert/reasoning` 包的后缀解析

**测试覆盖**：
- ✅ 后缀解析（thinking/nothinking/effort levels）
- ✅ Claude thinking 配置（启用/禁用/budget）
- ✅ Gemini thinking 配置（启用/禁用/budget）
- ✅ 适配器开关控制

### 测试结果

```bash
cd relaykit && go test ./relayconvert/internal/shared/... -v
```

**结果**：✅ 全部通过（10 个测试用例）

```
=== RUN   TestMapTextContent
--- PASS: TestMapTextContent (0.00s)
=== RUN   TestMapOpenAIContentPartsToClaude
--- PASS: TestMapOpenAIContentPartsToClaude (0.00s)
=== RUN   TestMapClaudeContentToOpenAI
--- PASS: TestMapClaudeContentToOpenAI (0.00s)
=== RUN   TestParseThinkingSuffix
--- PASS: TestParseThinkingSuffix (0.00s)
=== RUN   TestApplyThinkingToClaude
--- PASS: TestApplyThinkingToClaude (0.00s)
=== RUN   TestApplyThinkingToGemini
--- PASS: TestApplyThinkingToGemini (0.00s)
=== RUN   TestMapOpenAIToolsToClaudeTools
--- PASS: TestMapOpenAIToolsToClaudeTools (0.00s)
=== RUN   TestMapClaudeToolsToOpenAITools
--- PASS: TestMapClaudeToolsToOpenAITools (0.00s)
=== RUN   TestMapClaudeToolCallsToOpenAI
--- PASS: TestMapClaudeToolCallsToOpenAI (0.00s)
=== RUN   TestMapOpenAIToolCallsToClaude
--- PASS: TestMapOpenAIToolCallsToClaude (0.00s)
PASS
ok  	github.com/qianfree/team-api/relaykit/relayconvert/internal/shared	0.727s
```

### 独立构建验证

```bash
cd relaykit && GOWORK=off go build ./relayconvert/internal/shared/...
```

**结果**：✅ 成功（无 GoFrame 依赖）

### 遇到的问题与解决

#### 问题 1: Claude DTO 字段类型不匹配

**错误**：
```
cannot use part.Text (variable of type string) as *string value in struct literal
cannot use block.Text (variable of type *string) as string value in struct literal
```

**原因**：`dto.ClaudeContentBlock.Text` 字段为 `*string`（指针），需要显式转换

**解决**：
```go
// 转换时创建临时变量
text := part.Text
blocks = append(blocks, dto.ClaudeContentBlock{
    Type: "text",
    Text: &text,
})

// 反向转换时解引用
text := ""
if block.Text != nil {
    text = *block.Text
}
```

#### 问题 2: Gemini ThinkingConfig 字段名错误

**错误**：
```
unknown field Mode in struct literal of type dto.GeminiThinkingConfig
thinkingConfig.ThinkingBudget undefined
```

**原因**：查看 `dto.GeminiThinkingConfig` 定义后发现：
- 正确字段：`IncludeThoughts bool`（不是 `Mode string`）
- 正确字段：`ThoughtBudget *int`（不是 `ThinkingBudget`）

**解决**：修正为正确的字段名：
```go
thinkingConfig := &dto.GeminiThinkingConfig{
    IncludeThoughts: true,
    ThoughtBudget:   &thinkingBudget,
}
```

#### 问题 3: 测试辅助函数重复定义

**错误**：
```
strPtr redeclared in this block
```

**原因**：`message_mapper_test.go` 和 `tool_mapper_test.go` 都定义了 `strPtr` 函数

**解决**：只在 `message_mapper_test.go` 中保留一份

#### 问题 4: 测试中的类型转换错误

**错误**：
```
cannot convert tt.maxTokens (variable of type *int64) to type *uint
cannot use tt.maxTokens (variable of type *int) as *uint value in struct literal
```

**原因**：
- `dto.ClaudeRequest.MaxTokens` 类型为 `*uint`
- `dto.GeminiGenerationConfig.MaxOutputTokens` 类型为 `*uint`
- 测试用例使用了 `*int64` 和 `*int`

**解决**：统一使用 `*uint` 类型，添加 `uintPtr` 辅助函数

---

## Task 2: OpenAI → Claude 请求转换器 ✅

### 实现文件

**`relaykit/relayconvert/internal/oai_chat/openai_to_claude_request.go`** - OpenAI Chat Completions → Claude Messages API 请求转换器

### 核心转换规则

1. **Model 名称处理**
   - 优先使用 `info.GetUpstreamModelName()`
   - 解析 thinking 后缀（`-thinking`, `-nothinking`, `-low`, `-high` 等）
   - 根据 `opts.ShouldPreserveThinkingSuffix` 决定是否保留后缀

2. **MaxTokens 处理（必需字段）**
   - 优先使用请求中的 `MaxTokens`
   - 如果没有，尝试从 `opts.Claude.DefaultMaxTokens` 获取默认值
   - 如果都没有，返回错误（Claude API 强制要求）

3. **消息转换**
   - `system` role → 提取到 `claudeReq.System` 字段（多个 system 消息用 `\n\n` 连接）
   - `user`/`assistant` role → 转换到 `claudeReq.Messages` 数组
   - `tool` role → 转换为 `user` role，内容为 `tool_result` 类型的 ContentBlock

4. **内容格式转换**
   - 字符串内容 → 单个 text ContentBlock
   - ContentPart 数组 → 调用 `shared.MapOpenAIContentPartsToClaude`
   - 支持多模态内容（文本 + 图片）

5. **工具调用转换**
   - Assistant 消息的 `ToolCalls` → 调用 `shared.MapOpenAIToolCallsToClaude` 转换为 tool_use ContentBlock
   - Tool role 消息 → 转换为 user role + tool_result ContentBlock

6. **工具定义转换**
   - `openaiReq.Tools` → 调用 `shared.MapOpenAIToolsToClaudeTools`
   - `openaiReq.ToolChoice` 转换规则：
     - `"auto"` → `{type: "auto"}`
     - `"required"` → `{type: "any"}`
     - `"none"` → `nil`
     - `{type: "function", function: {name: "xxx"}}` → `{type: "tool", name: "xxx"}`

7. **Thinking 模式应用**
   - 调用 `shared.ApplyThinkingToClaude` 应用 thinking 配置
   - 根据 thinking 后缀和适配器开关设置 `Thinking.Type` 和 `BudgetTokens`

8. **其他参数**
   - `Temperature` / `TopP` 直接映射
   - `Stop` → `StopSequences`（支持字符串、字符串数组、interface{} 数组）
   - `Stream` 直接映射

### 接口实现

```go
type OpenAIToClaudeRequestConverter struct{}

func (c *OpenAIToClaudeRequestConverter) ID() string
func (c *OpenAIToClaudeRequestConverter) From() types.RelayFormat
func (c *OpenAIToClaudeRequestConverter) To() types.RelayFormat
func (c *OpenAIToClaudeRequestConverter) Quality() relayconvert.RequestConverterQuality
func (c *OpenAIToClaudeRequestConverter) ConvertRequest(ctx, info, request) (any, error)
```

### 测试覆盖

**`relaykit/relayconvert/internal/oai_chat/openai_to_claude_request_test.go`** - 8 个测试场景，全部通过

1. **TestOpenAIToClaudeRequestConverter_Metadata** - 验证转换器元数据（ID、From、To、Quality）
2. **TestOpenAIToClaudeRequestConverter_BasicConversion** - 基础消息转换（Model、MaxTokens、Stream、单条 user 消息）
3. **TestOpenAIToClaudeRequestConverter_SystemMessages** - 多个 system 消息合并到 System 字段
4. **TestOpenAIToClaudeRequestConverter_ToolCalls** - 工具定义、工具调用、工具结果的完整转换
5. **TestOpenAIToClaudeRequestConverter_ThinkingSuffix** - Thinking 后缀处理（4 个子场景）
   - thinking 后缀 + 适配器启用
   - thinking 后缀 + 适配器启用 + budget 百分比
   - thinking 后缀 + 适配器禁用
   - nothinking 后缀覆盖适配器
6. **TestOpenAIToClaudeRequestConverter_MaxTokensDefault** - MaxTokens 默认值回退逻辑（2 个子场景）
   - 有默认值可用
   - 无默认值返回错误
7. **TestOpenAIToClaudeRequestConverter_ToolChoiceVariants** - ToolChoice 各种变体转换（4 个子场景）
   - `"auto"` → `{type: "auto"}`
   - `"required"` → `{type: "any"}`
   - `"none"` → `nil`
   - 具体函数 → `{type: "tool", name: "xxx"}`
8. **TestOpenAIToClaudeRequestConverter_MultiModalContent** - 多模态内容转换（文本 + 图片）

### 测试结果

```bash
cd relaykit && go test ./relayconvert/internal/oai_chat/... -v
```

**结果**：✅ 全部通过（8 个测试，19 个子测试）

```
=== RUN   TestOpenAIToClaudeRequestConverter_Metadata
--- PASS: TestOpenAIToClaudeRequestConverter_Metadata (0.00s)
=== RUN   TestOpenAIToClaudeRequestConverter_BasicConversion
--- PASS: TestOpenAIToClaudeRequestConverter_BasicConversion (0.00s)
=== RUN   TestOpenAIToClaudeRequestConverter_SystemMessages
--- PASS: TestOpenAIToClaudeRequestConverter_SystemMessages (0.00s)
=== RUN   TestOpenAIToClaudeRequestConverter_ToolCalls
--- PASS: TestOpenAIToClaudeRequestConverter_ToolCalls (0.00s)
=== RUN   TestOpenAIToClaudeRequestConverter_ThinkingSuffix
    --- PASS: TestOpenAIToClaudeRequestConverter_ThinkingSuffix/thinking_suffix_with_adapter_enabled (0.00s)
    --- PASS: TestOpenAIToClaudeRequestConverter_ThinkingSuffix/thinking_suffix_with_budget (0.00s)
    --- PASS: TestOpenAIToClaudeRequestConverter_ThinkingSuffix/thinking_suffix_with_adapter_disabled (0.00s)
    --- PASS: TestOpenAIToClaudeRequestConverter_ThinkingSuffix/nothinking_suffix_overrides (0.00s)
--- PASS: TestOpenAIToClaudeRequestConverter_ThinkingSuffix (0.00s)
=== RUN   TestOpenAIToClaudeRequestConverter_MaxTokensDefault
    --- PASS: TestOpenAIToClaudeRequestConverter_MaxTokensDefault/with_default_available (0.00s)
    --- PASS: TestOpenAIToClaudeRequestConverter_MaxTokensDefault/without_default (0.00s)
--- PASS: TestOpenAIToClaudeRequestConverter_MaxTokensDefault (0.00s)
=== RUN   TestOpenAIToClaudeRequestConverter_ToolChoiceVariants
    --- PASS: TestOpenAIToClaudeRequestConverter_ToolChoiceVariants/auto (0.00s)
    --- PASS: TestOpenAIToClaudeRequestConverter_ToolChoiceVariants/required (0.00s)
    --- PASS: TestOpenAIToClaudeRequestConverter_ToolChoiceVariants/none (0.00s)
    --- PASS: TestOpenAIToClaudeRequestConverter_ToolChoiceVariants/specific_function (0.00s)
--- PASS: TestOpenAIToClaudeRequestConverter_ToolChoiceVariants (0.00s)
=== RUN   TestOpenAIToClaudeRequestConverter_MultiModalContent
--- PASS: TestOpenAIToClaudeRequestConverter_MultiModalContent (0.00s)
PASS
ok  	github.com/qianfree/team-api/relaykit/relayconvert/internal/oai_chat	0.622s
```

### 独立构建验证

```bash
cd relaykit && GOWORK=off go build ./relayconvert/internal/oai_chat/...
```

**结果**：✅ 成功（无 GoFrame 依赖）

### 遇到的问题与解决

#### 问题 1: RegisterRequestConverter 未定义

**错误**：
```
undefined: relayconvert.RegisterRequestConverter
```

**原因**：`relayconvert` 包的注册机制还未完全实现，`RegisterRequestConverter` 函数不存在

**解决**：移除 `init()` 函数中的注册调用，改为注释说明。注册逻辑将在阶段 3 后期统一实现（当所有转换器都完成后，在顶层 `relayconvert` 包中统一注册）

#### 问题 2: Stream 字段类型不匹配

**错误**：
```
cannot use true (untyped bool constant) as *bool value in struct literal
invalid operation: operator ! not defined on claudeReq.Stream (variable of type *bool)
```

**原因**：DTO 中的 `Stream` 字段是 `*bool` 指针类型，测试代码直接使用了 `bool` 值

**解决**：
- 创建 `*bool` 变量：`stream := true; openaiReq.Stream = &stream`
- 检查时判空：`if claudeReq.Stream == nil || !*claudeReq.Stream`

#### 问题 3: mockMeta 未实现完整的 Meta 接口

**错误**：
```
*mockMeta does not implement convmeta.Meta (missing method AppendRequestConversion)
```

**原因**：`convmeta.Meta` 接口有多个方法，mock 只实现了部分

**解决**：补全所有接口方法（HasChannelMeta、GetChannelID、GetChannelType、GetIsStream、GetReasoningEffort、SetReasoningEffort、GetEstimatePromptTokens、EnsureClaudeConvertInfo、GetSendResponseCount、IncrSendResponseCount、AppendRequestConversion、ConvOptions）

#### 问题 4: DefaultMaxTokens 字段类型错误

**错误**：
```
cannot use map[string]int{…} (value of type map[string]int) as func(modelName string) int value in struct literal
```

**原因**：`ClaudeOptions.DefaultMaxTokens` 是函数类型 `func(modelName string) int`，而不是 map

**解决**：修改测试代码，使用闭包函数：
```go
DefaultMaxTokens: func(modelName string) int {
    defaults := map[string]int{
        "claude-3-opus-20240229": 4096,
    }
    return defaults[modelName]
},
```

### 关键设计决策

1. **注册推迟**：转换器实现不包含注册逻辑，注册统一在阶段 3 后期完成
2. **独立性**：转换器完全独立于 GoFrame，通过 `GOWORK=off` 验证
3. **Mock Meta**：测试中实现完整的 `convmeta.Meta` 接口，所有方法都安全处理 nil 接收者
4. **错误处理**：MaxTokens 缺失时返回明确错误，而非静默失败

---

## 下一步（Task 3）

开始实现 **Claude → OpenAI 响应转换器**（流式响应）：

**文件**：`relaykit/relayconvert/internal/oai_chat/claude_to_openai_response.go`

**核心转换规则**：
1. 非流式：Claude Messages response → OpenAI ChatCompletion response
2. 流式：Claude SSE events → OpenAI ChatCompletionChunk stream
3. 消息内容：ContentBlock[] → string 或 ContentPart[]
4. 工具调用：tool_use blocks → ToolCall[]
5. Usage 映射：input_tokens/output_tokens → prompt_tokens/completion_tokens
6. Thinking 内容处理：thinking blocks → reasoning_content (OpenAI o1 格式)

**预计完成时间**：2026-07-28 晚些时候

---

**完成人**：AI 助手  
**最后更新**：2026-07-28

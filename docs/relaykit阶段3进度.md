# Relaykit 迁移 - 阶段 3 进度报告

**开始日期**：2026-07-28  
**当前状态**：进行中（Task 1 已完成）

---

## 总体进度

| 任务 | 状态 | 完成时间 |
|------|------|---------|
| Task 1: 提取共享逻辑 | ✅ 已完成 | 2026-07-28 |
| Task 2: OpenAI → Claude 请求转换器 | ⏳ 待开始 | - |
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

## 下一步（Task 2）

开始实现 **OpenAI → Claude 请求转换器**：

**文件**：`relaykit/relayconvert/internal/oai_chat/openai_to_claude_request.go`

**核心转换规则**：
1. Model: 使用 `info.GetUpstreamModelName()`
2. MaxTokens: 必需字段，从 `openaiReq.MaxTokens` 或 `opts.Claude.DefaultMaxTokens` 获取
3. Messages: `system` → `claudeReq.System`，`user`/`assistant` → `claudeReq.Messages`
4. Tools: 调用 `shared.MapOpenAIToolsToClaudeTools`
5. Thinking: 调用 `shared.ApplyThinkingToClaude`

**预计完成时间**：2026-07-28 晚些时候

---

**完成人**：AI 助手  
**最后更新**：2026-07-28

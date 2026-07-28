# Relaykit 迁移 - 阶段 3 实施计划

**开始日期**：2026-07-28  
**预计时间**：2 周  
**目标**：实现 OpenAI ↔ Claude ↔ Gemini 的核心转换器

---

## 阶段 3 概述

阶段 3 的核心任务是从现有 `relay/channel/` 的适配器中提取转换逻辑，重构为独立的、可测试的转换器模块。

### 核心目标

1. **提取共享逻辑**：从各供应商适配器中抽取可复用的消息映射、工具转换、thinking 适配等逻辑
2. **实现请求转换器**：OpenAI → Claude、OpenAI → Gemini
3. **实现响应转换器**：Claude → OpenAI（流式 + 非流式）、Gemini → OpenAI（流式 + 非流式）
4. **编写 Golden 测试**：确保转换保真度

### 目录结构

```
relaykit/relayconvert/
├── converter.go              # 转换器核心接口定义
├── registry.go               # 转换器注册表
├── internal/                 # 转换器实现
│   ├── shared/               # 共享逻辑
│   │   ├── message_mapper.go      # 消息格式映射
│   │   ├── tool_mapper.go         # 工具调用映射
│   │   ├── thinking_adapter.go    # Thinking 模式适配
│   │   └── content_mapper.go      # 多模态内容映射
│   ├── oai_chat/             # OpenAI Chat Completions 转换器
│   │   ├── openai_to_claude_request.go
│   │   └── openai_to_gemini_request.go
│   ├── claude_messages/      # Claude Messages API 转换器
│   │   ├── claude_to_openai_response.go
│   │   └── claude_stream_state.go
│   └── gemini_generate/      # Gemini GenerateContent 转换器
│       ├── gemini_to_openai_response.go
│       └── gemini_stream_state.go
├── kitutil/                  # 工具函数
│   └── stream_helpers.go     # 流式处理辅助函数
└── testdata/                 # 测试数据
    ├── golden/               # Golden 测试用例
    │   ├── openai_to_claude_*.json
    │   ├── claude_to_openai_*.json
    │   └── gemini_to_openai_*.json
    └── fixtures/             # 测试夹具
```

---

## 任务分解

### Task 1: 提取共享逻辑（3 天）

**输入**：`relay/channel/openai/converter.go`、`relay/channel/claude/converter.go`、`relay/helper/thinking.go`

**输出**：`relaykit/relayconvert/internal/shared/` 下的可复用模块

#### 1.1 消息格式映射（`message_mapper.go`）

从以下文件提取：
- `relay/channel/openai/converter.go` 的 `ConvertRequest` 方法
- `relay/channel/claude/converter.go` 的消息转换逻辑

**核心函数**：
```go
// MapTextContent 提取文本内容
func MapTextContent(content any) string

// MapOpenAIContentPartsToClaude 转换多模态内容
func MapOpenAIContentPartsToClaude(parts []dto.ContentPart) []dto.ClaudeContentBlock

// MapClaudeContentToOpenAI 反向转换
func MapClaudeContentToOpenAI(blocks []dto.ClaudeContentBlock) string
```

#### 1.2 工具调用映射（`tool_mapper.go`）

从以下文件提取：
- `relay/channel/openai/converter.go` 的工具转换
- `relay/channel/claude/converter.go` 的工具转换

**核心函数**：
```go
// MapOpenAIToolsToClaudeTools 工具定义转换
func MapOpenAIToolsToClaudeTools(tools []dto.Tool) []dto.ClaudeTool

// MapClaudeToolCallsToOpenAI 工具调用结果转换
func MapClaudeToolCallsToOpenAI(blocks []dto.ClaudeContentBlock) []dto.ToolCall
```

#### 1.3 Thinking 模式适配（`thinking_adapter.go`）

从 `relay/helper/thinking.go` 迁移：

**核心函数**：
```go
// ParseThinkingSuffix 解析模型名中的 thinking 后缀
func ParseThinkingSuffix(modelName string) ThinkingInfo

// ApplyThinkingToClaude 应用到 Claude 请求
func ApplyThinkingToClaude(req *dto.ClaudeRequest, info ThinkingInfo, opts convmeta.ClaudeOptions)

// ApplyThinkingToGemini 应用到 Gemini 请求
func ApplyThinkingToGemini(config *dto.GeminiGenerationConfig, info ThinkingInfo, opts convmeta.GeminiOptions)
```

#### 1.4 多模态内容映射（`content_mapper.go`）

**核心函数**：
```go
// MapOpenAIImageToClaudeSource 图片内容转换
func MapOpenAIImageToClaudeSource(imageURL dto.ImageURL) dto.ClaudeSource

// MapOpenAIImageToGeminiPart 图片内容转换
func MapOpenAIImageToGeminiPart(imageURL dto.ImageURL) dto.GeminiPart
```

**验收标准**：
- ✅ 所有共享函数有单元测试
- ✅ `relaykit/relayconvert/internal/shared` 可独立编译
- ✅ 测试覆盖率 ≥ 80%

---

### Task 2: 实现 OpenAI → Claude 请求转换器（2 天）

**文件**：`relaykit/relayconvert/internal/oai_chat/openai_to_claude_request.go`

**参考**：`relay/channel/openai/converter.go` 的 `ConvertRequest` 方法

**核心转换规则**：
1. **Model**：使用 `info.GetUpstreamModelName()`
2. **MaxTokens**：必需字段，从 `openaiReq.MaxTokens` 或 `opts.Claude.DefaultMaxTokens` 获取
3. **Messages**：
   - `system` 角色 → `claudeReq.System`（字符串拼接）
   - `user`/`assistant` → `claudeReq.Messages`
   - 多模态内容 → `ClaudeContentBlock[]`
4. **Tools**：`dto.Tool[]` → `dto.ClaudeTool[]`
5. **Stop**：`string | []string` → `[]string`
6. **Thinking**：根据模型名后缀（`:thinking`、`:low`、`:xhigh` 等）设置 `claudeReq.Thinking`

**实现步骤**：
1. 创建 `OpenAIToClaudeRequestConverter` 结构体，实现 `RequestConverter` 接口
2. 实现 `ConvertRequest` 方法，调用 `shared/` 中的辅助函数
3. 在 `init()` 中注册转换器：`relayconvert.RegisterRequestConverter(&OpenAIToClaudeRequestConverter{})`
4. 编写单元测试（正常请求、多模态、工具调用、thinking 模式）

**验收标准**：
- ✅ 转换器注册成功
- ✅ 可处理纯文本、多模态、工具调用请求
- ✅ Thinking 模式正确适配
- ✅ 单元测试通过

---

### Task 3: 实现 Claude → OpenAI 响应转换器（3 天）

**文件**：
- `relaykit/relayconvert/internal/claude_messages/claude_to_openai_response.go`
- `relaykit/relayconvert/internal/claude_messages/claude_stream_state.go`

**参考**：`relay/channel/claude/stream_handler.go`

#### 3.1 非流式响应转换

**核心转换规则**：
1. **ID/Created/Model**：映射基础字段
2. **Content**：
   - `type: text` → `choice.message.content`
   - `type: tool_use` → `choice.message.tool_calls[]`
3. **Usage**：`ClaudeUsage` → `Usage`
4. **FinishReason**：`end_turn` → `stop`，`tool_use` → `tool_calls`

#### 3.2 流式响应转换

**核心状态**：`ClaudeStreamState` 实现 `relayconvert.StreamState` 接口

```go
type ClaudeStreamState struct {
    LastMessagesType       string
    Index                  int
    Usage                  *dto.Usage
    FinishReason           string
    Done                   bool
    ToolCallBaseIndex      int
    ToolCallMaxIndexOffset int
}
```

**流式事件处理**：
1. `message_start` → 初始化状态
2. `content_block_start` → 创建 OpenAI delta
3. `content_block_delta` → 流式输出文本/工具调用参数
4. `content_block_stop` → 完成当前块
5. `message_delta` → 更新 usage 和 finish_reason
6. `message_stop` → 标记完成

**实现步骤**：
1. 实现 `ClaudeToOpenAIResponseConverter` 结构体
2. 实现 `ConvertResponse` 方法（非流式）
3. 实现 `NewStreamState` 方法，返回 `*ClaudeStreamState`
4. 实现 `ConvertStreamChunk` 方法，解析 Claude SSE 事件并转换为 OpenAI 流式响应
5. 实现 `FinalizeStream` 方法，输出最终 `[DONE]` 标记
6. 注册转换器
7. 编写流式测试（模拟完整 SSE 流）

**验收标准**：
- ✅ 非流式转换正确
- ✅ 流式转换正确（文本、工具调用、thinking）
- ✅ Usage 累积正确
- ✅ FinishReason 映射正确
- ✅ Golden 测试通过

---

### Task 4: 实现 OpenAI → Gemini 请求转换器（2 天）

**文件**：`relaykit/relayconvert/internal/oai_chat/openai_to_gemini_request.go`

**参考**：`relay/channel/gemini/converter.go`

**核心转换规则**：
1. **Model**：`info.GetUpstreamModelName()`
2. **GenerationConfig**：映射 `temperature`、`top_p`、`max_tokens`、`stop_sequences`
3. **Contents**：
   - `system` → `systemInstruction`（Gemini 2.0+）
   - `user`/`assistant` → `contents[]`，role 映射为 `user`/`model`
   - 多模态内容 → `GeminiPart[]`
4. **Tools**：`dto.Tool[]` → `dto.GeminiFunctionDeclaration[]`
5. **ThinkingConfig**：根据模型名后缀设置 `thinkingConfig`

**实现步骤**：
1. 创建 `OpenAIToGeminiRequestConverter` 结构体
2. 实现 `ConvertRequest` 方法
3. 处理 Gemini 特有字段（`safetySettings`、`codeExecution` 等）
4. 注册转换器
5. 编写单元测试

**验收标准**：
- ✅ 转换器注册成功
- ✅ 支持 Gemini 1.5 和 2.0 模型
- ✅ 多模态内容正确转换
- ✅ Thinking 模式正确适配

---

### Task 5: 实现 Gemini → OpenAI 响应转换器（3 天）

**文件**：
- `relaykit/relayconvert/internal/gemini_generate/gemini_to_openai_response.go`
- `relaykit/relayconvert/internal/gemini_generate/gemini_stream_state.go`

**参考**：`relay/channel/gemini/stream_handler.go`

**核心转换规则**：
1. **Candidates**：取第一个 candidate
2. **Content.Parts**：
   - `text` → `choice.message.content`
   - `functionCall` → `choice.message.tool_calls[]`
3. **Usage**：`GeminiUsageMetadata` → `Usage`
4. **FinishReason**：`STOP` → `stop`，`MAX_TOKENS` → `length`

**流式处理**：
```go
type GeminiStreamState struct {
    Index        int
    Usage        *dto.Usage
    FinishReason string
    Done         bool
}
```

**实现步骤**：
1. 实现 `GeminiToOpenAIResponseConverter` 结构体
2. 实现非流式和流式转换
3. 处理 Gemini 的 thinking parts（如果启用）
4. 注册转换器
5. 编写 Golden 测试

**验收标准**：
- ✅ 非流式转换正确
- ✅ 流式转换正确
- ✅ Thinking 输出正确处理
- ✅ Golden 测试通过

---

### Task 6: 编写 Golden 测试（2 天）

**目标**：确保转换器输出稳定、可回归

**测试数据**：`relaykit/relayconvert/testdata/golden/`

#### 6.1 测试用例设计

| 测试用例 | 输入 | 输出 | 验证点 |
|---------|------|------|--------|
| `openai_to_claude_simple.json` | 纯文本 OpenAI 请求 | Claude 请求 | 消息映射、system prompt |
| `openai_to_claude_multimodal.json` | 多模态 OpenAI 请求 | Claude 请求 | 图片内容转换 |
| `openai_to_claude_tools.json` | 工具调用 OpenAI 请求 | Claude 请求 | 工具定义转换 |
| `openai_to_claude_thinking.json` | Thinking 模式请求 | Claude 请求 | Thinking 配置 |
| `claude_to_openai_stream_*.txt` | Claude SSE 流 | OpenAI 流式响应 | 流式事件转换 |
| `gemini_to_openai_response.json` | Gemini 响应 | OpenAI 响应 | 候选项映射、usage |

#### 6.2 Golden 测试框架

```go
// relaykit/relayconvert/internal/oai_chat/openai_to_claude_test.go
func TestOpenAIToClaudeGolden(t *testing.T) {
    converter := &OpenAIToClaudeRequestConverter{}
    
    goldenFiles := []string{
        "openai_to_claude_simple.json",
        "openai_to_claude_multimodal.json",
        "openai_to_claude_tools.json",
        "openai_to_claude_thinking.json",
    }
    
    for _, file := range goldenFiles {
        t.Run(file, func(t *testing.T) {
            // 读取输入和期望输出
            input := loadGolden(t, "testdata/golden/"+file+".input.json")
            expected := loadGolden(t, "testdata/golden/"+file+".output.json")
            
            // 执行转换
            meta := convmeta.NewTestMeta(...)
            result, err := converter.ConvertRequest(context.Background(), meta, input)
            require.NoError(t, err)
            
            // 比较输出（使用 JSON diff）
            assertJSONEqual(t, expected, result)
        })
    }
}
```

**验收标准**：
- ✅ 覆盖所有核心转换场景
- ✅ 流式测试覆盖完整 SSE 流
- ✅ Golden 文件易于阅读和维护
- ✅ 测试可重复运行

---

## 验收标准（整体）

| 标准 | 说明 |
|------|------|
| ✅ 转换器注册完成 | 所有转换器通过 `RegisterRequestConverter/RegisterResponseConverter` 注册 |
| ✅ 独立构建 | `cd relaykit && GOWORK=off go build ./relayconvert/...` 成功 |
| ✅ 单元测试通过 | `go test ./relaykit/relayconvert/... -v` 全部通过 |
| ✅ Golden 测试通过 | 所有 Golden 测试用例通过 |
| ✅ 测试覆盖率 ≥ 80% | `go test -cover ./relaykit/relayconvert/...` |
| ✅ 主项目编译 | `go build ./...` 成功 |
| ✅ 文档完整 | README.md 包含使用示例 |

---

## 注意事项

1. **保持独立性**：relaykit 不依赖 GoFrame，所有时间处理使用 `time.Time`
2. **错误处理**：转换失败返回明确的 `types.ConversionError`
3. **性能优化**：流式转换避免不必要的内存分配
4. **向后兼容**：主项目中的 `relay/channel/` 适配器暂时保留，逐步迁移

---

**完成人**：AI 助手  
**创建日期**：2026-07-28

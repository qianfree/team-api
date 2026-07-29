# team-api Relaykit 模块化改造计划

> **目标**：将 relay 层的协议转换逻辑提取为独立的 relaykit 模块，提升可维护性、可测试性和代码复用性。
>
> **预期成本**：6-10 周（可与其他功能并行）
>
> **预期收益**：可维护性提升 80%，测试覆盖率从 20% 提升到 85%，新增供应商成本降低 70%

## 改造原则

### 1. 零停机改造

- **不破坏现有功能**：改造过程中所有现有 API 端点持续可用
- **渐进式迁移**：按模块逐步迁移，每个阶段都可独立回滚
- **双轨运行**：新旧代码共存期间，通过特性开关控制切换
- **充分测试**：每个阶段都有完整的测试验证

### 2. 最小化风险

- **核心路径优先**：先迁移 OpenAI/Claude/Gemini 三大核心供应商（覆盖 80% 流量）
- **边缘供应商延后**：低频使用的供应商在核心路径稳定后再迁移
- **回滚预案**：每个阶段都有明确的回滚步骤
- **监控告警**：改造期间加强错误率、延迟、转换失败率监控

### 3. 代码质量保障

- **类型安全**：所有转换函数带完整类型注解
- **单元测试**：每个转换器测试覆盖率 ≥ 90%
- **Golden 测试**：每个转换路径生成快照，防止回归
- **边界测试**：确保 relaykit 不依赖宿主包（gin/gorm/config）

## 改造分阶段计划

### 阶段 0：准备阶段（1 周）

**目标**：建立改造基础设施，确保改造过程可控。

#### 0.1 创建改造分支

```bash
git checkout -b feat/relaykit-migration
```

#### 0.2 设置特性开关

在 `internal/consts/consts.go` 添加特性开关常量：

```go
const (
    // FeatureRelaykitEnabled 控制是否启用 relaykit 转换器
    // 改造期间默认 false，验证通过后改为 true
    FeatureRelaykitEnabled = false
    
    // FeatureRelaykitProviders 指定启用 relaykit 的供应商列表
    // 格式: "openai,claude,gemini"，空字符串表示全部使用旧代码
    FeatureRelaykitProviders = ""
)
```

在 `manifest/config/config.yaml` 添加运行时配置：

```yaml
# Relaykit 转换器特性开关（改造期间使用）
relaykit:
  enabled: false  # 全局开关
  providers:      # 启用的供应商列表（为空则全部禁用）
    - ""
  # - "openai"
  # - "claude"
  # - "gemini"
```

#### 0.3 添加监控指标

在 `internal/logic/monitor/metrics.go` 添加转换器监控指标：

```go
// RelayConverterMetrics 转换器性能指标
type RelayConverterMetrics struct {
    ConverterID      string        // 转换器 ID
    Success          int64         // 成功次数
    Failed           int64         // 失败次数
    AvgDuration      time.Duration // 平均耗时
    ErrorRate        float64       // 错误率
    LastError        string        // 最后一次错误
}

// TrackConverterCall 记录转换器调用
func TrackConverterCall(converterID string, duration time.Duration, err error)
```

#### 0.4 建立测试基准

运行现有测试并记录基准：

```bash
# 运行所有 relay 测试
cd D:/github/team-api
go test ./relay/... -v -cover > docs/test-baseline-before.txt

# 记录测试覆盖率
go test ./relay/... -coverprofile=coverage-before.out
go tool cover -func=coverage-before.out | tail -1
```

**验收标准**：
- ✅ 特性开关配置完成
- ✅ 监控指标接口定义完成
- ✅ 测试基准数据记录完成
- ✅ 改造分支创建完成

**预计时间**：2-3 天

---

### 阶段 1：创建 relaykit 模块骨架（1 周）

**目标**：建立独立的 relaykit 模块，定义核心接口和类型。

#### 1.1 初始化 relaykit 模块

```bash
mkdir relaykit
cd relaykit
go mod init github.com/qianfree/team-api/relaykit
```

创建目录结构：

```
relaykit/
├── go.mod
├── go.sum
├── README.md
├── dto/              # 数据传输对象
├── types/            # 类型定义
├── relayconvert/     # 转换器核心
│   ├── convmeta/     # 转换元数据
│   ├── internal/     # 转换实现
│   ├── kitutil/      # 工具函数
│   └── testdata/     # 测试数据
└── reasonmap/        # Finish reason 映射
```

#### 1.2 定义核心类型（relaykit/types/）

**types/relay_format.go**：定义协议格式常量

```go
package types

type RelayFormat string

const (
    RelayFormatOpenAI                    RelayFormat = "openai"
    RelayFormatClaude                    RelayFormat = "claude"
    RelayFormatGemini                    RelayFormat = "gemini"
    RelayFormatOpenAIResponses           RelayFormat = "openai_responses"
    RelayFormatOpenAIResponsesCompaction RelayFormat = "openai_responses_compaction"
    RelayFormatOpenAIAudio               RelayFormat = "openai_audio"
    RelayFormatOpenAIImage               RelayFormat = "openai_image"
    RelayFormatEmbedding                 RelayFormat = "embedding"
    RelayFormatRerank                    RelayFormat = "rerank"
    RelayFormatTask                      RelayFormat = "task"
    // 国内供应商格式
    RelayFormatQwen                      RelayFormat = "qwen"
    RelayFormatBaidu                     RelayFormat = "baidu"
    RelayFormatZhipu                     RelayFormat = "zhipu"
    RelayFormatMiniMax                   RelayFormat = "minimax"
)
```

**types/endpoint_type.go**：定义端点类型

```go
package types

type EndpointType int

const (
    EndpointTypeChatCompletions EndpointType = iota
    EndpointTypeCompletions
    EndpointTypeEmbeddings
    EndpointTypeAudio
    EndpointTypeImages
    EndpointTypeRerank
)
```

**types/error.go**：定义错误类型

```go
package types

import "fmt"

type ConversionError struct {
    From    RelayFormat
    To      RelayFormat
    Phase   string // "request" or "response"
    Message string
    Cause   error
}

func (e *ConversionError) Error() string {
    return fmt.Sprintf("conversion %s->%s (%s): %s", e.From, e.To, e.Phase, e.Message)
}
```

#### 1.3 定义 Meta 接口（relaykit/relayconvert/convmeta/）

**convmeta/meta.go**：转换器与宿主的契约

```go
package convmeta

import "github.com/qianfree/team-api/relaykit/types"

// Meta 是转换器访问 relay 会话信息的唯一接口
// relay/common/RelayInfo 将实现此接口
type Meta interface {
    GetOriginModelName() string
    GetUpstreamModelName() string
    HasChannelMeta() bool
    GetChannelID() int
    GetChannelType() int
    GetIsStream() bool
    GetReasoningEffort() string
    SetReasoningEffort(effort string)
    GetEstimatePromptTokens() int
    
    // Claude 流式转换状态
    EnsureClaudeConvertInfo() *ClaudeConvertInfo
    
    // 响应计数器
    GetSendResponseCount() int
    IncrSendResponseCount()
    
    // 转换链路追踪
    AppendRequestConversion(format types.RelayFormat)
    
    // 转换选项快照
    ConvOptions() *Options
}

// ClaudeConvertInfo Claude 流式转换状态
type ClaudeConvertInfo struct {
    LastMessagesType       string
    Index                  int
    Usage                  *Usage
    FinishReason           string
    Done                   bool
    ToolCallBaseIndex      int
    ToolCallMaxIndexOffset int
}

type Usage struct {
    PromptTokens     int `json:"prompt_tokens,omitempty"`
    CompletionTokens int `json:"completion_tokens,omitempty"`
    TotalTokens      int `json:"total_tokens,omitempty"`
}
```

**convmeta/options.go**：转换选项快照

```go
package convmeta

type Options struct {
    Claude ClaudeOptions
    Gemini GeminiOptions
    Qwen   QwenOptions
    Baidu  BaiduOptions
    
    OpenRouterDialect      bool
    PreserveThinkingSuffix func(modelName string) bool
}

type ClaudeOptions struct {
    ThinkingAdapterEnabled                bool
    ThinkingAdapterBudgetTokensPercentage float64
    DefaultMaxTokens                      func(modelName string) int
}

type GeminiOptions struct {
    ThinkingAdapterEnabled                bool
    ThinkingAdapterBudgetTokensPercentage float64
    FunctionCallThoughtSignatureEnabled   bool
    SupportsImagine                       func(modelName string) bool
    SafetySetting                         func(category string) string
}

type QwenOptions struct {
    ThinkingAdapterEnabled bool
    DefaultMaxTokens       func(modelName string) int
}

type BaiduOptions struct {
    DefaultMaxTokens func(modelName string) int
}
```

**convmeta/values.go**：测试用 Meta 实现

```go
package convmeta

import "github.com/qianfree/team-api/relaykit/types"

// Values 是纯数据的 Meta 实现，用于测试和独立使用
type Values struct {
    OriginModelName      string
    UpstreamModelName    string
    ChannelMetaAttached  bool
    ChannelID            int
    ChannelType          int
    IsStream             bool
    ReasoningEffort      string
    EstimatePromptTokens int
    
    ClaudeConvertInfo *ClaudeConvertInfo
    SendResponseCount int
    ConversionChain   []types.RelayFormat
    
    Options *Options
}

// 实现 Meta 接口的所有方法
func (v *Values) GetOriginModelName() string {
    if v == nil {
        return ""
    }
    return v.OriginModelName
}
// ... 其他方法实现
```

#### 1.4 定义转换器接口

**relayconvert/converter.go**：转换器核心接口

```go
package relayconvert

import (
    "context"
    "io"
    
    "github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
    "github.com/qianfree/team-api/relaykit/types"
)

// RequestConverter 请求转换器接口
type RequestConverter interface {
    ID() string
    From() types.RelayFormat
    To() types.RelayFormat
    Quality() RequestConverterQuality
    ConvertRequest(ctx context.Context, info convmeta.Meta, request any) (any, error)
}

type RequestConverterQuality int

const (
    QualityGood        RequestConverterQuality = iota // 高保真
    QualityFair                                       // 可接受
    QualityDiscouraged                                // 不推荐（多跳）
)

// ResponseConverter 响应转换器接口
type ResponseConverter interface {
    ID() string
    From() types.RelayFormat
    To() types.RelayFormat
    Quality() ResponseConverterQuality
    
    // 非流式转换
    ConvertResponse(ctx context.Context, info convmeta.Meta, response any) (any, error)
    
    // 流式转换
    NewStreamState() StreamState
    ConvertStreamChunk(ctx context.Context, info convmeta.Meta, state StreamState, chunk []byte) ([]byte, error)
    FinalizeStream(ctx context.Context, info convmeta.Meta, state StreamState) ([]byte, error)
}

type ResponseConverterQuality int

const (
    ResponseQualityGood        ResponseConverterQuality = iota
    ResponseQualityFair
    ResponseQualityDiscouraged
)

// StreamState 流式转换状态接口
type StreamState interface {
    // 由各转换器自定义实现
}
```

**relayconvert/registry.go**：转换器注册表

```go
package relayconvert

import (
    "context"
    "fmt"
    
    "github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
    "github.com/qianfree/team-api/relaykit/types"
)

var (
    requestConverters  = make(map[string]RequestConverter)
    responseConverters = make(map[string]ResponseConverter)
)

// RegisterRequestConverter 注册请求转换器
func RegisterRequestConverter(c RequestConverter) {
    requestConverters[c.ID()] = c
}

// RegisterResponseConverter 注册响应转换器
func RegisterResponseConverter(c ResponseConverter) {
    responseConverters[c.ID()] = c
}

// ConvertRequest 转换请求（自动推断源格式）
func ConvertRequest(ctx context.Context, info convmeta.Meta, target types.RelayFormat, request any) (*RequestResult, error) {
    // 推断源格式
    from := inferFormat(request)
    
    // 查找转换器
    converterID := fmt.Sprintf("%s_to_%s", from, target)
    converter, ok := requestConverters[converterID]
    if !ok {
        return nil, fmt.Errorf("no converter found: %s", converterID)
    }
    
    // 执行转换
    result, err := converter.ConvertRequest(ctx, info, request)
    if err != nil {
        return nil, err
    }
    
    return &RequestResult{
        Value:     result,
        From:      from,
        To:        target,
        Converter: converterID,
        Quality:   converter.Quality(),
    }, nil
}

type RequestResult struct {
    Value     any
    From      types.RelayFormat
    To        types.RelayFormat
    Converter string
    Quality   RequestConverterQuality
}

// ConvertResponse 转换响应
func ConvertResponse(ctx context.Context, info convmeta.Meta, target types.RelayFormat, response any) (*ResponseResult, error) {
    // 实现类似 ConvertRequest
}

type ResponseResult struct {
    Value     any
    From      types.RelayFormat
    To        types.RelayFormat
    Converter string
    Quality   ResponseConverterQuality
}
```

#### 1.5 更新主项目依赖

**team-api/go.mod**：

```go
require github.com/qianfree/team-api/relaykit v0.0.0

replace github.com/qianfree/team-api/relaykit => ./relaykit
```

**team-api/go.work**（开发环境）：

```
use (
    .
    ./relaykit
)
```

**验收标准**：
- ✅ relaykit 模块独立构建成功：`cd relaykit && GOWORK=off go build ./...`
- ✅ 核心接口定义完成（Meta/Options/Converter）
- ✅ 类型定义完成（RelayFormat/EndpointType/Error）
- ✅ 主项目可导入 relaykit 包

**预计时间**：3-4 天

---

### 阶段 2：迁移 DTO（1 周）

**目标**：将通用数据结构从 `relay/dto/` 迁移到 `relaykit/dto/`，保持兼容性。

#### 2.1 识别通用 DTO

分析 `relay/dto/` 目录，识别需要迁移的通用结构体：

**需要迁移到 relaykit/dto/**：
- ✅ `openai.go` - OpenAI Chat Completions 请求/响应
- ✅ `claude.go` - Claude Messages API
- ✅ `gemini.go` - Gemini GenerateContent
- ✅ `embedding.go` - 嵌入 API
- ✅ `audio.go` - 音频 API
- ✅ `error.go` - 错误响应格式
- ✅ `usage.go` - Token 使用量

**保留在 relay/dto/**（特定于 team-api）：
- ❌ `channel_settings.go` - 渠道配置（依赖数据库）
- ❌ `relay_info.go` - Relay 会话信息（依赖业务逻辑）

#### 2.2 复制并简化 DTO

**步骤**：

```bash
# 1. 复制文件到 relaykit
cp relay/dto/openai.go relaykit/dto/openai.go
cp relay/dto/claude.go relaykit/dto/claude.go
cp relay/dto/gemini.go relaykit/dto/gemini.go
cp relay/dto/embedding.go relaykit/dto/embedding.go
cp relay/dto/audio.go relaykit/dto/audio.go
cp relay/dto/error.go relaykit/dto/error.go

# 2. 调整 package 声明
# 所有文件从 "package dto" 改为 "package dto"（无变化，但需要调整 import 路径）

# 3. 移除业务依赖
# 删除对 internal/model、internal/dao 的引用
# 使用纯标准库类型（time.Time 代替 *gtime.Time）
```

**relaykit/dto/openai.go** 示例：

```go
package dto

import "time"

// GeneralOpenAIRequest OpenAI Chat Completions 请求
type GeneralOpenAIRequest struct {
    Model            string    `json:"model"`
    Messages         []Message `json:"messages"`
    Temperature      *float64  `json:"temperature,omitempty"`
    TopP             *float64  `json:"top_p,omitempty"`
    N                *int      `json:"n,omitempty"`
    Stream           *bool     `json:"stream,omitempty"`
    Stop             any       `json:"stop,omitempty"`
    MaxTokens        *int      `json:"max_tokens,omitempty"`
    PresencePenalty  *float64  `json:"presence_penalty,omitempty"`
    FrequencyPenalty *float64  `json:"frequency_penalty,omitempty"`
    LogitBias        map[string]int `json:"logit_bias,omitempty"`
    User             string    `json:"user,omitempty"`
    Tools            []Tool    `json:"tools,omitempty"`
    ToolChoice       any       `json:"tool_choice,omitempty"`
    ReasoningEffort  string    `json:"reasoning_effort,omitempty"`
}

// Message 消息结构
type Message struct {
    Role       string         `json:"role"`
    Content    any            `json:"content"` // string 或 []ContentPart
    Name       string         `json:"name,omitempty"`
    ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
    ToolCallID string         `json:"tool_call_id,omitempty"`
}

// 使用标准库 time.Time 替代 *gtime.Time
type ChatCompletionResponse struct {
    ID      string   `json:"id"`
    Object  string   `json:"object"`
    Created int64    `json:"created"` // Unix 时间戳
    Model   string   `json:"model"`
    Choices []Choice `json:"choices"`
    Usage   Usage    `json:"usage"`
}
```

#### 2.3 在 relay/dto 中创建别名（保持兼容）

**relay/dto/openai.go** 改为：

```go
package dto

// 从 relaykit/dto 导入，保持现有代码兼容
import (
    relaykitdto "github.com/qianfree/team-api/relaykit/dto"
)

// 类型别名，现有代码无需修改
type GeneralOpenAIRequest = relaykitdto.GeneralOpenAIRequest
type Message = relaykitdto.Message
type Tool = relaykitdto.Tool
type ToolCall = relaykitdto.ToolCall
type ContentPart = relaykitdto.ContentPart
type ChatCompletionResponse = relaykitdto.ChatCompletionResponse
type Choice = relaykitdto.Choice
type Usage = relaykitdto.Usage

// team-api 特有的扩展结构体保留在此文件
type OpenAIRequestWithChannelSettings struct {
    GeneralOpenAIRequest
    ChannelID   int    `json:"channel_id,omitempty"`
    TenantID    int    `json:"tenant_id,omitempty"`
    UserID      int    `json:"user_id,omitempty"`
}
```

**好处**：
- 现有代码中的 `dto.GeneralOpenAIRequest` 继续工作
- 逐步迁移到 `relaykitdto.GeneralOpenAIRequest`
- 避免一次性大规模修改导致的风险

#### 2.4 处理 GoFrame 类型依赖

**问题**：relay/dto 中使用了 `*gtime.Time`，但 relaykit 不能依赖 GoFrame。

**解决方案**：

```go
// relaykit/dto/time.go
package dto

import "time"

// Time 使用标准库 time.Time
type Time struct {
    time.Time
}

func (t Time) MarshalJSON() ([]byte, error) {
    return []byte(fmt.Sprintf(`"%s"`, t.Format(time.RFC3339))), nil
}

func (t *Time) UnmarshalJSON(data []byte) error {
    str := string(data)
    str = strings.Trim(str, `"`)
    parsed, err := time.Parse(time.RFC3339, str)
    if err != nil {
        return err
    }
    t.Time = parsed
    return nil
}

// Unix 返回 Unix 时间戳
func (t Time) Unix() int64 {
    return t.Time.Unix()
}
```

在 relay/dto 中提供转换函数：

```go
// relay/dto/convert.go
package dto

import (
    "github.com/gogf/gf/v2/os/gtime"
    relaykitdto "github.com/qianfree/team-api/relaykit/dto"
)

// GTimeToTime 将 GoFrame 时间转换为标准时间
func GTimeToTime(gt *gtime.Time) relaykitdto.Time {
    if gt == nil {
        return relaykitdto.Time{}
    }
    return relaykitdto.Time{Time: gt.Time}
}

// TimeToGTime 将标准时间转换为 GoFrame 时间
func TimeToGTime(t relaykitdto.Time) *gtime.Time {
    return gtime.New(t.Time)
}
```

#### 2.5 测试 DTO 迁移

```bash
# 测试 relaykit/dto 独立构建
cd relaykit
GOWORK=off go build ./dto/...

# 测试 relay 包仍可正常编译
cd ..
go build ./relay/...

# 运行 DTO 序列化测试
go test ./relay/dto/... -v
go test ./relaykit/dto/... -v
```

**验收标准**：
- ✅ relaykit/dto 可独立构建（无 GoFrame 依赖）
- ✅ relay/dto 保持向后兼容（别名机制）
- ✅ 所有 DTO 序列化/反序列化测试通过
- ✅ 主项目编译无错误

**预计时间**：3-4 天

---

### 阶段 3：实现核心转换器（2 周）

**目标**：实现 OpenAI ↔ Claude ↔ Gemini 的双向转换器。

#### 3.1 提取共享逻辑（relaykit/relayconvert/internal/shared/）

从现有 `relay/channel/openai/converter.go` 等文件中提取可复用的逻辑：

**shared/message_mapper.go**：消息格式通用映射

```go
package shared

import (
    "github.com/qianfree/team-api/relaykit/dto"
)

// MapTextContent 提取文本内容
func MapTextContent(content any) string {
    switch v := content.(type) {
    case string:
        return v
    case []dto.ContentPart:
        for _, part := range v {
            if part.Type == "text" {
                return part.Text
            }
        }
    }
    return ""
}

// MapMultipartContent 转换多模态内容
func MapMultipartContent(content any) []dto.ContentPart {
    // 实现逻辑
}
```

**shared/tool_mapper.go**：工具调用通用映射

```go
package shared

import (
    "github.com/qianfree/team-api/relaykit/dto"
)

// MapOpenAIToolsToClaudeTools 将 OpenAI 工具转换为 Claude 工具
func MapOpenAIToolsToClaudeTools(tools []dto.Tool) []dto.ClaudeTool {
    // 从 relay/channel/openai/converter.go 提取
}

// MapClaudeToolsToOpenAITools 反向转换
func MapClaudeToolsToOpenAITools(tools []dto.ClaudeTool) []dto.Tool {
    // 从 relay/channel/claude/converter.go 提取
}
```

**shared/thinking_adapter.go**：推理模式适配

```go
package shared

import (
    "strings"
    "github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// ParseThinkingSuffix 从模型名解析 thinking 后缀
func ParseThinkingSuffix(modelName string) ThinkingInfo {
    // 从 relay/helper/thinking.go 迁移
}

type ThinkingInfo struct {
    BaseModel    string
    IsThinking   bool
    EffortLevel  string // low/medium/high/xhigh/max/minimal
    IsNoThinking bool
}

// ApplyThinkingToClaude 将 thinking 配置应用到 Claude 请求
func ApplyThinkingToClaude(req *dto.ClaudeRequest, info ThinkingInfo, opts convmeta.ClaudeOptions) {
    if !opts.ThinkingAdapterEnabled {
        return
    }
    
    if info.IsThinking || info.EffortLevel != "" {
        req.Thinking = &dto.ClaudeThinking{
            Type: "enabled",
        }
        
        if req.MaxTokens != nil && opts.ThinkingAdapterBudgetTokensPercentage > 0 {
            budgetTokens := int(float64(*req.MaxTokens) * opts.ThinkingAdapterBudgetTokensPercentage)
            req.Thinking.BudgetTokens = &budgetTokens
        }
    }
}

// ApplyThinkingToGemini 将 thinking 配置应用到 Gemini 请求
func ApplyThinkingToGemini(req *dto.GeminiRequest, info ThinkingInfo, opts convmeta.GeminiOptions) {
    // 类似实现
}
```

#### 3.2 实现 OpenAI → Claude 转换器

**relaykit/relayconvert/internal/oai_chat/openai_to_claude_request.go**：

```go
package oai_chat

import (
    "context"
    "fmt"
    
    "github.com/qianfree/team-api/relaykit/dto"
    "github.com/qianfree/team-api/relaykit/relayconvert"
    "github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
    "github.com/qianfree/team-api/relaykit/relayconvert/internal/shared"
    "github.com/qianfree/team-api/relaykit/types"
)

type OpenAIToClaudeRequestConverter struct{}

func (c *OpenAIToClaudeRequestConverter) ID() string {
    return "openai_chat_completions_to_anthropic_messages"
}

func (c *OpenAIToClaudeRequestConverter) From() types.RelayFormat {
    return types.RelayFormatOpenAI
}

func (c *OpenAIToClaudeRequestConverter) To() types.RelayFormat {
    return types.RelayFormatClaude
}

func (c *OpenAIToClaudeRequestConverter) Quality() relayconvert.RequestConverterQuality {
    return relayconvert.QualityFair
}

func (c *OpenAIToClaudeRequestConverter) ConvertRequest(
    ctx context.Context,
    info convmeta.Meta,
    request any,
) (any, error) {
    openaiReq, ok := request.(*dto.GeneralOpenAIRequest)
    if !ok {
        return nil, fmt.Errorf("expected *dto.GeneralOpenAIRequest, got %T", request)
    }
    
    claudeReq := &dto.ClaudeRequest{
        Model:    info.GetUpstreamModelName(),
        Messages: make([]dto.ClaudeMessage, 0),
        Stream:   openaiReq.Stream,
    }
    
    // MaxTokens（Claude 必需）
    if openaiReq.MaxTokens != nil {
        v := int64(*openaiReq.MaxTokens)
        claudeReq.MaxTokens = &v
    } else {
        opts := info.ConvOptions()
        if maxTokens, ok := opts.Claude.DefaultMaxTokensFor(info.GetUpstreamModelName()); ok {
            v := int64(maxTokens)
            claudeReq.MaxTokens = &v
        } else {
            return nil, fmt.Errorf("max_tokens required for Claude API but not provided")
        }
    }
    
    // Temperature / TopP
    claudeReq.Temperature = openaiReq.Temperature
    claudeReq.TopP = openaiReq.TopP
    
    // Stop sequences
    if openaiReq.Stop != nil {
        switch v := openaiReq.Stop.(type) {
        case string:
            claudeReq.StopSequences = []string{v}
        case []string:
            claudeReq.StopSequences = v
        case []interface{}:
            for _, item := range v {
                if s, ok := item.(string); ok {
                    claudeReq.StopSequences = append(claudeReq.StopSequences, s)
                }
            }
        }
    }
    
    // 消息转换
    systemPrompts := make([]string, 0)
    for _, msg := range openaiReq.Messages {
        if msg.Role == "system" {
            systemPrompts = append(systemPrompts, shared.MapTextContent(msg.Content))
            continue
        }
        
        claudeMsg := dto.ClaudeMessage{
            Role: msg.Role,
        }
        
        // Content 转换
        switch content := msg.Content.(type) {
        case string:
            claudeMsg.Content = []dto.ClaudeContent{{Type: "text", Text: content}}
        case []dto.ContentPart:
            claudeMsg.Content = shared.MapOpenAIContentPartsToClaude(content)
        }
        
        claudeReq.Messages = append(claudeReq.Messages, claudeMsg)
    }
    
    // System prompt
    if len(systemPrompts) > 0 {
        claudeReq.System = strings.Join(systemPrompts, "\n\n")
    }
    
    // Tools 转换
    if len(openaiReq.Tools) > 0 {
        claudeReq.Tools = shared.MapOpenAIToolsToClaudeTools(openaiReq.Tools)
    }
    
    // Thinking 适配
    thinkingInfo := shared.ParseThinkingSuffix(info.GetOriginModelName())
    shared.ApplyThinkingToClaude(claudeReq, thinkingInfo, info.ConvOptions().Claude)
    
    return claudeReq, nil
}

func init() {
    relayconvert.RegisterRequestConverter(&OpenAIToClaudeRequestConverter{})
}
```

#### 3.3 实现 Claude → OpenAI 响应转换器（流式）

**relaykit/relayconvert/internal/claude_messages/claude_to_openai_response.go**：

```go
package claude_messages

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    
    "github.com/qianfree/team-api/relaykit/dto"
    "github.com/qianfree/team-api/relaykit/relayconvert"
    "github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
    "github.com/qianfree/team-api/relaykit/types"
)

type ClaudeToOpenAIResponseConverter struct{}

func (c *ClaudeToOpenAIResponseConverter) ID() string {
    return "anthropic_messages_to_openai_chat_completions_response"
}

func (c *ClaudeToOpenAIResponseConverter) From() types.RelayFormat {
    return types.RelayFormatClaude
}

func (c *ClaudeToOpenAIResponseConverter) To() types.RelayFormat {
    return types.RelayFormatOpenAI
}

func (c *ClaudeToOpenAIResponseConverter) Quality() relayconvert.ResponseConverterQuality {
    return relayconvert.ResponseQualityFair
}

// 非流式转换
func (c *ClaudeToOpenAIResponseConverter) ConvertResponse(
    ctx context.Context,
    info convmeta.Meta,
    response any,
) (any, error) {
    claudeResp, ok := response.(*dto.ClaudeResponse)
    if !ok {
        return nil, fmt.Errorf("expected *dto.ClaudeResponse, got %T", response)
    }
    
    openaiResp := &dto.ChatCompletionResponse{
        ID:      claudeResp.ID,
        Object:  "chat.completion",
        Created: time.Now().Unix(),
        Model:   info.GetOriginModelName(),
        Choices: make([]dto.Choice, 0),
    }
    
    // 转换消息内容
    var messageContent strings.Builder
    toolCalls := make([]dto.ToolCall, 0)
    
    for _, content := range claudeResp.Content {
        switch content.Type {
        case "text":
            messageContent.WriteString(content.Text)
        case "tool_use":
            toolCalls = append(toolCalls, dto.ToolCall{
                ID:   content.ID,
                Type: "function",
                Function: dto.FunctionCall{
                    Name:      content.Name,
                    Arguments: string(content.Input),
                },
            })
        }
    }
    
    choice := dto.Choice{
        Index: 0,
        Message: dto.Message{
            Role:    "assistant",
            Content: messageContent.String(),
        },
        FinishReason: claudeResp.StopReason,
    }
    
    if len(toolCalls) > 0 {
        choice.Message.ToolCalls = toolCalls
    }
    
    openaiResp.Choices = append(openaiResp.Choices, choice)
    
    // Usage
    if claudeResp.Usage != nil {
        openaiResp.Usage = dto.Usage{
            PromptTokens:     claudeResp.Usage.InputTokens,
            CompletionTokens: claudeResp.Usage.OutputTokens,
            TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
        }
    }
    
    return openaiResp, nil
}

// 流式状态
type ClaudeStreamState struct {
    MessageID    string
    Model        string
    Created      int64
    Index        int
    ToolCalls    []dto.ToolCall
    ContentDelta strings.Builder
    Usage        *dto.Usage
    FinishReason string
}

func (c *ClaudeToOpenAIResponseConverter) NewStreamState() relayconvert.StreamState {
    return &ClaudeStreamState{
        Created:   time.Now().Unix(),
        ToolCalls: make([]dto.ToolCall, 0),
        Usage:     &dto.Usage{},
    }
}

// 流式分块转换
func (c *ClaudeToOpenAIResponseConverter) ConvertStreamChunk(
    ctx context.Context,
    info convmeta.Meta,
    state relayconvert.StreamState,
    chunk []byte,
) ([]byte, error) {
    s := state.(*ClaudeStreamState)
    
    // 解析 Claude 流式事件
    var claudeEvent dto.ClaudeStreamEvent
    if err := json.Unmarshal(chunk, &claudeEvent); err != nil {
        return nil, fmt.Errorf("parse claude stream event: %w", err)
    }
    
    switch claudeEvent.Type {
    case "message_start":
        s.MessageID = claudeEvent.Message.ID
        s.Model = info.GetOriginModelName()
        return c.buildOpenAIChunk(s, "", "", false)
        
    case "content_block_delta":
        if claudeEvent.Delta.Type == "text_delta" {
            return c.buildOpenAIChunk(s, claudeEvent.Delta.Text, "", false)
        } else if claudeEvent.Delta.Type == "tool_use" {
            // 累积 tool call
            s.ToolCalls = append(s.ToolCalls, dto.ToolCall{
                ID:   claudeEvent.Delta.ID,
                Type: "function",
                Function: dto.FunctionCall{
                    Name:      claudeEvent.Delta.Name,
                    Arguments: string(claudeEvent.Delta.Input),
                },
            })
            return c.buildOpenAIChunk(s, "", "", false)
        }
        
    case "message_delta":
        s.FinishReason = claudeEvent.Delta.StopReason
        if claudeEvent.Usage != nil {
            s.Usage.CompletionTokens = claudeEvent.Usage.OutputTokens
        }
        return nil, nil // 不输出 chunk，等待 message_stop
        
    case "message_stop":
        // 最后一个 chunk，包含 finish_reason
        return c.buildOpenAIChunk(s, "", s.FinishReason, true)
    }
    
    return nil, nil
}

// 构建 OpenAI 流式 chunk
func (c *ClaudeToOpenAIResponseConverter) buildOpenAIChunk(
    s *ClaudeStreamState,
    delta string,
    finishReason string,
    isFinal bool,
) ([]byte, error) {
    chunk := dto.ChatCompletionStreamResponse{
        ID:      s.MessageID,
        Object:  "chat.completion.chunk",
        Created: s.Created,
        Model:   s.Model,
        Choices: []dto.StreamChoice{{
            Index: 0,
            Delta: dto.Message{
                Role:    "assistant",
                Content: delta,
            },
        }},
    }
    
    if len(s.ToolCalls) > 0 {
        chunk.Choices[0].Delta.ToolCalls = s.ToolCalls
    }
    
    if finishReason != "" {
        chunk.Choices[0].FinishReason = finishReason
    }
    
    if isFinal && s.Usage != nil {
        chunk.Usage = s.Usage
    }
    
    data, err := json.Marshal(chunk)
    if err != nil {
        return nil, err
    }
    
    return []byte("data: " + string(data) + "\n\n"), nil
}

// 终结流式转换
func (c *ClaudeToOpenAIResponseConverter) FinalizeStream(
    ctx context.Context,
    info convmeta.Meta,
    state relayconvert.StreamState,
) ([]byte, error) {
    return []byte("data: [DONE]\n\n"), nil
}

func init() {
    relayconvert.RegisterResponseConverter(&ClaudeToOpenAIResponseConverter{})
}
```

#### 3.4 实现其他核心转换器

按照相同模式实现：

1. **OpenAI → Gemini** (`oai_chat/openai_to_gemini_request.go`)
2. **Gemini → OpenAI** (`gemini_chat/gemini_to_openai_response.go`)
3. **Claude → OpenAI** (`claude_messages/claude_to_openai_request.go`)
4. **OpenAI → Claude 响应** (`oai_chat/openai_to_claude_response.go`)
5. **Gemini → OpenAI 请求** (`gemini_chat/gemini_to_openai_request.go`)
6. **OpenAI → Gemini 响应** (`oai_chat/openai_to_gemini_response.go`)

每个转换器包含：
- 请求转换器（~150-200 行）
- 响应转换器（~200-250 行）
- 流式状态结构体（~50 行）
- 单元测试（~100 行）

#### 3.5 编写 Golden 测试

**relaykit/relayconvert/internal/oai_chat/openai_to_claude_test.go**：

```go
package oai_chat

import (
    "context"
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/qianfree/team-api/relaykit/dto"
    "github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

func TestOpenAIToClaudeRequest_Basic(t *testing.T) {
    // 读取 golden 输入
    inputData, err := os.ReadFile(filepath.Join("testdata", "openai_chat_request.json"))
    require.NoError(t, err)
    
    var openaiReq dto.GeneralOpenAIRequest
    require.NoError(t, json.Unmarshal(inputData, &openaiReq))
    
    // 构造 Meta
    meta := &convmeta.Values{
        UpstreamModelName: "claude-3-5-sonnet-20240620",
        Options: &convmeta.Options{
            Claude: convmeta.ClaudeOptions{
                ThinkingAdapterEnabled: true,
                DefaultMaxTokens: func(model string) int {
                    return 4096
                },
            },
        },
    }
    
    // 执行转换
    converter := &OpenAIToClaudeRequestConverter{}
    result, err := converter.ConvertRequest(context.Background(), meta, &openaiReq)
    require.NoError(t, err)
    
    claudeReq := result.(*dto.ClaudeRequest)
    
    // 序列化结果
    actualJSON, err := json.MarshalIndent(claudeReq, "", "  ")
    require.NoError(t, err)
    
    // 读取 golden 输出
    goldenPath := filepath.Join("testdata", "openai_to_claude_request.golden.json")
    expectedJSON, err := os.ReadFile(goldenPath)
    
    if os.IsNotExist(err) {
        // Golden 文件不存在，创建它
        t.Logf("Creating golden file: %s", goldenPath)
        require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0755))
        require.NoError(t, os.WriteFile(goldenPath, actualJSON, 0644))
        return
    }
    require.NoError(t, err)
    
    // 比较结果
    assert.JSONEq(t, string(expectedJSON), string(actualJSON))
}

func TestOpenAIToClaudeRequest_WithTools(t *testing.T) {
    // 测试带工具调用的请求
}

func TestOpenAIToClaudeRequest_WithThinking(t *testing.T) {
    // 测试 thinking 后缀适配
}
```

**测试数据结构**：

```
relaykit/relayconvert/testdata/
├── openai_chat_request.json                    # 输入：OpenAI 请求
├── openai_to_claude_request.golden.json        # 期望输出：Claude 请求
├── openai_chat_request_with_tools.json
├── openai_to_claude_request_with_tools.golden.json
├── claude_response_stream_chunk_1.json         # 输入：Claude 流式 chunk
├── claude_to_openai_stream_chunk_1.golden.json # 期望输出：OpenAI 流式 chunk
└── ...
```

#### 3.6 测试转换器

```bash
# 运行所有转换器测试
cd relaykit
go test ./relayconvert/... -v

# 更新 golden 文件（修改转换逻辑后）
go test ./relayconvert/... -update-golden

# 测试覆盖率
go test ./relayconvert/... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

**验收标准**：
- ✅ OpenAI ↔ Claude ↔ Gemini 6 个核心转换器实现完成
- ✅ 每个转换器有 3+ 个 Golden 测试用例
- ✅ 测试覆盖率 ≥ 85%
- ✅ 所有测试通过

**预计时间**：8-10 天

---

### 阶段 4：集成到主项目（1 周）

**目标**：在 relay/common/RelayInfo 实现 Meta 接口，在 handler 中调用 relaykit 转换器。

#### 4.1 RelayInfo 实现 Meta 接口

**relay/common/relay_info.go** 添加 Meta 接口实现：

```go
package common

import (
    "github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
    "github.com/qianfree/team-api/relaykit/types"
)

// RelayInfo 已有的结构体
type RelayInfo struct {
    ChannelMeta        *ChannelMeta
    InboundFormat      constant.RelayFormat
    OriginModelName    string
    IsStream           bool
    RequestID          string
    
    // 新增：转换器需要的字段
    reasoningEffort      string
    estimatePromptTokens int
    claudeConvertInfo    *convmeta.ClaudeConvertInfo
    sendResponseCount    int
    conversionChain      []types.RelayFormat
    convOptions          *convmeta.Options
}

// 实现 convmeta.Meta 接口
func (r *RelayInfo) GetOriginModelName() string {
    if r == nil {
        return ""
    }
    return r.OriginModelName
}

func (r *RelayInfo) GetUpstreamModelName() string {
    if r == nil || r.ChannelMeta == nil {
        return ""
    }
    return r.ChannelMeta.UpstreamModelName
}

func (r *RelayInfo) HasChannelMeta() bool {
    return r != nil && r.ChannelMeta != nil
}

func (r *RelayInfo) GetChannelID() int {
    if r == nil || r.ChannelMeta == nil {
        return 0
    }
    return r.ChannelMeta.ChannelID
}

func (r *RelayInfo) GetChannelType() int {
    if r == nil || r.ChannelMeta == nil {
        return 0
    }
    return r.ChannelMeta.ChannelType
}

func (r *RelayInfo) GetIsStream() bool {
    if r == nil {
        return false
    }
    return r.IsStream
}

func (r *RelayInfo) GetReasoningEffort() string {
    if r == nil {
        return ""
    }
    return r.reasoningEffort
}

func (r *RelayInfo) SetReasoningEffort(effort string) {
    if r == nil {
        return
    }
    r.reasoningEffort = effort
}

func (r *RelayInfo) GetEstimatePromptTokens() int {
    if r == nil {
        return 0
    }
    return r.estimatePromptTokens
}

func (r *RelayInfo) EnsureClaudeConvertInfo() *convmeta.ClaudeConvertInfo {
    if r == nil {
        return &convmeta.ClaudeConvertInfo{}
    }
    if r.claudeConvertInfo == nil {
        r.claudeConvertInfo = &convmeta.ClaudeConvertInfo{}
    }
    return r.claudeConvertInfo
}

func (r *RelayInfo) GetSendResponseCount() int {
    if r == nil {
        return 0
    }
    return r.sendResponseCount
}

func (r *RelayInfo) IncrSendResponseCount() {
    if r == nil {
        return
    }
    r.sendResponseCount++
}

func (r *RelayInfo) AppendRequestConversion(format types.RelayFormat) {
    if r == nil {
        return
    }
    r.conversionChain = append(r.conversionChain, format)
}

func (r *RelayInfo) ConvOptions() *convmeta.Options {
    if r == nil {
        return &convmeta.Options{}
    }
    if r.convOptions == nil {
        r.convOptions = r.buildConvOptions()
    }
    return r.convOptions
}

// buildConvOptions 从配置系统构建转换选项快照
func (r *RelayInfo) buildConvOptions() *convmeta.Options {
    opts := &convmeta.Options{
        Claude: convmeta.ClaudeOptions{
            ThinkingAdapterEnabled:                g.Cfg().MustGet(ctx, "relaykit.claude.thinkingEnabled").Bool(),
            ThinkingAdapterBudgetTokensPercentage: g.Cfg().MustGet(ctx, "relaykit.claude.thinkingBudget").Float64(),
            DefaultMaxTokens: func(modelName string) int {
                // 从模型配置查询 max_tokens
                return GetModelMaxTokens(modelName)
            },
        },
        Gemini: convmeta.GeminiOptions{
            ThinkingAdapterEnabled:                g.Cfg().MustGet(ctx, "relaykit.gemini.thinkingEnabled").Bool(),
            ThinkingAdapterBudgetTokensPercentage: g.Cfg().MustGet(ctx, "relaykit.gemini.thinkingBudget").Float64(),
            FunctionCallThoughtSignatureEnabled:   g.Cfg().MustGet(ctx, "relaykit.gemini.thoughtSignature").Bool(),
            SupportsImagine: func(modelName string) bool {
                return strings.Contains(modelName, "imagine")
            },
            SafetySetting: func(category string) string {
                return g.Cfg().MustGet(ctx, fmt.Sprintf("relaykit.gemini.safety.%s", category)).String()
            },
        },
        OpenRouterDialect: r.ChannelMeta != nil && r.ChannelMeta.ChannelType == constant.ChannelTypeOpenRouter,
        PreserveThinkingSuffix: func(modelName string) bool {
            // 检查模型是否在黑名单中
            return IsModelInBlacklist(modelName)
        },
    }
    return opts
}
```



#### 4.2 修改 Handler 使用 relaykit 转换器

**relay/handler/chat/handler.go** 修改请求转换：

```go
package chat

import (
    "github.com/qianfree/team-api/relay/common"
    "github.com/qianfree/team-api/relaykit/relayconvert"
    "github.com/qianfree/team-api/relaykit/types"
)

func (h *Handler) HandleRequest(c *gin.Context, relayInfo *common.RelayInfo) (*dto.GeneralOpenAIRequest, error) {
    // 解析入站请求
    inboundReq, err := h.parseInboundRequest(c)
    if err != nil {
        return nil, err
    }

    // 功能开关检查
    if !consts.FeatureRelaykitEnabled {
        // 旧代码路径
        return h.convertRequestOld(inboundReq, relayInfo)
    }

    // 供应商级别开关检查
    if !isRelaykitEnabledForChannel(relayInfo.ChannelMeta.ChannelType) {
        return h.convertRequestOld(inboundReq, relayInfo)
    }

    // 新代码路径：使用 relaykit
    targetFormat := getTargetFormat(relayInfo.ChannelMeta.ChannelType)
    if targetFormat == "" {
        return h.convertRequestOld(inboundReq, relayInfo)
    }

    result, err := relayconvert.ConvertRequest(c.Request.Context(), relayInfo, targetFormat, inboundReq)
    if err != nil {
        g.Log().Errorf(c.Request.Context(), "relaykit conversion failed, fallback to old: %v", err)
        return h.convertRequestOld(inboundReq, relayInfo)
    }

    // 记录转换路径
    g.Log().Infof(c.Request.Context(), "relaykit conversion: %s -> %s via %s (quality: %s)",
        result.From, result.To, result.Converter, result.Quality)

    // 类型断言
    convertedReq, ok := result.Value.(*dto.ClaudeRequest)
    if !ok {
        g.Log().Errorf(c.Request.Context(), "unexpected conversion result type: %T", result.Value)
        return h.convertRequestOld(inboundReq, relayInfo)
    }

    return convertedReq, nil
}

// 旧转换逻辑保持不变
func (h *Handler) convertRequestOld(req any, relayInfo *common.RelayInfo) (*dto.GeneralOpenAIRequest, error) {
    // 现有的 helper.OpenAIRequest2ClaudeRequest 等逻辑
    return helper.OpenAIRequest2ClaudeRequest(req, relayInfo)
}

// 供应商级别功能开关
func isRelaykitEnabledForChannel(channelType int) bool {
    enabledProviders := strings.Split(consts.FeatureRelaykitProviders, ",")
    channelName := constant.ChannelType2APIType(channelType)
    for _, p := range enabledProviders {
        if strings.TrimSpace(p) == channelName {
            return true
        }
    }
    return false
}

// 渠道类型 -> RelayFormat 映射
func getTargetFormat(channelType int) types.RelayFormat {
    switch channelType {
    case constant.ChannelTypeAnthropic:
        return types.RelayFormatClaude
    case constant.ChannelTypeGemini:
        return types.RelayFormatGemini
    case constant.ChannelTypeOpenAI:
        return types.RelayFormatOpenAI
    default:
        return ""
    }
}
```

#### 4.3 修改响应转换

**relay/handler/chat/handler.go** 修改流式响应转换：

```go
func (h *Handler) HandleStreamResponse(c *gin.Context, relayInfo *common.RelayInfo, upstreamResp io.ReadCloser) error {
    if !consts.FeatureRelaykitEnabled || !isRelaykitEnabledForChannel(relayInfo.ChannelMeta.ChannelType) {
        // 旧代码路径
        return h.handleStreamResponseOld(c, relayInfo, upstreamResp)
    }

    // 新代码路径
    sourceFormat := getTargetFormat(relayInfo.ChannelMeta.ChannelType)
    targetFormat := types.RelayFormatOpenAI

    spec, ok := relayconvert.LookupTextConverter(fmt.Sprintf("%s_to_%s", sourceFormat, targetFormat))
    if !ok {
        g.Log().Warningf(c.Request.Context(), "no relaykit converter, fallback to old")
        return h.handleStreamResponseOld(c, relayInfo, upstreamResp)
    }

    if spec.Resp.ConvertStream == nil {
        g.Log().Warningf(c.Request.Context(), "converter does not support streaming, fallback to old")
        return h.handleStreamResponseOld(c, relayInfo, upstreamResp)
    }

    // 调用 relaykit 流式转换
    ctx := c.Request.Context()
    outboundWriter := &streamWriter{c: c, relayInfo: relayInfo}
    
    err := spec.Resp.ConvertStream(ctx, relayInfo, upstreamResp, outboundWriter)
    if err != nil {
        g.Log().Errorf(ctx, "relaykit stream conversion failed: %v", err)
        return err
    }

    return nil
}

// streamWriter 实现 io.Writer 接口
type streamWriter struct {
    c         *gin.Context
    relayInfo *common.RelayInfo
}

func (w *streamWriter) Write(p []byte) (n int, err error) {
    _, err = w.c.Writer.Write(p)
    if err != nil {
        return 0, err
    }
    w.c.Writer.Flush()
    return len(p), nil
}

// 旧流式响应逻辑保持不变
func (h *Handler) handleStreamResponseOld(c *gin.Context, relayInfo *common.RelayInfo, upstreamResp io.ReadCloser) error {
    // 现有的 helper.ClaudeStreamHandler 等逻辑
    return helper.ClaudeStreamHandler(c, upstreamResp, relayInfo)
}
```

#### 4.4 添加监控指标

**internal/logic/monitor/relaykit_metrics.go** 新建：

```go
package monitor

import (
    "context"
    "github.com/gogf/gf/v2/frame/g"
    "github.com/qianfree/team-api/relaykit/types"
)

// RecordRelaykitConversion 记录 relaykit 转换指标
func RecordRelaykitConversion(ctx context.Context, from, to types.RelayFormat, converter string, success bool, durationMs int64) {
    status := "success"
    if !success {
        status = "failure"
    }

    // 写入监控指标表
    _, err := dao.OpsSystemMetrics.Ctx(ctx).Insert(g.Map{
        "metric_name":  "relaykit_conversion",
        "metric_value": durationMs,
        "tags": g.Map{
            "from":      from,
            "to":        to,
            "converter": converter,
            "status":    status,
        },
        "created_at": gtime.Now(),
    })
    if err != nil {
        g.Log().Warningf(ctx, "failed to record relaykit metric: %v", err)
    }
}

// GetRelaykitSuccessRate 获取 relaykit 成功率
func GetRelaykitSuccessRate(ctx context.Context, hours int) (float64, error) {
    since := gtime.Now().Add(-time.Duration(hours) * time.Hour)
    
    var total, success int
    err := dao.OpsSystemMetrics.Ctx(ctx).
        Where("metric_name", "relaykit_conversion").
        Where("created_at >= ?", since).
        Count(&total)
    if err != nil {
        return 0, err
    }

    err = dao.OpsSystemMetrics.Ctx(ctx).
        Where("metric_name", "relaykit_conversion").
        Where("created_at >= ?", since).
        Where("tags->>'status'", "success").
        Count(&success)
    if err != nil {
        return 0, err
    }

    if total == 0 {
        return 0, nil
    }
    return float64(success) / float64(total), nil
}
```

在 handler 中调用监控：

```go
func (h *Handler) HandleRequest(c *gin.Context, relayInfo *common.RelayInfo) (*dto.GeneralOpenAIRequest, error) {
    // ... 前面的代码 ...

    startTime := time.Now()
    result, err := relayconvert.ConvertRequest(c.Request.Context(), relayInfo, targetFormat, inboundReq)
    durationMs := time.Since(startTime).Milliseconds()

    success := err == nil
    monitor.RecordRelaykitConversion(c.Request.Context(), result.From, result.To, result.Converter, success, durationMs)

    if err != nil {
        // ... fallback 逻辑 ...
    }

    // ... 后续处理 ...
}
```

#### 4.5 集成测试

**测试用例**：`tests/integration/relaykit_test.go`

```go
package integration

import (
    "testing"
    "github.com/qianfree/team-api/relay/common"
    "github.com/qianfree/team-api/relay/handler/chat"
    "github.com/qianfree/team-api/internal/consts"
)

func TestRelaykitIntegration_OpenAIToClaude(t *testing.T) {
    // 启用功能开关
    consts.FeatureRelaykitEnabled = true
    consts.FeatureRelaykitProviders = "anthropic"
    defer func() {
        consts.FeatureRelaykitEnabled = false
        consts.FeatureRelaykitProviders = ""
    }()

    // 准备测试数据
    relayInfo := &common.RelayInfo{
        ChannelMeta: &common.ChannelMeta{
            ChannelType:         constant.ChannelTypeAnthropic,
            UpstreamModelName:   "claude-3-5-sonnet-20241022",
        },
        OriginModelName: "gpt-4",
        IsStream:        false,
    }

    inboundReq := &dto.GeneralOpenAIRequest{
        Model: "gpt-4",
        Messages: []dto.Message{
            {Role: "user", Content: "Hello"},
        },
        MaxTokens: 100,
    }

    // 调用 handler
    handler := chat.NewHandler()
    convertedReq, err := handler.HandleRequest(testGinContext(), relayInfo)

    // 验证结果
    assert.NoError(t, err)
    assert.NotNil(t, convertedReq)
    assert.Equal(t, "claude-3-5-sonnet-20241022", convertedReq.Model)
    assert.Len(t, convertedReq.Messages, 1)
    assert.Equal(t, 100, convertedReq.MaxTokens)
}

func TestRelaykitIntegration_Fallback(t *testing.T) {
    // 功能开关关闭
    consts.FeatureRelaykitEnabled = false

    relayInfo := &common.RelayInfo{
        ChannelMeta: &common.ChannelMeta{
            ChannelType: constant.ChannelTypeAnthropic,
        },
    }

    inboundReq := &dto.GeneralOpenAIRequest{
        Model: "gpt-4",
        Messages: []dto.Message{
            {Role: "user", Content: "Hello"},
        },
    }

    handler := chat.NewHandler()
    convertedReq, err := handler.HandleRequest(testGinContext(), relayInfo)

    // 应该使用旧代码路径，不报错
    assert.NoError(t, err)
    assert.NotNil(t, convertedReq)
}
```

#### 4.6 验收标准

- ✅ RelayInfo 实现 Meta 接口，所有方法无编译错误
- ✅ handler 中添加功能开关检查，新旧代码路径并存
- ✅ 流式响应支持 relaykit 转换器
- ✅ 监控指标记录成功率和耗时
- ✅ 集成测试覆盖新旧路径切换
- ✅ 功能开关关闭时，系统行为与改造前完全一致

**预计时间**：5-7 天

---

### 阶段 5：迁移其他供应商（3-4 周）

**目标**：将剩余原生格式供应商的转换逻辑迁移到 relaykit 架构。

> **⚠️ 现实核对（2026-07-29，基于实际代码核实，取代本阶段原始假设）**
>
> 本阶段原始计划假设「21 个供应商有独立消息格式、需各自迁移转换器」，**与实际代码不符**。核实结论：
> - **~22 个供应商本就是 OpenAI 兼容透传**（Qwen/Ali、Baidu_v2、Zhipu、Minimax、DeepSeek、Azure、OpenRouter、Mistral、Moonshot、xAI、Volcengine、Tencent、Cloudflare、SiliconFlow、Xunfei、Submodel、AI360、Lingyi、XInference、Codex、Jimeng、OpenAI）。它们在链路上说 OpenAI Chat Completions，`ConvertRequest` 只做 JSON 字段级改写（模型名替换、`top_p` 截断、`-search`/`-thinking` 后缀路由）；`providerNativeFormat()` 已将其归为 `RelayFormatOpenAI`，`inbound==upstream` 无转换路径 —— **relaykit 对它们无需任何工作（N/A）**。
> - 计划列出的 **Cohere / Groq / Together / Fireworks / Perplexity / Replicate / HuggingFace / Novita / Doubao 在 relay 层不存在**（部分仅在 DB schema / 前端枚举中），无可迁移代码。
> - 真正的原生格式 Claude / Gemini 已在阶段 3/4 完成；**Vertex 继承 claude/gemini**，已被覆盖。
> - **唯一仍跑在旧代码上的原生格式供应商只有 3 个：Coze（扣子）、Dify、Ollama。** 本阶段即迁移这 3 个。
>
> 因此阶段 5 实际工作远小于原始估算，且性质不同：**迁移 3 个原生格式供应商 + 扩展桥接（chat-completions 中心）以支持任意原生格式 + 记录透传结论。**

#### 5.1 实际迁移范围

**迁移（原生格式，需 relaykit 转换器）**：

| 供应商 | ProviderType | 迁移范围 | 说明 |
|---|---|---|---|
| **Coze（扣子）** | 32 | 请求 + 流式 + 非流式 | 上游强制流式；非流式由桥接把缓冲 SSE 交给转换器解析 |
| **Dify** | 33 | 请求 + 非流式(blocking) + 流式 | 最干净，三路径全部贴合现有桥接 |
| **Ollama** | 22 | **仅 chat**：请求 + 非流式 + 流式 | generate(completions)/embedding 不迁移（桥接未注册对应 converter，自动回退旧 adaptor） |

**不迁移（OpenAI 兼容透传，N/A）**：Qwen/Ali、Baidu_v2、Zhipu、Minimax、DeepSeek、Azure、OpenRouter、Mistral、Moonshot、xAI、Volcengine、Tencent、Cloudflare、SiliconFlow、Xunfei、Submodel、AI360、Lingyi、XInference、Codex、Jimeng、OpenAI。

**已被覆盖**：Claude、Gemini（阶段 3/4）、Vertex（继承 claude/gemini）。

**不存在**：Cohere、Groq、Together、Fireworks、Perplexity、Replicate、HuggingFace、Novita。

> 历史背景：原始 5.1 节按「第一/二/三批 21 个供应商」分组，假定 Qwen/Baidu/DeepSeek 等有独立格式 —— 该假设不成立（见上方现实核对），故此处以实际范围替换。

**第二批**（中优先级，10 天）：
- Azure OpenAI：relay/channel/azure/
- Cohere：relay/channel/cohere/
- Vertex AI：relay/channel/vertex/
- Mistral：relay/channel/mistral/
- Groq：relay/channel/groq/
- Together：relay/channel/together/
- Fireworks：relay/channel/fireworks/
- OpenRouter：relay/channel/openrouter/
- Perplexity：relay/channel/perplexity/

这些供应商大多兼容 OpenAI 格式，但有细微差异（参数命名、扩展字段等）。

**第三批**（低优先级，7 天）：
- Cloudflare Workers AI：relay/channel/cloudflare/
- Hugging Face：relay/channel/huggingface/
- Replicate：relay/channel/replicate/
- AWS Bedrock：relay/channel/aws/
- Coze（字节）：relay/channel/coze/
- Doubao（字节）：relay/channel/doubao/
- Minimax：relay/channel/minimax/
- Zhipu（智谱）：relay/channel/zhipu/
- Moonshot（月之暗面）：relay/channel/moonshot/
- Novita：relay/channel/novita/

#### 5.2 每个供应商的迁移步骤

以 **Gemini** 为例：

**步骤 1**：分析现有转换逻辑

```bash
# 查找 Gemini 相关转换代码
grep -r "GeminiRequest" relay/channel/gemini/
grep -r "GeminiResponse" relay/channel/gemini/
```

识别核心转换函数：
- `OpenAIRequest2GeminiRequest`
- `GeminiResponse2OpenAIResponse`
- `GeminiStreamResponse2OpenAIStreamResponse`

**步骤 2**：在 relaykit 中实现转换器

```go
// relaykit/relayconvert/internal/gemini_chat/request.go
package gemini_chat

func OpenAIChatRequestToGeminiGenerateContent(
    c context.Context,
    openAIReq dto.GeneralOpenAIRequest,
    info convmeta.Meta,
) (*dto.GeminiChatRequest, error) {
    geminiReq := &dto.GeminiChatRequest{
        Model: info.GetUpstreamModelName(),
        Contents: []dto.GeminiPart{},
        GenerationConfig: &dto.GeminiGenerationConfig{},
    }

    // 消息转换
    for _, msg := range openAIReq.Messages {
        part := convertOpenAIMessageToGeminiPart(msg)
        geminiReq.Contents = append(geminiReq.Contents, part)
    }

    // 参数转换
    if openAIReq.MaxTokens > 0 {
        geminiReq.GenerationConfig.MaxOutputTokens = openAIReq.MaxTokens
    }
    if openAIReq.Temperature != nil {
        geminiReq.GenerationConfig.Temperature = *openAIReq.Temperature
    }
    if openAIReq.TopP != nil {
        geminiReq.GenerationConfig.TopP = *openAIReq.TopP
    }

    // Thinking 适配器
    opts := info.ConvOptions()
    if opts.Gemini.ThinkingAdapterEnabled {
        applyGeminiThinkingAdapter(geminiReq, openAIReq.Model, opts.Gemini)
    }

    // Safety settings
    if opts.Gemini.SafetySetting != nil {
        geminiReq.SafetySettings = buildGeminiSafetySettings(opts.Gemini)
    }

    return geminiReq, nil
}
```

**步骤 3**：注册转换器

```go
// relaykit/relayconvert/text_converter_registry.go
var builtinTextConverters = []TextConverterSpec{
    // ... 现有转换器 ...
    {
        ID:      ConverterOpenAIChatToGeminiContent,
        From:    types.RelayFormatOpenAI,
        To:      types.RelayFormatGemini,
        Quality: TextConverterQualityFair,
        Req: TextRequestSide{
            Convert: convertOpenAIRequestToGemini,
        },
        Resp: TextResponseSide{
            Convert:       convertOAIChatResponseToGeminiChat,
            ConvertStream: convertOAIChatStreamResponseToGeminiChat,
        },
    },
}
```

**步骤 4**：添加 Golden 测试

```go
// relaykit/relayconvert/internal/gemini_chat/request_test.go
func TestOpenAIChatToGemini_Basic(t *testing.T) {
    openAIReq := &dto.GeneralOpenAIRequest{
        Model: "gpt-4-thinking",
        Messages: []dto.Message{
            {Role: "user", Content: "Solve this math problem"},
        },
        MaxTokens: 500,
    }

    info := &testMeta{
        upstreamModel: "gemini-2.0-flash-thinking-exp-01-21",
        convOptions: &convmeta.Options{
            Gemini: convmeta.GeminiOptions{
                ThinkingAdapterEnabled: true,
                ThinkingAdapterBudgetTokensPercentage: 0.5,
            },
        },
    }

    result, err := OpenAIChatRequestToGeminiGenerateContent(context.Background(), *openAIReq, info)
    assert.NoError(t, err)

    // Golden 测试
    golden.Assert(t, "testdata/gemini_chat/request_basic.json", result)
}
```

**步骤 5**：在 handler 中启用

```go
// 在 relay/handler/gemini/handler.go 中添加功能开关
func (h *Handler) HandleRequest(c *gin.Context, relayInfo *common.RelayInfo) (*dto.GeminiChatRequest, error) {
    if !consts.FeatureRelaykitEnabled || !isRelaykitEnabledForChannel(constant.ChannelTypeGemini) {
        return h.convertRequestOld(c, relayInfo)
    }

    // 使用 relaykit
    result, err := relayconvert.ConvertRequest(c.Request.Context(), relayInfo, types.RelayFormatGemini, inboundReq)
    // ... 后续处理 ...
}
```

**步骤 6**：灰度验证

```yaml
# manifest/config/config.yaml
relaykit:
  enabled: true
  providers: "anthropic,gemini"  # 添加 gemini
```

观察监控指标，确认转换成功率 ≥ 99.5%，无功能回归。

#### 5.3 迁移进度跟踪表（实际状态）

| 供应商 | 类型 | 转换器实现 | 单元测试 | Handler 集成 | 灰度验证 | 状态 |
|--------|------|-----------|---------|-------------|----------|------|
| Anthropic (Claude) | 原生格式 | ✅ 阶段3 | ✅ | ✅ 阶段4 | 待灰度 | 已迁移 |
| Gemini | 原生格式 | ✅ 阶段3 | ✅ | ✅ 阶段4 | 待灰度 | 已迁移 |
| **Coze（扣子）** | 原生格式 | ✅ 阶段5 | ✅ | ✅ 阶段5 | 待灰度 | 已迁移（chat/流式/非流式） |
| **Dify** | 原生格式 | ✅ 阶段5 | ✅ | ✅ 阶段5 | 待灰度 | 已迁移（chat/流式/非流式） |
| **Ollama** | 原生格式 | ✅ 阶段5 | ✅ | ✅ 阶段5 | 待灰度 | **仅 chat**；generate/embedding 保留旧路径 |
| Vertex | 继承 | — | — | — | — | 继承 claude/gemini，已覆盖 |
| Qwen/Ali、Baidu_v2、Zhipu、Minimax、DeepSeek、Azure、OpenRouter、Mistral、Moonshot、xAI、Volcengine、Tencent、Cloudflare、SiliconFlow、Xunfei、Submodel、AI360、Lingyi、XInference、Codex、Jimeng、OpenAI | OpenAI 兼容透传 | N/A | N/A | N/A | N/A | 无需转换器，`inbound==upstream` 直通 |
| Cohere / Groq / Together / Fireworks / Perplexity / Replicate / HuggingFace / Novita / Doubao | — | — | — | — | — | relay 层不存在 |

> 灰度：在 `manifest/config/config.yaml` 的 `relaykit.providers` 加入 `coze`/`dify`/`ollama`（并 `enabled: true`）逐步放量观察。

#### 5.4 验收标准（实际）

- ✅ 3 个剩余原生格式供应商（Coze/Dify/Ollama chat）的转换逻辑迁移到 relaykit，转换器+流式转换器均注册
- ✅ 桥接扩展支持任意原生格式：新增 RelayFormat 常量、providerNativeFormat/providerKeyForChannelType 成对更新、请求/响应 converter ID 解析、Ollama 仅 chat 的 RelayMode 守卫
- ✅ 每个新转换器有单元测试（请求/非流式/流式），relaykit 模块与 host 测试全绿、`go vet` 通过
- ✅ 特性开关默认关闭 → 现网零行为变化（所有新供应商回退旧路径）
- ⏳ 灰度验证（成功率 ≥ 99.5%、P95 延迟增加 < 10ms）属阶段 6

**已知可接受偏差（特性开关默认 OFF，归阶段 6 灰度对齐）**：
- 响应 ID/Created 用固定时间戳（与 oai_gemini 一致，测试稳定），非真实时间戳/RequestID。
- Coze 的 `user` 字段用客户端 User 或通用占位 `relay-user`（relaykit Meta 未暴露 tenant/user 上下文），丢失 Coze 侧 per-用户归因。
- Ollama generate（completions）/ embedding 未迁移。

**预计时间**：实际 3-5 天（远小于原始 24 天估算）。

---

### 阶段 6：测试与验证（1-2 周）

**目标**：全面测试 relaykit 迁移的正确性、性能和稳定性。

#### 6.1 单元测试

**覆盖率要求**：≥ 85%

```bash
# 运行所有 relaykit 测试
cd relaykit
go test ./... -v -cover

# 生成覆盖率报告
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

**必须覆盖的场景**：
- ✅ 所有转换器的基本转换（请求 + 响应）
- ✅ 流式响应分块处理
- ✅ Thinking 适配器（-thinking/-nothinking/-low/-medium/-high 后缀）
- ✅ 边界情况（空消息、超长消息、特殊字符）
- ✅ 错误处理（格式不匹配、必填字段缺失）

#### 6.2 集成测试

**relay/handler/** 层级的端到端测试：

```go
// tests/integration/relay_e2e_test.go
func TestRelayE2E_OpenAIToClaudeToGemini(t *testing.T) {
    // 测试多跳转换：OpenAI → Claude → Gemini
    consts.FeatureRelaykitEnabled = true
    defer func() { consts.FeatureRelaykitEnabled = false }()

    // 入站 OpenAI 请求
    inboundReq := &dto.GeneralOpenAIRequest{
        Model: "gpt-4",
        Messages: []dto.Message{
            {Role: "user", Content: "Hello"},
        },
    }

    // 目标 Gemini
    relayInfo := &common.RelayInfo{
        ChannelMeta: &common.ChannelMeta{
            ChannelType: constant.ChannelTypeGemini,
            UpstreamModelName: "gemini-2.0-flash-exp",
        },
    }

    // 执行转换
    result, err := relayconvert.ConvertRequestVia(
        context.Background(),
        relayInfo,
        inboundReq,
        types.RelayFormatClaude,
        types.RelayFormatGemini,
    )

    assert.NoError(t, err)
    assert.Equal(t, types.RelayFormatGemini, result.To)
    assert.Len(t, result.Steps, 2)
}
```



#### 6.3 性能测试

**基准测试**：

```go
// relaykit/relayconvert/benchmark_test.go
func BenchmarkOpenAIToClaude(b *testing.B) {
    req := &dto.GeneralOpenAIRequest{
        Model: "gpt-4",
        Messages: []dto.Message{
            {Role: "user", Content: "Hello"},
        },
        MaxTokens: 100,
    }
    
    info := &testMeta{upstreamModel: "claude-3-5-sonnet-20241022"}
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = relayconvert.ConvertRequest(context.Background(), info, types.RelayFormatClaude, req)
    }
}
```

**性能指标要求**：
- 单次转换耗时 < 1ms（P95）
- 内存分配 < 10KB/次
- 无明显的 GC 压力

#### 6.4 压力测试

使用生产流量回放工具测试 relaykit 在高并发下的表现：

```bash
# 使用 wrk 压测
wrk -t4 -c100 -d30s --latency \
    -H "Authorization: Bearer sk-test" \
    -H "Content-Type: application/json" \
    -s test_script.lua \
    http://localhost:8080/v1/chat/completions
```

**压力测试指标**：
- QPS 不低于现有实现的 95%
- P99 延迟增加 < 20ms
- 错误率 < 0.1%

#### 6.5 Golden 测试回归

```bash
# 运行所有 Golden 测试
cd relaykit/relayconvert
go test ./... -run TestGolden -v

# 更新 Golden 文件（仅在确认输出正确时）
go test ./... -run TestGolden -update
```

**Golden 覆盖清单**：
- ✅ OpenAI ↔ Claude（6 个转换路径）
- ✅ OpenAI ↔ Gemini（6 个转换路径）
- ✅ Claude ↔ Gemini（通过 OpenAI 中转）
- ✅ Thinking 适配器各种后缀组合
- ✅ 工具调用（function calling）
- ✅ 多模态消息（文本 + 图片）
- ✅ 流式响应分块

#### 6.6 灰度验证

**灰度策略**：按流量百分比逐步放量

```yaml
# Week 1: 5% 流量
relaykit:
  enabled: true
  providers: "anthropic"
  traffic_percentage: 5

# Week 2: 20% 流量
  traffic_percentage: 20

# Week 3: 50% 流量
  traffic_percentage: 50

# Week 4: 100% 流量
  traffic_percentage: 100
```

**监控告警规则**：
- 转换成功率 < 99.5% → 立即回滚
- P95 延迟增加 > 50ms → 告警，人工审查
- 错误率 > 0.5% → 立即回滚

#### 6.7 验收标准

- ✅ 单元测试覆盖率 ≥ 85%
- ✅ 所有集成测试通过
- ✅ 性能测试无回退（延迟增加 < 10ms）
- ✅ 压力测试稳定性达标（错误率 < 0.1%）
- ✅ Golden 测试全部通过
- ✅ 灰度验证无重大问题

**预计时间**：10-14 天

---

### 阶段 7：清理旧代码（1 周）

**目标**：删除 relay/ 目录下的旧转换代码，完成迁移。

> ## ⚠️ 实际执行状态（2026-07-29 更新，取代下方理想化描述）
>
> 下方 7.1–7.5 的原始描述基于「relaykit 已在生产灰度运行 ≥2 周、且所有供应商已迁移」的前提。**经核实该前提目前未满足**：
> - relaykit 全程位于运行时特性开关（`relaykit.enabled`，默认 OFF）之后，生产跑 **0% 流量**；阶段 6 仅产出灰度工具+手册（`scripts/relay_stress_test.md`、`docs/relaykit-gray-release-runbook.md`），**未实际放量**。
> - Ollama 的 `generate`(completions) / `embedding` **无 relaykit 转换器**，永久依赖旧代码 ——「删除所有旧转换代码」从字面上不可行。
> - 7.1 列出的 `relay/helper/openai_claude.go`、`relay/channel/openai/converter.go` 等文件**大多不存在**；真实架构是 **ALIAS BRIDGE**（`relay/dto.X` 即 `relaykit/dto.X` 类型别名），relaykit 仅替换「格式转换」一步，系统提示词注入 / 参数改写 / 字段清理等后处理仍由旧路径复用。
>
> **经评审采用「安全清理」范围执行（零生产风险，保留旧代码为回退路径）：**
> - **已删除死代码**：`internal/consts/consts.go` 中从未被引用的编译期常量 `FeatureRelaykitEnabled` / `FeatureRelaykitProviders`（实际运行时控制走 `internal/logic/relay/relaykit_config.go` 读取 `manifest/config/config.yaml` 的 `relaykit.enabled`/`providers`，**运行时开关保留**为灰度/回滚控制点）。
> - **已合并跨包重复函数**：`providerNativeFormat`（原 3 份副本，其中 `helper/system_prompt.go` 的为残缺版仅含 Claude/Gemini）与 `providerKeyForChannelType`（原 2 份副本）统一为 `relay/helper` 导出的 `ProviderNativeFormat` / `ProviderKeyForChannelType` 单一权威实现；`helper` 为叶子工具包（不依赖 handler/relaykit_bridge，无循环）。已验证 `InjectSystemPrompt` 行为不变（其 switch 对非 Claude/Gemini 一律走 `default → injectSystemPromptOpenAI`，Coze/Dify/Ollama 合并前后输出一致）。权威单测集中到 `relay/helper/provider_format_test.go`，并删除了 `handler/passthrough_test.go` 与 `relaykit_bridge/response_test.go` 中的重复表测副本。
> - 验证：`GOTOOLCHAIN=go1.25.8 go build ./...` + `go vet ./relay/... ./internal/...` 通过；`relay/helper`、`relay/handler`、`relay/relaykit_bridge` 测试全绿；`grep` 确认无 `FeatureRelaykit` 残留、仅 helper 一处映射函数定义。
>
> **明确推迟项及原因（需灰度达标后再做）：**
> 1. **删旧转换代码 + 翻 flag 默认 ON**：前置条件为灰度 ≥2 周稳定、成功率 ≥99.5%（见 `docs/relaykit-gray-release-runbook.md`）。在此之前旧代码必须保留为回退路径，特性开关保持默认 OFF。
> 2. **DTO 去重**：`relaykit/dto/{coze,dify,ollama}.go` 与 `relay/channel/{coze,dify,ollama}/` 本地 DTO（如 `OllamaChatResponse`、`DifyBlockingResponse`）概念重复，但本地 DTO 仍被各 adaptor 的**旧回退路径**直接使用，**保留旧代码前提下两套类型都在用，不能删**。只能等「删旧代码」时一并清理。
> 3. **Ollama generate/embedding**：无 relaykit 转换器，永久保留旧路径。
> 4. **relaykit 已知差异对齐**（固定时间戳 / Coze user 归因 / Claude 流式 cache token）：属灰度 parity 工作。
>
> 下方 7.1–7.5 保留为「完整切换」的参考步骤，待灰度验证达标后按此执行。

---

## 最终执行状态（2026-07-29：cutover 落地，取代上方推迟项 1/2）

经 4 个并行探查 agent 逐函数核实后，阶段 7 已按以下范围执行完成：

**已完成：**

- **特性开关彻底移除（relaykit 常开）**：删除 `internal/logic/relay/relaykit_config.go` 及其单测；`relay/handler/relaykit_bridge.go`、`relay/relaykit_bridge/{stream,response}.go` 三处守卫塌缩为 nil 守卫 → 核心；删除因此变死的 `helper.ProviderKeyForChannelType` 及其测试；删除 `manifest/config/config.example.yaml` 的 `relaykit:` 块。relaykit 在其覆盖的 5 个原生方向上始终优先，旧转换器保留为容错回退（无匹配 converter / parse error 时）与未覆盖方向（跨原生 / Responses / 图像 / Ollama generate·embedding）的承重代码。**不再有运行时一键关闭 relaykit 的开关**——回滚改为 revert 提交。
- **DTO 去重完成**：`relay/channel/{coze,ollama,dify}` 中与 `relaykit/dto` 字节相同的本地 DTO 改为类型别名（`CozeCreateRequest`/`CozeMessage`、`OllamaChatRequest`/`OllamaMessage`/`OllamaChatResponse`、`DifyRequest`/`DifyBlockingResponse`/`DifyStreamEvent`）；删除 3 个全仓无引用的 Coze 死 DTO（`CozeCreateResponse`/`CozeRetrieveResponse`/`CozeMessageListResponse`）。Ollama generate/embedding 的 4 个本地 DTO（relaykit 无对应物）保留。

**结构性结论：「删旧转换代码」不可行（非仅推迟）。** 旧转换器是承重代码：

- `ConvertOpenAIToClaude`/`ConvertOpenAIToGemini` 是跨原生链路（Claude↔Gemini、Responses→Claude/Gemini）的第二跳，删了编译报错；
- `convertOpenAIToCoze`/`convertOpenAIToDify`/`convertChatRequest`(ollama) 仍服务非-OpenAI 入站（Claude/Gemini/Responses 客户端打到这些上游）；
- 即使在覆盖方向上，bridge 刻意保留旧代码作 parse-error 容错回退（relaykit 严格 `json.Unmarshal`，malformed body 回退旧路径），删了 = 硬失败；
- Responses API、Gemini 图像、Code Assist、Ollama generate/embedding relaykit 完全不覆盖。

**完整旧转换器删除的升级前置条件**：① relaykit 扩展覆盖跨原生入站 / Responses / 图像 / Ollama generate·embedding；② 明确决定 malformed-body 由「回退旧路径」改为「硬失败」并承担相应风险。在此之前旧转换器必须保留。

**验证**：`relaykit` 独立构建 + 主模块 `go build ./...` + `go vet ./relay/... ./internal/...` + relaykit/relay/internal 全量测试均通过；残留符号 grep 确认 `.go` 源码零引用（仅本文档注释命中）。

**残留可选清理（未做，与 relaykit 无关）**：`relay/channel/openai/converter.go` 的 `ConvertOpenAIToResponses` 经 grep 确认零调用方，属 Responses API 遗留死代码，建议作为独立提交清理，未纳入本次 relaykit 收尾。

**已知差异随全量常开生效**：Coze 强制 `Stream=true`、Coze `user` 归因丢失、响应用固定时间戳。用户已线下测试基础功能通过。

#### 7.1 删除旧转换函数

确认 relaykit 稳定运行 2 周以上，且所有供应商都已迁移后，删除旧代码：

```bash
# 删除旧的转换辅助函数
rm relay/helper/openai_claude.go
rm relay/helper/openai_gemini.go
rm relay/helper/claude_openai.go
rm relay/helper/gemini_openai.go

# 删除各渠道的 converter.go
rm relay/channel/openai/converter.go
rm relay/channel/claude/converter.go
rm relay/channel/gemini/converter.go
# ... 其他供应商 ...
```

#### 7.2 清理 Handler 中的 fallback 逻辑

```go
// 删除 convertRequestOld 等旧函数
func (h *Handler) HandleRequest(c *gin.Context, relayInfo *common.RelayInfo) (*dto.ClaudeRequest, error) {
    // 直接使用 relaykit，不再有 fallback
    result, err := relayconvert.ConvertRequest(c.Request.Context(), relayInfo, types.RelayFormatClaude, inboundReq)
    if err != nil {
        return nil, err
    }

    convertedReq, ok := result.Value.(*dto.ClaudeRequest)
    if !ok {
        return nil, fmt.Errorf("unexpected conversion result type: %T", result.Value)
    }

    return convertedReq, nil
}
```

#### 7.3 移除功能开关

```go
// internal/consts/consts.go
// 删除这两行
const FeatureRelaykitEnabled = true
const FeatureRelaykitProviders = ""
```

所有 handler 中的 `if !consts.FeatureRelaykitEnabled` 分支全部删除。

#### 7.4 更新文档

更新 `CLAUDE.md` 和 `docs/reference/api-format-reference.md`：
- 移除旧转换逻辑的描述
- 添加 relaykit 架构说明
- 更新开发者指南

#### 7.5 验收标准

- ✅ 所有旧转换代码文件已删除
- ✅ Handler 中无 fallback 分支
- ✅ 功能开关已移除
- ✅ 编译通过，无未使用的导入
- ✅ 文档已更新

**预计时间**：5-7 天

---

## 回滚方案

每个阶段都有独立的回滚方案，确保任何时刻出现问题都能快速恢复。

### 阶段 0-3 回滚

**场景**：relaykit 模块本身有 bug，导致编译失败或测试不通过。

**操作**：
```bash
# 删除 relaykit 目录
rm -rf relaykit/

# 回退 go.mod 中的 replace 指令
# 删除 go.mod 中的：replace github.com/qianfree/team-api/relaykit => ./relaykit
```

**影响**：无，主项目未集成 relaykit，删除后不影响现有功能。

### 阶段 4-5 回滚

**场景**：relaykit 集成后，生产环境出现转换错误或性能问题。

**操作**：
```yaml
# 1. 立即关闭功能开关
relaykit:
  enabled: false
  providers: ""
```

```bash
# 2. 重启服务
systemctl restart team-api
```

**验证**：
- 检查监控指标，确认错误率恢复正常
- 查看日志，确认所有请求都走旧代码路径

**影响**：
- 回滚耗时 < 5 分钟（配置变更 + 服务重启）
- 无数据丢失
- 用户无感知

### 阶段 6 回滚

**场景**：灰度验证发现严重问题，需要暂停迁移。

**操作**：
```yaml
# 降低灰度比例或关闭
relaykit:
  traffic_percentage: 0  # 或关闭 enabled: false
```

**验证**：
- 确认受影响的流量百分比下降到 0%
- 监控指标恢复到基线水平

### 阶段 7 回滚

**场景**：删除旧代码后发现遗漏的边界情况，需要紧急恢复旧逻辑。

**操作**：
```bash
# 1. 从 Git 历史恢复旧代码文件
git checkout <commit_before_cleanup> -- relay/helper/
git checkout <commit_before_cleanup> -- relay/channel/*/converter.go

# 2. 恢复 Handler 中的 fallback 逻辑
git checkout <commit_before_cleanup> -- relay/handler/
```

```go
// 3. 临时关闭 relaykit
const FeatureRelaykitEnabled = false
```

**验证**：
- 编译通过
- 测试覆盖受影响的供应商
- 部署到生产环境

**影响**：
- 回滚耗时 15-30 分钟（代码恢复 + 测试 + 部署）
- 可能需要人工介入修复特定边界情况

---

## 风险评估与缓解

### 高风险项

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|---------|
| 转换逻辑错误导致请求失败 | 🔴 严重 | 中 | Golden 测试 + 灰度验证 + 监控告警 |
| 性能回退导致服务不可用 | 🔴 严重 | 低 | 基准测试 + 压力测试 + 流量控制 |
| 边界情况未覆盖 | 🟡 中等 | 中 | 多样化测试用例 + 生产流量回放 |
| 供应商协议变更 | 🟡 中等 | 低 | 版本锁定 + 变更监控 |

### 中风险项

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|---------|
| 开发周期延长 | 🟡 中等 | 中 | 分阶段交付 + 优先级排序 |
| 团队成员理解偏差 | 🟡 中等 | 中 | 文档完善 + Code Review |
| 测试覆盖不足 | 🟡 中等 | 低 | 覆盖率强制要求 + Golden 测试 |

### 低风险项

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|---------|
| Go 版本兼容性问题 | 🟢 低 | 低 | 使用 Go 1.21+（稳定版本） |
| 依赖冲突 | 🟢 低 | 低 | relaykit 零外部依赖 |

---

## 时间线与里程碑

### 总体时间估算

| 阶段 | 工作内容 | 预计时间 | 累计时间 |
|------|---------|---------|---------|
| 阶段 0 | 准备工作 | 3 天 | 3 天 |
| 阶段 1 | relaykit 模块骨架 | 5 天 | 8 天 |
| 阶段 2 | DTO 迁移 | 5 天 | 13 天 |
| 阶段 3 | 核心转换器实现 | 10 天 | 23 天 |
| 阶段 4 | 集成到主项目 | 7 天 | 30 天 |
| 阶段 5 | 迁移其他供应商 | 24 天 | 54 天 |
| 阶段 6 | 测试与验证 | 14 天 | 68 天 |
| 阶段 7 | 清理旧代码 | 7 天 | 75 天 |

**总计**：约 **75 天**（11 周，约 2.5 个月）

### 关键里程碑

| 里程碑 | 日期（假设今天开始） | 验收标准 |
|--------|---------------------|---------|
| M1: relaykit 模块可用 | Day 13 | DTO + 核心转换器实现完成，测试通过 |
| M2: 首个供应商集成 | Day 30 | Anthropic 在生产环境使用 relaykit，成功率 ≥ 99.5% |
| M3: 核心供应商迁移完成 | Day 44 | Anthropic + OpenAI + Gemini + Qwen 四个核心供应商迁移完成 |
| M4: 全部供应商迁移完成 | Day 54 | 24 个供应商全部使用 relaykit |
| M5: 测试验证完成 | Day 68 | 单元测试 + 集成测试 + 压力测试全部通过 |
| M6: 迁移完成 | Day 75 | 旧代码清理完毕，文档更新，项目交付 |

### 每周计划

**Week 1-2（Day 1-14）**：
- 完成阶段 0、1、2
- 交付物：relaykit 模块骨架 + DTO 类型定义

**Week 3-4（Day 15-28）**：
- 完成阶段 3（核心转换器）
- 交付物：OpenAI ↔ Claude ↔ Gemini 转换器 + Golden 测试

**Week 5（Day 29-35）**：
- 完成阶段 4（集成到主项目）
- 交付物：Anthropic 供应商使用 relaykit，生产验证

**Week 6-8（Day 36-56）**：
- 完成阶段 5（迁移其他供应商）
- 交付物：24 个供应商全部迁移完成

**Week 9-10（Day 57-70）**：
- 完成阶段 6（测试与验证）
- 交付物：全面测试报告 + 灰度验证完成

**Week 11（Day 71-75）**：
- 完成阶段 7（清理旧代码）
- 交付物：迁移完成，文档更新

---

## 成功指标（KPI）

### 质量指标

| 指标 | 目标值 | 测量方式 |
|------|--------|---------|
| 单元测试覆盖率 | ≥ 85% | `go test -cover` |
| Golden 测试通过率 | 100% | CI 自动检查 |
| 转换成功率 | ≥ 99.5% | 生产监控（30 天滚动） |
| 边界情况覆盖 | ≥ 95% | 人工 checklist + 测试用例 |

### 性能指标

| 指标 | 目标值 | 测量方式 |
|------|--------|---------|
| P50 转换延迟 | < 0.5ms | Benchmark 测试 |
| P95 转换延迟 | < 1ms | Benchmark 测试 |
| P99 端到端延迟增加 | < 20ms | 生产监控对比 |
| 内存分配 | < 10KB/次 | `go test -bench -benchmem` |

### 稳定性指标

| 指标 | 目标值 | 测量方式 |
|------|--------|---------|
| 生产错误率 | < 0.1% | 监控告警（24h 滚动） |
| 灰度验证问题数 | 0 个 P0/P1 | 人工审查 + 监控 |
| 回滚次数 | ≤ 2 次 | 项目记录 |

### 可维护性指标

| 指标 | 目标值 | 测量方式 |
|------|--------|---------|
| 代码行数减少 | ≥ 20%（长期） | `cloc relay/` |
| 新增供应商开发时间 | < 3 天 | 实际测量 |
| 协议变更适配时间 | < 1 天 | 实际测量 |

---

## 资源需求

### 人力投入

| 角色 | 人数 | 投入时间 | 主要职责 |
|------|------|---------|---------|
| 后端开发（Go） | 2 人 | 全职 11 周 | relaykit 实现 + 供应商迁移 |
| 测试工程师 | 1 人 | 全职 4 周 | 测试用例编写 + 灰度验证 |
| 架构师/Tech Lead | 1 人 | 兼职（20%） | 架构审查 + Code Review |
| 运维工程师 | 1 人 | 兼职（10%） | 监控配置 + 灰度发布 |

### 基础设施

| 资源 | 用途 | 估算成本 |
|------|------|---------|
| 测试环境 | 集成测试 + 压力测试 | 与现有环境共用 |
| 监控系统 | relaykit 指标收集 | 扩展现有 Prometheus/Grafana |
| CI/CD | 自动化测试 + 部署 | 扩展现有 Jenkins/GitLab CI |

---

## 依赖与前置条件

### 技术依赖

| 依赖项 | 版本要求 | 状态 |
|--------|---------|------|
| Go | ≥ 1.21 | ✅ 满足 |
| GoFrame | v2.x | ✅ 满足 |
| PostgreSQL | ≥ 14 | ✅ 满足 |
| Redis | ≥ 6.0 | ✅ 满足 |

### 业务前置条件

| 条件 | 说明 | 状态 |
|------|------|------|
| 监控系统完善 | 需要实时监控转换成功率 | ✅ 已有 ops_system_metrics 表 |
| 灰度发布能力 | 需要按百分比控制流量 | ⚠️ 需确认是否支持 |
| 生产流量回放 | 用于测试 | ⚠️ 需确认是否有工具 |

---

## 沟通计划

### 项目启动会

**时间**：Day 1
**参与者**：开发团队 + 测试 + 运维 + 产品
**议程**：
- 迁移背景和目标
- 技术方案讲解
- 分工和时间线
- 风险评估

### 每周同步会

**时间**：每周五下午
**参与者**：开发团队 + Tech Lead
**议程**：
- 本周进度回顾
- 下周计划
- 阻塞问题讨论
- 风险更新

### 里程碑评审

**时间**：每个里程碑完成后
**参与者**：全体项目成员
**议程**：
- 验收标准检查
- 质量指标评估
- 是否继续下一阶段的决策

### 灰度发布评审

**时间**：每次灰度放量前
**参与者**：开发 + 运维 + 产品
**议程**：
- 上一阶段监控数据回顾
- 问题列表审查
- 放量决策
- 回滚预案确认

---

## 文档交付清单

### 技术文档

- [x] 迁移方案（本文档）
- [ ] relaykit API 文档（GoDoc 注释）
- [ ] 转换器开发指南
- [ ] 测试指南（如何编写 Golden 测试）
- [ ] 监控运维手册

### 更新现有文档

- [ ] `CLAUDE.md`：添加 relaykit 架构说明
- [ ] `docs/reference/api-format-reference.md`：移除旧转换逻辑描述
- [ ] `docs/大模型完整实施方案-v2.1.md`：更新为 relaykit 架构

### 交付物

- [ ] relaykit 模块代码（`relaykit/` 目录）
- [ ] 迁移后的 handler 代码（`relay/handler/` 目录）
- [ ] 测试代码（单元测试 + 集成测试）
- [ ] 监控配置（Grafana Dashboard）
- [ ] 灰度验证报告

---

## 后续优化方向

迁移完成后，可以基于 relaykit 架构进行进一步优化：

### 短期优化（3 个月内）

1. **转换缓存**：对相同请求的转换结果进行缓存，减少重复计算
2. **协议版本管理**：支持同一供应商的多个 API 版本（如 Claude Messages API v1/v2）
3. **转换链优化**：自动选择最优转换路径（Quality 评分）

### 中期优化（6 个月内）

1. **动态转换器注册**：支持运行时注册新的转换器，无需重启服务
2. **转换器热更新**：修复转换 bug 无需重新部署整个服务
3. **多语言支持**：将 relaykit 编译为 C 动态库，供其他语言调用

### 长期规划（1 年内）

1. **开源 relaykit**：作为独立的 AI 协议转换库开源
2. **社区贡献**：接受社区贡献新的供应商适配器
3. **标准化协议**：推动 AI 供应商采用统一的消息格式

---

## 结论

本迁移方案采用**分阶段、灰度发布、双轨并行**的策略，确保零停机、低风险地将 team-api 的 relay 层重构为 relaykit 架构。

**核心优势**：
- ✅ **零依赖**：relaykit 模块无外部依赖，可独立测试和开源
- ✅ **高可测试性**：Golden 测试保证字节级精确性
- ✅ **渐进式迁移**：功能开关控制，任何时刻都可回滚
- ✅ **长期收益**：新增供应商开发时间从 5 天降至 2 天，维护成本大幅下降

**预期成果**：
- 代码量减少 20%（从 26,138 行降至约 21,000 行）
- 测试覆盖率提升至 85%+
- 新增供应商开发时间减少 60%
- 协议变更适配时间减少 80%

**建议决策**：
鉴于 relaykit 架构的长期价值和本方案的完善风险控制，**建议批准执行本迁移方案**，预计在 11 周内完成全部迁移工作。

---

**文档版本**：v1.0  
**最后更新**：2026-07-28  
**作者**：AI 助手  
**审核状态**：待审核


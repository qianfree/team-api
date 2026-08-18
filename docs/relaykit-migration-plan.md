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

**残留可选清理（未做，与 relaykit 无关）**：~~`relay/channel/openai/converter.go` 的 `ConvertOpenAIToResponses` 经 grep 确认零调用方，属 Responses API 遗留死代码~~
> ⚠️ **2026-08-18 更正**：该结论已过时。`feat/responses` 分支的 responses 系列提交重新启用了它——`relay/channel/openai/adaptor.go` 的 ChatViaResponses 桥接分支（`info.UseResponsesAPI && mode == ChatCompletions`）是活调用方。**禁止按旧结论删除。**

**已知差异随全量常开生效**：Coze 强制 `Stream=true`、Coze `user` 归因丢失、响应用固定时间戳。用户已线下测试基础功能通过。

---

## Responses 协议收编（2026-08-18，feat/responses 分支，执行前置条件①的 Responses 部分）

将 `feat/responses` 分支实现在 relay/ 层的 Responses 协议转换收编进 relaykit，旧代码全部保留为回退路径（bridge 失败/未覆盖自动回退，与 2026-07-29 cutover 架构一致）。分四个提交落地（地基 / r2c / c2r+链 / 响应侧）。

### 收编范围

| 方向 | 转换器 | 注册 ID | 桥接入口 |
|------|--------|---------|---------|
| Responses→OpenAI Chat（请求侧） | `oai_responses.ResponsesToOpenAIChatRequestConverter` | `openai_responses_to_openai_chat_completions` | `relay/handler/relaykit_bridge.go`（responses 入站 + `!UpstreamSpeaksResponses()` 守卫） |
| OpenAI Chat→Responses（请求侧） | `oai_responses.OpenAIChatToResponsesRequestConverter` | `openai_chat_completions_to_openai_responses` | 同上（`UseResponsesAPI && ChatCompletions && upstream==openai`，置于同格式早退之前） |
| Responses→Claude（请求链） | StepConverters 两跳 spec（复用上述 r2c + `oai_chat` 转换器） | `openai_responses_to_claude_messages` | 同上（upstream==claude 分支） |
| Claude→Responses（非流式响应） | `oai_responses.ClaudeToResponsesResponseConverter` | 挂链 spec Resp 侧 | `relay/relaykit_bridge/responses.go` `TryConvertResponsesResponseViaRelaykit` |
| Claude→Responses（流式响应） | `oai_responses.ClaudeToResponsesStreamConverter` | `anthropic_messages_to_openai_responses_stream`（stream registry） | 同文件 `TryConvertResponsesStreamViaRelaykit` |

### 配套基础设施变更

- **RelayFormat 值对齐**：relaykit `RelayFormatOpenAIResponses` 由 `"openai_responses"` 改为 `"responses"`（原值全仓库零引用；桥接依赖字符串值相等 + 强转约定）
- **DTO 下沉**：24 个 Responses 类型从 `relay/dto/openai_responses.go` 平移至 `relaykit/dto/openai_responses.go`，relay 侧改类型别名（宿主零改动）；新增 `ResponsesStreamEvent{Type, Data}` 流式 chunk 载荷
- **链执行器**：新增 `relayconvert.ExecuteRequestConverter`——StepConverters 此前只有注册校验、没有执行引擎
- **单侧 spec 放宽**：`registerBuiltinTextConverter` 允许 Req-only / Resp-only spec（双侧全空仍 panic）
- **echo 接口**：relaykit 定义 `responsesEchoProvider{ ResponsesRequestSnapshot() }` 可选接口，宿主 `RelayInfo` 实现提供请求快照
- **有状态预检/stash 留桥接层**：`previous_response_id != ""` → 回退 legacy 由 `ConvertResponsesToOpenAI` 返回哨兵驱动 failover；`info.ResponsesRequest` stash 在桥接层完成

### 已知差异清单（计划内接受，对拍测试逐条覆盖）

1. **链第二跳语义**（legacy `ConvertOpenAIToClaude` vs `oai_chat` 转换器，后者为线上常开主路径）：纯文本 content 形态（string vs `[{"type":"text"}]` 块数组，Claude API 均合法）；max_tokens 缺省来源（legacy 固定 4096 vs 宿主 `DefaultMaxTokens` hook，模型相关更正确）；nil content 的 `"<nil>"` 垃圾文本块（**legacy 既有 bug**，relaykit 输出空块）
2. **计费口径**：Claude→Responses 流式/非流式计费 usage 由 Claude 语义（input 不含缓存）换为 OpenAI 语义（input 含缓存，`CacheIncludedInPrompt=true` 由计费侧扣减）——金额等价、明细口径变化
3. **模型名字段**：响应对象 model 优先上游返回值（legacy 在 IsModelMapped 时强制 OriginModelName）；流式 message_start 的模型名同理（convmeta 无 IsModelMapped 信号，不为它扩 Meta 接口）
4. **序列化形态**：类型化 DTO 输出 `prompt:null`/`conversation:null` 恒存在（legacy map 不含；官方 Responses API 本就含这两个 null 字段，语义等价）。
   > ⚠️ **2026-08-18 修复**：初版曾将 `annotations:[]` 与 usage 细分零值键（`reasoning_tokens` 等）也列为 omitempty 省略的"无害形态差"——**实为致命 bug**：codex 客户端（Rust serde 非Option字段）解析 `response.completed` 时要求这些键必须存在，缺失报 `missing field reasoning_tokens` 并触发反复重连。已将 `InputTokenDetails`/`OutputTokenDetails` 的 legacy 曾输出键与 `ResponsesOutputContent.Annotations` 改为恒输出（含零值），并新增回归测试 `TestCompletedEventStrictJSONKeys` 锁定。教训：**合成 JSON 的"省略零值键"对严格客户端不是无害差异**，对拍测试归一化时必须区分"客户端可见的键集合差异"与"纯序列化形态差"。
5. **c2r input_audio 怪癖忠实保留**：legacy 读扁平 `part["data"]`（标准 chat 格式数据在 `part["input_audio"]["data"]`），音频数据在 c2r 桥接中两侧同样丢失——golden 如实记录，未修复（修复=偏离 legacy）

### 对拍验证结论

- r2c / c2r 请求侧：同输入双跑 legacy 完整路径（转换器 + adaptor 后处理链）与 relaykit 转换器，`map[string]any` + `reflect.DeepEqual` **深度相等**（`converter_r2c_parity_test.go` / `converter_c2r_parity_test.go`）
- Responses→Claude 链：语义归一化对拍（文本提取 + `<nil>` 剔除 + 缺省 max_tokens 剔除），tool_use/tool_result/tools/tool_choice **严格相等**（`converter_chain_parity_test.go`）
- Claude→Responses 响应侧：流式事件序列逐条深度相等（归一化时间戳/ID/null 键形态差，`responses_bridge_parity_test.go`），计费口径换算断言成立（input 50 + cache_read 10 = prompt 60）
- golden fixture 8 个（r2c×4 / c2r×2 / 响应×2），首次由 relaykit 输出生成、与 legacy 输出并排审阅

### 后续待办

- 灰度验证（monitor.TrackConverterCall 面板观察 5 个新转换器成功率）
- ~~Responses↔Chat **响应侧**合成器（openai/responses.go 的 `handleResponsesInbound*`）与 r2c 响应侧（`chatCompletionToResponsesResponse`）未收编（P1+ 范围）~~ → 已由 P1-R 完成（见下节）
- B 方向流式（`HandleResponsesStreamToChat`）明确排除：依赖 StreamScannerHandler 超时治理，收编需桥接层重构且 ChatViaResponses 场景较小众，保持 legacy
- gemini adaptor 的 responses 入站、Responses→Gemini 方向未覆盖（保持 legacy）

---

## P1-R：Responses 响应侧合成器收编（2026-08-18，feat/responses 分支）

codex 打 chat-only 渠道的**响应合成主路径**收编进 relaykit，旧代码保留为回退。5 个提交落地（地基 / A 非流式+夹具修正 / B 非流式 / A 流式 / 收尾）。

### 收编范围与挂载点

| 方向 | 转换器 | 挂载 |
|------|--------|------|
| A 非流式（chat 上游→responses 客户端） | `OpenAIChatToResponsesResponseConverter` | r2c spec Resp 侧 |
| A 流式（chat SSE→responses 事件） | `OpenAIChatToResponsesStreamConverter` | 流式注册表 (openai→responses) |
| B 非流式（responses 上游→chat 客户端） | `ResponsesToOpenAIChatResponseConverter` | c2r spec Resp 侧 |

**排除 B 流式**（StreamScannerHandler 超时治理依赖）。

### 配套基础设施

- 错误契约 `relaykit/types/convert_error.go`：`ErrProtocolMismatch` 哨兵（假成功防护）+ `EmbeddedUpstreamError`（SSE 内嵌错误原文）；桥接经 errors.Is/As 分类——mismatch 首事件前 502+原文体、embedded 首事件前 200+原文透传（均 ResponseWritten）
- Meta 能力接口：`RelayInfo.ModelNameMapped()/GetRequestID()`（沿 responsesEchoProvider 先例），B 方向模型名三段逻辑因此精确复刻 legacy（消灭原计划的已知差异）

### 顺手修复的 legacy 确定性 bug（golden/单测锁定）

1. **A 非流式 nil panic**：`chatCompletionToResponsesResponse` 直接解引用 `Usage.PromptTokensDetails`（上游不带 details 时 panic）——relaykit 版 nil 安全，legacy 回退路径同补 `detailField` 判空
2. **A 流式多工具顺序随机**：done 事件与 completed output 遍历 map → 改有序 slice（登记顺序）
3. **A 流式重复 done**：重复 finish_reason 无去重 → done 标志
4. **时间戳口径**：CompletedAt=Created+1 / completed<created 可能 → 统一同刻（max(Now,created)）
5. **`responsesUsageOf` 漏拷 CacheWriteTokens**（P0 遗留）

### 更正的探索误判

- ~~「文本+工具并存时丢文本」~~：golden 13 证实 content 恒保留文本、finish=stop——真实风险是按 finish_reason 分派的客户端可能忽略工具调用（legacy 取舍保持）

### 已知差异（桥接文件头注释固化，7 项）

计费口径统一 OpenAI 语义（B 非流式漏设 CacheIncludedInPrompt 的自相矛盾顺带修正）；transferredTextLen 含 reasoning；SetFirstResponseTime 时机；prompt/conversation null 两键；completed_at 差恒 0；工具顺序确定；usage details 键恒存在。

### 验证

golden 9 个新增（09-15，suffix 驱动 runner）；strictjson 键集回归（A 非流式）；单测 9 个（错误契约/顺序/去重/Index 反查/估算）；宿主对拍 3 个（A 非流式深度相等、B 非流式模型名四象限、A 流式事件序列归一化比较）；roundtrip 补两 spec Resp 执行用例。夹具修正：claude 桥接测试补 ChannelType（原缺省 0 被 ProviderNativeFormat 归为 openai 导致误路由）。

---

## P1-A/P1-B：Claude/Gemini 入站 + openai→claude/gemini 响应映射收编（2026-08-18，feat/responses 分支）

Claude Code（claude 协议）与 Gemini 客户端打 openai 兼容渠道的**请求转换**（c2o/g2o）与**非流式响应映射**收编 relaykit。5 个提交（C1 转换器纯增量 → C2 请求接线 → C3 响应转换器纯增量 → C4 响应接线 → C5 文档）。

### 核心设计决策：c2o/g2o 接管点在共享函数内部（D1）

`ConvertToOpenAI` 被 **20 个 openai 兼容 adaptor** 共用（ali/aws/baidu_v2/…/zhipu），各 adaptor 在其后有定制后处理（volcengine 删 reasoning_effort、tencent 参数截断等）。若在 handler 桥接层路由会跳过全部后处理造成行为回退。**接管点在 `relay/relaykit_bridge/request.go` 的 `TryConvertInboundToOpenAIChat`**，由 ConvertToOpenAI 与 openai adaptor 内联分支收敛调用——签名零变化、后处理照常执行、**无需吸收任何后处理逻辑**。两侧交叉注释固化「严禁在 handler 层加同方向路由」的双通道禁令。

### 收编范围与挂载点

| 方向 | 转换器 | 挂载 |
|------|--------|------|
| Claude→OpenAI Chat（请求侧） | `oai_chat.ClaudeToOpenAIRequestConverter` | spec A，宿主接管点在 ConvertToOpenAI 内部 |
| Gemini→OpenAI Chat（请求侧） | `oai_gemini.GeminiToOpenAIRequestConverter` | spec B，同上 |
| Gemini→Claude（请求链） | StepConverters [B, openai→claude] | spec C，handler 桥接路由（P0 responses→claude 同构） |
| OpenAI→Claude（非流式响应） | `oai_chat.OpenAIToClaudeResponseConverter` | spec A Resp 侧（方向反转约定） |
| OpenAI→Gemini（非流式响应） | `oai_gemini.OpenAIToGeminiResponseConverter` | spec B Resp 侧 |

流式全部保留 legacy（P2 范围）；B 方向流式明确排除（StreamScannerHandler 超时治理依赖）。

### 配套基础设施

- 能力接口助手上移 `convmeta/metacap.go`（`ModelNameMappedOf`/`RequestIDOf`；原方案 kitutil 撞 import 环 kitutil→convmeta→types→kitutil，改放 Meta 本家包）
- reasonmap 新增 `OpenAIFinishReasonToClaudeLegacySemantics`（精确复刻宿主：不 ToLower、空串→end_turn——与 relaykit 既有函数语义不同，注释固化勿混用）与 `OpenAIFinishReasonToGeminiFinishReason`（tool_calls→STOP 怪癖保持）

### legacy 怪癖清单（golden/对拍锁定，24 项精华）

**c2o**：Model 无条件 UpstreamModelName / MaxTokens 缺省 4096 / thinking→effort 阈值（nil→medium、≤2048→low、≤16384→medium）/ system []any 只收 text 块 / user content 非 string-[]any → `Sprintf`（nil→`"<nil>"` 字面量）/ tool_result 三形态 / 空补 user 空消息 / thinking signature 丢 / tool_use input 缺失→`"{}"` / tool_choice string 原样透传（含非法 "any"）/ 非 user-assistant 角色整条丢 / TopK 丢弃（g2o 映射——不对称）
**g2o**：MaxOutputTokens 无默认 / ThinkingConfig 只看 ThoughtBudget / ResponseMimeType 任意非空→json_object / call_N 合成 ID + map[name] 反查（同名函数 ID 复用错配）/ pending tool 重排 / 未知 role 文本丢失但 functionResponse 仍产出 / user 三态 / assistant 图文互斥图赢 / inlineData 一律 image_url / FileData 丢 / toolConfig 非 map→nil
**openai→claude 响应**：块序 thinking→text→tool_use / Content 仅 string / thinking 无 signature / 空 choices 仍产骨架 / finish_reason LegacySemantics / msg_<RequestID> / usage 扣减判空
**openai→gemini 响应**：空 choices 完全空对象（与 claude 不对称）/ usageMetadata 无条件非 nil / candidates 扣 reasoning（2f0cc01 口径）/ ModelName 不填 / tool_calls→STOP

### 已知差异与缺陷记录

- **gemini→claude 链（R3）**：与 P0 responses→claude 链同款——thinking budget 80%↔50%、temperature 处理、effort adaptive 形态、max_tokens 缺省 4096↔DefaultMaxTokens hook、第二跳 content 形态（string↔块数组）；第二跳把 tool 消息包装为 user 消息内 tool_result 块（Claude 协议正确形态）
- **#18 现存缺陷（本批不修）**：claude/gemini adaptor 对交叉客户端（gemini 客户端打 claude 渠道、claude 客户端打 gemini 渠道）的响应落到 default OpenAI handler——请求侧链修好后端到端仍破，需 claude→gemini / gemini→claude 响应转换器（无 legacy 蓝本）
- **R5 方向反转约定**：spec A/B 的 Resp 实际转换方向与 response 注册表 route 键相反（(claude,openai) route 指向的 spec 做 openai→claude）——register 注释固化，route 查找仅测试使用

### 验证

golden 6 个（c2o×3 含怪癖集 / g2o×3 含同名函数 ID 错配与未知 role）；链集成测试（R6 类型契约 + 工具链路无损）；对拍 5 个（c2o/g2o 字节级 DeepEqual——同构 typed struct 优势、gemini→claude 链语义归一化、openai→claude/gemini 响应 + handler 全路径一致性）；register/roundtrip 扩展；passthrough_test 三行断言（链路由 + 双通道禁令）。

---

## P2：收尾批次——openai→claude/gemini 流式 + 交叉客户端修复 + Responses→Gemini（2026-08-18，feat/responses 分支）

转换矩阵最后三个空格收编/修复，3 个提交（D1 流式转换器 / D2+D3 链组合与 adaptor 修复 / D4 文档）。

### D1: openai→claude/gemini 流式（Claude Code / Gemini 客户端打 openai 兼容渠道的流式）

- `OpenAIToClaudeStreamConverter`（chat SSE→Claude 事件流，块状态机全移植）+ **确定性修复**：参数 delta 按 tc.Index 反查所属块（legacy 错挂当前块）、意外断流补发 message_delta（legacy 只发 message_stop，客户端拿不到 stop_reason 与最终 usage）
- `OpenAIToGeminiStreamConverter`（chat SSE→Gemini 流，尾 chunk 收尾）+ **确定性修复**：分片 arguments 聚合为完整 functionCall part（legacy 逐片 unmarshal 产出垃圾 part——Gemini 协议 args 无增量语义）
- 流式桥接 chunkWriter 三态分派（chat chunk→data: 行 / ClaudeStreamEvent→event:+data: 行 / GeminiChatResponse→data: 行 + 各自 usage 提取还原 OpenAI 计费口径）；收尾按客户端格式（claude 无 [DONE]）；terminal 补发按格式分派
- dto 新增 `ClaudeStreamEvent` 载荷

### D2: 跨原生链组合（全部经 openai 中间态）

| 方向 | 请求 | 非流式响应 | 流式 |
|------|------|-----------|------|
| claude↔gemini 双向 | StepConverters 两跳链（新注册） | Resp 直挂两跳组合函数 | io.Pipe 串联组合 |
| responses→gemini | 链（启用预留常量） | Resp 组合 | io.Pipe 组合 |
| gemini→responses（响应方向） | — | Resp 组合（同 spec） | io.Pipe 组合 |

**io.Pipe 流式组合**：第一跳的 chat chunk 输出序列化为 `data:` 行写 pipe，第二跳从 pipe 读取——错误经 CloseWithError 传递。

### D3: #18 修复（比记录的更早断点：GetRequestURL 就拒绝交叉 mode）

探索证实交叉客户端（gemini 打 claude 渠道 / claude/responses 打 gemini 渠道）在 DoRequest 阶段就报 `unsupported relay mode`，#18 的 DoResponse default 缺陷实际不可达。修复：
- claude adaptor GetRequestURL 收 `RelayModeGeminiChat`（打 /v1/messages）；DoResponse 加 Gemini 分支
- gemini adaptor GetRequestURL 收 `RelayModeClaudeMessages/Responses/ResponsesCompact`；DoResponse 加 Claude/Responses 分支
- 未命中桥接时重构 body reader 回退旧 OpenAI handler（维持旧行为兜底）

### 已知差异与修复项

- gemini args 聚合为**行为改进**（修复 legacy 垃圾 part）；claude 断流补 message_delta 为修复项
- claude 流式模型名口径与非流式相反（legacy 保持：流式 OriginModelName/映射→Upstream；非流式 resp.Model/映射→Origin）
- 流式桥接 gemini 中断路径的 terminal 补发为 chat 格式终止行（legacy 同样不完美，注释记录）

### 转换矩阵终态

| 客户端 \ 上游 | OpenAI chat | Claude | Gemini | Coze/Dify/Ollama | Responses |
|---|---|---|---|---|---|
| **OpenAI chat** | 直连 | ✅ 全侧 | ✅ 全侧 | ✅ 全侧 | ✅ 全侧（非流式+流式 B 方向遗留） |
| **Responses** | ✅ 全侧 | ✅ 全侧 | ✅ 全侧（P2） | ❌ legacy | 直连 |
| **Claude** | ✅ 全侧 | 直连 | ✅ 全侧（P2） | ❌ legacy | ❌ 无 |
| **Gemini** | ✅ 全侧 | ✅ 全侧（P2） | 直连 | ❌ legacy | ❌ 无 |

剩余 legacy：Coze/Dify/Ollama 上游 × claude/gemini/responses 客户端（低频，组合件已在库可按需补）。

---

## P3：Claude/Gemini 客户端 → Responses 上游（2026-08-18，转换矩阵最后两个象限）

3 个提交（E1 B 流式转换器 / E2 组合注册+桥接+接线 / E3 文档）。

### E1: B 方向流式转换器（当初被排除的方向补齐）

`ResponsesToOpenAIChatStreamConverter`（responses SSE→chat chunks）：移植 HandleResponsesStreamToChat 事件状态机（前缀差分 args / name 去重 / finish 判据 / 独立 usage chunk / error 事件→EmbeddedUpstreamError），**去除 StreamScannerHandler 超时治理**（PingTicker 兜底，与 P2 D1 同款取舍）。B 流式的补齐使 claude/gemini 客户端×responses 上游的流式组合链闭环。

### E2: 组合与接线

- 请求链：claude→responses / gemini→responses（StepConverters 两跳）+ Resp 组合 + 两条流式 io.Pipe 组合
- 桥接 `bridgeUpstreamFormat` helper：claude/gemini 客户端 + `info.UseResponsesAPI` → 上游按 responses 路由（openai 客户端不受影响）
- handler `UseResponsesAPI` 条件扩到 ClaudeMessages/GeminiChat 模式（此前注释明说"暂不支持会 404"——限制移除）
- **修复 P1-A 遗留 bug**：openai adaptor `GetRequestURL` 补 `RelayModeGeminiChat`（gemini 客户端打普通 openai 兼容渠道此前直接报 unsupported relay mode——g2o 转换层做了但 URL 路由漏了）
- adaptor c2r 桥接分支扩到 claude/gemini 模式（body 已转 chat，与 chat 入站路径同构）

### 转换矩阵最终态（P3 后）

| 客户端 \ 上游 | OpenAI chat | Claude | Gemini | Coze/Dify/Ollama | Responses |
|---|---|---|---|---|---|
| **OpenAI chat** | 直连 | ✅ 全侧 | ✅ 全侧 | ✅ 全侧 | ✅ 全侧 |
| **Responses** | ✅ 全侧 | ✅ 全侧 | ✅ 全侧 | ❌ legacy | 直连 |
| **Claude** | ✅ 全侧 | 直连 | ✅ 全侧 | ❌ legacy | ✅ 全侧（P3） |
| **Gemini** | ✅ 全侧 | ✅ 全侧 | 直连 | ❌ legacy | ✅ 全侧（P3） |

除 Coze/Dify/Ollama 上游的低频交叉方向外，**转换矩阵全面 relaykit 化**（16/20 象限，其中 3 个直连）。旧代码全部保留为回退。

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
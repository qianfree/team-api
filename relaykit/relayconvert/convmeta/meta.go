// Package convmeta 定义了格式转换器（未来的 relaykit）与宿主应用之间的转换上下文契约。
// 转换器仅通过 Meta 接口读取协议状态和单次请求的选项；由宿主的 RelayInfo 实现该接口。
package convmeta

import (
	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/types"
)

// Meta 是格式转换器可以使用的 relay 会话唯一视图。
// 由宿主端的 *relaycommon.RelayInfo 实现；其他嵌入方（测试、外部 relaykit 用户）可以使用 *Values。
// 由指针类型支撑的实现必须保证每个方法在 nil 接收者上都是安全的：存入 Meta 的有类型 nil 指针仍是一个非 nil 接口，
// relaykit 刻意不使用反射来检测这种情况。
type Meta interface {
	GetOriginModelName() string
	GetUpstreamModelName() string
	// HasChannelMeta 返回是否已附加上游渠道信息；
	// 转换器用它判断 GetUpstreamModelName 是否有意义。
	HasChannelMeta() bool
	GetChannelID() int
	GetChannelType() int
	GetIsStream() bool
	GetReasoningEffort() string
	// SetReasoningEffort 记录转换器从模型名后缀推导出的 effort 级别，
	// 以便下游计费/日志能够看到。
	SetReasoningEffort(effort string)
	GetEstimatePromptTokens() int

	// EnsureClaudeConvertInfo 懒创建并返回可变的 OpenAI→Claude 流式转换状态。
	// 对于非 nil 接收者，在一次流式会话的整个生命周期内必须返回同一个实例；
	// nil 接收者可以返回一个临时初始化的状态。
	EnsureClaudeConvertInfo() *ClaudeConvertInfo

	// GetSendResponseCount / IncrSendResponseCount 暴露共享的下游 chunk 计数器
	// （宿主也可以对其进行自增）。
	GetSendResponseCount() int
	IncrSendResponseCount()

	// AppendRequestConversion 记录请求格式链中的一次跳转。
	AppendRequestConversion(format types.RelayFormat)

	// ConvOptions 返回请求作用域内的转换选项快照。
	// 绝不能返回 nil。
	ConvOptions() *Options
}

// ClaudeConvertInfo 承载 OpenAI chat → Claude Messages 流式转换的可变状态。
// 从 relay/common 迁移至此（后者保留了一个别名）。
type ClaudeConvertInfo struct {
	LastMessagesType string
	Index            int
	Usage            *dto.Usage
	FinishReason     string
	Done             bool

	ToolCallBaseIndex      int
	ToolCallMaxIndexOffset int
}

const (
	LastMessageTypeNone     = "none"
	LastMessageTypeText     = "text"
	LastMessageTypeTools    = "tools"
	LastMessageTypeThinking = "thinking"
)

// Values 是用于测试和非 RelayInfo 宿主（relaykit 原生入口）的纯结构体 Meta 实现。
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

var _ Meta = (*Values)(nil)

func (v *Values) GetOriginModelName() string {
	if v == nil {
		return ""
	}
	return v.OriginModelName
}

func (v *Values) GetUpstreamModelName() string {
	if v == nil {
		return ""
	}
	return v.UpstreamModelName
}

func (v *Values) HasChannelMeta() bool {
	return v != nil && v.ChannelMetaAttached
}

func (v *Values) GetChannelID() int {
	if v == nil {
		return 0
	}
	return v.ChannelID
}

func (v *Values) GetChannelType() int {
	if v == nil {
		return 0
	}
	return v.ChannelType
}

func (v *Values) GetIsStream() bool {
	return v != nil && v.IsStream
}

func (v *Values) GetReasoningEffort() string {
	if v == nil {
		return ""
	}
	return v.ReasoningEffort
}

func (v *Values) SetReasoningEffort(effort string) {
	if v != nil {
		v.ReasoningEffort = effort
	}
}

func (v *Values) GetEstimatePromptTokens() int {
	if v == nil {
		return 0
	}
	return v.EstimatePromptTokens
}

func (v *Values) EnsureClaudeConvertInfo() *ClaudeConvertInfo {
	if v == nil {
		return &ClaudeConvertInfo{LastMessagesType: LastMessageTypeNone}
	}
	if v.ClaudeConvertInfo == nil {
		v.ClaudeConvertInfo = &ClaudeConvertInfo{LastMessagesType: LastMessageTypeNone}
	}
	return v.ClaudeConvertInfo
}

func (v *Values) GetSendResponseCount() int {
	if v == nil {
		return 0
	}
	return v.SendResponseCount
}

func (v *Values) IncrSendResponseCount() {
	if v != nil {
		v.SendResponseCount++
	}
}

func (v *Values) AppendRequestConversion(format types.RelayFormat) {
	if v == nil || format == "" {
		return
	}
	if n := len(v.ConversionChain); n > 0 && v.ConversionChain[n-1] == format {
		return
	}
	v.ConversionChain = append(v.ConversionChain, format)
}

func (v *Values) ConvOptions() *Options {
	if v == nil {
		return &Options{}
	}
	if v.Options == nil {
		v.Options = &Options{}
	}
	return v.Options
}

// UpstreamModelName / ChannelTypeOf 是可选 Meta 值的 nil 安全访问器
// （在测试和兼容性 shim 中，转换器经常以 nil Meta 被调用）。
func UpstreamModelName(m Meta) string {
	if m == nil || !m.HasChannelMeta() {
		return ""
	}
	return m.GetUpstreamModelName()
}

func ChannelTypeOf(m Meta) int {
	if m == nil || !m.HasChannelMeta() {
		return 0
	}
	return m.GetChannelType()
}

// OptionsOf 返回 m 的转换选项，当 m 为 nil 时返回空默认值。
func OptionsOf(m Meta) *Options {
	if m == nil {
		return &Options{}
	}
	return m.ConvOptions()
}

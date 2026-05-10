// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ChnChannels is the golang structure of table chn_channels for DAO operations like Where/Data.
type ChnChannels struct {
	g.Meta                   `orm:"table:chn_channels, do:true"`
	Id                       any         // 主键ID
	Name                     any         // 渠道显示名称（如 "OpenAI 主力"、"Claude 备用"）
	Type                     any         // 供应商类型：1=OpenAI, 2=Anthropic Claude, 3=Google Gemini, 4=阿里云百炼, 5=百度文心, 6=腾讯混元, 7=智谱AI, 8=DeepSeek, 9=Moonshot, 10=火山引擎, 11=AWS Bedrock, 12=Azure OpenAI, 13=Google Vertex AI, 14=Cohere, 15=Mistral, 16=xAI
	BaseUrl                  any         // API 基础地址
	Status                   any         // 状态：active（启用）/ disabled（禁用）/ testing（测试中）
	Priority                 any         // 优先级（数字越大越优先，调度时优先选择高优先级渠道）
	Weight                   any         // 权重（同优先级下按权重随机选择，范围 1-100）
	MaxConcurrency           any         // 最大并发请求数
	Settings                 any         // 渠道配置（JSONB）：超时时间、重试次数等
	TestModel                any         // 测试使用的模型名
	Remark                   any         // 备注
	CreatedBy                any         // 创建者管理员ID
	CreatedAt                *gtime.Time // 创建时间
	UpdatedAt                *gtime.Time // 更新时间
	IsVip                    any         // 是否VIP专属渠道
	SharingThreshold         any         // 允许普通租户借用的利用率阈值（如0.6表示利用率<60%时可借用）
	PreemptionThreshold      any         // 触发VIP抢占的利用率阈值（如0.8表示利用率>=80%时VIP可抢占）
	BorrowingCooldownSeconds any         // 普通租户被抢占后的冷却时间（秒）
	AutoDisabled             any         // 是否被自动禁用：0=否, 1=是（由连续失败触发）
}

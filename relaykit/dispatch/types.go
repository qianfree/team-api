// Package dispatch 是渠道调度核心库（纯函数 + 端口）。
//
// 本包为纯逻辑库：不做任何 I/O，所有外部状态通过 ports.go 中的端口接口注入。
// relaykit module 不依赖 gf/redis/dao，纯度由模块边界强制。
package dispatch

// Tier 渠道调度层级，固定三档枚举（不采用数值优先级）。
type Tier string

const (
	TierPrimary   Tier = "primary"   // 首选
	TierSecondary Tier = "secondary" // 备用
	TierReserve   Tier = "reserve"   // 保底
)

// tierOrder 层级从高到低的固定顺序，用于扩组兜底。
var tierOrder = []Tier{TierPrimary, TierSecondary, TierReserve}

// BreakerState 熔断器状态。
type BreakerState int

const (
	BreakerClosed   BreakerState = iota // 正常
	BreakerOpen                         // 熔断中，硬排除
	BreakerHalfOpen                     // 探活中，每窗口放行一个真实请求
)

// Replayability 请求可重放性（按 relay mode 静态映射）。
type Replayability uint8

const (
	ReplaySafe   Replayability = iota // 查询 / embedding / rerank：可安全重放
	ReplayCostly                      // 文本生成：重放有成本但无外部副作用
	ReplayUnsafe                      // 图片/视频生成、异步任务提交：重放可能产生重复任务
)

// DeliveryState 请求送达状态（由执行器在错误发生点标注）。
type DeliveryState uint8

const (
	DeliveryNotSent          DeliveryState = iota // 连接失败 / 请求体未发出
	DeliveryMaybeSent                             // 写出后连接重置 / 读超时 / EOF，可能已送达上游
	DeliveryResponseReceived                      // 已收到完整错误响应，尚未写给客户端
	DeliveryResponseStarted                       // 已向客户端写出响应字节，禁止任何重试
)

// Channel 候选渠道快照。由 CatalogPort 提供，字段包含调度所需的
// 静态配置与最近一次刷新时的健康/负载读值（快照间隔内的滞后可接受）。
type Channel struct {
	ID   int64
	Name string
	Tier Tier

	BaseWeight float64 // 运营配置的组内权重（chn_channels.weight）
	CostRatio  float64 // 成本比例（chn_abilities.cost_ratio），上游实际价/平台基准价，<=0 视为 1.0

	SuccEwma  float64 // 成功率 EWMA ∈ [0,1]，无数据时为 1
	LatEwmaMs float64 // 延迟 EWMA（毫秒），无数据时为 0

	Inflight  int // 当前并发（租约计数）
	SoftLimit int // 软并发上限；<=0 表示无容量信息（headroom 因子恒为 1）

	Breaker      BreakerState // 渠道级熔断状态
	ModelBreaker BreakerState // 渠道×模型级熔断状态

	RampElapsedMs int64 // 距新启用/熔断恢复的毫秒数；<0 表示不在爬坡期

	KeyIDs         []int64 // active 状态的渠道 Key ID 列表（按 id 升序）；空表示由适配层自行取 Key
	StrictCapacity bool    // Redis 故障时 fail-closed

	// 协议能力（chn_abilities 渠道×模型级），参与 protoFactor 软偏好：
	// responses 入站偏好 SupportsResponses 渠道（避免经 chat 转换丢失有状态 Responses 特性），
	// chat 入站偏好原生 chat 渠道（ChatViaResponses 桥接渠道降权但仍可用）。
	SupportsResponses bool // 模型在该渠道支持 /v1/responses 原生协议
	ChatViaResponses  bool // 上游仅有 Responses 协议，chat 需经桥接发送
}

// ProtoPreference 入站端点协议偏好（软偏好，只降权不排除）。
type ProtoPreference uint8

const (
	ProtoAny       ProtoPreference = iota // 非文本生成端点 / 无偏好
	ProtoResponses                        // responses 入站：偏好 SupportsResponses 渠道
	ProtoChat                             // chat 入站：偏好原生 chat 渠道
)

// ScoredChannel 打分后的候选。
type ScoredChannel struct {
	Channel
	Weight    float64
	Breakdown WeightBreakdown
}

// WeightBreakdown 权重分解，用于决策日志与 ForwardingTrace。
type WeightBreakdown struct {
	Base      float64 `json:"base"`
	Tier      float64 `json:"tier"`
	Health    float64 `json:"health"`
	Headroom  float64 `json:"headroom"`
	Cost      float64 `json:"cost"`
	Ramp      float64 `json:"ramp"`
	Proto     float64 `json:"proto"`
	Effective float64 `json:"effective"`
}

// RequestProfile 一次请求的调度输入。
type RequestProfile struct {
	RequestID string
	TenantID  int64
	UserID    int64
	APIKeyID  int64
	Model     string
	Scope     []int64 // 租户渠道范围（空 = 不限制），由适配层填充
	Replay    Replayability
	Signals   SessionSignals
	// Proto 入站端点协议偏好（ProtoAny=无偏好）。responses 入站软偏好
	// SupportsResponses 渠道、chat 入站软偏好原生 chat 渠道，只降权不排除。
	Proto ProtoPreference
	// Policy 请求级策略覆盖（租户级差异化）。
	// nil = 使用协调器全局策略。由适配层解析（全局 + 租户浅合并）后传入。
	Policy *RoutingPolicy
}

// DecisionReason 选择原因（对应 SelectionReason，前端渠道日志展示）。
type DecisionReason string

const (
	ReasonBind       DecisionReason = "bind"        // 绑定守卫命中
	ReasonHRW        DecisionReason = "hrw"         // HRW 新选择
	ReasonOverflow   DecisionReason = "overflow"    // 选中渠道低于当前可用的最高层级（溢出）
	ReasonProbe      DecisionReason = "probe"       // HALF_OPEN 探测放行
	ReasonCredRotate DecisionReason = "cred_rotate" // 凭证轮换（同渠道换 Key）
)

// ExclusionStats 本次选择的排除统计。
type ExclusionStats struct {
	Breaker int `json:"breaker"` // 熔断 OPEN 排除
	Lease   int `json:"lease"`   // 容量租约获取失败排除
	Request int `json:"request"` // 本请求已失败排除
}

// Decision 一次应尝试的调度决定。
type Decision struct {
	Channel        Channel
	KeyID          int64 // 0 = 无 Key 快照信息，适配层按旧逻辑自行取
	SessionKey     SessionKey
	Reason         DecisionReason
	Breakdown      WeightBreakdown
	CandidateCount int
	Excluded       ExclusionStats
}

// Outcome 一次尝试的结果，用于健康上报（fire-and-forget）。
// Class 决定健康 EWMA 的衰减档位；CLIENT / CREDENTIAL 类不上报。
type Outcome struct {
	ChannelID int64
	KeyID     int64
	Model     string
	Success   bool
	Class     ErrorClass
	LatencyMs float64
	// Probe 标记探测类结果（管理后台手动测试 / 自动探测 cron），非真实用户流量。
	// 探测失败只喂熔断窗口不衰减健康 EWMA：无流量渠道若探测持续失败（如 test_model
	// 配置错误），周期性 ×decay 会把健康分指数拖垮且无真实流量对冲回升。
	Probe bool
}

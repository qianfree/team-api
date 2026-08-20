package dispatch

import (
	"fmt"
	"slices"
)

// RoutingPolicy 路由策略。
// 单一版本化对象，适配层从 sys_options 加载并做 Schema 校验，非法配置拒绝生效。
type RoutingPolicy struct {
	Version int `json:"version"`

	TierFactors map[Tier]float64 `json:"tierFactors"`

	Health struct {
		Alpha    float64 `json:"alpha"`    // 成功率指数，默认 2
		LatRefMs float64 `json:"latRefMs"` // 延迟基准（毫秒），默认 3000
	} `json:"health"`

	Load struct {
		Gamma        float64 `json:"gamma"`        // 余量指数，默认 2
		LeaseSeconds int     `json:"leaseSeconds"` // 租约时长，默认 90
	} `json:"load"`

	Cost struct {
		Beta float64 `json:"beta"` // 成本指数，默认 0.5
		Min  float64 `json:"min"`  // 因子下限，默认 0.5
		Max  float64 `json:"max"`  // 因子上限，默认 2.0
	} `json:"cost"`

	Ramp struct {
		WindowSeconds int     `json:"windowSeconds"` // 爬坡窗口，默认 120
		Floor         float64 `json:"floor"`         // 爬坡下限，默认 0.05
	} `json:"ramp"`

	Binding struct {
		TTLSeconds      int     `json:"ttlSeconds"`      // 绑定 TTL，默认 1800
		KeepHealthMin   float64 `json:"keepHealthMin"`   // 守卫：健康因子下限，默认 0.5
		KeepHeadroomMin float64 `json:"keepHeadroomMin"` // 守卫：余量下限，默认 0.1
	} `json:"binding"`

	Retry RetryPolicy `json:"retry"`

	Breaker BreakerPolicy `json:"breaker"`

	Session SessionPolicy `json:"session"`

	Replay ReplayPolicy `json:"replay"`

	Degrade struct {
		MaxReplicas int `json:"maxReplicas"` // 实例级保守限额分母，默认 1
	} `json:"degrade"`

	Proto struct {
		// ResponsesMismatch responses 入站打在不支持 Responses 协议渠道上的降权因子，
		// 默认 0.25（经 chat 转换丢失有状态 Responses 特性，降权较重）。
		ResponsesMismatch float64 `json:"responsesMismatch"`
		// ChatBridgeMismatch chat 入站打在 responses-only 桥接渠道上的降权因子，
		// 默认 0.75（桥接基本无损，降权较轻）。
		ChatBridgeMismatch float64 `json:"chatBridgeMismatch"`
	} `json:"proto"`
}

// ReplayPolicy 可重放性策略。
type ReplayPolicy struct {
	// UnsafeModes 归为 ReplayUnsafe 的 relay mode 字符串列表（与 handler 的
	// relayModeString 取值一致）。重放可能产生重复任务/高额成本的端点应列入。
	UnsafeModes []string `json:"unsafeModes"`
	// SafeModes 归为 ReplaySafe 的 relay mode 列表（可安全重放的查询类端点）。
	SafeModes []string `json:"safeModes"`
}

// ReplayabilityForMode 按 relay mode 字符串查可重放性：Unsafe 优先，Safe 次之，默认 Costly。
func (p ReplayPolicy) ReplayabilityForMode(mode string) Replayability {
	if slices.Contains(p.UnsafeModes, mode) {
		return ReplayUnsafe
	}
	if slices.Contains(p.SafeModes, mode) {
		return ReplaySafe
	}
	return ReplayCostly
}

// RetryPolicy 重试策略。
type RetryPolicy struct {
	InPlaceBudget        int   `json:"inPlaceBudget"`        // 原地重试预算（每渠道），默认 2
	FailoverBudget       int   `json:"failoverBudget"`       // 换渠道预算，默认 2
	CredRotateBudget     int   `json:"credRotateBudget"`     // 凭证轮换预算（每渠道），默认 1
	CredCooldownSeconds  int   `json:"credCooldownSeconds"`  // 凭证冷却时长，默认 300
	TotalDeadlineSeconds int   `json:"totalDeadlineSeconds"` // 总时限，默认 30
	BackoffBaseMs        int64 `json:"backoffBaseMs"`        // 退避基数，默认 100
	BackoffMaxMs         int64 `json:"backoffMaxMs"`         // 退避上限，默认 1000
	RateLimitWaitMaxMs   int64 `json:"rateLimitWaitMaxMs"`   // 429 最大等待，默认 2000

	// ReplayUnsafe 请求的收紧预算（默认 0/0，仅 NotSent 允许 failover）
	UnsafeInPlaceBudget  int `json:"unsafeInPlaceBudget"`
	UnsafeFailoverBudget int `json:"unsafeFailoverBudget"`
}

// BreakerPolicy 熔断策略。
type BreakerPolicy struct {
	WindowSeconds           int `json:"windowSeconds"`           // 滑动窗口，默认 60
	FailThreshold           int `json:"failThreshold"`           // 渠道级窗口失败阈值，默认 8
	ModelFailThreshold      int `json:"modelFailThreshold"`      // 模型级窗口失败阈值，默认 4（低于渠道级，单模型故障先在模型级熔断隔离）
	CooldownSeconds         int `json:"cooldownSeconds"`         // OPEN 冷却起始，默认 30
	CooldownMaxSeconds      int `json:"cooldownMaxSeconds"`      // 冷却上限，默认 300
	ProbeWindowSeconds      int `json:"probeWindowSeconds"`      // HALF_OPEN 探测窗口，默认 10
	AutoDisableAfterSeconds int `json:"autoDisableAfterSeconds"` // OPEN 持续自动禁用，默认 600
}

// SessionPolicy 会话解析策略。
type SessionPolicy struct {
	HeaderName             string `json:"headerName"`             // 显式头名，默认 X-Session-Id
	ParseAnthropicMetadata bool   `json:"parseAnthropicMetadata"` // 解析 metadata.user_id
	ParseOpenAIResponses   bool   `json:"parseOpenAIResponses"`   // 解析 previous_response_id / conversation_id
}

// DefaultRoutingPolicy 返回默认策略。
func DefaultRoutingPolicy() *RoutingPolicy {
	p := &RoutingPolicy{
		Version: 1,
		TierFactors: map[Tier]float64{
			TierPrimary:   1.0,
			TierSecondary: 0.15,
			TierReserve:   0.02,
		},
	}
	p.Health.Alpha = 2.0
	p.Health.LatRefMs = 3000
	p.Load.Gamma = 2.0
	p.Load.LeaseSeconds = 90
	p.Cost.Beta = 0.5
	p.Cost.Min = 0.5
	p.Cost.Max = 2.0
	p.Ramp.WindowSeconds = 120
	p.Ramp.Floor = 0.05
	p.Binding.TTLSeconds = 1800
	p.Binding.KeepHealthMin = 0.5
	p.Binding.KeepHeadroomMin = 0.1
	p.Retry = RetryPolicy{
		InPlaceBudget:        2,
		FailoverBudget:       2,
		CredRotateBudget:     1,
		CredCooldownSeconds:  300,
		TotalDeadlineSeconds: 30,
		BackoffBaseMs:        100,
		BackoffMaxMs:         1000,
		RateLimitWaitMaxMs:   2000,
		UnsafeInPlaceBudget:  0,
		UnsafeFailoverBudget: 0,
	}
	p.Breaker = BreakerPolicy{
		WindowSeconds:           60,
		FailThreshold:           8,
		ModelFailThreshold:      4,
		CooldownSeconds:         30,
		CooldownMaxSeconds:      300,
		ProbeWindowSeconds:      10,
		AutoDisableAfterSeconds: 600,
	}
	p.Session = SessionPolicy{
		HeaderName:             "X-Session-Id",
		ParseAnthropicMetadata: true,
		ParseOpenAIResponses:   true,
	}
	p.Replay = ReplayPolicy{
		UnsafeModes: []string{"images_generations", "images_edits", "video_generations"},
		SafeModes:   []string{"embeddings", "rerank"},
	}
	p.Degrade.MaxReplicas = 1
	p.Proto.ResponsesMismatch = 0.25
	p.Proto.ChatBridgeMismatch = 0.75
	return p
}

// Validate 策略校验。非法配置必须被拒绝生效（杜绝 O1 式静默降级）。
func (p *RoutingPolicy) Validate() error {
	if p == nil {
		return fmt.Errorf("routing policy 为空")
	}
	if len(p.TierFactors) == 0 {
		return fmt.Errorf("tierFactors 不能为空")
	}
	for t, f := range p.TierFactors {
		if t != TierPrimary && t != TierSecondary && t != TierReserve {
			return fmt.Errorf("未知层级: %s", t)
		}
		if f < 0 {
			return fmt.Errorf("tierFactors[%s] 不能为负: %v", t, f)
		}
	}
	if f, ok := p.TierFactors[TierPrimary]; !ok || f <= 0 {
		return fmt.Errorf("tierFactors.primary 必须 > 0")
	}
	if p.Health.Alpha <= 0 {
		return fmt.Errorf("health.alpha 必须 > 0")
	}
	if p.Load.Gamma < 0 {
		return fmt.Errorf("load.gamma 不能为负")
	}
	if p.Cost.Beta < 0 || p.Cost.Min <= 0 || p.Cost.Max < p.Cost.Min {
		return fmt.Errorf("cost 参数非法: beta=%v min=%v max=%v", p.Cost.Beta, p.Cost.Min, p.Cost.Max)
	}
	if p.Ramp.Floor < 0 || p.Ramp.Floor > 1 {
		return fmt.Errorf("ramp.floor 必须在 [0,1]")
	}
	if p.Binding.TTLSeconds <= 0 {
		return fmt.Errorf("binding.ttlSeconds 必须 > 0")
	}
	if p.Binding.KeepHealthMin < 0 || p.Binding.KeepHealthMin > 1 ||
		p.Binding.KeepHeadroomMin < 0 || p.Binding.KeepHeadroomMin > 1 {
		return fmt.Errorf("binding 守卫阈值必须在 [0,1]")
	}
	r := p.Retry
	if r.InPlaceBudget < 0 || r.FailoverBudget < 0 || r.CredRotateBudget < 0 ||
		r.UnsafeInPlaceBudget < 0 || r.UnsafeFailoverBudget < 0 {
		return fmt.Errorf("retry 预算不能为负")
	}
	if r.TotalDeadlineSeconds <= 0 || r.BackoffBaseMs <= 0 || r.BackoffMaxMs < r.BackoffBaseMs {
		return fmt.Errorf("retry 时间参数非法")
	}
	b := p.Breaker
	if b.WindowSeconds <= 0 || b.FailThreshold <= 0 || b.ModelFailThreshold <= 0 ||
		b.CooldownSeconds <= 0 || b.CooldownMaxSeconds < b.CooldownSeconds || b.ProbeWindowSeconds <= 0 {
		return fmt.Errorf("breaker 参数非法")
	}
	if p.Degrade.MaxReplicas <= 0 {
		return fmt.Errorf("degrade.maxReplicas 必须 > 0")
	}
	// proto 偏好因子必须为 (0,1]：>1 会反向惩罚匹配渠道，=0 会把不匹配渠道硬排除
	if p.Proto.ResponsesMismatch <= 0 || p.Proto.ResponsesMismatch > 1 ||
		p.Proto.ChatBridgeMismatch <= 0 || p.Proto.ChatBridgeMismatch > 1 {
		return fmt.Errorf("proto 因子必须在 (0,1]: responsesMismatch=%v chatBridgeMismatch=%v",
			p.Proto.ResponsesMismatch, p.Proto.ChatBridgeMismatch)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 重试决策 FSM
// ---------------------------------------------------------------------------

// RetryDecision 重试决策。
type RetryDecision int

const (
	DecisionAbort            RetryDecision = iota // 终止，返回错误
	DecisionInPlaceRetry                          // 同渠道同 Key 退避重试
	DecisionRotateCredential                      // 同渠道换下一个 Key
	DecisionFailover                              // 排除当前渠道重新选择
)

// String 实现 fmt.Stringer。
func (d RetryDecision) String() string {
	switch d {
	case DecisionInPlaceRetry:
		return "inplace"
	case DecisionRotateCredential:
		return "cred_rotate"
	case DecisionFailover:
		return "failover"
	default:
		return "abort"
	}
}

// AttemptState 请求级重试状态。原地与凭证轮换计数按当前渠道计，换渠道后由协调器清零。
type AttemptState struct {
	InPlaceUsed       int   // 当前渠道已用原地重试次数
	CredRotationsUsed int   // 当前渠道已用凭证轮换次数
	FailoverUsed      int   // 已换渠道次数
	ElapsedMs         int64 // 请求已耗时（毫秒）
	HasAlternateKey   bool  // 当前渠道是否还有可用的其它 Key（未冷却、未在本请求排除）
	ResponseStarted   bool  // 是否已向客户端写出响应字节
}

// Decide 重试决策 FSM（纯函数）。返回决策与建议退避毫秒数（不含 jitter，由协调器叠加）。
//
// 硬规则优先级（高于错误分类）：
//  1. 已向客户端写出响应 → Abort；
//  2. ReplayUnsafe + MaybeSent → Abort（状态码无关）；
//  3. 总时限耗尽 → Abort。
func Decide(class ErrorClass, delivery DeliveryState, replay Replayability, retryAfterMs int64, s AttemptState, p RetryPolicy) (RetryDecision, int64) {
	// 硬规则
	if s.ResponseStarted || delivery == DeliveryResponseStarted {
		return DecisionAbort, 0
	}
	if replay == ReplayUnsafe && delivery == DeliveryMaybeSent {
		return DecisionAbort, 0
	}
	if class == ErrClassClient || class == ErrClassNone {
		return DecisionAbort, 0
	}
	if s.ElapsedMs >= int64(p.TotalDeadlineSeconds)*1000 {
		return DecisionAbort, 0
	}

	// ReplayUnsafe 使用收紧预算
	inPlaceBudget, failoverBudget := p.InPlaceBudget, p.FailoverBudget
	if replay == ReplayUnsafe {
		inPlaceBudget, failoverBudget = p.UnsafeInPlaceBudget, p.UnsafeFailoverBudget
	}

	// failover 是否允许：预算内，且 ReplayUnsafe 仅在明确未送达时允许
	failoverAllowed := s.FailoverUsed < failoverBudget &&
		!(replay == ReplayUnsafe && delivery != DeliveryNotSent)
	failoverOrAbort := func() (RetryDecision, int64) {
		if failoverAllowed {
			return DecisionFailover, 0
		}
		return DecisionAbort, 0
	}

	switch class {
	case ErrClassCredential:
		// 先轮换同渠道 Key，耗尽后按渠道级致命处理
		if s.CredRotationsUsed < p.CredRotateBudget && s.HasAlternateKey {
			return DecisionRotateCredential, 0
		}
		return failoverOrAbort()

	case ErrClassRateLimit:
		// Retry-After 足够短则原地等待一次，否则立即 failover
		if retryAfterMs > 0 && retryAfterMs <= p.RateLimitWaitMaxMs && s.InPlaceUsed < inPlaceBudget {
			return DecisionInPlaceRetry, retryAfterMs
		}
		return failoverOrAbort()

	case ErrClassTimeout:
		// 超时（含 504）不原地重试
		return failoverOrAbort()

	case ErrClassTransient:
		// MaybeSent 禁止原地重试（可能已送达，原地重发有重复风险），只允许 failover
		if delivery != DeliveryMaybeSent && s.InPlaceUsed < inPlaceBudget {
			return DecisionInPlaceRetry, backoffMs(s.InPlaceUsed, p)
		}
		return failoverOrAbort()

	case ErrClassChannelFatal, ErrClassModelFatal:
		return failoverOrAbort()
	}
	return DecisionAbort, 0
}

// backoffMs 指数退避曲线：min(base × 2^attempt, max)。jitter 由协调器叠加。
func backoffMs(attempt int, p RetryPolicy) int64 {
	b := p.BackoffBaseMs
	for range attempt {
		b *= 2
		if b >= p.BackoffMaxMs {
			return p.BackoffMaxMs
		}
	}
	if b > p.BackoffMaxMs {
		return p.BackoffMaxMs
	}
	return b
}

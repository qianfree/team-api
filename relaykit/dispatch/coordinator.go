package dispatch

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// Coordinator 调度协调器：handler 的唯一入口。
// 自身无状态（策略热更新用 atomic 指针），可安全并发使用。
type Coordinator struct {
	catalog CatalogPort
	state   StatePort
	policy  atomic.Pointer[RoutingPolicy]
	clock   Clock
	entropy Entropy
}

// NewCoordinator 构造协调器。policy 为 nil 时使用默认策略。
func NewCoordinator(catalog CatalogPort, state StatePort, policy *RoutingPolicy, clock Clock, entropy Entropy) *Coordinator {
	if policy == nil {
		policy = DefaultRoutingPolicy()
	}
	if clock == nil {
		clock = SystemClock{}
	}
	co := &Coordinator{catalog: catalog, state: state, clock: clock, entropy: entropy}
	co.policy.Store(policy)
	return co
}

// UpdatePolicy 热更新策略（调用方需先 Validate，非法配置不得传入）。
func (co *Coordinator) UpdatePolicy(p *RoutingPolicy) {
	if p != nil {
		co.policy.Store(p)
	}
}

// Policy 返回当前生效策略。
func (co *Coordinator) Policy() *RoutingPolicy {
	return co.policy.Load()
}

// Route 开启一次请求的调度会话。profile.Policy 非空时本会话使用该策略
// （租户级覆盖），否则捕获协调器当前全局策略（会话内保持一致，不受热更新影响）。
func (co *Coordinator) Route(ctx context.Context, profile RequestProfile) *RouteSession {
	pol := profile.Policy
	if pol == nil {
		pol = co.policy.Load()
	}
	return &RouteSession{
		co:            co,
		pol:           pol,
		profile:       profile,
		sessionKey:    ResolveSessionKey(profile, pol.Session),
		excludedChans: make(map[int64]struct{}),
		excludedKeys:  make(map[int64]map[int64]struct{}),
		startedAt:     co.clock.Now(),
	}
}

// RouteSession 一次请求的调度会话。非并发安全（单请求内串行使用）。
type RouteSession struct {
	co         *Coordinator
	pol        *RoutingPolicy // 本会话生效的策略（全局或租户覆盖，Route 时捕获）
	profile    RequestProfile
	sessionKey SessionKey

	excludedChans map[int64]struct{}           // 本请求已排除的渠道
	excludedKeys  map[int64]map[int64]struct{} // 渠道 → 本请求已排除的 KeyID

	current      *Decision
	lastDecision RetryDecision
	started      bool // 是否已做过首次选择
	finished     bool

	attempt        AttemptState
	startedAt      time.Time
	leaseChannelID int64 // 当前持有租约的渠道（0 = 未持有）

	lastNoChannelDiag NoChannelDiag // 最近一次 selectChannel 的排除明细（无渠道诊断用）
}

// NoChannelDiag 无可用渠道诊断：候选快照规模与各排除原因的精确计数。
// 各计数相互独立（一个渠道只计入首个命中的排除原因）。
type NoChannelDiag struct {
	Snapshot        int // 目录快照中的候选渠道数（0 = 目录/权限层就没有该模型的渠道）
	BreakerOpen     int // 熔断 OPEN 硬排除
	ProbeDenied     int // HALF_OPEN 半开探测令牌拒绝（恢复期每窗口全局仅放行 1 个探测请求）
	LeaseFull       int // 容量租约满（渠道并发已达 softLimit / max_concurrency）
	RequestExcluded int // 本请求内先前已失败而被排除的渠道
	CredUnavailable int // 渠道全部 Key 处于冷却/已排除
}

// Summary 人类可读的原因摘要（仅列出非零项），供无可用渠道时的日志定位。
func (d NoChannelDiag) Summary() string {
	if d.Snapshot == 0 {
		return "目录快照为空：该租户+模型无候选渠道（检查渠道启用状态/模型能力/渠道范围）"
	}
	var parts []string
	if d.BreakerOpen > 0 {
		parts = append(parts, fmt.Sprintf("熔断OPEN×%d", d.BreakerOpen))
	}
	if d.ProbeDenied > 0 {
		parts = append(parts, fmt.Sprintf("半开探测限流×%d", d.ProbeDenied))
	}
	if d.LeaseFull > 0 {
		parts = append(parts, fmt.Sprintf("容量租约满×%d", d.LeaseFull))
	}
	if d.CredUnavailable > 0 {
		parts = append(parts, fmt.Sprintf("凭证全部冷却×%d", d.CredUnavailable))
	}
	if d.RequestExcluded > 0 {
		parts = append(parts, fmt.Sprintf("本请求已排除×%d", d.RequestExcluded))
	}
	if len(parts) == 0 {
		// 快照非空且无任何排除计数：候选全部打分为非正权重（PickHRW 无可选）
		return "候选渠道权重均为 0（健康/余量/层级因子坍缩）"
	}
	return strings.Join(parts, "，")
}

// SessionKey 返回本次请求解析出的会话键（供日志/指标）。
func (s *RouteSession) SessionKey() SessionKey { return s.sessionKey }

// Next 返回下一个应尝试的调度决定。首次调用 = 初始选择；此后行为由上一次
// Report 的决策驱动（原地重试 → 同渠道同 Key；凭证轮换 → 同渠道换 Key；
// failover → 排除后重新选择）。返回 nil 表示无渠道可用或已终止。
func (s *RouteSession) Next(ctx context.Context) *Decision {
	if s.finished {
		return nil
	}
	if s.started {
		switch s.lastDecision {
		case DecisionInPlaceRetry:
			return s.current

		case DecisionRotateCredential:
			if d := s.rotateKey(ctx); d != nil {
				return d
			}
			// 无可轮换 Key：排除当前渠道，落入重新选择
			s.excludeCurrent(ctx)

		case DecisionFailover:
			// Report 已完成排除与租约释放

		default: // Abort 或未 Report 就再次 Next
			return nil
		}
	}
	s.started = true
	return s.selectChannel(ctx)
}

// Report 上报本次尝试结果，返回重试决策与退避时长（已含 jitter）。
func (s *RouteSession) Report(ctx context.Context, statusCode int, err error, delivery DeliveryState, latencyMs float64, retryAfter time.Duration) (RetryDecision, time.Duration) {
	if s.finished || s.current == nil {
		return DecisionAbort, 0
	}
	pol := s.pol
	class := Classify(statusCode, err, delivery)

	s.attempt.ElapsedMs = s.co.clock.Now().Sub(s.startedAt).Milliseconds()
	s.attempt.HasAlternateKey = s.alternateKeyAvailable(ctx)
	if delivery == DeliveryResponseStarted {
		s.attempt.ResponseStarted = true
	}

	decision, backoff := Decide(class, delivery, s.profile.Replay, retryAfter.Milliseconds(), s.attempt, pol.Retry)

	// 结果上报（CLIENT / CREDENTIAL 类不计渠道健康）
	switch class {
	case ErrClassCredential:
		if s.current.KeyID > 0 {
			s.co.state.CoolCredential(ctx, s.current.KeyID, time.Duration(pol.Retry.CredCooldownSeconds)*time.Second)
			s.excludeKey(s.current.Channel.ID, s.current.KeyID)
		}
	case ErrClassClient, ErrClassNone:
		// 不上报
	default:
		s.co.state.ReportOutcome(Outcome{
			ChannelID: s.current.Channel.ID,
			KeyID:     s.current.KeyID,
			Model:     s.profile.Model,
			Success:   false,
			Class:     class,
			LatencyMs: latencyMs,
		})
	}

	// 按决策更新预算与排除状态。失败全程不删除绑定。
	switch decision {
	case DecisionInPlaceRetry:
		s.attempt.InPlaceUsed++
	case DecisionRotateCredential:
		s.attempt.CredRotationsUsed++
	case DecisionFailover:
		s.attempt.FailoverUsed++
		s.excludeCurrent(ctx)
	}
	s.lastDecision = decision

	// 叠加 jitter(0~50%)
	if backoff > 0 && s.co.entropy != nil {
		backoff += int64(float64(backoff) * 0.5 * s.co.entropy())
	}
	return decision, time.Duration(backoff) * time.Millisecond
}

// Finish 请求结束（成功或最终失败）：释放租约、绑定续期、上报健康。
func (s *RouteSession) Finish(ctx context.Context, success bool, latencyMs float64) {
	if s.finished {
		return
	}
	s.finished = true
	s.releaseLease(ctx)
	if success && s.current != nil {
		pol := s.pol
		s.co.state.TouchBinding(ctx, s.sessionKey.Key, time.Duration(pol.Binding.TTLSeconds)*time.Second)
		s.co.state.ReportOutcome(Outcome{
			ChannelID: s.current.Channel.ID,
			KeyID:     s.current.KeyID,
			Model:     s.profile.Model,
			Success:   true,
			LatencyMs: latencyMs,
		})
	}
}

// RefreshLease 长流式请求续租（handler 在流转发期间周期调用）。
func (s *RouteSession) RefreshLease(ctx context.Context) {
	if s.leaseChannelID > 0 {
		s.co.state.RefreshLease(ctx, s.leaseChannelID, s.profile.RequestID)
	}
}

// NoChannelDiagnosis 返回最近一次渠道选择的排除明细诊断。
// Next 返回 nil（无可用渠道）时供 handler 打出可定位原因的日志，
// 各字段含义见 NoChannelDiag，摘要文本用 Summary()。
func (s *RouteSession) NoChannelDiagnosis() NoChannelDiag {
	return s.lastNoChannelDiag
}

// ---------------------------------------------------------------------------
// 内部：选择
// ---------------------------------------------------------------------------

// selectChannel 选择流程：
// 快照 → 熔断硬排除 → 打分 → 绑定守卫 → HRW → 探测令牌 → 租约。
// 租约/探测失败只排除重选，不扣预算。
func (s *RouteSession) selectChannel(ctx context.Context) *Decision {
	pol := s.pol
	snapshot := s.co.catalog.Snapshot(ctx, s.profile.TenantID, s.profile.Model, s.profile.Scope)
	s.lastNoChannelDiag = NoChannelDiag{Snapshot: len(snapshot)}

	// 本轮选择中被探测令牌 / 租约 / 凭证拒绝的渠道（不进入 excludedChans，不影响绑定守卫语义）
	roundExcluded := make(map[int64]struct{})
	var probeDenied, leaseDenied, credDenied int

	for range len(snapshot) + 3 { // 每轮至少排除一个渠道，防御性上限
		scored, halfOpen, exclRound := s.buildCandidates(snapshot, roundExcluded, pol)
		excl := ExclusionStats{
			Breaker: exclRound.Breaker + probeDenied,
			Lease:   leaseDenied,
			Request: exclRound.Request + credDenied,
		}
		// 每轮刷新诊断明细（各原因独立计数）：返回 nil 时调用方通过 NoChannelDiagnosis 定位原因
		s.lastNoChannelDiag = NoChannelDiag{
			Snapshot:        len(snapshot),
			BreakerOpen:     exclRound.Breaker,
			ProbeDenied:     probeDenied,
			LeaseFull:       leaseDenied,
			RequestExcluded: exclRound.Request,
			CredUnavailable: credDenied,
		}

		if len(scored) == 0 {
			return nil
		}

		chosen, reason := s.chooseFrom(ctx, scored, pol)
		if chosen == nil {
			return nil
		}

		// HALF_OPEN 渠道被选中：取探测令牌，每窗口全局只放行一个
		if _, isHalfOpen := halfOpen[chosen.ID]; isHalfOpen {
			if !s.co.state.TryProbeToken(ctx, chosen.ID) {
				roundExcluded[chosen.ID] = struct{}{}
				probeDenied++
				continue
			}
			reason = ReasonProbe
		}

		// 凭证选取：全部 Key 冷却/排除 → 该渠道本请求不可用
		keyID, keyOK := s.pickKey(ctx, chosen.Channel)
		if !keyOK {
			roundExcluded[chosen.ID] = struct{}{}
			credDenied++
			continue
		}

		// 容量租约：满 → 排除重选，不扣预算
		if !s.co.state.AcquireLease(ctx, chosen.ID, chosen.SoftLimit, s.profile.RequestID) {
			roundExcluded[chosen.ID] = struct{}{}
			leaseDenied++
			continue
		}
		s.leaseChannelID = chosen.ID

		// 新渠道：原地与凭证轮换计数清零（预算按渠道独立计）
		s.attempt.InPlaceUsed = 0
		s.attempt.CredRotationsUsed = 0

		s.current = &Decision{
			Channel:        chosen.Channel,
			KeyID:          keyID,
			SessionKey:     s.sessionKey,
			Reason:         reason,
			Breakdown:      chosen.Breakdown,
			CandidateCount: len(scored),
			Excluded:       excl,
		}
		return s.current
	}
	return nil
}

// buildCandidates 构建打分候选集：排除本请求已失败渠道与熔断 OPEN 渠道；
// HALF_OPEN 渠道进入候选（选中时再取探测令牌）；tierFactor=0 的层级在
// 无候选时按层级顺序扩组兜底。
//
// 注意：Channel.Breaker / ModelBreaker 由目录适配层提供，已做过冷却期的
// 惰性 OPEN→HALF_OPEN 判定（EffectiveBreakerState）。
func (s *RouteSession) buildCandidates(snapshot []Channel, roundExcluded map[int64]struct{}, pol *RoutingPolicy) ([]ScoredChannel, map[int64]struct{}, ExclusionStats) {
	halfOpen := make(map[int64]struct{})
	excl := ExclusionStats{}
	var scored []ScoredChannel
	var zeroTier []Channel // 因 tierFactor=0 被排除的渠道，供扩组兜底

	for _, ch := range snapshot {
		if _, ok := s.excludedChans[ch.ID]; ok {
			excl.Request++
			continue
		}
		if _, ok := roundExcluded[ch.ID]; ok {
			continue // 已计数
		}
		if ch.Breaker == BreakerOpen || ch.ModelBreaker == BreakerOpen {
			excl.Breaker++
			continue
		}
		if ch.Breaker == BreakerHalfOpen || ch.ModelBreaker == BreakerHalfOpen {
			halfOpen[ch.ID] = struct{}{}
		}
		if tierFactor(ch.Tier, pol) <= 0 {
			zeroTier = append(zeroTier, ch)
			continue
		}
		w, bd := EffectiveWeight(ch, pol)
		scored = append(scored, ScoredChannel{Channel: ch, Weight: w, Breakdown: bd})
	}

	if !hasPositiveWeight(scored) && len(zeroTier) > 0 {
		// 硬性扩组兜底：按 primary → secondary → reserve 顺序启用 tierFactor=0 的层级
		const epsilonTier = 0.01
		for _, t := range tierOrder {
			var added bool
			for _, ch := range zeroTier {
				if ch.Tier != t {
					continue
				}
				w, bd := EffectiveWeight(ch, pol)
				// tierFactor 为 0，用 epsilon 重算
				bd.Tier = epsilonTier
				bd.Effective = bd.Base * bd.Tier * bd.Health * bd.Headroom * bd.Cost * bd.Ramp
				w = bd.Effective
				if w > 0 {
					added = true
				}
				scored = append(scored, ScoredChannel{Channel: ch, Weight: w, Breakdown: bd})
			}
			if added {
				break
			}
		}
	}
	return scored, halfOpen, excl
}

// chooseFrom 绑定守卫 + HRW。
// 守卫判据是被绑渠道自身状态（健康因子、原始余量），与 tier/cost 解耦——
// 溢出到 secondary 的绑定在守卫下保持稳定。
func (s *RouteSession) chooseFrom(ctx context.Context, scored []ScoredChannel, pol *RoutingPolicy) (*ScoredChannel, DecisionReason) {
	if boundID, ok := s.co.state.GetBinding(ctx, s.sessionKey.Key); ok {
		for i := range scored {
			sc := &scored[i]
			if sc.ID != boundID {
				continue
			}
			if sc.Weight > 0 &&
				sc.Breakdown.Health >= pol.Binding.KeepHealthMin &&
				rawHeadroom(sc.Channel) >= pol.Binding.KeepHeadroomMin {
				return sc, ReasonBind
			}
			break // 被绑渠道不合格 → 重跑 HRW 重绑
		}
	}

	pick := PickHRW(scored, s.sessionKey.Key)
	if pick == nil {
		return nil, ReasonHRW
	}
	s.co.state.SetBinding(ctx, s.sessionKey.Key, pick.ID, time.Duration(pol.Binding.TTLSeconds)*time.Second)

	reason := ReasonHRW
	if best := bestAvailableTier(scored); best != "" && pick.Tier != best {
		reason = ReasonOverflow
	}
	return pick, reason
}

// rotateKey 凭证轮换：同渠道选下一个可用 Key。无可用 Key 返回 nil。
func (s *RouteSession) rotateKey(ctx context.Context) *Decision {
	if s.current == nil {
		return nil
	}
	keyID, ok := s.pickKey(ctx, s.current.Channel)
	if !ok || keyID == 0 || keyID == s.current.KeyID {
		return nil
	}
	next := *s.current
	next.KeyID = keyID
	next.Reason = ReasonCredRotate
	s.current = &next
	return s.current
}

// pickKey 选取渠道当前可用的 Key：跳过本请求已排除与全局冷却中的 Key。
// KeyIDs 为空（目录未提供 Key 快照）时返回 (0, true)，由适配层按旧逻辑自行取。
func (s *RouteSession) pickKey(ctx context.Context, ch Channel) (int64, bool) {
	if len(ch.KeyIDs) == 0 {
		return 0, true
	}
	excluded := s.excludedKeys[ch.ID]
	for _, id := range ch.KeyIDs {
		if _, ok := excluded[id]; ok {
			continue
		}
		if s.co.state.IsCredentialCooled(ctx, id) {
			continue
		}
		return id, true
	}
	return 0, false
}

// alternateKeyAvailable 当前渠道除当前 Key 外是否还有可用 Key（FSM 轮换判据）。
func (s *RouteSession) alternateKeyAvailable(ctx context.Context) bool {
	if s.current == nil || s.current.KeyID == 0 {
		return false
	}
	ch := s.current.Channel
	excluded := s.excludedKeys[ch.ID]
	for _, id := range ch.KeyIDs {
		if id == s.current.KeyID {
			continue
		}
		if _, ok := excluded[id]; ok {
			continue
		}
		if s.co.state.IsCredentialCooled(ctx, id) {
			continue
		}
		return true
	}
	return false
}

func (s *RouteSession) excludeCurrent(ctx context.Context) {
	if s.current != nil {
		s.excludedChans[s.current.Channel.ID] = struct{}{}
	}
	s.releaseLease(ctx)
}

func (s *RouteSession) excludeKey(channelID, keyID int64) {
	m, ok := s.excludedKeys[channelID]
	if !ok {
		m = make(map[int64]struct{})
		s.excludedKeys[channelID] = m
	}
	m[keyID] = struct{}{}
}

func (s *RouteSession) releaseLease(ctx context.Context) {
	if s.leaseChannelID > 0 {
		s.co.state.ReleaseLease(ctx, s.leaseChannelID, s.profile.RequestID)
		s.leaseChannelID = 0
	}
}

// rawHeadroom 原始负载余量（未经 γ 指数），用于绑定守卫判据。
func rawHeadroom(c Channel) float64 {
	if c.SoftLimit <= 0 {
		return 1
	}
	h := 1 - float64(c.Inflight)/float64(c.SoftLimit)
	if h < 0 {
		return 0
	}
	return h
}

// bestAvailableTier 候选集中存在（未被熔断/排除）的最高层级，用于溢出判定：
// 高层级渠道仍在候选集但因饱和/降权未被选中（如 headroom 坍缩为 0）→ 记为 overflow；
// 高层级渠道整体不在候选集（全部熔断/排除）→ 是 failover 而非溢出，记为 hrw。
func bestAvailableTier(scored []ScoredChannel) Tier {
	present := make(map[Tier]bool)
	for i := range scored {
		present[scored[i].Tier] = true
	}
	for _, t := range tierOrder {
		if present[t] {
			return t
		}
	}
	return ""
}

func hasPositiveWeight(scored []ScoredChannel) bool {
	for i := range scored {
		if scored[i].Weight > 0 {
			return true
		}
	}
	return false
}

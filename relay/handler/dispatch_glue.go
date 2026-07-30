package handler

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/qianfree/team-api/internal/logic/dispatchadapter"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relaykit/dispatch"
	"github.com/qianfree/team-api/relaykit/types"
)

// 本文件是 handler 与新调度引擎（relaykit/dispatch）之间的胶水层：
// 送达状态标注、状态码提取、退避执行、租约续期、决策明细挂 trace。

// dispatchStatusCode 从错误中提取上游 HTTP 状态码。
// RelayError 直接携带；relaykit NewAPIError 由分类器自行解包，此处返回 0 即可。
func dispatchStatusCode(err error) int {
	var relayErr *constant.RelayError
	if errors.As(err, &relayErr) {
		return relayErr.StatusCode
	}
	return 0
}

// deliveryStateOfRequestErr DoRequest 阶段错误的送达状态标注（修订 R2）：
// 连接拒绝 / DNS 失败 / TLS 建连失败 = 请求确定未发出（可安全重试）；
// 其余（写出后 RST/EOF/读超时）= 可能已送达上游，非幂等请求禁止重放。
func deliveryStateOfRequestErr(err error) dispatch.DeliveryState {
	if err == nil {
		return dispatch.DeliveryNotSent
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dispatch.DeliveryNotSent
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return dispatch.DeliveryNotSent
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"connection refused", "no such host", "dial tcp",
		"tls: handshake", "certificate", "proxyconnect",
	} {
		if strings.Contains(msg, marker) {
			return dispatch.DeliveryNotSent
		}
	}
	return dispatch.DeliveryMaybeSent
}

// reportMaterializeFailure 选择物化失败（Key 解密失败/目录缺失）时按渠道级致命上报，
// 让 FSM 决定换渠道或终止。不发起过上游请求，送达状态为 NotSent。
func reportMaterializeFailure(ctx context.Context, sess *dispatch.RouteSession, mErr error) dispatch.RetryDecision {
	decision, _ := sess.Report(ctx, 0,
		types.NewError(mErr, types.ErrorCodeChannelNoAvailableKey),
		dispatch.DeliveryNotSent, 0, 0)
	return decision
}

// sleepBackoff 执行退避等待，客户端断开时立即返回。
func sleepBackoff(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

// appendSchedulerDecision 调度决策明细挂 ForwardingTrace（修订 R5）。
func appendSchedulerDecision(trace *common.ForwardingTrace, d *dispatch.Decision, attempt int) {
	if trace == nil || d == nil {
		return
	}
	trace.Scheduler = append(trace.Scheduler, common.SchedulerDecision{
		Attempt:       attempt,
		ChannelID:     d.Channel.ID,
		KeyID:         d.KeyID,
		Reason:        string(d.Reason),
		Tier:          string(d.Channel.Tier),
		SessionSource: string(d.SessionKey.Source),
		Weights: map[string]float64{
			"base":      d.Breakdown.Base,
			"tier":      d.Breakdown.Tier,
			"health":    d.Breakdown.Health,
			"headroom":  d.Breakdown.Headroom,
			"cost":      d.Breakdown.Cost,
			"ramp":      d.Breakdown.Ramp,
			"effective": d.Breakdown.Effective,
		},
		Candidates:      d.CandidateCount,
		ExcludedBreaker: d.Excluded.Breaker,
		ExcludedLease:   d.Excluded.Lease,
		ExcludedRequest: d.Excluded.Request,
	})
}

// dispatchLeaseRefresher 长请求（流式/websocket）的调度租约续期器。
// 直连 Redis 状态续期而非经 RouteSession（RouteSession 非并发安全）。
type dispatchLeaseRefresher struct {
	stop chan struct{}
	once sync.Once
}

// startDispatchLeaseRefresher 每 30s 续期一次调度租约（租约默认 90s，实例崩溃后自然过期）。
func startDispatchLeaseRefresher(channelID int64, requestID string) *dispatchLeaseRefresher {
	r := &dispatchLeaseRefresher{stop: make(chan struct{})}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				dispatchadapter.RefreshDispatchLease(ctx, channelID, requestID)
				cancel()
			case <-r.stop:
				return
			}
		}
	}()
	return r
}

// Stop 停止续期（幂等）。
func (r *dispatchLeaseRefresher) Stop() {
	if r == nil {
		return
	}
	r.once.Do(func() { close(r.stop) })
}

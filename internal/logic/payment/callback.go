package payment

import (
	"context"
	"net/http"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
)

// 订单级互斥锁，防止回调重复处理。
//
// 采用 sharded mutex（按 orderNo 哈希到固定数量分片）替代原先 sync.Map + 手工引用计数
// 的实现：原 UnlockOrder 在 rcm.mu.Unlock() 之后才 refCount--，存在「归零后误 Delete」的
// 竞态窗口——并发 LockOrder 在 Delete 前后可能拿到不同的 rcm 对象，导致两路回调各自持有
// 「订单锁」却互不互斥。sharded 方案无动态分配、无引用计数、无 Delete，内存恒定为分片数，
// 且同一 orderNo 必然命中同一分片 → 互斥语义对同一订单始终正确。跨实例并发由 DB 条件更新
// （WHERE status='pending'）兜底，进程内锁仅用于降低同实例并发回调时的 DB 竞争。
const orderLockShardCount = 256

var orderLockShards [orderLockShardCount]sync.Mutex

// orderLockShardIndex 按 orderNo 选定分片。FNV-1a 简单快速、分布均匀，避免不同订单扎堆。
func orderLockShardIndex(orderNo string) int {
	const (
		offset32 = uint32(2166136261)
		prime32  = uint32(16777619)
	)
	h := offset32
	for i := 0; i < len(orderNo); i++ {
		h ^= uint32(orderNo[i])
		h *= prime32
	}
	return int(h % orderLockShardCount)
}

// paymentAmountTolerance 回调金额与订单金额容差（0.01 CNY）。
// 用 decimal 表达，避免 float64 在 0.01 这种边界值上的二进制表示误差导致误判。
var paymentAmountTolerance = decimal.NewFromFloat(0.01)

// LockOrder 对订单号加锁（按 orderNo 哈希到固定分片，同订单始终命中同一分片）。
func LockOrder(orderNo string) {
	orderLockShards[orderLockShardIndex(orderNo)].Lock()
}

// UnlockOrder 释放订单锁。
func UnlockOrder(orderNo string) {
	orderLockShards[orderLockShardIndex(orderNo)].Unlock()
}

// ProcessCallback 统一回调处理流程。
// channelType 为渠道类型字符串（如 "epay"）。
func ProcessCallback(ctx context.Context, r *http.Request, channelType string) error {
	// 1. 从 sys_options 加载渠道配置
	cfg, err := GetChannelConfigAndProvider(ctx, channelType)
	if err != nil {
		return gerror.Wrapf(err, "加载支付渠道配置失败")
	}

	// 2. 获取 Provider
	provider := GetProvider(channelType)
	if provider == nil {
		return gerror.Newf("不支持的支付渠道: %s", channelType)
	}

	// 3. 调用 Provider 验签并解析回调
	result, err := provider.HandleCallback(ctx, r, cfg)
	if err != nil {
		return gerror.Wrapf(err, "回调验证失败")
	}

	// 4. 订单级加锁
	LockOrder(result.OrderNo)
	defer UnlockOrder(result.OrderNo)

	// 5. 读取订单当前状态与支付信息（可履约性判断见下方 claimable）
	var order *struct {
		ID          int64       `json:"id"`
		Status      string      `json:"status"`
		FinalAmount float64     `json:"final_amount"`
		PaymentNo   string      `json:"payment_no"`
		ExpiredAt   *gtime.Time `json:"expired_at"`
	}
	err = dao.OrdOrders.Ctx(ctx).
		Where("order_no", result.OrderNo).Scan(&order)
	if err != nil {
		return gerror.Wrapf(err, "查询订单失败")
	}
	if order == nil {
		return common.NewNotFoundError("订单")
	}
	// 5. 幂等 / 可履约状态检查。
	// 可履约状态：pending，以及「支付成功但订单已被过期任务置为 expired」——用户在渠道
	// 完成扣款可能晚于订单有效期（或回调延迟到达），此时渠道已实际收款，拒绝入账会造成
	// 用户已付款却得不到权益，只能人工兜底；因此过期订单的成功回调照常履约，仅记告警日志。
	// 其余状态（paid/fulfilled/cancelled/refunded 等）幂等返回。
	claimable := order.Status == "pending" || (result.Success && order.Status == "expired")
	if !claimable {
		if result.Success && order.Status == "cancelled" {
			// 渠道已收款但订单已被取消：无法自动履约，记录告警等待人工处理（线下退款或补单）
			g.Log().Errorf(ctx, "[Payment] order=%s status=cancelled but received SUCCESS callback (trade_no=%s amount=%.2f), money captured without fulfillment, manual intervention required",
				result.OrderNo, result.TradeNo, result.PaidAmount)
		}
		// 重复支付检测：订单已 paid/fulfilled，又收到【不同流水号】的成功回调——说明用户
		// 对同一订单在多个渠道/多次完成了支付，渠道端已重复收款，第二笔在系统内无对应
		// 入账，必须人工在渠道侧退回。幂等吞掉不留痕会造成资金无声滞留。
		if result.Success && (order.Status == "paid" || order.Status == "fulfilled") &&
			result.TradeNo != "" && result.TradeNo != order.PaymentNo {
			g.Log().Errorf(ctx, "[Payment] order=%s already %s with trade_no=%s but received ANOTHER success callback trade_no=%s amount=%.2f, duplicate payment captured, manual refund required",
				result.OrderNo, order.Status, order.PaymentNo, result.TradeNo, result.PaidAmount)
		}
		return nil // 已处理或不可履约终态，幂等返回
	}
	if result.Success && (order.Status == "expired" ||
		(order.ExpiredAt != nil && !order.ExpiredAt.IsZero() && order.ExpiredAt.Before(gtime.Now()))) {
		g.Log().Warningf(ctx, "[Payment] order=%s paid after expiration, proceed with fulfillment", result.OrderNo)
	}

	// 6. 金额校验：回调金额与订单金额必须一致（容差 0.01 元，CNY）
	//    用 decimal 比较，避免 float64 直接相减在边界值（如 0.01）上的二进制误差误判。
	if result.Success && result.PaidAmount > 0 && order.FinalAmount > 0 {
		paid := billing.NewFromFloat(result.PaidAmount)
		expected := billing.NewFromFloat(order.FinalAmount)
		diff := paid.Sub(expected).Abs()
		if diff.GreaterThan(paymentAmountTolerance) {
			g.Log().Warningf(ctx, "[Payment] amount mismatch: order=%s expected=%.2f received=%.2f",
				result.OrderNo, order.FinalAmount, result.PaidAmount)
			return gerror.Newf("支付金额不一致: 期望 %.2f 实付 %.2f", order.FinalAmount, result.PaidAmount)
		}
	}

	if result.Success {
		// 7. 原子领取订单：pending/expired → paid。
		// 该条件更新的 RowsAffected 是【跨实例】幂等闸门：多实例并发回调时，两个回调都会
		// 通过上面第 5 步的状态检查，但条件原子更新只有一个能命中（另一个在行锁释放后重读
		// 到 status 已是 paid → 0 行）。据此仅让真正把订单翻成 paid 的那次回调去履约，
		// 杜绝重复入账/重复发套餐。进程内 orderLockShards 仅单实例有效，不能作为并发保护依赖。
		// expired 一并纳入条件：过期订单的成功回调照常入账（见第 5 步说明），
		// 同时覆盖「第 5 步读到 pending、过期任务随后将其置 expired」的竞态窗口。
		res, err := dao.OrdOrders.Ctx(ctx).
			Where("id", order.ID).
			WhereIn("status", g.Slice{"pending", "expired"}).
			Data(do.OrdOrders{
				Status:    "paid",
				PaidAt:    gtime.Now(),
				PaymentNo: result.TradeNo,
			}).Update()
		if err != nil {
			return gerror.Wrapf(err, "更新订单状态失败")
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return gerror.Wrapf(err, "确认订单状态更新结果失败")
		}
		if affected == 0 {
			// 另一并发回调已抢先领取并负责履约，本次幂等返回，绝不重复履约
			g.Log().Infof(ctx, "[Payment] order=%s already claimed by a concurrent callback, skip duplicate fulfill", result.OrderNo)
			return nil
		}

		// 8. 履约（仅领取成功者执行）
		return FulfillOrder(ctx, order.ID)
	}

	return nil
}

// QueryOrderPaid 查询订单是否已完成支付（paid 或 fulfilled）。
// 用于浏览器同步回跳时展示结果，不参与履约处理。
func QueryOrderPaid(ctx context.Context, orderNo string) bool {
	if orderNo == "" {
		return false
	}
	var status string
	_ = dao.OrdOrders.Ctx(ctx).
		Where("order_no", orderNo).
		Fields("status").
		Scan(&status)
	return status == "paid" || status == "fulfilled"
}

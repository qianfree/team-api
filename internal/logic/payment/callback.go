package payment

import (
	"context"
	"net/http"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
)

// 订单级互斥锁，防止回调重复处理。
var (
	orderLocks sync.Map
	createLock sync.Mutex
)

type refCountedMutex struct {
	mu       sync.Mutex
	refCount int
}

// LockOrder 对订单号加锁。
func LockOrder(orderNo string) {
	createLock.Lock()
	var rcm *refCountedMutex
	if v, ok := orderLocks.Load(orderNo); ok {
		rcm = v.(*refCountedMutex)
	} else {
		rcm = &refCountedMutex{}
		orderLocks.Store(orderNo, rcm)
	}
	rcm.refCount++
	createLock.Unlock()
	rcm.mu.Lock()
}

// UnlockOrder 释放订单锁。
func UnlockOrder(orderNo string) {
	v, ok := orderLocks.Load(orderNo)
	if !ok {
		return
	}
	rcm := v.(*refCountedMutex)
	rcm.mu.Unlock()

	createLock.Lock()
	rcm.refCount--
	if rcm.refCount == 0 {
		orderLocks.Delete(orderNo)
	}
	createLock.Unlock()
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

	// 5. 幂等检查：仅处理 pending 状态
	var order *struct {
		ID          int64       `json:"id"`
		Status      string      `json:"status"`
		FinalAmount float64     `json:"final_amount"`
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
	if order.Status != "pending" {
		return nil // 已处理，幂等返回
	}
	if order.ExpiredAt != nil && !order.ExpiredAt.IsZero() && order.ExpiredAt.Before(gtime.Now()) {
		return gerror.Newf("订单已过期: %s", result.OrderNo)
	}

	// 6. 金额校验：回调金额与订单金额必须一致（容差 0.01 元）
	if result.Success && result.PaidAmount > 0 && order.FinalAmount > 0 {
		diff := result.PaidAmount - order.FinalAmount
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.01 {
			g.Log().Warningf(ctx, "[Payment] amount mismatch: order=%s expected=%.2f received=%.2f",
				result.OrderNo, order.FinalAmount, result.PaidAmount)
			return gerror.Newf("支付金额不一致: 期望 %.2f 实付 %.2f", order.FinalAmount, result.PaidAmount)
		}
	}

	if result.Success {
		// 7. 更新订单为已支付
		_, err = dao.OrdOrders.Ctx(ctx).
			Where("id", order.ID).
			Where("status", "pending").
			Data(do.OrdOrders{
				Status:    "paid",
				PaidAt:    gtime.Now(),
				PaymentNo: result.TradeNo,
			}).Update()
		if err != nil {
			return gerror.Wrapf(err, "更新订单状态失败")
		}

		// 8. 履约
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

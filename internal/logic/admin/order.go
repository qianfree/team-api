package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/payment"
	"github.com/qianfree/team-api/internal/utility/export"
)

// ListOrders 获取全部订单列表
func (s *sAdmin) ListOrders(ctx context.Context, req *v1.OrderListReq) (*v1.OrderListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := dao.OrdOrders.Ctx(ctx)
	if req.Status != "" {
		query = query.Where("status", req.Status)
	}
	if req.TenantID != "" {
		query = query.Where("tenant_id", req.TenantID)
	}

	var total int
	orders := make([]*v1.OrderItem, 0)
	err := query.OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&orders, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.OrderListRes{
		List:     orders,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetOrder 获取订单详情
func (s *sAdmin) GetOrder(ctx context.Context, req *v1.OrderDetailReq) (*v1.OrderDetailRes, error) {
	var order *v1.OrderItem
	err := dao.OrdOrders.Ctx(ctx).
		Where("id", req.Id).
		Scan(&order)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, common.NewNotFoundError("订单")
	}
	return &v1.OrderDetailRes{OrderItem: order}, nil
}

// RefundOrder 发起退款（线上侧一步完成：状态流转 + 权益收回；渠道打款由管理员线下原路退回）。
//
// 退款语义（与《系统货币规则》对齐）：
//   - 退款认订单原始 CNY 金额（ord_ 层永远 CNY），线下按此金额原路退给用户；
//   - 已履约的充值订单必须按【履约当时入账的 USD】原额扣回钱包（从入账流水取值，
//     禁止按当前汇率反算，防止汇率波动套利）；
//   - 已履约的套餐订单撤销对应的活跃订阅；
//   - 已支付未履约（paid）的订单权益尚未发放，仅做状态流转。
func (s *sAdmin) RefundOrder(ctx context.Context, req *v1.OrderRefundReq) (*v1.OrderRefundRes, error) {
	adminUserID := common.GetCtxUserID(ctx)

	// redisDeductedAmount 记录事务内已发生的 Redis 钱包扣款：Redis 是资金提交点、
	// 不受 DB 事务回滚，事务失败时必须按此补偿逆转。
	var redisDeductedAmount decimal.Decimal
	var orderTenantID int64
	res := &v1.OrderRefundRes{}

	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 行锁串行化同一订单的并发退款：两个管理员同时发起时，后到者阻塞至前者提交
		// （状态已翻成 refunded / 退款记录已存在）后再读到最新状态，据此拒绝重复退款。
		var order *struct {
			TenantID       int64   `json:"tenant_id"`
			OrderType      string  `json:"order_type"`
			PlanID         int64   `json:"plan_id"`
			FinalAmount    float64 `json:"final_amount"`
			Status         string  `json:"status"`
			PaymentChannel string  `json:"payment_channel"`
		}
		err := dao.OrdOrders.Ctx(ctx).
			Where("id", req.Id).
			LockUpdate().
			Scan(&order)
		if err != nil {
			return err
		}
		if order == nil {
			return common.NewNotFoundError("订单")
		}
		if order.Status != "paid" && order.Status != "fulfilled" {
			return common.NewBadRequestError("订单状态不支持退款")
		}

		// 幂等：已有退款记录直接拒绝（持有订单行锁，检查无并发窗口）
		refundCount, err := dao.OrdRefunds.Ctx(ctx).Where("order_id", req.Id).Count()
		if err != nil {
			return err
		}
		if refundCount > 0 {
			return common.NewBadRequestError("该订单已存在退款记录，不能重复退款")
		}

		// 按订单类型收回已发放的权益
		switch order.OrderType {
		case "recharge":
			// 已履约：钱包已入账 USD，必须原额扣回；未履约（paid）钱包未入账，无需扣减
			if order.Status == "fulfilled" {
				orderTenantID = order.TenantID
				deducted, err := deductWalletForRefundTx(ctx, req.Id, order.TenantID)
				// 先记录已发生的 Redis 扣款（可能 >0 即使整体失败），供事务回滚后补偿逆转
				redisDeductedAmount = deducted
				if err != nil {
					return err
				}
				res.WalletDeductedUsd = billing.InexactFloat64(deducted)
			}
		case "new_plan", "renew", "upgrade":
			// 已履约：撤销该套餐当前生效的订阅。若订阅已被后续订单替换（active 行的
			// plan_id 不同）或已过期，则 0 行命中——无权益可收回，照常完成退款流转。
			if order.Status == "fulfilled" && order.PlanID > 0 {
				if _, err = dao.PlnTenantPlans.Ctx(ctx).
					Where("tenant_id", order.TenantID).
					Where("plan_id", order.PlanID).
					Where("status", "active").
					Data(do.PlnTenantPlans{
						Status:      "cancelled",
						CancelledAt: gtime.Now(),
					}).Update(); err != nil {
					return gerror.Wrapf(err, "撤销套餐订阅失败")
				}
			}
		}

		// 原子状态流转：paid/fulfilled → refunded。持有行锁时理论上必命中；
		// 0 行说明状态被并发改动，回滚整个退款事务（含钱包扣减）。
		result, err := dao.OrdOrders.Ctx(ctx).
			Where("id", req.Id).
			WhereIn("status", g.Slice{"paid", "fulfilled"}).
			Data(do.OrdOrders{Status: "refunded"}).
			Update()
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return gerror.Newf("order %d refund aborted: status changed concurrently", req.Id)
		}

		// 退款记录：金额为订单原始 CNY。线上侧（状态流转 + 余额扣减）已完成，
		// 渠道打款由管理员线下原路退回，故直接置 completed。
		if _, err = dao.OrdRefunds.Ctx(ctx).Insert(do.OrdRefunds{
			OrderId:        req.Id,
			TenantId:       order.TenantID,
			Amount:         order.FinalAmount,
			Reason:         req.Reason,
			Status:         "completed",
			PaymentChannel: order.PaymentChannel,
			ApprovedBy:     adminUserID,
			ApprovedAt:     gtime.Now(),
		}); err != nil {
			return err
		}
		res.RefundAmountCny = order.FinalAmount
		return nil
	})
	if err != nil {
		// 事务回滚：补偿逆转已发生的 Redis 钱包扣款（退款未生效，钱包不得扣回）。
		// 补偿自身失败意味着钱包被多扣——打 CRITICAL 日志人工追回。
		if redisDeductedAmount.GreaterThan(billing.Zero) && orderTenantID > 0 {
			compCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			if _, _, compErr := billing.CreditWalletRedis(compCtx, orderTenantID, redisDeductedAmount); compErr != nil {
				g.Log().Errorf(ctx,
					"CRITICAL: compensate refund redis debit failed: tenant=%d usd=%s order=%d: %v — wallet under-credited, manual fix required",
					orderTenantID, redisDeductedAmount.String(), req.Id, compErr)
			}
		}
		return nil, err
	}
	return res, nil
}

// deductWalletForRefundTx 在事务内按履约当时入账的 USD 扣回钱包余额（充值订单退款用），
// 返回实际扣减的 USD 金额。依赖调用方传入携带事务的 ctx，并持有订单行锁。
func deductWalletForRefundTx(ctx context.Context, orderID int64, tenantID int64) (decimal.Decimal, error) {
	// 1. 定位履约时的入账流水，取当时换算的 USD。优先按 related_id 精确匹配；
	//    兼容存量数据（related_id 补记前的流水）按 description 前缀匹配。
	type creditRow struct {
		WalletId int64           `json:"wallet_id"`
		Amount   decimal.Decimal `json:"amount"`
	}
	var credit *creditRow
	err := dao.BilTransactions.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("type", "recharge").
		Where("related_type", "order").
		Where("related_id", orderID).
		Fields("wallet_id, amount").
		Scan(&credit)
	if err != nil {
		return billing.Zero, err
	}
	if credit == nil {
		err = dao.BilTransactions.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("type", "recharge").
			WhereLike("description", fmt.Sprintf("Recharge: order #%d (%%", orderID)).
			Fields("wallet_id, amount").
			Scan(&credit)
		if err != nil {
			return billing.Zero, err
		}
	}
	if credit == nil {
		return billing.Zero, common.NewBadRequestError("未找到该订单的充值入账流水，无法确定应扣回的美元金额")
	}
	if !credit.Amount.GreaterThan(billing.Zero) {
		return billing.Zero, common.NewBadRequestError("充值入账流水金额异常，无法退款")
	}

	// 2. 扣回余额与累计充值（cumulative_recharge 同步回退，保持等级门槛口径真实）。
	//    Redis 权威化架构：可用余额门槛在 Redis 权威值上原子判断（balance - frozen >= amount，
	//    不足说明充值额度已被消费，继续退款平台将净亏，拒绝并提示人工协商处理）；
	//    bil_wallets.balance 由物化器随后从 Redis 覆盖，DB 侧只回退累计充值。
	balanceAfter, frozenAfter, ok, err := billing.DebitWalletRedis(ctx, tenantID, credit.Amount)
	if err != nil {
		return billing.Zero, err
	}
	if !ok {
		return billing.Zero, common.NewBadRequestError("钱包可用余额不足（充值额度可能已被消费），无法退款")
	}

	if _, err = g.DB().Ctx(ctx).Exec(ctx,
		"UPDATE bil_wallets SET cumulative_recharge = cumulative_recharge - ?, updated_at = NOW() WHERE tenant_id = ?",
		credit.Amount, tenantID); err != nil {
		return credit.Amount, err
	}

	// 3. 记录扣减流水（金额记负值，与 consume 类型的方向约定一致；快照取 Redis 返回值）
	if _, err = dao.BilTransactions.Ctx(ctx).Insert(do.BilTransactions{
		TenantId:     tenantID,
		WalletId:     credit.WalletId,
		Type:         "refund",
		Amount:       credit.Amount.Neg(),
		BalanceAfter: balanceAfter,
		FrozenAfter:  frozenAfter,
		RelatedId:    orderID,
		RelatedType:  "order",
		Description:  fmt.Sprintf("Refund: order #%d, deduct USD %.6f (original recharge credit)", orderID, credit.Amount.InexactFloat64()),
	}); err != nil {
		return credit.Amount, err
	}

	return credit.Amount, nil
}

// OrderComplete 手动完成订单
func (s *sAdmin) OrderComplete(ctx context.Context, req *v1.OrderCompleteReq) (*v1.OrderCompleteRes, error) {
	adminUserID := common.GetCtxUserID(ctx)

	orderNo, err := getOrderForComplete(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	payment.LockOrder(orderNo)
	defer payment.UnlockOrder(orderNo)

	if err := markOrderPaidByAdmin(ctx, req.Id, adminUserID); err != nil {
		return nil, err
	}

	if err := payment.FulfillOrder(ctx, req.Id); err != nil {
		return nil, err
	}

	return &v1.OrderCompleteRes{}, nil
}

// GetPaymentChannels 获取所有渠道配置（单例模式，从 sys_options 读取）
func (s *sAdmin) GetPaymentChannels(ctx context.Context, _ *v1.PaymentChannelListReq) (*v1.PaymentChannelListRes, error) {
	return &v1.PaymentChannelListRes{List: payment.ListAllChannels(ctx)}, nil
}

// SavePaymentChannel 保存指定渠道的配置（整体覆盖）
func (s *sAdmin) SavePaymentChannel(ctx context.Context, req *v1.PaymentChannelSaveReq) (*v1.PaymentChannelSaveRes, error) {
	// 校验 config JSON 是否合法
	_, err := payment.ParseChannelConfig(req.Channel, req.Config)
	if err != nil {
		return nil, common.NewBadRequestError("配置 JSON 格式无效: " + err.Error())
	}
	if err := payment.SaveChannelConfig(ctx, req.Channel, req.Config); err != nil {
		return nil, err
	}
	return &v1.PaymentChannelSaveRes{}, nil
}

// GetPaymentSettings 获取全局支付设置。
func (s *sAdmin) GetPaymentSettings(ctx context.Context, _ *v1.PaymentSettingsGetReq) (*v1.PaymentSettingsGetRes, error) {
	settings, err := payment.GetGlobalPaymentSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.PaymentSettingsGetRes{
		AmountOptions:   settings.AmountOptions,
		AmountDiscount:  settings.AmountDiscount,
		MinTopUp:        settings.MinTopUp,
		Currency:        settings.Currency,
		CallbackBaseURL: settings.CallbackBaseURL,
	}, nil
}

// UpdatePaymentSettings 更新全局支付设置。
func (s *sAdmin) UpdatePaymentSettings(ctx context.Context, req *v1.PaymentSettingsUpdateReq) (*v1.PaymentSettingsUpdateRes, error) {
	settings := &payment.GlobalPaymentSettings{
		AmountOptions:   req.AmountOptions,
		AmountDiscount:  req.AmountDiscount,
		MinTopUp:        req.MinTopUp,
		Currency:        req.Currency,
		CallbackBaseURL: req.CallbackBaseURL,
	}
	if err := payment.SaveGlobalPaymentSettings(ctx, settings); err != nil {
		return nil, err
	}
	return &v1.PaymentSettingsUpdateRes{}, nil
}

// getOrderForComplete 获取待完成的订单信息（供 OrderComplete 方法内部调用）。
func getOrderForComplete(ctx context.Context, orderID int64) (orderNo string, err error) {
	var order *struct {
		OrderNo string `json:"order_no"`
		Status  string `json:"status"`
	}
	err = dao.OrdOrders.Ctx(ctx).
		Where("id", orderID).Scan(&order)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return "", err
	}
	if order == nil {
		return "", common.NewNotFoundError("订单")
	}
	if order.Status != "pending" {
		return "", common.NewBadRequestError("订单状态不是待支付，无法完成")
	}
	return order.OrderNo, nil
}

// markOrderPaidByAdmin 将订单标记为已支付（管理员手动完成）。
func markOrderPaidByAdmin(ctx context.Context, orderID int64, adminUserID int64) error {
	_, err := dao.OrdOrders.Ctx(ctx).
		Where("id", orderID).
		Where("status", "pending").
		Data(do.OrdOrders{
			Status:    "paid",
			PaidAt:    gtime.Now(),
			PaymentNo: "ADMIN_" + fmt.Sprint(adminUserID),
		}).Update()
	return err
}

// ExportOrders exports order list to CSV or Excel.
func (s *sAdmin) ExportOrders(ctx context.Context, req *v1.OrderExportReq) (*v1.OrderExportRes, error) {
	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "order_no", Header: "订单号"},
		{Field: "tenant_id", Header: "租户ID"},
		{Field: "order_type", Header: "订单类型"},
		{Field: "final_amount", Header: "最终金额"},
		{Field: "payment_channel", Header: "支付渠道"},
		{Field: "status", Header: "状态"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   req.Format,
		Filename: "订单_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	orderFields := "id, order_no, tenant_id, order_type, final_amount, payment_channel, status, created_at"

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			query := dao.OrdOrders.Ctx(ctx)
			if req.Status != "" {
				query = query.Where("status", req.Status)
			}
			if req.TenantID != "" {
				query = query.Where("tenant_id", req.TenantID)
			}
			var batch []struct {
				Id             int64       `json:"id"`
				OrderNo        string      `json:"order_no"`
				TenantId       int64       `json:"tenant_id"`
				OrderType      string      `json:"order_type"`
				FinalAmount    float64     `json:"final_amount"`
				PaymentChannel string      `json:"payment_channel"`
				Status         string      `json:"status"`
				CreatedAt      *gtime.Time `json:"created_at"`
			}
			if err := query.Fields(orderFields).OrderDesc("created_at").Limit(1000).Offset(offset).Scan(&batch); err != nil {
				return
			}
			for _, o := range batch {
				if !yield(map[string]any{
					"id":              o.Id,
					"order_no":        o.OrderNo,
					"tenant_id":       o.TenantId,
					"order_type":      o.OrderType,
					"final_amount":    o.FinalAmount,
					"payment_channel": o.PaymentChannel,
					"status":          o.Status,
					"created_at":      o.CreatedAt.String(),
				}) {
					return
				}
			}
			if len(batch) < 1000 {
				break
			}
			offset += 1000
		}
	})
}

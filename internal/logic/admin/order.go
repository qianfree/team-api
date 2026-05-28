package admin

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/os/gtime"
	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"

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
	record, err := dao.OrdOrders.Ctx(ctx).
		Where("id", req.Id).
		One()
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, common.NewNotFoundError("订单")
	}
	return &v1.OrderDetailRes{Data: record.Map()}, nil
}

// RefundOrder 发起退款
func (s *sAdmin) RefundOrder(ctx context.Context, req *v1.OrderRefundReq) (*v1.OrderRefundRes, error) {
	adminUserID := common.GetCtxUserID(ctx)

	var order struct {
		TenantID       int64   `json:"tenant_id"`
		FinalAmount    float64 `json:"final_amount"`
		Status         string  `json:"status"`
		PaymentChannel string  `json:"payment_channel"`
	}
	err := dao.OrdOrders.Ctx(ctx).
		Where("id", req.Id).
		Scan(&order)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if order.Status != "paid" && order.Status != "fulfilled" {
		return nil, common.NewBadRequestError("订单状态不支持退款")
	}

	_, err = dao.OrdOrders.Ctx(ctx).
		Where("id", req.Id).
		Data(do.OrdOrders{
			Status: "refunding",
		}).Update()
	if err != nil {
		return nil, err
	}

	_, err = dao.OrdRefunds.Ctx(ctx).Insert(do.OrdRefunds{
		OrderId:        req.Id,
		TenantId:       order.TenantID,
		Amount:         order.FinalAmount,
		Reason:         req.Reason,
		Status:         "approved",
		PaymentChannel: order.PaymentChannel,
		ApprovedBy:     adminUserID,
		ApprovedAt:     gtime.Now(),
	})
	if err != nil {
		return nil, err
	}

	return &v1.OrderRefundRes{}, nil
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
	var order struct {
		OrderNo string `json:"order_no"`
		Status  string `json:"status"`
	}
	err = dao.OrdOrders.Ctx(ctx).
		Where("id", orderID).Scan(&order)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return "", err
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

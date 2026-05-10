package tenant

import (
	"context"
	"fmt"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"time"

	v1 "github.com/qianfree/team-api/api/tenant/v1"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/payment"
	"github.com/qianfree/team-api/internal/utility/export"
)

// OrderList 获取租户订单列表
func (s *sTenant) OrderList(ctx context.Context, req *v1.TenantOrderListReq) (*v1.TenantOrderListRes, error) {
	role := ctxUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := ctxTenantID(ctx)
	page, pageSize := lcommon.NormalizePagination(req.Page, req.PageSize)
	status := req.Status

	orders := make([]*v1.TenantOrderItem, 0)
	var total int
	query := dao.OrdOrders.Ctx(ctx).Where("tenant_id", tenantID)
	if status != "" {
		query = query.Where("status", status)
	}
	var err error
	err = query.OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&orders, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.TenantOrderListRes{
		List:     orders,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// OrderDetail 获取订单详情
func (s *sTenant) OrderDetail(ctx context.Context, req *v1.TenantOrderDetailReq) (*v1.TenantOrderDetailRes, error) {
	role := ctxUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := ctxTenantID(ctx)
	var order map[string]any
	err := dao.OrdOrders.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Scan(&order)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, lcommon.NewNotFoundError("订单")
	}
	return &v1.TenantOrderDetailRes{Data: order}, nil
}

// OrderCreate 创建订单
func (s *sTenant) OrderCreate(ctx context.Context, req *v1.TenantOrderCreateReq) (*v1.TenantOrderCreateRes, error) {
	role := ctxUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)
	planID := req.PlanID
	months := req.Months

	if months <= 0 {
		months = 1
	}

	// 查套餐价格
	var plan struct {
		MonthlyPrice float64 `json:"monthly_price"`
		YearlyPrice  float64 `json:"yearly_price"`
		Status       string  `json:"status"`
	}
	err := dao.PlnPlans.Ctx(ctx).
		Where("id", planID).
		Scan(&plan)
	if err != nil {
		return nil, err
	}
	if plan.Status != "active" {
		return nil, lcommon.NewBusinessError(422, "套餐不可用")
	}

	var amount float64
	if months >= 12 {
		amount = plan.YearlyPrice
	} else {
		amount = plan.MonthlyPrice * float64(months)
	}

	// 生成订单号
	orderNo := fmt.Sprintf("ORD%s%04d", time.Now().Format("20060102150405"), time.Now().UnixNano()%10000)

	result, err := dao.OrdOrders.Ctx(ctx).Insert(do.OrdOrders{
		OrderNo:        orderNo,
		TenantId:       tenantID,
		UserId:         userID,
		OrderType:      "new_plan",
		PlanId:         planID,
		Amount:         amount,
		DiscountAmount: 0,
		FinalAmount:    amount,
		Currency:       "CNY",
		PaymentChannel: "mock",
		Status:         "pending",
		ExpiredAt:      gtime.Now().Add(30 * time.Minute),
		Description:    fmt.Sprintf("Order for new_plan"),
	})
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &v1.TenantOrderCreateRes{Data: g.Map{
		"id":           id,
		"order_no":     orderNo,
		"final_amount": amount,
		"status":       "pending",
	}}, nil
}

// OrderCancel 取消订单
func (s *sTenant) OrderCancel(ctx context.Context, req *v1.TenantOrderCancelReq) (*v1.TenantOrderCancelRes, error) {
	role := ctxUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := ctxTenantID(ctx)
	result, err := dao.OrdOrders.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Where("status", "pending").
		Data(do.OrdOrders{
			Status:      "cancelled",
			CancelledAt: gtime.Now(),
		}).Update()
	if err != nil {
		return nil, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, lcommon.NewBusinessError(422, "订单不存在或无法取消")
	}
	return &v1.TenantOrderCancelRes{}, nil
}

// OrderPay 支付订单
func (s *sTenant) OrderPay(ctx context.Context, req *v1.TenantOrderPayReq) (*v1.TenantOrderPayRes, error) {
	role := ctxUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := ctxTenantID(ctx)
	orderID := req.Id

	// channelID=0 走 mock 支付（向后兼容）
	if req.PaymentChannelID == 0 {
		if err := payment.MockPay(ctx, orderID); err != nil {
			return nil, err
		}
		return &v1.TenantOrderPayRes{Data: g.Map{"status": "paid"}}, nil
	}

	// 真实支付渠道
	orderNo, finalAmount, currency, orderType, description, err := getOrderForPay(ctx, tenantID, orderID)
	if err != nil {
		return nil, err
	}

	var channel struct {
		Channel   string `json:"channel"`
		Config    string `json:"config"`
		IsEnabled bool   `json:"is_enabled"`
	}
	err = dao.OrdPaymentChannels.Ctx(ctx).
		Where("id", req.PaymentChannelID).Scan(&channel)
	if err != nil {
		return nil, err
	}
	if !channel.IsEnabled {
		return nil, lcommon.NewBusinessError(422, "支付渠道已禁用")
	}

	updateOrderPaymentChannel(ctx, orderID, channel.Channel, req.PaymentMethod)

	provider := payment.GetProvider(channel.Channel)
	if provider == nil {
		return nil, lcommon.NewBusinessError(422, "不支持的支付渠道")
	}

	cfg, err := payment.ParseChannelConfig(channel.Channel, channel.Config)
	if err != nil {
		return nil, err
	}

	settings, _ := payment.GetGlobalPaymentSettings(ctx)
	baseURL := settings.CallbackBaseURL
	notifyURL := baseURL + "/api/payment/callback/" + gconv.String(req.PaymentChannelID)
	if channel.Channel == "stripe" {
		notifyURL = baseURL + "/api/payment/stripe/webhook/" + gconv.String(req.PaymentChannelID)
	}

	payOrder := &payment.PaymentOrder{
		OrderID:       orderID,
		OrderNo:       orderNo,
		TenantID:      tenantID,
		Amount:        finalAmount,
		Currency:      currency,
		OrderType:     orderType,
		Description:   description,
		PaymentMethod: req.PaymentMethod,
		NotifyURL:     notifyURL,
	}

	result, err := provider.CreatePayment(ctx, payOrder, cfg)
	if err != nil {
		return nil, err
	}

	return &v1.TenantOrderPayRes{Data: g.Map{
		"payment_url": result.PaymentURL,
		"payment_no":  result.PaymentNo,
		"params":      result.Params,
		"is_redirect": result.IsRedirect,
	}}, nil
}

// PaymentInfo 获取租户可用的支付信息（渠道列表、金额选项、折扣）
func (s *sTenant) PaymentInfo(ctx context.Context, req *v1.TenantPaymentInfoReq) (*v1.TenantPaymentInfoRes, error) {
	role := ctxUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}
	channels := make([]map[string]any, 0)
	err := dao.OrdPaymentChannels.Ctx(ctx).
		Where("is_enabled", true).
		OrderAsc("sort_order").
		Fields("id, channel, name, payment_type").
		Scan(&channels)
	if err != nil {
		return nil, err
	}

	return &v1.TenantPaymentInfoRes{Data: g.Map{
		"channels": channels,
	}}, nil
}

// getOrderForPay 获取待支付订单信息（供 OrderPay 内部调用）
func getOrderForPay(ctx context.Context, tenantID int64, orderID int64) (orderNo string, finalAmount float64, currency string, orderType string, description string, err error) {
	var order struct {
		OrderNo     string  `json:"order_no"`
		FinalAmount float64 `json:"final_amount"`
		Currency    string  `json:"currency"`
		OrderType   string  `json:"order_type"`
		Description string  `json:"description"`
		Status      string  `json:"status"`
	}
	err = dao.OrdOrders.Ctx(ctx).
		Where("id", orderID).Where("tenant_id", tenantID).Scan(&order)
	if err != nil {
		return
	}
	if order.Status != "pending" {
		err = lcommon.NewBusinessError(422, "订单状态不是待支付")
		return
	}
	return order.OrderNo, order.FinalAmount, order.Currency, order.OrderType, order.Description, nil
}

// updateOrderPaymentChannel 更新订单的支付渠道信息（供 OrderPay 内部调用）
func updateOrderPaymentChannel(ctx context.Context, orderID int64, channel, paymentMethod string) {
	dao.OrdOrders.Ctx(ctx).
		Where("id", orderID).
		Data(do.OrdOrders{
			PaymentChannel: channel,
			PaymentMethod:  paymentMethod,
		}).Update()
}

// ExportOrders exports the tenant order list as CSV or Excel.
func (s *sTenant) ExportOrders(ctx context.Context, req *v1.TenantOrderExportReq) (*v1.TenantOrderExportRes, error) {
	role := ctxUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}

	r := g.RequestFromCtx(ctx)
	format := req.Format
	if format == "" {
		format = "csv"
	}

	tenantID := ctxTenantID(ctx)

	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "order_no", Header: "订单号"},
		{Field: "order_type", Header: "订单类型"},
		{Field: "amount", Header: "金额"},
		{Field: "final_amount", Header: "最终金额"},
		{Field: "payment_channel", Header: "支付渠道"},
		{Field: "status", Header: "状态"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   format,
		Filename: "订单列表_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	if format == "xlsx" {
		orders := make([]*v1.TenantOrderItem, 0)
		query := dao.OrdOrders.Ctx(ctx).Where("tenant_id", tenantID)
		if req.Status != "" {
			query = query.Where("status", req.Status)
		}
		err := query.OrderDesc("created_at").Scan(&orders)
		if err != nil {
			return nil, err
		}

		data := make([]map[string]any, 0, len(orders))
		for _, o := range orders {
			data = append(data, map[string]any{
				"id":              o.Id,
				"order_no":        o.OrderNo,
				"order_type":      o.OrderType,
				"amount":          o.Amount,
				"final_amount":    o.FinalAmount,
				"payment_channel": o.PaymentChannel,
				"status":          o.Status,
				"created_at":      o.CreatedAt,
			})
		}
		return nil, export.WriteExcel(r, config, data)
	}

	return nil, export.StreamCSV(r, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			orders := make([]*v1.TenantOrderItem, 0)
			query := dao.OrdOrders.Ctx(ctx).Where("tenant_id", tenantID)
			if req.Status != "" {
				query = query.Where("status", req.Status)
			}
			err := query.OrderDesc("created_at").Limit(1000).Offset(offset).Scan(&orders)
			if err != nil {
				return
			}
			for _, o := range orders {
				if !yield(map[string]any{
					"id":              o.Id,
					"order_no":        o.OrderNo,
					"order_type":      o.OrderType,
					"amount":          o.Amount,
					"final_amount":    o.FinalAmount,
					"payment_channel": o.PaymentChannel,
					"status":          o.Status,
					"created_at":      o.CreatedAt,
				}) {
					return
				}
			}
			if len(orders) < 1000 {
				break
			}
			offset += 1000
		}
	})
}

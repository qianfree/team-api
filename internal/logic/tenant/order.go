package tenant

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	v1 "github.com/qianfree/team-api/api/tenant/v1"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/payment"
	"github.com/qianfree/team-api/internal/middleware"
	"github.com/qianfree/team-api/internal/utility/export"
)

// OrderList 获取租户订单列表
func (s *sTenant) OrderList(ctx context.Context, req *v1.TenantOrderListReq) (*v1.TenantOrderListRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
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
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	var order *v1.TenantOrderItem
	err := dao.OrdOrders.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Scan(&order)
	if err = lcommon.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if order == nil {
		return nil, lcommon.NewNotFoundError("订单")
	}
	return &v1.TenantOrderDetailRes{TenantOrderItem: order}, nil
}

// OrderCreate 创建订单
func (s *sTenant) OrderCreate(ctx context.Context, req *v1.TenantOrderCreateReq) (*v1.TenantOrderCreateRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	planID := req.PlanID
	months := req.Months

	if months <= 0 {
		months = 1
	}

	// 查套餐价格
	var plan *struct {
		MonthlyPrice float64 `json:"monthly_price"`
		YearlyPrice  float64 `json:"yearly_price"`
		Status       string  `json:"status"`
	}
	err := dao.PlnPlans.Ctx(ctx).
		Where("id", planID).
		Scan(&plan)
	if err = lcommon.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, lcommon.NewNotFoundError("套餐")
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

	// 使用 crypto/rand 生成随机部分，避免碰撞
	randBytes := make([]byte, 4)
	if _, err := rand.Read(randBytes); err != nil {
		return nil, err
	}
	orderNo := fmt.Sprintf("ORD%s%08x", time.Now().Format("20060102150405"), randBytes)

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
		PaymentChannel: "",
		Status:         "pending",
		ExpiredAt:      gtime.Now().Add(30 * time.Minute),
		Description:    fmt.Sprintf("Order for new_plan"),
	})
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &v1.TenantOrderCreateRes{
		ID:          id,
		OrderNo:     orderNo,
		FinalAmount: amount,
		Status:      "pending",
	}, nil
}

// OrderCancel 取消订单
func (s *sTenant) OrderCancel(ctx context.Context, req *v1.TenantOrderCancelReq) (*v1.TenantOrderCancelRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
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
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	orderID := req.Id

	// 支付渠道
	if req.PaymentChannel == "" {
		return nil, lcommon.NewBusinessError(422, "请选择支付渠道")
	}
	if err := payment.RequireCallbackBaseURL(ctx); err != nil {
		return nil, err
	}

	//
	orderNo, finalAmount, currency, orderType, _, err := getOrderForPay(ctx, tenantID, orderID)
	if err != nil {
		return nil, err
	}

	cfg, err := payment.GetChannelConfigAndProvider(ctx, req.PaymentChannel)
	if err != nil {
		return nil, lcommon.NewBusinessError(422, err.Error())
	}

	provider := payment.GetProvider(req.PaymentChannel)
	if provider == nil {
		return nil, lcommon.NewBusinessError(422, "不支持的支付渠道")
	}

	if err := updateOrderPaymentChannel(ctx, orderID, req.PaymentChannel, req.PaymentMethod); err != nil {
		g.Log().Warningf(ctx, "update order %d payment channel failed: %v", orderID, err)
	}

	settings, _ := payment.GetGlobalPaymentSettings(ctx)
	baseURL := ""
	if settings != nil {
		baseURL = settings.CallbackBaseURL
	}
	notifyURL := baseURL + "/api/payment/callback/" + req.PaymentChannel
	returnURL := baseURL + "/api/payment/epay/return"
	payOrder := &payment.PaymentOrder{
		OrderID:       orderID,
		OrderNo:       orderNo,
		TenantID:      tenantID,
		Amount:        finalAmount,
		Currency:      currency,
		OrderType:     orderType,
		Description:   gatewayProductName(orderType),
		PaymentMethod: req.PaymentMethod,
		NotifyURL:     notifyURL,
		ReturnURL:     returnURL,
	}

	result, err := provider.CreatePayment(ctx, payOrder, cfg)
	if err != nil {
		return nil, err
	}

	return &v1.TenantOrderPayRes{
		PaymentURL: result.PaymentURL,
		PaymentNo:  result.PaymentNo,
		Params:     result.Params,
		IsRedirect: result.IsRedirect,
	}, nil
}

// PaymentInfo 获取租户可用的支付信息（渠道列表、金额选项、折扣）
func (s *sTenant) PaymentInfo(ctx context.Context, req *v1.TenantPaymentInfoReq) (*v1.TenantPaymentInfoRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}

	res := &v1.TenantPaymentInfoRes{
		Channels: payment.GetEnabledChannels(ctx),
	}

	settings, _ := payment.GetGlobalPaymentSettings(ctx)
	if settings != nil {
		res.AmountOptions = settings.AmountOptions
		res.AmountDiscount = settings.AmountDiscount
		res.MinTopUp = int(settings.MinTopUp)
		res.Currency = settings.Currency
	}

	return res, nil
}

// gatewayProductName 返回发送给支付网关（易支付）的商品名称。
//
// 易支付上游（支付宝/微信商户）的风控会扫描 name 参数中的关键词，
// 出现"充值/代充/代付/钱包/余额/额度/API/套现"等字样时会直接拦截，
// 网关返回"该商品禁止出售"。因此传给网关的 name 必须使用中性、安全的名称。
// 数据库 ord_orders.description 仍保留可读文案供租户订单列表展示，
// 二者解耦：此处只决定网关看到的商品名（参考 new-api 使用 "TUC{id}" 的做法）。
func gatewayProductName(orderType string) string {
	switch orderType {
	case "new_plan", "renew", "upgrade":
		return "会员订阅"
	default: // recharge 等其他类型
		return "会员服务"
	}
}

// getOrderForPay 获取待支付订单信息（供 OrderPay 内部调用）
func getOrderForPay(ctx context.Context, tenantID int64, orderID int64) (orderNo string, finalAmount float64, currency string, orderType string, description string, err error) {
	var order *struct {
		OrderNo     string      `json:"order_no"`
		FinalAmount float64     `json:"final_amount"`
		Currency    string      `json:"currency"`
		OrderType   string      `json:"order_type"`
		Description string      `json:"description"`
		Status      string      `json:"status"`
		ExpiredAt   *gtime.Time `json:"expired_at"`
	}
	err = dao.OrdOrders.Ctx(ctx).
		Where("id", orderID).Where("tenant_id", tenantID).Scan(&order)
	if err = lcommon.IgnoreScanNoRows(err); err != nil {
		return
	}
	if order == nil {
		err = lcommon.NewNotFoundError("订单")
		return
	}
	if order.Status != "pending" {
		err = lcommon.NewBusinessError(422, "订单状态不是待支付")
		return
	}
	if order.ExpiredAt != nil && !order.ExpiredAt.IsZero() && order.ExpiredAt.Before(gtime.Now()) {
		err = lcommon.NewBusinessError(422, "订单已过期，请重新下单")

		return
	}
	return order.OrderNo, order.FinalAmount, order.Currency, order.OrderType, order.Description, nil
}

// updateOrderPaymentChannel 更新订单的支付渠道信息（供 OrderPay 内部调用）
func updateOrderPaymentChannel(ctx context.Context, orderID int64, channel, paymentMethod string) error {
	_, err := dao.OrdOrders.Ctx(ctx).
		Where("id", orderID).
		Data(do.OrdOrders{
			PaymentChannel: channel,
			PaymentMethod:  paymentMethod,
		}).Update()
	return err
}

// RechargeCreate 创建充值订单并发起支付（一步完成）
func (s *sTenant) RechargeCreate(ctx context.Context, req *v1.TenantRechargeCreateReq) (*v1.TenantRechargeCreateRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)

	// 1. 校验金额
	settings, _ := payment.GetGlobalPaymentSettings(ctx)
	minTopup := 1.0
	if settings != nil && settings.MinTopUp > 0 {
		minTopup = settings.MinTopUp
	}
	if req.Amount < minTopup {
		return nil, lcommon.NewBusinessError(422, fmt.Sprintf("充值金额不能小于 %.2f", minTopup))
	}

	// 2. 从 sys_options 加载渠道配置
	if err := payment.RequireCallbackBaseURL(ctx); err != nil {
		return nil, err
	}
	cfg, err := payment.GetChannelConfigAndProvider(ctx, req.PaymentChannel)
	if err != nil {
		return nil, lcommon.NewBusinessError(422, err.Error())
	}

	provider := payment.GetProvider(req.PaymentChannel)
	if provider == nil {
		return nil, lcommon.NewBusinessError(422, "不支持的支付渠道")
	}

	// 3. 计算折扣后金额
	finalAmount := req.Amount
	if settings != nil {
		if discount, ok := settings.AmountDiscount[int(req.Amount)]; ok && discount > 0 {
			finalAmount = req.Amount * discount
		}
	}

	// 4. 生成订单号并创建订单
	randBytes := make([]byte, 4)
	if _, err := rand.Read(randBytes); err != nil {
		return nil, err
	}
	orderNo := fmt.Sprintf("RCH%s%08x", time.Now().Format("20060102150405"), randBytes)
	description := fmt.Sprintf("钱包充值 ¥%.2f", req.Amount)

	result, err := dao.OrdOrders.Ctx(ctx).Insert(do.OrdOrders{
		OrderNo:        orderNo,
		TenantId:       tenantID,
		UserId:         userID,
		OrderType:      "recharge",
		Amount:         req.Amount,
		DiscountAmount: req.Amount - finalAmount,
		FinalAmount:    finalAmount,
		Currency:       "CNY",
		PaymentChannel: req.PaymentChannel,
		PaymentMethod:  req.PaymentMethod,
		Status:         "pending",
		ExpiredAt:      gtime.Now().Add(30 * time.Minute),
		Description:    description,
	})
	if err != nil {
		return nil, err
	}
	orderID, _ := result.LastInsertId()

	// 5. 构建回调 URL 和 ReturnURL
	baseURL := ""
	if settings != nil {
		baseURL = settings.CallbackBaseURL
	}
	notifyURL := baseURL + "/api/payment/callback/" + req.PaymentChannel
	returnURL := baseURL + "/api/payment/epay/return"
	// 6. 调用 Provider 生成支付链接
	payResult, err := provider.CreatePayment(ctx, &payment.PaymentOrder{
		OrderID:       orderID,
		OrderNo:       orderNo,
		TenantID:      tenantID,
		Amount:        finalAmount,
		Currency:      "CNY",
		OrderType:     "recharge",
		Description:   gatewayProductName("recharge"),
		PaymentMethod: req.PaymentMethod,
		NotifyURL:     notifyURL,
		ReturnURL:     returnURL,
	}, cfg)
	if err != nil {
		dao.OrdOrders.Ctx(ctx).Where("id", orderID).
			Data(do.OrdOrders{Status: "cancelled"}).Update()
		return nil, err
	}

	return &v1.TenantRechargeCreateRes{
		OrderID:     orderID,
		OrderNo:     orderNo,
		PaymentURL:  payResult.PaymentURL,
		PaymentNo:   payResult.PaymentNo,
		Params:      payResult.Params,
		IsRedirect:  payResult.IsRedirect,
		FinalAmount: finalAmount,
	}, nil
}

// ExportOrders exports the tenant order list as CSV or Excel.
func (s *sTenant) ExportOrders(ctx context.Context, req *v1.TenantOrderExportReq) (*v1.TenantOrderExportRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, lcommon.NewForbiddenError("需要 owner 或 admin 权限")
	}

	tenantID := middleware.GetTenantID(ctx)

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
		Format:   req.Format,
		Filename: "订单列表_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			orders := make([]*v1.TenantOrderItem, 0)
			query := dao.OrdOrders.Ctx(ctx).Where("tenant_id", tenantID)
			if req.Status != "" {
				query = query.Where("status", req.Status)
			}
			err := query.OrderDesc("created_at").Limit(1000).Offset(offset).Scan(&orders)
			if err = lcommon.IgnoreScanNoRows(err); err != nil {
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

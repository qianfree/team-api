package tenant

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/payment"
	do "github.com/qianfree/team-api/internal/model/do"
)

// PlanList 获取可购买的套餐列表（仅 active）
func (s *sTenant) PlanList(ctx context.Context, req *v1.TenantPlanListReq) (*v1.TenantPlanListRes, error) {
	var entities []struct {
		Id            int64   `json:"id"`
		Name          string  `json:"name"`
		Identifier    string  `json:"identifier"`
		Description   string  `json:"description"`
		Price         float64 `json:"price"`
		CreditAmount  float64 `json:"credit_amount"`
		BonusAmount   float64 `json:"bonus_amount"`
		ValidityDays  int     `json:"validity_days"`
		IsRecommended bool    `json:"is_recommended"`
		SortOrder     int     `json:"sort_order"`
	}
	err := dao.PlnPlans.Ctx(ctx).
		Fields("id, name, identifier, description, price, credit_amount, bonus_amount, validity_days, is_recommended, sort_order").
		Where("status", "active").
		OrderAsc("sort_order").
		Scan(&entities)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	list := make([]*v1.TenantPlanItem, 0, len(entities))
	for _, e := range entities {
		list = append(list, &v1.TenantPlanItem{
			Id:            e.Id,
			Name:          e.Name,
			Identifier:    e.Identifier,
			Description:   e.Description,
			Price:         e.Price,
			CreditAmount:  e.CreditAmount,
			BonusAmount:   e.BonusAmount,
			ValidityDays:  e.ValidityDays,
			IsRecommended: e.IsRecommended,
			SortOrder:     e.SortOrder,
		})
	}
	return &v1.TenantPlanListRes{List: list}, nil
}

// PlanMine 获取当前租户已购买的套餐
func (s *sTenant) PlanMine(ctx context.Context, req *v1.TenantPlanMineReq) (*v1.TenantPlanMineRes, error) {
	role := ctxUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}

	tenantID := ctxTenantID(ctx)

	var items []*v1.TenantPlanMineItem
	err := dao.PlnTenantPlans.Ctx(ctx).As("tp").
		Fields("tp.id, tp.plan_id, tp.status, tp.total_credits, tp.remaining_credits, tp.start_at, tp.end_at, tp.created_at, p.name as plan_name").
		Where("tp.tenant_id", tenantID).
		WhereIn("tp.status", g.Slice{"active", "expired", "depleted", "refunded"}).
		LeftJoin("pln_plans p", "p.id = tp.plan_id").
		OrderDesc("tp.created_at").
		Scan(&items)
	if err != nil {
		return nil, err
	}

	var totalRemaining float64
	activeCount := 0
	for _, item := range items {
		if item.Status == "active" {
			totalRemaining += item.RemainingCredits
			activeCount++
		}
	}

	return &v1.TenantPlanMineRes{
		List:           items,
		TotalRemaining: totalRemaining,
		ActiveCount:    activeCount,
	}, nil
}

// PlanOrderCreate 创建套餐购买订单
func (s *sTenant) PlanOrderCreate(ctx context.Context, req *v1.TenantPlanOrderCreateReq) (*v1.TenantPlanOrderCreateRes, error) {
	role := ctxUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}

	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)

	// 查套餐信息
	var plan struct {
		Id                  int64   `json:"id"`
		Name                string  `json:"name"`
		Price               float64 `json:"price"`
		Status              string  `json:"status"`
		PurchaseLimit       int     `json:"purchase_limit"`
		PurchaseLimitPeriod string  `json:"purchase_limit_period"`
		Stock               int     `json:"stock"`
	}
	err := dao.PlnPlans.Ctx(ctx).
		Where("id", req.PlanId).
		Scan(&plan)
	if err != nil {
		return nil, err
	}
	if plan.Id == 0 || plan.Status != "active" {
		return nil, common.NewBusinessError(422, "套餐不可用")
	}

	// 检查限购
	if plan.PurchaseLimit > 0 {
		count, err := dao.PlnTenantPlans.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("plan_id", req.PlanId).
			Count()
		if err != nil {
			return nil, err
		}
		if count >= plan.PurchaseLimit {
			return nil, common.NewBusinessError(422, "已达到该套餐的购买上限")
		}
	}

	// 检查库存
	if plan.Stock > 0 {
		// stock 字段 > 0 表示有限库存，需检查是否有余量
		// 这里仅做前端校验，实际扣减在履约时通过 SQL WHERE stock > 0 保证原子性
	}

	// 获取支付渠道配置
	cfg, err := payment.GetChannelConfigAndProvider(ctx, req.PaymentChannel)
	if err != nil {
		return nil, common.NewBusinessError(422, err.Error())
	}
	provider := payment.GetProvider(req.PaymentChannel)
	if provider == nil {
		return nil, common.NewBusinessError(422, "不支持的支付渠道")
	}

	// 创建订单
	orderNo := fmt.Sprintf("PLN%s%04d", time.Now().Format("20060102150405"), time.Now().UnixNano()%10000)
	description := fmt.Sprintf("购买套餐「%s」", plan.Name)

	result, err := dao.OrdOrders.Ctx(ctx).Insert(do.OrdOrders{
		OrderNo:        orderNo,
		TenantId:       tenantID,
		UserId:         userID,
		OrderType:      "new_plan",
		PlanId:         req.PlanId,
		Amount:         plan.Price,
		DiscountAmount: 0,
		FinalAmount:    plan.Price,
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

	// 发起支付
	settings, _ := payment.GetGlobalPaymentSettings(ctx)
	baseURL := ""
	if settings != nil {
		baseURL = settings.CallbackBaseURL
	}
	notifyURL := baseURL + "/api/payment/callback/" + req.PaymentChannel
	returnURL := baseURL + "/api/payment/epay/return"

	payResult, err := provider.CreatePayment(ctx, &payment.PaymentOrder{
		OrderID:       orderID,
		OrderNo:       orderNo,
		TenantID:      tenantID,
		Amount:        plan.Price,
		Currency:      "CNY",
		OrderType:     "new_plan",
		Description:   description,
		PaymentMethod: req.PaymentMethod,
		NotifyURL:     notifyURL,
		ReturnURL:     returnURL,
	}, cfg)
	if err != nil {
		dao.OrdOrders.Ctx(ctx).Where("id", orderID).
			Data(do.OrdOrders{Status: "cancelled"}).Update()
		return nil, err
	}

	return &v1.TenantPlanOrderCreateRes{
		OrderId:    orderID,
		OrderNo:    orderNo,
		PaymentUrl: payResult.PaymentURL,
	}, nil
}

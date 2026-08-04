package admin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/utility/export"
)

// GetDashboardStats 获取管理后台仪表盘统计
func (s *sAdmin) GetDashboardStats(ctx context.Context, req *v1.AdminDashboardReq) (*v1.AdminDashboardRes, error) {
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	monthStart := time.Now().Format("2006-01") + "-01"

	// 租户数
	tenantCount, err := dao.TntTenants.Ctx(ctx).
		Where("status", "active").
		Count()
	if err != nil {
		g.Log().Warningf(ctx, "GetDashboardStats: query tenant count failed: %v", err)
	}

	// 成员数
	memberCount, err := dao.TntUsers.Ctx(ctx).Count()
	if err != nil {
		g.Log().Warningf(ctx, "GetDashboardStats: query member count failed: %v", err)
	}

	// 活跃渠道
	activeChannels, err := dao.ChnChannels.Ctx(ctx).
		Where("status", "active").
		Count()
	if err != nil {
		g.Log().Warningf(ctx, "GetDashboardStats: query active channels failed: %v", err)
	}

	// 今日统计
	type dayStatsRow struct {
		Requests      int     `json:"requests"`
		ActiveTenants int     `json:"active_tenants"`
		InputTokens   int     `json:"input_tokens"`
		OutputTokens  int     `json:"output_tokens"`
		TotalCost     float64 `json:"total_cost"`
		SuccessRate   float64 `json:"success_rate"`
	}
	var todayRow dayStatsRow
	if err := dao.BilUsageLogs.Ctx(ctx).
		Where("created_at >= ?", today+" 00:00:00").
		Fields("COUNT(*) as requests, COUNT(DISTINCT tenant_id) as active_tenants, COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens, COALESCE(SUM(total_cost), 0) as total_cost, ROUND(COUNT(CASE WHEN status = 'success' THEN 1 END) * 100.0 / NULLIF(COUNT(*), 0), 2) as success_rate").
		Scan(&todayRow); err != nil {
		g.Log().Warningf(ctx, "GetDashboardStats: query today stats failed: %v", err)
	}

	// 昨日统计
	var yesterdayRow dayStatsRow
	if err := dao.BilUsageLogs.Ctx(ctx).
		Where("created_at >= ?", yesterday+" 00:00:00").
		Where("created_at < ?", today+" 00:00:00").
		Fields("COUNT(*) as requests, COUNT(DISTINCT tenant_id) as active_tenants, COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens, COALESCE(SUM(total_cost), 0) as total_cost, ROUND(COUNT(CASE WHEN status = 'success' THEN 1 END) * 100.0 / NULLIF(COUNT(*), 0), 2) as success_rate").
		Scan(&yesterdayRow); err != nil {
		g.Log().Warningf(ctx, "GetDashboardStats: query yesterday stats failed: %v", err)
	}

	// 本月统计
	var monthRow dayStatsRow
	if err := dao.BilUsageLogs.Ctx(ctx).
		Where("created_at >= ?", monthStart+" 00:00:00").
		Fields("COUNT(*) as requests, COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens, COALESCE(SUM(total_cost), 0) as total_cost").
		Scan(&monthRow); err != nil {
		g.Log().Warningf(ctx, "GetDashboardStats: query month stats failed: %v", err)
	}

	// 本月收入（已结算金额）
	var revenue struct {
		Total float64 `json:"total"`
	}
	if err := dao.BilRecords.Ctx(ctx).
		Where("status", "settled").
		Where("settled_at >= ?", monthStart+" 00:00:00").
		Fields("COALESCE(SUM(total_cost), 0) as total").
		Scan(&revenue); err != nil {
		g.Log().Warningf(ctx, "GetDashboardStats: query revenue stats failed: %v", err)
	}

	return &v1.AdminDashboardRes{
		Tenants:        tenantCount,
		Members:        memberCount,
		ActiveChannels: activeChannels,
		Today: &v1.DayStats{
			Requests:      todayRow.Requests,
			ActiveTenants: todayRow.ActiveTenants,
			InputTokens:   todayRow.InputTokens,
			OutputTokens:  todayRow.OutputTokens,
			TotalCost:     todayRow.TotalCost,
			SuccessRate:   todayRow.SuccessRate,
		},
		Yesterday: &v1.DayStats{
			Requests:      yesterdayRow.Requests,
			ActiveTenants: yesterdayRow.ActiveTenants,
			InputTokens:   yesterdayRow.InputTokens,
			OutputTokens:  yesterdayRow.OutputTokens,
			TotalCost:     yesterdayRow.TotalCost,
			SuccessRate:   yesterdayRow.SuccessRate,
		},
		Month: &v1.MonthStats{
			Requests:     monthRow.Requests,
			InputTokens:  monthRow.InputTokens,
			OutputTokens: monthRow.OutputTokens,
			TotalCost:    monthRow.TotalCost,
			Revenue:      revenue.Total,
		},
	}, nil
}

// GetDashboardTrends returns daily revenue and request trends for the past N days.
func (s *sAdmin) GetDashboardTrends(ctx context.Context, req *v1.AdminDashboardTrendsReq) (*v1.AdminDashboardTrendsRes, error) {
	days := req.Days
	if days <= 0 || days > 90 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	result, err := g.DB().Ctx(ctx).Raw(`
		SELECT
			DATE(created_at) as date,
			COUNT(*) as requests,
			COUNT(DISTINCT tenant_id) as active_tenants,
			COALESCE(SUM(total_cost), 0) as revenue
		FROM bil_usage_logs
		WHERE created_at >= ?
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`, startDate+" 00:00:00").All()
	if err != nil {
		return nil, err
	}

	records := result.List()
	return &v1.AdminDashboardTrendsRes{List: records}, nil
}

// GetTopTenants returns the top 10 tenants by revenue.
func (s *sAdmin) GetTopTenants(ctx context.Context, req *v1.AdminDashboardTopTenantsReq) (*v1.AdminDashboardTopTenantsRes, error) {
	days := req.Days
	if days <= 0 || days > 90 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	result, err := g.DB().Ctx(ctx).Raw(`
		SELECT
			t.id as tenant_id,
			t.name as tenant_name,
			COALESCE(SUM(ul.total_cost), 0) as total_cost,
			COUNT(*) as requests,
			COUNT(DISTINCT ul.user_id) as active_members
		FROM bil_usage_logs ul
		JOIN tnt_tenants t ON t.id = ul.tenant_id
		WHERE ul.created_at >= ?
		GROUP BY t.id, t.name
		ORDER BY total_cost DESC
		LIMIT 10
	`, startDate+" 00:00:00").All()
	if err != nil {
		return nil, err
	}

	records := result.List()
	return &v1.AdminDashboardTopTenantsRes{List: records}, nil
}

// GetModelDistribution returns the model usage distribution.
func (s *sAdmin) GetModelDistribution(ctx context.Context, req *v1.AdminDashboardModelDistributionReq) (*v1.AdminDashboardModelDistributionRes, error) {
	days := req.Days
	if days <= 0 || days > 90 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	result, err := g.DB().Ctx(ctx).Raw(`
		SELECT
			model_name,
			COUNT(*) as requests,
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost
		FROM bil_usage_logs
		WHERE created_at >= ?
		GROUP BY model_name
		ORDER BY total_cost DESC
		LIMIT 20
	`, startDate+" 00:00:00").All()
	if err != nil {
		return nil, err
	}

	records := result.List()
	return &v1.AdminDashboardModelDistributionRes{List: records}, nil
}

// GetModelHourlyCost 模型费用按小时堆叠统计，用于仪表盘堆叠柱状图：
// 取最近 N 小时内费用最高的 TopN 个模型，按小时聚合费用，非 TopN 模型归并为"其他"。
func (s *sAdmin) GetModelHourlyCost(ctx context.Context, req *v1.AdminDashboardModelHourlyReq) (*v1.AdminDashboardModelHourlyRes, error) {
	hours := req.Hours
	if hours <= 0 || hours > 168 {
		hours = 24
	}
	topN := req.TopN
	if topN <= 0 || topN > 20 {
		topN = 8
	}

	// 步骤1：取费用最高的 TopN 个模型名
	topResult, err := g.DB().Ctx(ctx).Raw(`
		SELECT model_name
		FROM bil_usage_logs
		WHERE created_at >= now() - ? * interval '1 hour'
		GROUP BY model_name
		ORDER BY SUM(total_cost) DESC
		LIMIT ?
	`, hours, topN).All()
	if err != nil {
		return nil, err
	}
	topModels := make([]string, 0, topN)
	for _, row := range topResult {
		if name := row["model_name"].String(); name != "" {
			topModels = append(topModels, name)
		}
	}

	// 步骤2：用 generate_series 预先生成完整的小时桶（保证无数据的小时也显示 X 轴刻度），
	// 再 LEFT JOIN 聚合数据；非 TopN 模型归为"其他"
	args := make([]any, 0, len(topModels)+2)
	modelExpr := "model_name"
	if len(topModels) > 0 {
		placeholders := make([]string, len(topModels))
		for i, m := range topModels {
			placeholders[i] = "?"
			args = append(args, m)
		}
		modelExpr = "CASE WHEN model_name IN (" + strings.Join(placeholders, ",") + ") THEN model_name ELSE '其他' END"
	}
	args = append(args, hours, hours)

	result, err := g.DB().Ctx(ctx).Raw(`
		WITH hourly_agg AS (
			SELECT
				date_trunc('hour', created_at) AS h,
				`+modelExpr+` AS model,
				SUM(total_cost) AS cost
			FROM bil_usage_logs
			WHERE created_at >= now() - ? * interval '1 hour'
			GROUP BY 1, 2
		),
		time_buckets AS (
			SELECT generate_series(
				date_trunc('hour', now()) - (? - 1) * interval '1 hour',
				date_trunc('hour', now()),
				interval '1 hour'
			) AS h
		)
		SELECT
			to_char(t.h, 'YYYY-MM-DD HH24:00') AS hour,
			a.model,
			COALESCE(a.cost, 0) AS cost
		FROM time_buckets t
		LEFT JOIN hourly_agg a ON a.h = t.h
		ORDER BY t.h
	`, args...).All()
	if err != nil {
		return nil, err
	}

	// pivot 为 hours（X 轴，含无数据小时）+ series（每个模型一条）
	// 无数据的小时桶经 LEFT JOIN 得到 model 为 NULL（转空串）：跳过其模型记录，
	// 但该小时仍纳入 hourList，从而在 X 轴保留刻度、柱体区域留空
	hourList := make([]string, 0)
	modelOrder := make([]string, 0)
	hourIdx := make(map[string]bool)
	modelIdx := make(map[string]bool)
	costs := make(map[string]float64) // key: hour + "\x00" + model
	for _, row := range result {
		hour := row["hour"].String()
		if !hourIdx[hour] {
			hourIdx[hour] = true
			hourList = append(hourList, hour)
		}
		model := row["model"].String()
		if model == "" {
			continue
		}
		if !modelIdx[model] {
			modelIdx[model] = true
			modelOrder = append(modelOrder, model)
		}
		costs[hour+"\x00"+model] += row["cost"].Float64()
	}

	// 图例顺序：TopN 模型（费用降序）在前，"其他"/"未知" 在后
	modelsOrdered := make([]string, 0, len(modelOrder))
	added := make(map[string]bool)
	for _, m := range topModels {
		if modelIdx[m] && !added[m] {
			modelsOrdered = append(modelsOrdered, m)
			added[m] = true
		}
	}
	for _, m := range modelOrder {
		if !added[m] {
			modelsOrdered = append(modelsOrdered, m)
			added[m] = true
		}
	}

	series := make([]v1.ModelHourlySeriesItem, 0, len(modelsOrdered))
	for _, m := range modelsOrdered {
		data := make([]float64, len(hourList))
		for i, h := range hourList {
			data[i] = costs[h+"\x00"+m]
		}
		series = append(series, v1.ModelHourlySeriesItem{
			Model: m,
			Data:  data,
		})
	}

	return &v1.AdminDashboardModelHourlyRes{
		Hours:  hourList,
		Models: modelsOrdered,
		Series: series,
	}, nil
}

// GetAllUsageLogs 获取所有租户的用量日志（管理后台）
func (s *sAdmin) GetAllUsageLogs(ctx context.Context, req *v1.AdminUsageLogListReq) (*v1.AdminUsageLogListRes, error) {
	if err := common.ValidateDateParam(req.StartDate, "开始日期"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateParam(req.EndDate, "结束日期"); err != nil {
		return nil, err
	}

	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var conditions []string
	var args []any

	if req.TenantID > 0 {
		conditions = append(conditions, "u.tenant_id = ?")
		args = append(args, req.TenantID)
	}
	if req.Username != "" {
		conditions = append(conditions, "t.username LIKE ?")
		args = append(args, "%"+req.Username+"%")
	}
	if req.Model != "" {
		conditions = append(conditions, "u.model_name = ?")
		args = append(args, req.Model)
	}
	if req.Status != "" {
		conditions = append(conditions, "u.status = ?")
		args = append(args, req.Status)
	}
	if req.RequestType > 0 {
		conditions = append(conditions, "u.request_type = ?")
		args = append(args, req.RequestType)
	}
	if req.StartDate != "" {
		conditions = append(conditions, "u.created_at >= ?")
		args = append(args, req.StartDate+" 00:00:00")
	}
	if req.EndDate != "" {
		conditions = append(conditions, "u.created_at <= ?")
		args = append(args, req.EndDate+" 23:59:59")
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	fromClause := "bil_usage_logs u LEFT JOIN tnt_users t ON u.user_id = t.id AND u.tenant_id = t.tenant_id LEFT JOIN tnt_projects p ON u.project_id = p.id LEFT JOIN tnt_tenants tn ON u.tenant_id = tn.id LEFT JOIN api_keys ak ON u.api_key_id = ak.id"

	countSQL := "SELECT COUNT(*) AS total FROM " + fromClause + where
	countResult, err := g.DB().Ctx(ctx).Query(ctx, countSQL, args...)
	if err != nil {
		return nil, err
	}
	total := 0
	if len(countResult) > 0 {
		total = countResult[0]["total"].Int()
	}

	dataSQL := `SELECT u.id, u.tenant_id, COALESCE(tn.name, '') AS tenant_name, u.user_id, COALESCE(t.username, '') AS username, u.project_id, COALESCE(p.name, '') AS project_name, u.api_key_id, COALESCE(ak.name, '') AS api_key_name, u.channel_id, u.channel_name, u.channel_type, u.model_name, u.requested_model, u.upstream_model, u.relay_mode, u.request_type, u.input_tokens, u.output_tokens, u.cache_creation_tokens, u.cache_read_tokens, u.cache_creation_5m_tokens, u.cache_creation_1h_tokens, u.reasoning_tokens, u.audio_input_tokens, u.audio_output_tokens, u.image_output_tokens, u.input_cost, u.output_cost, u.cache_creation_cost, u.cache_read_cost, u.total_cost, u.actual_cost, u.currency, u.billing_mode, u.billing_source, u.rate_multiplier, u.latency_ms, u.first_token_ms, u.status, u.error_message, u.retry_index, u.client_ip, u.user_agent, u.service_tier, u.reasoning_effort, u.stream_end_reason, u.image_count, u.image_size, u.pre_deduct_amount, u.refund_amount, u.supplement_amount, u.billing_summary, u.billing_snapshot, u.inbound_endpoint, u.request_id, u.task_id, u.created_at
		 FROM ` + fromClause + where + ` ORDER BY u.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, (page-1)*pageSize)
	result, err := g.DB().Ctx(ctx).Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, err
	}

	logs := make([]*v1.AdminUsageLogItem, 0, len(result))
	for _, row := range result {
		item := &v1.AdminUsageLogItem{}
		if err := row.Struct(item); err != nil {
			continue
		}
		logs = append(logs, item)
	}

	return &v1.AdminUsageLogListRes{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		List:     logs,
	}, nil
}

// GetAllBillingRecords 获取所有计费记录（管理后台）
func (s *sAdmin) GetAllBillingRecords(ctx context.Context, req *v1.AdminBillingRecordListReq) (*v1.AdminBillingRecordListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := dao.BilRecords.Ctx(ctx)
	if req.TenantID > 0 {
		query = query.Where("tenant_id", req.TenantID)
	}

	var total int
	records := make([]*v1.AdminBillingRecordItem, 0)
	err := query.OrderDesc("created_at").
		Fields("id, tenant_id, user_id, channel_id, model_name, relay_mode, input_tokens, output_tokens, input_price, output_price, total_cost, currency, status, settled_at, created_at").
		Page(page, pageSize).
		ScanAndCount(&records, &total, false)
	if err != nil {
		return nil, err
	}

	// 批量填充名称
	if len(records) > 0 {
		tenantIds, userIds, channelIds := make([]int64, 0), make([]int64, 0), make([]int64, 0)
		for _, r := range records {
			tenantIds = append(tenantIds, r.TenantId)
			userIds = append(userIds, r.UserId)
			channelIds = append(channelIds, r.ChannelId)
		}

		tenantNames := make(map[int64]string)
		tenantNameEntities := make([]struct {
			Id   int64  `orm:"id"`
			Name string `orm:"name"`
		}, 0)
		if err := dao.TntTenants.Ctx(ctx).Fields("id, name").WhereIn("id", tenantIds).Scan(&tenantNameEntities); err == nil {
			for _, e := range tenantNameEntities {
				tenantNames[e.Id] = e.Name
			}
		}

		userNames := make(map[int64]string)
		userNameEntities := make([]struct {
			Id          int64  `orm:"id"`
			DisplayName string `orm:"display_name"`
		}, 0)
		if err := dao.TntUsers.Ctx(ctx).Fields("id, display_name").WhereIn("id", userIds).Scan(&userNameEntities); err == nil {
			for _, e := range userNameEntities {
				userNames[e.Id] = e.DisplayName
			}
		}

		channelNames := make(map[int64]string)
		channelNameEntities := make([]struct {
			Id   int64  `orm:"id"`
			Name string `orm:"name"`
		}, 0)
		if err := dao.ChnChannels.Ctx(ctx).Fields("id, name").WhereIn("id", channelIds).Scan(&channelNameEntities); err == nil {
			for _, e := range channelNameEntities {
				channelNames[e.Id] = e.Name
			}
		}

		for _, r := range records {
			r.TenantName = tenantNames[r.TenantId]
			r.UserName = userNames[r.UserId]
			r.ChannelName = channelNames[r.ChannelId]
		}
	}

	return &v1.AdminBillingRecordListRes{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		List:     records,
	}, nil
}

// GetTenantWallets 获取所有租户钱包（管理后台）
func (s *sAdmin) GetTenantWallets(ctx context.Context, req *v1.AdminWalletListReq) (*v1.AdminWalletListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := dao.BilWallets.Ctx(ctx)

	var total int
	wallets := make([]*v1.AdminWalletItem, 0)
	err := query.OrderDesc("updated_at").
		Fields("id, tenant_id, balance, frozen_balance, warning_threshold, currency, created_at, updated_at").
		Page(page, pageSize).
		ScanAndCount(&wallets, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.AdminWalletListRes{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		List:     wallets,
	}, nil
}

// AdjustBalance 调整租户余额（管理后台）
func (s *sAdmin) AdjustBalance(ctx context.Context, req *v1.AdminWalletAdjustReq) (*v1.AdminWalletAdjustRes, error) {
	tenantID := req.TenantID
	// 入口即转 decimal（float64 仅允许出现在 API 边界），后续 SQL 参数与流水全程 decimal 直传
	amount := billing.NewFromFloat(req.Amount)
	description := req.Description

	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 原子更新余额，避免并发竞态。扣减（负数）时以【可用余额】（balance - frozen_balance）
		// 为下限：frozen_balance 是支付中/退款中的占用，穿透冻结会破坏预扣一致性。
		updateQuery := "UPDATE bil_wallets SET balance = balance + ?, updated_at = ? WHERE tenant_id = ?"
		args := []any{amount, gtime.Now(), tenantID}
		if amount.IsNegative() {
			updateQuery += " AND balance - frozen_balance >= ?"
			args = append(args, amount.Neg())
		}
		result, err := g.DB().Ctx(ctx).Exec(ctx, updateQuery, args...)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return common.NewBadRequestError("钱包不存在或可用余额不足")
		}

		// 查询更新后的余额，用于记录流水
		var wallet *struct {
			ID            int64           `json:"id"`
			Balance       decimal.Decimal `json:"balance"`
			FrozenBalance decimal.Decimal `json:"frozen_balance"`
		}
		if err = dao.BilWallets.Ctx(ctx).
			Where("tenant_id", tenantID).
			Fields("id, balance, frozen_balance").
			Scan(&wallet); err != nil {
			return err
		}
		if wallet == nil {
			return common.NewBadRequestError("钱包不存在")
		}

		// 记录流水
		if _, err = dao.BilTransactions.Ctx(ctx).Insert(do.BilTransactions{
			TenantId:     tenantID,
			WalletId:     wallet.ID,
			Type:         "adjust",
			Amount:       amount,
			BalanceAfter: wallet.Balance,
			FrozenAfter:  wallet.FrozenBalance,
			Description:  description,
		}); err != nil {
			return gerror.Wrapf(err, "record balance adjust transaction")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 事务提交后清除钱包两级缓存（进程内 + Redis），否则调整后的余额在 300s TTL 内
	// 不生效——下调余额时租户仍按旧的高余额预扣，可短时超支。
	billing.InvalidateWallet(ctx, tenantID)

	// 管理员调整余额后，重置低余额预警标记（可能余额已恢复到阈值以上）
	billing.ResetLowBalanceNotified(ctx, tenantID)

	return &v1.AdminWalletAdjustRes{}, nil
}

// OfflineRecharge 线下充值入账（管理后台）
// 场景：用户线下银行转账（人民币 CNY），运营确认到账后按平台汇率换算为 USD 入账。
// 与 AdjustBalance 的区别：走正规充值链路——余额与累计充值同步累加、触发等级检查、
// 流水类型为 recharge，并在描述中携带 CNY 快照（原始人民币 + 汇率 + 入账 USD + 转账流水号），
// 供现金对账与开票追溯。
func (s *sAdmin) OfflineRecharge(ctx context.Context, req *v1.AdminWalletOfflineRechargeReq) (*v1.AdminWalletOfflineRechargeRes, error) {
	tenantID := req.TenantID
	cnyAmount := billing.NewFromFloat(req.Amount)
	if !cnyAmount.IsPositive() {
		return nil, common.NewBadRequestError("入账金额必须大于 0")
	}

	// 唯一换汇点：CNY→USD 只经 billing.ConvertCNYToUSD（与充值履约 FulfillOrder 同一函数），
	// 汇率取一次用于入账与快照，保证换算可重建。
	rate := billing.GetExchangeRateCNYToUSD(ctx)
	usdAmount := billing.ConvertCNYToUSD(ctx, req.Amount)
	if usdAmount.IsZero() {
		return nil, common.NewBadRequestError("按当前汇率换算后到账金额为 0，请检查汇率配置")
	}

	// 转账流水号软去重（防重复入账）：同一租户下已入账过该流水号则拒绝。
	// 非强原子，配合前端 loading 禁用按钮，拦截人工重复提交足够。
	if req.TransactionNo != "" {
		count, err := dao.BilTransactions.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("type", "recharge").
			Where("description like ?", "%转账流水号 "+req.TransactionNo).Count()
		if err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, common.NewBadRequestError("该转账流水号已入账，请勿重复操作")
		}
	}

	var creditedUsd decimal.Decimal
	var balanceAfter decimal.Decimal

	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 钱包存在性校验
		var wallet *struct {
			ID int64 `json:"id"`
		}
		if err := dao.BilWallets.Ctx(ctx).
			Where("tenant_id", tenantID).
			Fields("id").
			Scan(&wallet); err != nil {
			return err
		}
		if wallet == nil {
			return common.NewBadRequestError("钱包不存在")
		}

		// 原子入账：余额与累计充值同步累加（与 creditWalletTx 一致），
		// 单条 UPDATE 避免 read-modify-write 竞态，并发下不丢更新。
		if _, err := g.DB().Ctx(ctx).Exec(ctx,
			"UPDATE bil_wallets SET balance = balance + ?, cumulative_recharge = cumulative_recharge + ?, updated_at = NOW() WHERE id = ?",
			usdAmount, usdAmount, wallet.ID); err != nil {
			return err
		}

		// 读回入账后余额，用于流水快照
		var bal struct {
			Balance       decimal.Decimal `json:"balance"`
			FrozenBalance decimal.Decimal `json:"frozen_balance"`
		}
		if err := dao.BilWallets.Ctx(ctx).
			Where("id", wallet.ID).
			Fields("balance, frozen_balance").
			Scan(&bal); err != nil {
			return err
		}

		// 流水（type=recharge）：描述拼装 CNY 快照 + 转账流水号，供现金对账与开票追溯
		desc := fmt.Sprintf("线下充值入账 CNY %.2f × 汇率 %.6f = USD %s", cnyAmount.InexactFloat64(), rate, usdAmount)
		if req.Description != "" {
			desc = req.Description + "；" + desc
		}
		if req.TransactionNo != "" {
			desc += "；转账流水号 " + req.TransactionNo
		}
		if _, err := dao.BilTransactions.Ctx(ctx).Insert(do.BilTransactions{
			TenantId:     tenantID,
			WalletId:     wallet.ID,
			Type:         "recharge",
			Amount:       usdAmount,
			BalanceAfter: bal.Balance,
			FrozenAfter:  bal.FrozenBalance,
			Description:  desc,
		}); err != nil {
			return gerror.Wrapf(err, "record offline recharge transaction")
		}

		// 充值后检查租户等级（仅升不降）并重置低余额预警标记
		if err := billing.CheckAndUpgradeLevel(ctx, tenantID); err != nil {
			return gerror.Wrapf(err, "check upgrade level after offline recharge")
		}
		billing.ResetLowBalanceNotified(ctx, tenantID)

		creditedUsd = usdAmount
		balanceAfter = bal.Balance
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 事务提交后清除钱包两级缓存（进程内 + Redis），避免 300s TTL 内读到旧余额
	billing.InvalidateWallet(ctx, tenantID)

	return &v1.AdminWalletOfflineRechargeRes{
		CreditedUSD: billing.InexactFloat64(creditedUsd),
		Rate:        rate,
		Balance:     billing.InexactFloat64(balanceAfter),
	}, nil
}

// GetWalletInfo 获取租户钱包信息（管理后台）
func (s *sAdmin) GetWalletInfo(ctx context.Context, req *v1.AdminWalletInfoReq) (*v1.AdminWalletInfoRes, error) {
	type walletRow struct {
		ID               int64    `json:"id"`
		Balance          float64  `json:"balance"`
		FrozenBalance    float64  `json:"frozen_balance"`
		WarningThreshold *float64 `json:"warning_threshold"`
	}
	var w *walletRow
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", req.TenantID).
		Fields("id, balance, frozen_balance, warning_threshold").
		Scan(&w)
	if err != nil || w == nil {
		return nil, common.NewNotFoundError("钱包")
	}

	return &v1.AdminWalletInfoRes{
		Balance:          w.Balance,
		FrozenBalance:    w.FrozenBalance,
		WarningThreshold: w.WarningThreshold,
	}, nil
}

// GetWalletTransactions 获取租户钱包交易流水（管理后台）
func (s *sAdmin) GetWalletTransactions(ctx context.Context, req *v1.AdminWalletTransactionListReq) (*v1.AdminWalletTransactionListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	// 获取钱包 ID
	var w *struct {
		ID int64 `json:"id"`
	}
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", req.TenantID).
		Fields("id").
		Scan(&w)
	if err != nil || w == nil {
		return nil, common.NewNotFoundError("钱包")
	}

	query := dao.BilTransactions.Ctx(ctx).Where("wallet_id", w.ID)
	if req.Type != "" {
		query = query.Where("type", req.Type)
	}
	var total int
	records := make([]*v1.AdminWalletTransactionItem, 0)
	err = query.OrderDesc("created_at").
		Fields("id, type, amount, balance_after, frozen_after, description, user_id, request_id, model_name, created_at").
		Page(page, pageSize).
		ScanAndCount(&records, &total, false)
	if err != nil {
		return nil, err
	}

	// 批量关联用户名：user_id → tnt_users.username（consume 类型为实际消费用户，其余类型一般为空）。
	// 复用审计模块的批量查询工具，一次 IN 查询解决，避免逐行 N+1。
	if len(records) > 0 {
		userKeys := make([]string, 0, len(records))
		seen := make(map[int64]bool, len(records))
		for _, r := range records {
			if r.UserId > 0 && !seen[r.UserId] {
				seen[r.UserId] = true
				userKeys = append(userKeys, fmt.Sprintf("%d:%d", req.TenantID, r.UserId))
			}
		}
		if len(userKeys) > 0 {
			userMap := common.BatchQueryUserNames(ctx, userKeys)
			for _, r := range records {
				if r.UserId > 0 {
					r.Username = userMap[fmt.Sprintf("%d:%d", req.TenantID, r.UserId)]
				}
			}
		}
	}

	return &v1.AdminWalletTransactionListRes{
		List:     records,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// SetWarningThreshold 设置租户钱包预警阈值（管理后台）
func (s *sAdmin) SetWarningThreshold(ctx context.Context, req *v1.AdminWalletSetWarningThresholdReq) (*v1.AdminWalletSetWarningThresholdRes, error) {
	type walletRow struct {
		ID int64 `json:"id"`
	}
	var w *walletRow
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", req.TenantID).
		Fields("id").
		Scan(&w)
	if err != nil || w == nil {
		return nil, common.NewNotFoundError("钱包")
	}

	thresholdDecimal := billing.NewFromFloat(req.Threshold)
	_, err = dao.BilWallets.Ctx(ctx).
		Where("id", w.ID).
		Data(do.BilWallets{
			WarningThreshold: &thresholdDecimal,
		}).Update()
	if err != nil {
		return nil, err
	}

	// 阈值变更后重置预警标记，使新阈值能触发新的预警
	billing.ResetLowBalanceNotified(ctx, req.TenantID)

	return &v1.AdminWalletSetWarningThresholdRes{}, nil
}

// frozenReleaseMinAge 手动释放冻结的保护期。冻结时长低于此值的预扣大概率仍有请求在途
// （长流式/realtime 会话），释放会使该请求失去超扣保护，必须显式 force 并由前端二次确认。
const frozenReleaseMinAge = 10 * time.Minute

// frozenTaskInfo 冻结项关联的异步任务信息（用于释放护栏判断）
type frozenTaskInfo struct {
	Status         string `json:"status"`
	BillingSettled bool   `json:"billing_settled"`
}

// isTaskTerminal 异步任务是否已到终态
func isTaskTerminal(status string) bool {
	return status == "SUCCESS" || status == "FAILURE"
}

// frozenItemGuard 计算单笔冻结项的释放护栏结论。
// 返回：是否可释放、是否需要强制、拦截原因。
func frozenItemGuard(task *frozenTaskInfo, age time.Duration) (releasable bool, needForce bool, blockReason string) {
	if task != nil {
		if !isTaskTerminal(task.Status) {
			// 任务仍在推进：预扣由任务结算流程负责（成功结算/失败退款/30分钟超时退款），
			// 手动释放会让后续结算认领不到冻结、track 终态错乱
			return false, false, "关联异步任务进行中，任务完成或超时后将自动结算/退款"
		}
		if !task.BillingSettled {
			// 终态但未结算：轮询器 15 秒内会重试结算/退款，手动释放只会与之竞争
			return false, false, "关联异步任务待结算，系统将自动重试，请稍后刷新"
		}
		// 终态且已结算但 track 仍 frozen：结算认领异常留下的真孤儿，允许释放
	}
	if age < frozenReleaseMinAge {
		return true, true, ""
	}
	return true, false, ""
}

// queryFrozenTaskInfos 批量查询冻结项关联的异步任务（request_id 剥离 _adjust 后缀后关联 tsk_model_tasks）
func queryFrozenTaskInfos(ctx context.Context, tenantID int64, requestIDs []string) map[string]*frozenTaskInfo {
	result := make(map[string]*frozenTaskInfo)
	if len(requestIDs) == 0 {
		return result
	}
	baseIDs := make([]string, 0, len(requestIDs))
	seen := make(map[string]bool, len(requestIDs))
	for _, id := range requestIDs {
		base := strings.TrimSuffix(id, "_adjust")
		if !seen[base] {
			seen[base] = true
			baseIDs = append(baseIDs, base)
		}
	}

	type taskRow struct {
		RequestId      string `json:"request_id"`
		Status         string `json:"status"`
		BillingSettled bool   `json:"billing_settled"`
	}
	var rows []taskRow
	if err := dao.TskModelTasks.Ctx(ctx).
		Where("tenant_id", tenantID).
		WhereIn("request_id", baseIDs).
		Fields("request_id, status, billing_settled").
		Scan(&rows); err != nil {
		g.Log().Warningf(ctx, "query frozen task infos: tenant=%d: %v", tenantID, err)
		return result
	}
	for _, r := range rows {
		result[r.RequestId] = &frozenTaskInfo{Status: r.Status, BillingSettled: r.BillingSettled}
	}
	return result
}

// GetWalletFrozenItems 获取租户钱包冻结明细（管理后台）。
// 数据源为 DB 预扣追踪表（释放操作的权威依据），而非 Redis 明细缓存。
func (s *sAdmin) GetWalletFrozenItems(ctx context.Context, req *v1.AdminWalletFrozenItemListReq) (*v1.AdminWalletFrozenItemListRes, error) {
	type trackRow struct {
		RequestId string          `json:"request_id"`
		ModelName string          `json:"model_name"`
		Amount    decimal.Decimal `json:"amount"`
		CreatedAt *gtime.Time     `json:"created_at"`
	}
	var rows []trackRow
	err := dao.BilPredeductTracks.Ctx(ctx).
		Where("tenant_id", req.TenantID).
		Where("status", "frozen").
		Fields("request_id, model_name, amount, created_at").
		OrderAsc("created_at").
		Scan(&rows)
	if err != nil {
		return nil, err
	}

	requestIDs := make([]string, 0, len(rows))
	for _, r := range rows {
		requestIDs = append(requestIDs, r.RequestId)
	}
	taskInfos := queryFrozenTaskInfos(ctx, req.TenantID, requestIDs)

	now := time.Now()
	items := make([]*v1.AdminWalletFrozenItem, 0, len(rows))
	for _, r := range rows {
		age := now.Sub(r.CreatedAt.Time)
		task := taskInfos[strings.TrimSuffix(r.RequestId, "_adjust")]
		releasable, needForce, blockReason := frozenItemGuard(task, age)
		taskStatus := ""
		if task != nil {
			taskStatus = task.Status
		}
		items = append(items, &v1.AdminWalletFrozenItem{
			RequestID:   r.RequestId,
			ModelName:   r.ModelName,
			Amount:      billing.InexactFloat64(r.Amount),
			CreatedAt:   r.CreatedAt.String(),
			AgeSeconds:  int64(age.Seconds()),
			Releasable:  releasable,
			NeedForce:   needForce,
			BlockReason: blockReason,
			TaskStatus:  taskStatus,
		})
	}

	return &v1.AdminWalletFrozenItemListRes{List: items}, nil
}

// ReleaseWalletFrozenItem 按笔释放冻结（管理后台运维逃生舱）。
// 释放走 billing.UnfreezePreDeduct 的 status='frozen' 原子 claim 路径：与并发结算竞争安全、
// 幂等、逐笔精确，绝不直接改写 bil_wallets.frozen_balance 汇总值。
func (s *sAdmin) ReleaseWalletFrozenItem(ctx context.Context, req *v1.AdminWalletFrozenReleaseReq) (*v1.AdminWalletFrozenReleaseRes, error) {
	type trackRow struct {
		Amount    decimal.Decimal `json:"amount"`
		Status    string          `json:"status"`
		CreatedAt *gtime.Time     `json:"created_at"`
	}
	var track *trackRow
	err := dao.BilPredeductTracks.Ctx(ctx).
		Where("tenant_id", req.TenantID).
		Where("request_id", req.RequestID).
		Fields("amount, status, created_at").
		Scan(&track)
	if err != nil {
		return nil, err
	}
	if track == nil {
		return nil, common.NewNotFoundError("冻结项")
	}
	if track.Status != "frozen" {
		return nil, common.NewBadRequestError(fmt.Sprintf("该冻结项已处理（当前状态：%s），无需释放", track.Status))
	}

	// 护栏一：关联异步任务的预扣由任务结算流程负责，禁止手动释放（force 也不放行）
	task := queryFrozenTaskInfos(ctx, req.TenantID, []string{req.RequestID})[strings.TrimSuffix(req.RequestID, "_adjust")]
	age := time.Since(track.CreatedAt.Time)
	releasable, needForce, blockReason := frozenItemGuard(task, age)
	if !releasable {
		return nil, common.NewBadRequestError(blockReason)
	}

	// 护栏二：保护期内（可能仍有请求在途）必须显式强制释放
	if needForce && !req.Force {
		return nil, common.NewBadRequestError(fmt.Sprintf(
			"该笔冻结仅 %.0f 分钟，对应请求可能仍在进行中（长流式/实时会话），释放将使其失去超扣保护；确认后请使用强制释放",
			age.Minutes()))
	}

	if err := billing.UnfreezePreDeduct(ctx, req.TenantID, req.RequestID); err != nil {
		return nil, err
	}

	// 业务日志补充释放原因与操作上下文（HTTP 层审计由 OperationLog 中间件记录）
	g.Log().Infof(ctx, "admin release frozen prededuct: tenant=%d request=%s amount=%s age=%s force=%v reason=%s",
		req.TenantID, req.RequestID, track.Amount, age.Round(time.Second), req.Force, req.Reason)

	return &v1.AdminWalletFrozenReleaseRes{
		ReleasedAmount: billing.InexactFloat64(track.Amount),
	}, nil
}

// GetAllTransactions 获取所有租户交易流水（管理后台）
func (s *sAdmin) GetAllTransactions(ctx context.Context, req *v1.AdminTransactionListReq) (*v1.AdminTransactionListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := billing.BuildTransactionQuery(ctx, billing.TransactionQueryParams{
		TenantID:  req.TenantID,
		Type:      req.Type,
		Username:  req.Username,
		ModelName: req.ModelName,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
	})

	type transactionRow struct {
		Id           int64       `json:"id"`
		TenantId     int64       `json:"tenant_id"`
		TenantName   string      `json:"tenant_name"`
		Type         string      `json:"type"`
		Amount       float64     `json:"amount"`
		BalanceAfter float64     `json:"balance_after"`
		Description  string      `json:"description"`
		UserId       int64       `json:"user_id"`
		Username     string      `json:"username"`
		RequestId    string      `json:"request_id"`
		ModelName    string      `json:"model_name"`
		CreatedAt    *gtime.Time `json:"created_at"`
	}

	var records []*transactionRow
	var total int
	err := query.Fields("bil_transactions.id, bil_transactions.tenant_id, COALESCE(tn.name, '') AS tenant_name, bil_transactions.type, bil_transactions.amount, bil_transactions.balance_after, bil_transactions.description, bil_transactions.user_id, COALESCE(tu.username, '') AS username, bil_transactions.request_id, bil_transactions.model_name, bil_transactions.created_at").
		LeftJoin("tnt_users tu", "bil_transactions.user_id = tu.id AND bil_transactions.tenant_id = tu.tenant_id").
		LeftJoin("tnt_tenants tn", "bil_transactions.tenant_id = tn.id").
		OrderDesc("bil_transactions.created_at").
		Page(page, pageSize).
		ScanAndCount(&records, &total, false)
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = make([]*transactionRow, 0)
	}

	list := make([]*v1.AdminTransactionItem, 0, len(records))
	for _, r := range records {
		list = append(list, &v1.AdminTransactionItem{
			Id:           r.Id,
			TenantId:     r.TenantId,
			TenantName:   r.TenantName,
			Type:         r.Type,
			Amount:       r.Amount,
			BalanceAfter: r.BalanceAfter,
			Description:  r.Description,
			UserId:       r.UserId,
			Username:     r.Username,
			RequestId:    r.RequestId,
			ModelName:    r.ModelName,
			CreatedAt:    r.CreatedAt.String(),
		})
	}

	return &v1.AdminTransactionListRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetDashboardChannelHealth 获取渠道健康概览（最不健康的5个活跃渠道）
func (s *sAdmin) GetDashboardChannelHealth(ctx context.Context, req *v1.AdminDashboardChannelHealthReq) (*v1.AdminDashboardChannelHealthRes, error) {
	var items []v1.ChannelHealthItem
	err := g.DB().Ctx(ctx).Raw(`
		SELECT
			c.id as channel_id,
			c.name as channel_name,
			c.status,
			h.health_score,
			h.success_rate,
			CAST(h.latency_ms AS INTEGER) as latency_ms
		FROM chn_health_scores h
		JOIN chn_channels c ON c.id = h.channel_id
		WHERE c.status IN ('active', 'testing')
		ORDER BY h.health_score ASC
		LIMIT 5
	`).Scan(&items)
	if err != nil {
		return nil, err
	}

	return &v1.AdminDashboardChannelHealthRes{List: items}, nil
}

// GetDashboardRecentAlerts 获取最近5条告警
func (s *sAdmin) GetDashboardRecentAlerts(ctx context.Context, req *v1.AdminDashboardRecentAlertsReq) (*v1.AdminDashboardRecentAlertsRes, error) {
	var alerts []v1.RecentAlertItem
	err := dao.OpsAlertEvents.Ctx(ctx).
		Fields("id, rule_name, level, status, trigger_message, created_at").
		OrderDesc("created_at").
		Limit(5).
		Scan(&alerts)
	if err != nil {
		return nil, err
	}

	return &v1.AdminDashboardRecentAlertsRes{List: alerts}, nil
}

// ExportUsageLogs exports usage logs to CSV or Excel.
func (s *sAdmin) ExportUsageLogs(ctx context.Context, req *v1.AdminUsageLogExportReq) (*v1.AdminUsageLogExportRes, error) {
	if err := common.ValidateDateParam(req.StartDate, "开始日期"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateParam(req.EndDate, "结束日期"); err != nil {
		return nil, err
	}

	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "tenant_name", Header: "租户名称"},
		{Field: "username", Header: "用户名"},
		{Field: "model_name", Header: "模型"},
		{Field: "request_type", Header: "请求类型"},
		{Field: "input_tokens", Header: "输入Token"},
		{Field: "output_tokens", Header: "输出Token"},
		{Field: "total_cost", Header: "费用"},
		{Field: "status", Header: "状态"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   req.Format,
		Filename: "用量日志_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	buildUsageWhere := func() (string, []any) {
		var conditions []string
		var args []any
		if req.TenantID > 0 {
			conditions = append(conditions, "u.tenant_id = ?")
			args = append(args, req.TenantID)
		}
		if req.Username != "" {
			conditions = append(conditions, "t.username LIKE ?")
			args = append(args, "%"+req.Username+"%")
		}
		if req.Model != "" {
			conditions = append(conditions, "u.model_name = ?")
			args = append(args, req.Model)
		}
		if req.Status != "" {
			conditions = append(conditions, "u.status = ?")
			args = append(args, req.Status)
		}
		if req.RequestType > 0 {
			conditions = append(conditions, "u.request_type = ?")
			args = append(args, req.RequestType)
		}
		if req.StartDate != "" {
			conditions = append(conditions, "u.created_at >= ?")
			args = append(args, req.StartDate+" 00:00:00")
		}
		if req.EndDate != "" {
			conditions = append(conditions, "u.created_at <= ?")
			args = append(args, req.EndDate+" 23:59:59")
		}
		where := ""
		if len(conditions) > 0 {
			where = " WHERE " + strings.Join(conditions, " AND ")
		}
		return where, args
	}

	fromClause := "bil_usage_logs u LEFT JOIN tnt_users t ON u.user_id = t.id AND u.tenant_id = t.tenant_id LEFT JOIN tnt_tenants tn ON u.tenant_id = tn.id"
	selectFields := "u.id, COALESCE(tn.name, '') AS tenant_name, COALESCE(t.username, '') AS username, u.model_name, u.request_type, u.input_tokens, u.output_tokens, u.total_cost, u.status, u.created_at"

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			where, args := buildUsageWhere()
			sql := "SELECT " + selectFields + " FROM " + fromClause + where + " ORDER BY u.created_at DESC LIMIT ? OFFSET ?"
			batchArgs := append(args, 1000, offset)
			result, err := g.DB().Ctx(ctx).Query(ctx, sql, batchArgs...)
			if err != nil {
				g.Log().Errorf(ctx, "ExportUsageLogs: query batch at offset %d failed: %v", offset, err)
				return
			}
			for _, row := range result {
				createdAt := ""
				if t, ok := row["created_at"]; ok {
					createdAt = fmt.Sprintf("%v", t.Val())
				}
				if !yield(map[string]any{
					"id":            row["id"].Val(),
					"tenant_name":   row["tenant_name"].Val(),
					"username":      row["username"].Val(),
					"model_name":    row["model_name"].Val(),
					"request_type":  row["request_type"].Val(),
					"input_tokens":  row["input_tokens"].Val(),
					"output_tokens": row["output_tokens"].Val(),
					"total_cost":    row["total_cost"].Val(),
					"status":        row["status"].Val(),
					"created_at":    createdAt,
				}) {
					return
				}
			}
			if len(result) < 1000 {
				break
			}
			offset += 1000
		}
	})
}

// ExportBillingRecords exports billing records to CSV or Excel.
func (s *sAdmin) ExportBillingRecords(ctx context.Context, req *v1.AdminBillingRecordExportReq) (*v1.AdminBillingRecordExportRes, error) {
	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "tenant_name", Header: "租户名称"},
		{Field: "user_name", Header: "用户名"},
		{Field: "channel_name", Header: "渠道名称"},
		{Field: "model_name", Header: "模型"},
		{Field: "input_tokens", Header: "输入Token"},
		{Field: "output_tokens", Header: "输出Token"},
		{Field: "total_cost", Header: "费用"},
		{Field: "status", Header: "状态"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   req.Format,
		Filename: "计费记录_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	fetchRecords := func(offset, limit int) ([]map[string]any, error) {
		query := dao.BilRecords.Ctx(ctx)
		if req.TenantID > 0 {
			query = query.Where("tenant_id", req.TenantID)
		}
		var records []struct {
			Id           int64       `json:"id"`
			TenantId     int64       `json:"tenant_id"`
			UserId       int64       `json:"user_id"`
			ChannelId    int64       `json:"channel_id"`
			ModelName    string      `json:"model_name"`
			InputTokens  int         `json:"input_tokens"`
			OutputTokens int         `json:"output_tokens"`
			TotalCost    float64     `json:"total_cost"`
			Status       string      `json:"status"`
			CreatedAt    *gtime.Time `json:"created_at"`
		}
		if err := query.Fields("id, tenant_id, user_id, channel_id, model_name, input_tokens, output_tokens, total_cost, status, created_at").
			OrderDesc("created_at").Limit(limit).Offset(offset).Scan(&records); err != nil {
			return nil, err
		}

		// Batch resolve names
		tenantIds, userIds, channelIds := make([]int64, 0), make([]int64, 0), make([]int64, 0)
		for _, r := range records {
			tenantIds = append(tenantIds, r.TenantId)
			userIds = append(userIds, r.UserId)
			channelIds = append(channelIds, r.ChannelId)
		}

		tenantNames := make(map[int64]string)
		if len(tenantIds) > 0 {
			var entities []struct {
				Id   int64  `orm:"id"`
				Name string `orm:"name"`
			}
			if err := dao.TntTenants.Ctx(ctx).Fields("id, name").WhereIn("id", tenantIds).Scan(&entities); err == nil {
				for _, e := range entities {
					tenantNames[e.Id] = e.Name
				}
			}
		}

		userNames := make(map[int64]string)
		if len(userIds) > 0 {
			var entities []struct {
				Id          int64  `orm:"id"`
				DisplayName string `orm:"display_name"`
			}
			if err := dao.TntUsers.Ctx(ctx).Fields("id, display_name").WhereIn("id", userIds).Scan(&entities); err == nil {
				for _, e := range entities {
					userNames[e.Id] = e.DisplayName
				}
			}
		}

		channelNames := make(map[int64]string)
		if len(channelIds) > 0 {
			var entities []struct {
				Id   int64  `orm:"id"`
				Name string `orm:"name"`
			}
			if err := dao.ChnChannels.Ctx(ctx).Fields("id, name").WhereIn("id", channelIds).Scan(&entities); err == nil {
				for _, e := range entities {
					channelNames[e.Id] = e.Name
				}
			}
		}

		data := make([]map[string]any, len(records))
		for i, r := range records {
			data[i] = map[string]any{
				"id":            r.Id,
				"tenant_name":   tenantNames[r.TenantId],
				"user_name":     userNames[r.UserId],
				"channel_name":  channelNames[r.ChannelId],
				"model_name":    r.ModelName,
				"input_tokens":  r.InputTokens,
				"output_tokens": r.OutputTokens,
				"total_cost":    r.TotalCost,
				"status":        r.Status,
				"created_at":    r.CreatedAt.String(),
			}
		}
		return data, nil
	}

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			batch, err := fetchRecords(offset, 1000)
			if err != nil {
				g.Log().Errorf(ctx, "ExportBillingRecords: query batch at offset %d failed: %v", offset, err)
				return
			}
			for _, row := range batch {
				if !yield(row) {
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

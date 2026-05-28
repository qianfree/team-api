package admin

import (
	"context"
	"fmt"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/utility/export"
)

// GetDashboardStats 获取管理后台仪表盘统计
func (s *sAdmin) GetDashboardStats(ctx context.Context, req *v1.AdminDashboardReq) (*v1.AdminDashboardRes, error) {
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	monthStart := time.Now().Format("2006-01") + "-01"

	// 租户数
	tenantCount, _ := dao.TntTenants.Ctx(ctx).
		Where("status", "active").
		Count()

	// 成员数
	memberCount, _ := dao.TntUsers.Ctx(ctx).Count()

	// 活跃渠道
	activeChannels, _ := dao.ChnChannels.Ctx(ctx).
		Where("status", "active").
		Count()

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
	dao.BilUsageLogs.Ctx(ctx).
		Where("created_at >= ?", today+" 00:00:00").
		Fields("COUNT(*) as requests, COUNT(DISTINCT tenant_id) as active_tenants, COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens, COALESCE(SUM(total_cost), 0) as total_cost, ROUND(COUNT(CASE WHEN status = 'success' THEN 1 END) * 100.0 / NULLIF(COUNT(*), 0), 2) as success_rate").
		Scan(&todayRow)

	// 昨日统计
	var yesterdayRow dayStatsRow
	dao.BilUsageLogs.Ctx(ctx).
		Where("created_at >= ?", yesterday+" 00:00:00").
		Where("created_at < ?", today+" 00:00:00").
		Fields("COUNT(*) as requests, COUNT(DISTINCT tenant_id) as active_tenants, COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens, COALESCE(SUM(total_cost), 0) as total_cost, ROUND(COUNT(CASE WHEN status = 'success' THEN 1 END) * 100.0 / NULLIF(COUNT(*), 0), 2) as success_rate").
		Scan(&yesterdayRow)

	// 本月统计
	var monthRow dayStatsRow
	dao.BilUsageLogs.Ctx(ctx).
		Where("created_at >= ?", monthStart+" 00:00:00").
		Fields("COUNT(*) as requests, COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens, COALESCE(SUM(total_cost), 0) as total_cost").
		Scan(&monthRow)

	// 本月收入（已结算金额）
	var revenue struct {
		Total float64 `json:"total"`
	}
	dao.BilRecords.Ctx(ctx).
		Where("status", "settled").
		Where("settled_at >= ?", monthStart+" 00:00:00").
		Fields("COALESCE(SUM(total_cost), 0) as total").
		Scan(&revenue)

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

	result, err := g.DB().Ctx(ctx).Raw(fmt.Sprintf(`
		SELECT
			DATE(created_at) as date,
			COUNT(*) as requests,
			COUNT(DISTINCT tenant_id) as active_tenants,
			COALESCE(SUM(total_cost), 0) as revenue
		FROM bil_usage_logs
		WHERE created_at >= '%s 00:00:00'
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`, startDate)).All()
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

	result, err := g.DB().Ctx(ctx).Raw(fmt.Sprintf(`
		SELECT
			t.id as tenant_id,
			t.name as tenant_name,
			COALESCE(SUM(ul.total_cost), 0) as total_cost,
			COUNT(*) as requests,
			COUNT(DISTINCT ul.user_id) as active_members
		FROM bil_usage_logs ul
		JOIN tnt_tenants t ON t.id = ul.tenant_id
		WHERE ul.created_at >= '%s 00:00:00'
		GROUP BY t.id, t.name
		ORDER BY total_cost DESC
		LIMIT 10
	`, startDate)).All()
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

	result, err := g.DB().Ctx(ctx).Raw(fmt.Sprintf(`
		SELECT
			model_name,
			COUNT(*) as requests,
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost
		FROM bil_usage_logs
		WHERE created_at >= '%s 00:00:00'
		GROUP BY model_name
		ORDER BY total_cost DESC
		LIMIT 20
	`, startDate)).All()
	if err != nil {
		return nil, err
	}

	records := result.List()
	return &v1.AdminDashboardModelDistributionRes{List: records}, nil
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

	dataSQL := fmt.Sprintf(
		`SELECT u.id, u.tenant_id, COALESCE(tn.name, '') AS tenant_name, u.user_id, COALESCE(t.username, '') AS username, u.project_id, COALESCE(p.name, '') AS project_name, u.api_key_id, COALESCE(ak.name, '') AS api_key_name, u.channel_id, u.channel_name, u.channel_type, u.model_name, u.requested_model, u.upstream_model, u.relay_mode, u.request_type, u.input_tokens, u.output_tokens, u.cache_creation_tokens, u.cache_read_tokens, u.cache_creation_5m_tokens, u.cache_creation_1h_tokens, u.reasoning_tokens, u.audio_input_tokens, u.audio_output_tokens, u.image_output_tokens, u.input_cost, u.output_cost, u.cache_creation_cost, u.cache_read_cost, u.total_cost, u.actual_cost, u.currency, u.billing_mode, u.billing_source, u.rate_multiplier, u.latency_ms, u.first_token_ms, u.status, u.error_message, u.retry_index, u.client_ip, u.user_agent, u.service_tier, u.reasoning_effort, u.stream_end_reason, u.image_count, u.image_size, u.pre_deduct_amount, u.refund_amount, u.supplement_amount, u.billing_summary, u.billing_snapshot, u.inbound_endpoint, u.request_id, u.task_id, u.created_at
		 FROM %s%s ORDER BY u.created_at DESC LIMIT %d OFFSET %d`,
		fromClause, where, pageSize, (page-1)*pageSize,
	)
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
	amount := req.Amount
	description := req.Description

	// 原子更新余额，避免并发竞态
	db := g.DB()
	updateQuery := "UPDATE bil_wallets SET balance = balance + ?, updated_at = ? WHERE tenant_id = ?"
	args := []interface{}{amount, gtime.Now(), tenantID}
	if amount < 0 {
		updateQuery += " AND balance >= ?"
		args = append(args, -amount)
	}
	result, err := db.Exec(ctx, updateQuery, args...)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, gerror.New("钱包不存在或余额不足")
	}

	// 查询更新后的余额，用于记录流水
	var wallet struct {
		ID            int64   `json:"id"`
		Balance       float64 `json:"balance"`
		FrozenBalance float64 `json:"frozen_balance"`
	}
	err = dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("id, balance, frozen_balance").
		Scan(&wallet)
	if err != nil {
		return nil, err
	}

	// 记录流水
	dao.BilTransactions.Ctx(ctx).Insert(do.BilTransactions{
		TenantId:     tenantID,
		WalletId:     wallet.ID,
		Type:         "adjust",
		Amount:       amount,
		BalanceAfter: wallet.Balance,
		FrozenAfter:  wallet.FrozenBalance,
		Description:  description,
	})

	return &v1.AdminWalletAdjustRes{}, nil
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
		return nil, gerror.Newf("wallet not found for tenant %d", req.TenantID)
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
		return nil, gerror.Newf("wallet not found for tenant %d", req.TenantID)
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
		return nil, gerror.Newf("wallet not found for tenant %d", req.TenantID)
	}

	_, err = dao.BilWallets.Ctx(ctx).
		Where("id", w.ID).
		Data(do.BilWallets{
			WarningThreshold: req.Threshold,
		}).Update()
	return &v1.AdminWalletSetWarningThresholdRes{}, err
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
	err := g.DB().Ctx(ctx).Model("ops_alert_events").
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
			sql := fmt.Sprintf("SELECT %s FROM %s%s ORDER BY u.created_at DESC LIMIT 1000 OFFSET %d", selectFields, fromClause, where, offset)
			result, err := g.DB().Ctx(ctx).Query(ctx, sql, args...)
			if err != nil {
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

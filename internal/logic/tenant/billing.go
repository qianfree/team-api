package tenant

import (
	"context"
	"fmt"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/middleware"
	"github.com/qianfree/team-api/internal/utility/export"
)

// Wallet 获取租户钱包余额
func (s *sTenant) Wallet(ctx context.Context, req *v1.TenantWalletReq) (*v1.TenantWalletRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)

	var w *entity.BilWallets
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Scan(&w)
	if err != nil {
		return nil, err
	}
	if w == nil {
		// 钱包不存在，初始化（预警阈值默认 0 = 关闭，用户可自行开启）
		threshold := billing.Zero
		_, err = dao.BilWallets.Ctx(ctx).Insert(do.BilWallets{
			TenantId:         tenantID,
			Balance:          billing.Zero,
			FrozenBalance:    billing.Zero,
			WarningThreshold: &threshold,
			Currency:         billing.Currency(ctx),
		})
		if err != nil {
			return nil, err
		}
		return &v1.TenantWalletRes{
			Balance:          0,
			FrozenBalance:    0,
			AvailableBalance: 0,
			WarningThreshold: 0,
			Currency:         billing.Currency(ctx),
		}, nil
	}

	// 语义：warning_threshold = 0 表示关闭预警。该列当前可空，存量/建钱包路径漏设时会为 NULL，
	// 这里对 NULL 行按 0（关闭）返回，避免解引用 nil 指针
	warningThreshold := billing.Zero
	if w.WarningThreshold != nil {
		warningThreshold = *w.WarningThreshold
	}

	return &v1.TenantWalletRes{
		Balance:          billing.InexactFloat64(w.Balance),
		FrozenBalance:    billing.InexactFloat64(w.FrozenBalance),
		AvailableBalance: billing.InexactFloat64(billing.SubtractMoney(w.Balance, w.FrozenBalance)),
		WarningThreshold: billing.InexactFloat64(warningThreshold),
		Currency:         w.Currency,
	}, nil
}

// WalletTransactions 获取租户钱包流水
func (s *sTenant) WalletTransactions(ctx context.Context, req *v1.TenantWalletTransactionsReq) (*v1.TenantWalletTransactionsRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := billing.BuildTransactionQuery(ctx, billing.TransactionQueryParams{
		TenantID:  tenantID,
		Type:      req.Type,
		Username:  req.Username,
		ModelName: req.ModelName,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
	})

	if req.AmountMin != 0 {
		query = query.Where("bil_transactions.amount >= ?", req.AmountMin)
	}
	if req.AmountMax != 0 {
		query = query.Where("bil_transactions.amount <= ?", req.AmountMax)
	}

	type transactionRow struct {
		Id           int64       `json:"id"`
		Type         string      `json:"type"`
		Amount       float64     `json:"amount"`
		BalanceAfter float64     `json:"balance_after"`
		FrozenAfter  float64     `json:"frozen_after"`
		RelatedId    int64       `json:"related_id"`
		RelatedType  string      `json:"related_type"`
		Description  string      `json:"description"`
		UserId       int64       `json:"user_id"`
		Username     string      `json:"username"`
		RequestId    string      `json:"request_id"`
		ModelName    string      `json:"model_name"`
		ProjectId    int64       `json:"project_id"`
		ApiKeyId     int64       `json:"api_key_id"`
		TaskId       string      `json:"task_id"`
		CreatedAt    *gtime.Time `json:"created_at"`
	}

	// COUNT 与列表拆开执行：计数不需要用户名称列，筛选条件也不依赖 JOIN
	//（Username 过滤在 BuildTransactionQuery 内是 EXISTS 子查询），大表
	// bil_transactions 的计数免背联表。不用 ScanAndCount —— 它的 COUNT
	// 会携带模型上的全部 JOIN。
	total, err := query.Count()
	if err != nil {
		return nil, err
	}

	records := []transactionRow{}
	if total > 0 {
		if err := query.Fields("bil_transactions.id, bil_transactions.type, bil_transactions.amount, bil_transactions.balance_after, bil_transactions.frozen_after, bil_transactions.related_id, bil_transactions.related_type, bil_transactions.description, bil_transactions.user_id, COALESCE(tu.username, '') AS username, bil_transactions.request_id, bil_transactions.model_name, bil_transactions.project_id, bil_transactions.api_key_id, bil_transactions.task_id, bil_transactions.created_at").
			LeftJoin("tnt_users tu", "bil_transactions.user_id = tu.id AND bil_transactions.tenant_id = tu.tenant_id").
			OrderDesc("bil_transactions.created_at").
			Page(page, pageSize).
			Scan(&records); err != nil {
			return nil, err
		}
	}

	list := make([]map[string]any, 0, len(records))
	for _, r := range records {
		list = append(list, map[string]any{
			"id":            r.Id,
			"type":          r.Type,
			"amount":        r.Amount,
			"balance_after": r.BalanceAfter,
			"frozen_after":  r.FrozenAfter,
			"related_id":    r.RelatedId,
			"related_type":  r.RelatedType,
			"description":   r.Description,
			"user_id":       r.UserId,
			"username":      r.Username,
			"request_id":    r.RequestId,
			"model_name":    r.ModelName,
			"project_id":    r.ProjectId,
			"api_key_id":    r.ApiKeyId,
			"task_id":       r.TaskId,
			"created_at":    r.CreatedAt,
		})
	}

	return &v1.TenantWalletTransactionsRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// tenantUsageLogFilter 用量日志查询的筛选参数（列表 / 统计 / 导出三个入口共用）。
type tenantUsageLogFilter struct {
	TenantID    int64
	UserID      int64
	Role        string
	Username    string
	Model       string
	Status      string
	RequestType int
	StartDate   string
	EndDate     string
}

// buildTenantUsageLogFilter 构建用量日志查询的 WHERE 条件与绑定参数（不含 WHERE 前缀；
// tenant_id 恒在条件里，结果必非空）。三个入口共用，保证筛选口径一致。
//
// 条件只引用主表别名 u，不依赖任何 JOIN —— 计数与统计路径因此可以完全不联表，
// 只有需要展示名称的列表 / 导出才挂 LEFT JOIN。租户隔离（u.tenant_id）与
// member 角色只见自己（u.user_id）在此统一强制，调用方不得绕过。
func buildTenantUsageLogFilter(f tenantUsageLogFilter) (where string, args []any) {
	conditions := []string{"u.tenant_id = ?"}
	args = append(args, f.TenantID)

	// member 角色只能查看自己的用量日志
	if f.Role == "member" {
		conditions = append(conditions, "u.user_id = ?")
		args = append(args, f.UserID)
	} else if f.Username != "" {
		// EXISTS 子查询替代对 JOIN 别名 t.username 的引用：条件不再依赖联表，
		// 且避免把 tnt_users 拖成驱动表（EXISTS 内是主键等值探测，代价低）。
		// 语义与 LEFT JOIN 后过滤一致：用户不存在的行同样被过滤掉。
		conditions = append(conditions, "EXISTS (SELECT 1 FROM tnt_users ut WHERE ut.id = u.user_id AND ut.tenant_id = u.tenant_id AND ut.username LIKE ?)")
		args = append(args, "%"+f.Username+"%")
	}
	if f.Model != "" {
		conditions = append(conditions, "u.model_name = ?")
		args = append(args, f.Model)
	}
	if f.Status != "" {
		conditions = append(conditions, "u.status = ?")
		args = append(args, f.Status)
	}
	if f.RequestType > 0 {
		conditions = append(conditions, "u.request_type = ?")
		args = append(args, f.RequestType)
	}
	if f.StartDate != "" {
		conditions = append(conditions, "u.created_at >= ?")
		args = append(args, common.StartOfRange(f.StartDate))
	}
	if f.EndDate != "" {
		conditions = append(conditions, "u.created_at <= ?")
		args = append(args, common.EndOfRange(f.EndDate))
	}
	return strings.Join(conditions, " AND "), args
}

// UsageLogs 获取租户用量日志
func (s *sTenant) UsageLogs(ctx context.Context, req *v1.TenantUsageLogsReq) (*v1.TenantUsageLogsRes, error) {
	if err := common.ValidateDateTimeParam(req.StartDate, "开始时间"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateTimeParam(req.EndDate, "结束时间"); err != nil {
		return nil, err
	}

	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	where, args := buildTenantUsageLogFilter(tenantUsageLogFilter{
		TenantID:    tenantID,
		UserID:      userID,
		Role:        role,
		Username:    req.Username,
		Model:       req.Model,
		Status:      req.Status,
		RequestType: req.RequestType,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
	})

	// 列表查询不 JOIN mdl_models（model_display_name 仅导出使用）
	fromClause := "bil_usage_logs u LEFT JOIN tnt_users t ON u.user_id = t.id AND u.tenant_id = t.tenant_id LEFT JOIN tnt_projects p ON u.project_id = p.id LEFT JOIN api_keys ak ON u.api_key_id = ak.id"

	// 计数不展示任何名称，且筛选条件只引用 u.*（Username 过滤是 EXISTS 子查询），
	// 3 个展示用 LEFT JOIN 整体剥离 —— bil_usage_logs 是分区大表，列表每翻一页
	// 都要重算一次计数，这里是本接口最重的一次查询。
	countSQL := "SELECT COUNT(*) AS total FROM bil_usage_logs u WHERE " + where
	countResult, err := g.DB().Ctx(ctx).Query(ctx, countSQL, args...)
	if err != nil {
		return nil, err
	}
	total := 0
	if len(countResult) > 0 {
		total = countResult[0]["total"].Int()
	}

	// 白名单查询：仅返回租户端展示所需字段。
	// 禁止改回 SELECT u.* —— bil_usage_logs 含平台侧敏感字段，曾因此泄露
	// upstream_model（模型映射）、account_cost（上游成本/利润）、upstream_endpoint、
	// billing_snapshot（含上游模型与模型倍率）给租户。新增展示需求时在此追加列。
	dataSQL := fmt.Sprintf(
		`SELECT u.id, u.request_id, u.task_id, u.api_key_id, u.model_name, u.relay_mode, u.inbound_endpoint,
		       u.request_type, u.billing_mode, u.billing_source, u.rate_multiplier, u.status, u.retry_index,
		       u.stream_end_reason, u.error_message, u.client_ip, u.user_agent, u.service_tier, u.reasoning_effort,
		       u.input_tokens, u.output_tokens, u.cache_creation_tokens, u.cache_read_tokens,
		       u.cache_creation_5m_tokens, u.cache_creation_1h_tokens, u.reasoning_tokens,
		       u.audio_input_tokens, u.audio_output_tokens, u.image_output_tokens, u.image_count, u.image_size,
		       u.input_cost, u.output_cost, u.cache_creation_cost, u.cache_read_cost, u.total_cost, u.actual_cost,
		       u.latency_ms, u.first_token_ms, u.created_at, u.billing_summary,
		       COALESCE(t.username, '') AS username, COALESCE(p.name, '') AS project_name, COALESCE(ak.name, '') AS api_key_name
		 FROM %s WHERE %s ORDER BY u.created_at DESC LIMIT ? OFFSET ?`,
		fromClause, where,
	)
	args = append(args, pageSize, (page-1)*pageSize)
	result, err := g.DB().Ctx(ctx).Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, err
	}

	list := make([]map[string]any, 0, len(result))
	for _, row := range result {
		m := make(map[string]any, len(row))
		for k, v := range row {
			switch raw := v.Val().(type) {
			case []byte:
				s := string(raw)
				if f, err := strconv.ParseFloat(s, 64); err == nil {
					m[k] = f
				} else {
					m[k] = s
				}
			default:
				m[k] = raw
			}
		}
		list = append(list, m)
	}

	return &v1.TenantUsageLogsRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// UsageLogsSummary 用量日志统计汇总（与 UsageLogs 共用筛选口径：强制 tenant_id 隔离，
// member 角色只能统计自己的日志；总费用与租户端列表费用列同口径：actual_cost 优先，0/NULL 回退 total_cost）
func (s *sTenant) UsageLogsSummary(ctx context.Context, req *v1.TenantUsageLogsSummaryReq) (*v1.TenantUsageLogsSummaryRes, error) {
	if err := common.ValidateDateTimeParam(req.StartDate, "开始时间"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateTimeParam(req.EndDate, "结束时间"); err != nil {
		return nil, err
	}

	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)

	where, args := buildTenantUsageLogFilter(tenantUsageLogFilter{
		TenantID:    tenantID,
		UserID:      userID,
		Role:        role,
		Username:    req.Username,
		Model:       req.Model,
		Status:      req.Status,
		RequestType: req.RequestType,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
	})

	// 统计聚合不展示名称，Username 过滤已由 EXISTS 子查询承担，全程无需联表：
	// 大表 SUM 不再背负任何 join。
	summarySQL := `SELECT
		COALESCE(SUM(COALESCE(NULLIF(u.actual_cost, 0), u.total_cost)), 0) AS total_cost,
		COALESCE(SUM(u.output_tokens), 0) AS total_output_tokens,
		COALESCE(SUM(u.input_tokens), 0) AS total_input_tokens,
		COALESCE(SUM(u.cache_read_tokens), 0) AS total_cache_read
		FROM bil_usage_logs u WHERE ` + where

	summaryResult, err := g.DB().Ctx(ctx).Query(ctx, summarySQL, args...)
	if err != nil {
		return nil, err
	}
	// 无 GROUP BY 的聚合必返回一行；空结果兜底为零值
	if len(summaryResult) == 0 {
		return &v1.TenantUsageLogsSummaryRes{}, nil
	}

	var row struct {
		TotalCost         float64 `json:"total_cost"`
		TotalOutputTokens int64   `json:"total_output_tokens"`
		TotalInputTokens  int64   `json:"total_input_tokens"`
		TotalCacheRead    int64   `json:"total_cache_read"`
	}
	if err := summaryResult[0].Struct(&row); err != nil {
		return nil, err
	}

	// 缓存读取占比 = cache_read / input_tokens。
	// input_tokens 入库口径为「含缓存的总输入」（见 relay/common/usage.go TotalInputTokens），
	// cache_read 是其子集，分母直接取总和即可，禁止再加 cache_read（会重复计入导致占比偏低）。
	// 口径与 admin 侧 GetUsageLogSummary 保持一致。
	var cacheReadRatio float64
	if row.TotalInputTokens > 0 {
		cacheReadRatio = float64(row.TotalCacheRead) / float64(row.TotalInputTokens) * 100
	}

	return &v1.TenantUsageLogsSummaryRes{
		TotalCost:         row.TotalCost,
		TotalOutputTokens: row.TotalOutputTokens,
		CacheReadRatio:    cacheReadRatio,
	}, nil
}

// ExportUsageLogs exports the tenant usage logs as CSV or Excel.
func (s *sTenant) ExportUsageLogs(ctx context.Context, req *v1.TenantUsageLogsExportReq) (*v1.TenantUsageLogsExportRes, error) {
	if err := common.ValidateDateTimeParam(req.StartDate, "开始时间"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateTimeParam(req.EndDate, "结束时间"); err != nil {
		return nil, err
	}
	// 导出护栏：时间窗必填且不超上限（与管理后台导出同一规则）
	if err := common.ValidateExportTimeWindow(req.StartDate, req.EndDate, common.UsageLogExportMaxWindowDays); err != nil {
		return nil, err
	}

	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)

	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "username", Header: "用户名"},
		{Field: "model_display_name", Header: "模型显示名称"},
		{Field: "model_name", Header: "模型标识"},
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

	where, args := buildTenantUsageLogFilter(tenantUsageLogFilter{
		TenantID:    tenantID,
		UserID:      userID,
		Role:        role,
		Username:    req.Username,
		Model:       req.Model,
		Status:      req.Status,
		RequestType: req.RequestType,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
	})

	fromClause := "bil_usage_logs u LEFT JOIN tnt_users t ON u.user_id = t.id AND u.tenant_id = t.tenant_id LEFT JOIN tnt_projects p ON u.project_id = p.id LEFT JOIN api_keys ak ON u.api_key_id = ak.id LEFT JOIN mdl_models mdl ON u.model_name = mdl.model_id"

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		// keyset（游标）翻页替代 OFFSET：OFFSET 每翻一批都要重新扫过并丢弃前面的
		// 所有行，深翻页代价线性上涨；id 游标（bigserial 单调递增，走 PK 索引）
		// 让每批从上一批断点续读。排序 created_at DESC → id DESC，追加写表近似同序，
		// 且避免时间戳做游标时亚秒精度在驱动往返中丢失导致的跳行/重行。
		var cursorID int64
		for {
			dataSQL := `SELECT u.id, COALESCE(t.username, '') AS username, COALESCE(mdl.model_name, '') AS model_display_name, u.model_name, u.request_type,
			        u.input_tokens, u.output_tokens, u.total_cost, u.status, u.created_at
			 FROM ` + fromClause + ` WHERE ` + where
			exportArgs := append([]any{}, args...)
			if cursorID > 0 {
				dataSQL += " AND u.id < ?"
				exportArgs = append(exportArgs, cursorID)
			}
			dataSQL += " ORDER BY u.id DESC LIMIT 1000"
			result, err := g.DB().Ctx(ctx).Query(ctx, dataSQL, exportArgs...)
			if err != nil {
				g.Log().Errorf(ctx, "ExportUsageLogs: query batch failed (cursor id=%d): %v", cursorID, err)
				return
			}
			for _, row := range result {
				m := make(map[string]any, len(row))
				for k, v := range row {
					switch raw := v.Val().(type) {
					case []byte:
						s := string(raw)
						if f, err := strconv.ParseFloat(s, 64); err == nil {
							m[k] = f
						} else {
							m[k] = s
						}
					default:
						m[k] = raw
					}
				}
				if !yield(m) {
					return
				}
				cursorID = row["id"].Int64()
			}
			if len(result) < 1000 {
				break
			}
		}
	})
}

// ExportWalletTransactions exports the tenant wallet transactions as CSV or Excel.
func (s *sTenant) ExportWalletTransactions(ctx context.Context, req *v1.TenantWalletTransactionsExportReq) (*v1.TenantWalletTransactionsExportRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}

	tenantID := middleware.GetTenantID(ctx)

	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "type", Header: "类型"},
		{Field: "amount", Header: "金额"},
		{Field: "balance_after", Header: "变动后余额"},
		{Field: "description", Header: "描述"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   req.Format,
		Filename: "交易记录_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		// keyset 翻页：id 游标替代 OFFSET，避免深翻页时重复扫弃前面的行；
		// 排序取 id DESC（追加写表上与 created_at DESC 近似同序）
		var cursorID int64
		for {
			type transactionRow struct {
				Id           int64       `json:"id"`
				Type         string      `json:"type"`
				Amount       float64     `json:"amount"`
				BalanceAfter float64     `json:"balance_after"`
				UserId       int64       `json:"user_id"`
				RequestId    string      `json:"request_id"`
				ModelName    string      `json:"model_name"`
				Description  string      `json:"description"`
				CreatedAt    *gtime.Time `json:"created_at"`
			}

			q := dao.BilTransactions.Ctx(ctx).
				Where("tenant_id", tenantID)

			if req.Type != "" {
				q = q.Where("type", req.Type)
			}
			if req.StartDate != "" {
				q = q.Where("created_at >= ?", req.StartDate+" 00:00:00")
			}
			if req.EndDate != "" {
				q = q.Where("created_at <= ?", req.EndDate+" 23:59:59")
			}
			if req.AmountMin != 0 {
				q = q.Where("amount >= ?", req.AmountMin)
			}
			if req.AmountMax != 0 {
				q = q.Where("amount <= ?", req.AmountMax)
			}
			if req.ModelName != "" {
				q = q.Where("model_name LIKE ?", "%"+req.ModelName+"%")
			}
			if cursorID > 0 {
				q = q.Where("id < ?", cursorID)
			}

			var records []transactionRow
			err := q.Fields("id, type, amount, balance_after, user_id, request_id, model_name, description, created_at").
				OrderDesc("id").
				Limit(1000).
				Scan(&records)
			if err != nil {
				return
			}
			for _, r := range records {
				if !yield(map[string]any{
					"id":            r.Id,
					"type":          r.Type,
					"amount":        r.Amount,
					"balance_after": r.BalanceAfter,
					"description":   r.Description,
					"created_at":    r.CreatedAt,
				}) {
					return
				}
				cursorID = r.Id
			}
			if len(records) < 1000 {
				break
			}
		}
	})
}

// WalletFrozenItems 获取冻结明细
func (s *sTenant) WalletFrozenItems(ctx context.Context, req *v1.TenantWalletFrozenItemsReq) (*v1.TenantWalletFrozenItemsRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)

	items, err := billing.GetFrozenItems(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	details := make([]v1.FrozenItemDetail, 0, len(items))
	for _, item := range items {
		details = append(details, v1.FrozenItemDetail{
			RequestID: item.RequestID,
			ModelName: item.ModelName,
			Amount:    item.Amount,
			CreatedAt: item.CreatedAt,
			Remaining: item.Remaining,
		})
	}

	return &v1.TenantWalletFrozenItemsRes{Items: details}, nil
}

// UpdateWarningThreshold 修改钱包预警阈值
func (s *sTenant) UpdateWarningThreshold(ctx context.Context, req *v1.TenantWalletUpdateWarningThresholdReq) (*v1.TenantWalletUpdateWarningThresholdRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)

	threshold := billing.NewFromFloat(req.Threshold)
	_, err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Data(do.BilWallets{
			WarningThreshold: &threshold,
		}).Update()
	if err != nil {
		return nil, err
	}

	// 清除钱包静态字段缓存（Redis 钱包 hash 是权威余额，不失效）
	billing.InvalidateWalletStaticCache(ctx, tenantID)

	// 阈值变更后重置预警标记，使新阈值能触发新的预警
	billing.ResetLowBalanceNotified(ctx, tenantID)

	return &v1.TenantWalletUpdateWarningThresholdRes{}, nil
}

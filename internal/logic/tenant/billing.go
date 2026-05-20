package tenant

import (
	"context"
	"fmt"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/utility/export"
)

// Wallet 获取租户钱包余额
func (s *sTenant) Wallet(ctx context.Context, req *v1.TenantWalletReq) (*v1.TenantWalletRes, error) {
	role := ctxUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := ctxTenantID(ctx)

	type walletRow struct {
		Balance          float64 `json:"balance"`
		FrozenBalance    float64 `json:"frozen_balance"`
		WarningThreshold float64 `json:"warning_threshold"`
		Currency         string  `json:"currency"`
	}

	var w *walletRow
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("balance, frozen_balance, warning_threshold, currency").
		Scan(&w)
	if err != nil {
		return nil, err
	}
	if w == nil {
		// 钱包不存在，初始化
		// 钱包不存在，初始化
		_, err = dao.BilWallets.Ctx(ctx).Insert(do.BilWallets{
			TenantId:         tenantID,
			Balance:          0,
			FrozenBalance:    0,
			WarningThreshold: 1.00,
			Currency:         "USD",
		})
		if err != nil {
			return nil, err
		}
		return &v1.TenantWalletRes{
			Balance:          0,
			FrozenBalance:    0,
			AvailableBalance: 0,
			WarningThreshold: 1.00,
			Currency:         "USD",
		}, nil
	}

	return &v1.TenantWalletRes{
		Balance:          w.Balance,
		FrozenBalance:    w.FrozenBalance,
		AvailableBalance: w.Balance - w.FrozenBalance,
		WarningThreshold: w.WarningThreshold,
		Currency:         w.Currency,
	}, nil
}

// WalletTransactions 获取租户钱包流水
func (s *sTenant) WalletTransactions(ctx context.Context, req *v1.TenantWalletTransactionsReq) (*v1.TenantWalletTransactionsRes, error) {
	role := ctxUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := ctxTenantID(ctx)
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := dao.BilTransactions.Ctx(ctx).
		Where("tenant_id", tenantID)

	type transactionRow struct {
		Id           int64       `json:"id"`
		Type         string      `json:"type"`
		Amount       float64     `json:"amount"`
		BalanceAfter float64     `json:"balance_after"`
		FrozenAfter  float64     `json:"frozen_after"`
		RelatedType  string      `json:"related_type"`
		Description  string      `json:"description"`
		CreatedAt    *gtime.Time `json:"created_at"`
	}

	var records []transactionRow
	var err error
	var total int
	err = query.Fields("id, type, amount, balance_after, frozen_after, related_type, description, created_at").
		OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&records, &total, false)
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = []transactionRow{}
	}

	list := make([]map[string]any, 0, len(records))
	for _, r := range records {
		list = append(list, map[string]any{
			"id":            r.Id,
			"type":          r.Type,
			"amount":        r.Amount,
			"balance_after": r.BalanceAfter,
			"frozen_after":  r.FrozenAfter,
			"related_type":  r.RelatedType,
			"description":   r.Description,
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

// BillingRecords 获取租户计费记录
func (s *sTenant) BillingRecords(ctx context.Context, req *v1.TenantBillingRecordsReq) (*v1.TenantBillingRecordsRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)
	role := ctxUserRole(ctx)
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := dao.BilRecords.Ctx(ctx).
		Where("tenant_id", tenantID)

	// member 角色只能查看自己的账单记录
	if role == "member" {
		query = query.Where("user_id", userID)
	}

	type billingRecordRow struct {
		Id           int64       `json:"id"`
		ModelName    string      `json:"model_name"`
		RelayMode    string      `json:"relay_mode"`
		InputTokens  int         `json:"input_tokens"`
		OutputTokens int         `json:"output_tokens"`
		InputPrice   float64     `json:"input_price"`
		OutputPrice  float64     `json:"output_price"`
		TotalCost    float64     `json:"total_cost"`
		Currency     string      `json:"currency"`
		Status       string      `json:"status"`
		SettledAt    *gtime.Time `json:"settled_at"`
		CreatedAt    *gtime.Time `json:"created_at"`
	}

	var records []billingRecordRow
	var err error
	var total int
	err = query.Fields("id, model_name, relay_mode, input_tokens, output_tokens, input_price, output_price, total_cost, currency, status, settled_at, created_at").
		OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&records, &total, false)
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = []billingRecordRow{}
	}

	list := make([]map[string]any, 0, len(records))
	for _, r := range records {
		list = append(list, map[string]any{
			"id":            r.Id,
			"model_name":    r.ModelName,
			"relay_mode":    r.RelayMode,
			"input_tokens":  r.InputTokens,
			"output_tokens": r.OutputTokens,
			"input_price":   r.InputPrice,
			"output_price":  r.OutputPrice,
			"total_cost":    r.TotalCost,
			"currency":      r.Currency,
			"status":        r.Status,
			"settled_at":    r.SettledAt,
			"created_at":    r.CreatedAt,
		})
	}

	return &v1.TenantBillingRecordsRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// UsageLogs 获取租户用量日志
func (s *sTenant) UsageLogs(ctx context.Context, req *v1.TenantUsageLogsReq) (*v1.TenantUsageLogsRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)
	role := ctxUserRole(ctx)
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var conditions []string
	var args []any

	conditions = append(conditions, "u.tenant_id = ?")
	args = append(args, tenantID)

	// member 角色只能查看自己的用量日志
	if role == "member" {
		conditions = append(conditions, "u.user_id = ?")
		args = append(args, userID)
	} else if req.Username != "" {
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

	where := strings.Join(conditions, " AND ")
	fromClause := "bil_usage_logs u LEFT JOIN tnt_users t ON u.user_id = t.id AND u.tenant_id = t.tenant_id LEFT JOIN tnt_projects p ON u.project_id = p.id LEFT JOIN api_keys ak ON u.api_key_id = ak.id"

	countSQL := "SELECT COUNT(*) AS total FROM " + fromClause + " WHERE " + where
	countResult, err := g.DB().Ctx(ctx).Query(ctx, countSQL, args...)
	if err != nil {
		return nil, err
	}
	total := 0
	if len(countResult) > 0 {
		total = countResult[0]["total"].Int()
	}

	dataSQL := fmt.Sprintf(
		`SELECT u.*, COALESCE(t.username, '') AS username, COALESCE(p.name, '') AS project_name, COALESCE(ak.name, '') AS api_key_name
		 FROM %s WHERE %s ORDER BY u.created_at DESC LIMIT %d OFFSET %d`,
		fromClause, where, pageSize, (page-1)*pageSize,
	)
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

// ExportUsageLogs exports the tenant usage logs as CSV or Excel.
func (s *sTenant) ExportUsageLogs(ctx context.Context, req *v1.TenantUsageLogsExportReq) (*v1.TenantUsageLogsExportRes, error) {
	r := g.RequestFromCtx(ctx)
	format := req.Format
	if format == "" {
		format = "csv"
	}

	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)
	role := ctxUserRole(ctx)

	columns := []export.Column{
		{Field: "id", Header: "ID"},
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
		Format:   format,
		Filename: "用量日志_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	var conditions []string
	var args []any

	conditions = append(conditions, "u.tenant_id = ?")
	args = append(args, tenantID)

	// member 角色只能导出自己的用量日志
	if role == "member" {
		conditions = append(conditions, "u.user_id = ?")
		args = append(args, userID)
	} else if req.Username != "" {
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

	where := strings.Join(conditions, " AND ")
	fromClause := "bil_usage_logs u LEFT JOIN tnt_users t ON u.user_id = t.id AND u.tenant_id = t.tenant_id LEFT JOIN tnt_projects p ON u.project_id = p.id LEFT JOIN api_keys ak ON u.api_key_id = ak.id"

	if format == "xlsx" {
		dataSQL := fmt.Sprintf(
			`SELECT u.id, COALESCE(t.username, '') AS username, u.model_name, u.request_type,
			        u.input_tokens, u.output_tokens, u.total_cost, u.status, u.created_at
			 FROM %s WHERE %s ORDER BY u.created_at DESC`,
			fromClause, where,
		)
		result, err := g.DB().Ctx(ctx).Query(ctx, dataSQL, args...)
		if err != nil {
			return nil, err
		}

		data := make([]map[string]any, 0, len(result))
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
			data = append(data, m)
		}
		return nil, export.WriteExcel(r, config, data)
	}

	return nil, export.StreamCSV(r, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			dataSQL := fmt.Sprintf(
				`SELECT u.id, COALESCE(t.username, '') AS username, u.model_name, u.request_type,
				        u.input_tokens, u.output_tokens, u.total_cost, u.status, u.created_at
				 FROM %s WHERE %s ORDER BY u.created_at DESC LIMIT 1000 OFFSET %d`,
				fromClause, where, offset,
			)
			result, err := g.DB().Ctx(ctx).Query(ctx, dataSQL, args...)
			if err != nil {
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
			}
			if len(result) < 1000 {
				break
			}
			offset += 1000
		}
	})
}

// ExportWalletTransactions exports the tenant wallet transactions as CSV or Excel.
func (s *sTenant) ExportWalletTransactions(ctx context.Context, req *v1.TenantWalletTransactionsExportReq) (*v1.TenantWalletTransactionsExportRes, error) {
	role := ctxUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}

	r := g.RequestFromCtx(ctx)
	format := req.Format
	if format == "" {
		format = "csv"
	}

	tenantID := ctxTenantID(ctx)

	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "type", Header: "类型"},
		{Field: "amount", Header: "金额"},
		{Field: "balance_after", Header: "变动后余额"},
		{Field: "description", Header: "描述"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   format,
		Filename: "交易记录_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	if format == "xlsx" {
		type transactionRow struct {
			Id           int64       `json:"id"`
			Type         string      `json:"type"`
			Amount       float64     `json:"amount"`
			BalanceAfter float64     `json:"balance_after"`
			Description  string      `json:"description"`
			CreatedAt    *gtime.Time `json:"created_at"`
		}

		var records []transactionRow
		err := dao.BilTransactions.Ctx(ctx).
			Where("tenant_id", tenantID).
			Fields("id, type, amount, balance_after, description, created_at").
			OrderDesc("created_at").
			Scan(&records)
		if err != nil {
			return nil, err
		}

		data := make([]map[string]any, 0, len(records))
		for _, r := range records {
			data = append(data, map[string]any{
				"id":            r.Id,
				"type":          r.Type,
				"amount":        r.Amount,
				"balance_after": r.BalanceAfter,
				"description":   r.Description,
				"created_at":    r.CreatedAt,
			})
		}
		return nil, export.WriteExcel(r, config, data)
	}

	return nil, export.StreamCSV(r, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			type transactionRow struct {
				Id           int64       `json:"id"`
				Type         string      `json:"type"`
				Amount       float64     `json:"amount"`
				BalanceAfter float64     `json:"balance_after"`
				Description  string      `json:"description"`
				CreatedAt    *gtime.Time `json:"created_at"`
			}

			var records []transactionRow
			err := dao.BilTransactions.Ctx(ctx).
				Where("tenant_id", tenantID).
				Fields("id, type, amount, balance_after, description, created_at").
				OrderDesc("created_at").
				Limit(1000).Offset(offset).
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
			}
			if len(records) < 1000 {
				break
			}
			offset += 1000
		}
	})
}

// WalletFrozenItems 获取冻结明细
func (s *sTenant) WalletFrozenItems(ctx context.Context, req *v1.TenantWalletFrozenItemsReq) (*v1.TenantWalletFrozenItemsRes, error) {
	role := ctxUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := ctxTenantID(ctx)

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

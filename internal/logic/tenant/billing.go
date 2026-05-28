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

	type walletRow struct {
		Balance            float64 `json:"balance"`
		FrozenBalance      float64 `json:"frozen_balance"`
		WarningThreshold   float64 `json:"warning_threshold"`
		Currency           string  `json:"currency"`
		CumulativeRecharge float64 `json:"cumulative_recharge"`
	}

	var w *walletRow
	err := dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("balance, frozen_balance, warning_threshold, currency, cumulative_recharge").
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
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := dao.BilTransactions.Ctx(ctx).
		Where("bil_transactions.tenant_id", tenantID)

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

	var records []transactionRow
	var err error
	var total int
	err = query.Fields("bil_transactions.id, bil_transactions.type, bil_transactions.amount, bil_transactions.balance_after, bil_transactions.frozen_after, bil_transactions.related_id, bil_transactions.related_type, bil_transactions.description, bil_transactions.user_id, COALESCE(tu.username, '') AS username, bil_transactions.request_id, bil_transactions.model_name, bil_transactions.project_id, bil_transactions.api_key_id, bil_transactions.task_id, bil_transactions.created_at").
		LeftJoin("tnt_users tu", "bil_transactions.user_id = tu.id AND bil_transactions.tenant_id = tu.tenant_id").
		LeftJoin("bil_records br", "bil_transactions.related_id = br.id AND bil_transactions.related_type = 'billing_record'").
		OrderDesc("bil_transactions.created_at").
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

// UsageLogs 获取租户用量日志
func (s *sTenant) UsageLogs(ctx context.Context, req *v1.TenantUsageLogsReq) (*v1.TenantUsageLogsRes, error) {
	if err := common.ValidateDateParam(req.StartDate, "开始日期"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateParam(req.EndDate, "结束日期"); err != nil {
		return nil, err
	}

	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)
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
	if err := common.ValidateDateParam(req.StartDate, "开始日期"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateParam(req.EndDate, "结束日期"); err != nil {
		return nil, err
	}

	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)

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
		Format:   req.Format,
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

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
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
		offset := 0
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

			var records []transactionRow
			err := dao.BilTransactions.Ctx(ctx).
				Where("tenant_id", tenantID).
				Fields("id, type, amount, balance_after, user_id, request_id, model_name, description, created_at").
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

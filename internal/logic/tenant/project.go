package tenant

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/internal/model/do"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/middleware"
)

type projectRow struct {
	Id          int64   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	Budget      *string `json:"budget"`
	CreatedBy   int64   `json:"created_by"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type apiKeyRow struct {
	Id                   int64       `json:"id"`
	Name                 string      `json:"name"`
	KeyPrefix            string      `json:"key_prefix"`
	Scope                string      `json:"scope"`
	Status               string      `json:"status"`
	RateLimitQps         *int        `json:"rate_limit_qps"`
	RateLimitConcurrency *int        `json:"rate_limit_concurrency"`
	IpWhitelist          []string    `json:"ip_whitelist"`
	TotalQuota           *float64    `json:"total_quota"`
	UsedQuota            *float64    `json:"used_quota"`
	CreatedAt            *gtime.Time `json:"created_at"`
	ExpiresAt            *gtime.Time `json:"expires_at"`
}

type usageLogRow struct {
	Id           int64       `json:"id"`
	ModelName    string      `json:"model_name"`
	RelayMode    string      `json:"relay_mode"`
	InputTokens  int         `json:"input_tokens"`
	OutputTokens int         `json:"output_tokens"`
	TotalCost    float64     `json:"total_cost"`
	LatencyMs    int         `json:"latency_ms"`
	Status       string      `json:"status"`
	ErrorMessage string      `json:"error_message"`
	CreatedAt    *gtime.Time `json:"created_at"`
}

type projectStatusRow struct {
	ID       int64   `json:"id"`
	TenantID int64   `json:"tenant_id"`
	Status   string  `json:"status"`
	Budget   float64 `json:"budget"`
}

func invalidateApiKeysByProject(ctx context.Context, tenantID, projectID int64) error {
	type keyRow struct {
		KeyPrefix string `json:"key_prefix"`
	}
	var keys []keyRow
	err := dao.ApiKeys.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("project_id", projectID).
		Fields("key_prefix").
		Scan(&keys)
	if err != nil {
		return err
	}
	for _, key := range keys {
		relay.InvalidateApiKey(ctx, key.KeyPrefix)
	}
	return nil
}

func ensureProjectActiveForKey(ctx context.Context, tenantID, projectID int64) error {
	project, err := getProjectStatus(ctx, tenantID, projectID)
	if err != nil {
		return err
	}
	if project == nil {
		return common.NewNotFoundError("项目")
	}
	if project.Status != "active" {
		return common.NewBusinessError(consts.CodeProjectNotActive, consts.MsgProjectNotActive)
	}
	return nil
}

func getProjectStatus(ctx context.Context, tenantID, projectID int64) (*projectStatusRow, error) {
	var project *projectStatusRow
	err := dao.TntProjects.Ctx(ctx).
		Where("id", projectID).
		Where("tenant_id", tenantID).
		Fields("id, tenant_id, status, COALESCE(budget, 0) as budget").
		Scan(&project)
	if err != nil {
		return nil, err
	}
	return project, nil
}

func projectTotalCost(ctx context.Context, tenantID, projectID int64) (float64, error) {
	var usage struct {
		TotalCost float64 `json:"total_cost"`
	}
	err := dao.BilTransactions.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("project_id", projectID).
		Where("type", "consume").
		Fields("COALESCE(SUM(-amount), 0) as total_cost").
		Scan(&usage)
	if err != nil {
		return 0, err
	}
	return usage.TotalCost, nil
}

func exhaustProjectBudget(ctx context.Context, tenantID, projectID int64) error {
	// 事务采用 ctx 传播式写法：闭包内所有 dao 操作统一使用 dao.Xxx.Ctx(ctx)，
	// 由 ctx 自动挂载当前事务（不直接使用 tx 句柄）。务必保证 ctx 逐层传递。
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		_, err := dao.TntProjects.Ctx(ctx).
			Where("id", projectID).
			Where("tenant_id", tenantID).
			Where("status", "active").
			Data(do.TntProjects{Status: "budget_exhausted"}).
			Update()
		if err != nil {
			return err
		}

		_, err = dao.ApiKeys.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("project_id", projectID).
			Where("status", "active").
			Data(do.ApiKeys{Status: "revoked"}).
			Update()
		return err
	})
}

func CheckProjectBudget(ctx context.Context, tenantID, projectID int64) error {
	if tenantID <= 0 || projectID <= 0 {
		return nil
	}

	project, err := getProjectStatus(ctx, tenantID, projectID)
	if err != nil {
		return err
	}
	if project == nil || project.Status != "active" || project.Budget <= 0 {
		return nil
	}

	totalCost, err := projectTotalCost(ctx, tenantID, projectID)
	if err != nil {
		return err
	}
	if totalCost < project.Budget {
		return nil
	}

	if err = exhaustProjectBudget(ctx, tenantID, projectID); err != nil {
		return err
	}
	return invalidateApiKeysByProject(ctx, tenantID, projectID)
}

// ProjectList returns a paginated list of projects for a tenant.
func (s *sTenant) ProjectList(ctx context.Context, req *v1.TenantProjectListReq) (*v1.TenantProjectListRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var projects []projectRow
	var total int
	var err error
	err = dao.TntProjects.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("id, name, description, status, budget, created_by, created_at, updated_at").
		OrderDesc("id").
		Page(page, pageSize).
		ScanAndCount(&projects, &total, false)
	if err != nil {
		return nil, err
	}
	if projects == nil {
		projects = []projectRow{}
	}

	return &v1.TenantProjectListRes{
		List:     convertProjectRowsToMaps(projects),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ProjectCreate creates a new project for a tenant.
func (s *sTenant) ProjectCreate(ctx context.Context, req *v1.TenantProjectCreateReq) (*v1.TenantProjectCreateRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)

	name := req.Name
	if name == "" {
		return nil, common.NewBadRequestError("项目名称不能为空")
	}

	insertData := do.TntProjects{
		TenantId:    tenantID,
		Name:        name,
		Description: "",
		Status:      "active",
		Budget:      nil,
		CreatedBy:   userID,
	}

	if req.Description != "" {
		insertData.Description = req.Description
	}
	if req.Budget > 0 {
		insertData.Budget = req.Budget
	}

	result, err := dao.TntProjects.Ctx(ctx).Data(insertData).Insert()
	if err != nil {
		return nil, gerror.Wrapf(err, "创建项目")
	}
	id, _ := result.LastInsertId()
	return &v1.TenantProjectCreateRes{ID: id}, nil
}

// ProjectUpdate updates a project.
func (s *sTenant) ProjectUpdate(ctx context.Context, req *v1.TenantProjectUpdateReq) (*v1.TenantProjectUpdateRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)

	var project *struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	err := dao.TntProjects.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Scan(&project)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, common.NewNotFoundError("项目")
	}

	if project.Status == "archived" {
		return nil, common.NewBadRequestError("归档的项目不能直接编辑，请先取消归档")
	}

	updateData := do.TntProjects{}
	if req.Name != "" {
		updateData.Name = req.Name
	}
	if req.Description != "" {
		updateData.Description = req.Description
	}
	if req.Budget > 0 {
		updateData.Budget = req.Budget
	} else if req.Budget == 0 {
		updateData.Budget = nil
	}
	// If budget was exhausted, updating budget should reactivate
	if project.Status == "budget_exhausted" && req.Budget > 0 {
		updateData.Status = "active"
	}

	_, err = dao.TntProjects.Ctx(ctx).
		Where("id", req.Id).
		Data(updateData).Update()
	if err != nil {
		return nil, gerror.Wrapf(err, "更新项目")
	}
	return nil, nil
}

// ProjectArchive archives a project and revokes all its keys.
func (s *sTenant) ProjectArchive(ctx context.Context, req *v1.TenantProjectArchiveReq) (*v1.TenantProjectArchiveRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)

	var project *struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	err := dao.TntProjects.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Scan(&project)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, common.NewNotFoundError("项目")
	}
	if project.Status == "archived" {
		return nil, common.NewBadRequestError("项目已归档")
	}

	// Revoke all keys belonging to this project
	_, err = dao.ApiKeys.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("project_id", req.Id).
		Where("status", "active").
		Data(do.ApiKeys{
			Status: "revoked",
		}).Update()
	if err != nil {
		return nil, err
	}
	_ = invalidateApiKeysByProject(ctx, tenantID, req.Id)

	_, err = dao.TntProjects.Ctx(ctx).
		Where("id", req.Id).
		Data(do.TntProjects{
			Status: "archived",
		}).Update()
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// ProjectUnarchive restores an archived project. Keys are NOT auto-restored.
func (s *sTenant) ProjectUnarchive(ctx context.Context, req *v1.TenantProjectUnarchiveReq) (*v1.TenantProjectUnarchiveRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	var project *struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	err := dao.TntProjects.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Scan(&project)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, common.NewNotFoundError("项目")
	}
	if project.Status != "archived" {
		return nil, common.NewBadRequestError("只有归档状态的项目可以取消归档")
	}

	_, err = dao.TntProjects.Ctx(ctx).
		Where("id", req.Id).
		Data(do.TntProjects{
			Status: "active",
		}).Update()
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// CheckBudgetExhausted scans projects and marks those that exceeded budget.
func CheckBudgetExhausted(ctx context.Context) error {
	// Find active projects with budget set
	var projects []struct {
		ID       int64   `json:"id"`
		TenantID int64   `json:"tenant_id"`
		Budget   float64 `json:"budget"`
	}
	err := dao.TntProjects.Ctx(ctx).
		Where("status", "active").
		Where("budget IS NOT NULL").
		Where("budget > 0").
		Scan(&projects)
	if err != nil {
		return err
	}

	for _, p := range projects {
		if err = CheckProjectBudget(ctx, p.TenantID, p.ID); err != nil {
			g.Log().Warningf(ctx, "检查项目预算失败: tenant=%d project=%d err=%v", p.TenantID, p.ID, err)
		}
	}

	return nil
}

// ProjectGet 根据 ID 获取单个项目详情（含统计摘要）
func (s *sTenant) ProjectGet(ctx context.Context, req *v1.TenantProjectGetReq) (*v1.TenantProjectGetRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	var p *projectRow
	err := dao.TntProjects.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Fields("id, name, description, status, budget, created_by, created_at, updated_at").
		Scan(&p)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, common.NewNotFoundError("项目")
	}

	// 统计密钥数量
	keyStats := struct {
		Total  int `json:"total"`
		Active int `json:"active"`
	}{}
	dao.ApiKeys.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("project_id", req.Id).
		Fields("COUNT(*) as total, COUNT(*) FILTER (WHERE status = 'active') as active").
		Scan(&keyStats)

	// 获取项目关联的 API Key ID 列表
	keyIDs, _ := dao.ApiKeys.Ctx(ctx).
		Where("project_id", req.Id).
		Where("tenant_id", tenantID).
		Fields("id").
		Array()

	// 统计本月用量
	var monthUsage struct {
		TotalCost    float64 `json:"total_cost"`
		RequestCount int     `json:"request_count"`
	}
	dao.BilUsageLogs.Ctx(ctx).
		Where("tenant_id", tenantID).
		WhereIn("api_key_id", keyIDs).
		Where("created_at >= date_trunc('month', NOW())").
		Fields("COALESCE(SUM(total_cost), 0) as total_cost, COUNT(*) as request_count").
		Scan(&monthUsage)

	return &v1.TenantProjectGetRes{
		Id:            p.Id,
		Name:          p.Name,
		Description:   p.Description,
		Status:        p.Status,
		Budget:        p.Budget,
		CreatedBy:     p.CreatedBy,
		ActiveKeys:    keyStats.Active,
		TotalKeys:     keyStats.Total,
		MonthCost:     monthUsage.TotalCost,
		MonthRequests: int64(monthUsage.RequestCount),
	}, nil
}

// ProjectApiKeyList 获取项目密钥列表（owner/admin 权限）
func (s *sTenant) ProjectApiKeyList(ctx context.Context, req *v1.TenantProjectApiKeyListReq) (*v1.TenantProjectApiKeyListRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}

	tenantID := middleware.GetTenantID(ctx)
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	var keys []apiKeyRow
	var total int
	var err error
	err = dao.ApiKeys.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("project_id", req.Id).
		Fields("id, name, key_prefix, scope, status, rate_limit_qps, rate_limit_concurrency, ip_whitelist, total_quota, used_quota, created_at, expires_at").
		OrderDesc("id").
		Page(page, pageSize).
		ScanAndCount(&keys, &total, false)
	if err != nil {
		return nil, err
	}
	if keys == nil {
		keys = []apiKeyRow{}
	}

	return &v1.TenantProjectApiKeyListRes{
		List:     convertApiKeyRowsToMaps(ctx, keys),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ProjectApiKeyCreate 创建项目密钥（owner/admin 权限）
func (s *sTenant) ProjectApiKeyCreate(ctx context.Context, req *v1.TenantProjectApiKeyCreateReq) (*v1.TenantProjectApiKeyCreateRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}

	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)

	if err := ensureProjectActiveForKey(ctx, tenantID, req.Id); err != nil {
		return nil, err
	}

	// Generate API key
	rawKey, prefix, encryptedKey, err := relay.GenerateApiKey(ctx)
	if err != nil {
		return nil, gerror.Wrapf(err, "生成密钥")
	}

	insertData := do.ApiKeys{
		TenantId:     tenantID,
		ProjectId:    req.Id,
		Name:         req.Name,
		EncryptedKey: encryptedKey,
		KeyPrefix:    prefix,
		KeyType:      "project",
		Status:       "active",
		UserId:       userID,
		Scope:        req.Scope,
	}

	if req.ExpiresAt != nil {
		insertData.ExpiresAt = req.ExpiresAt
	}
	if req.RateLimitQps != nil {
		if *req.RateLimitQps < 0 {
			return nil, common.NewBadRequestError("QPS 限制不能小于 0")
		}
		insertData.RateLimitQps = *req.RateLimitQps
	}
	if req.RateLimitConcurrency != nil {
		if *req.RateLimitConcurrency < 0 {
			return nil, common.NewBadRequestError("并发限制不能小于 0")
		}
		insertData.RateLimitConcurrency = *req.RateLimitConcurrency
	}
	if req.IpWhitelist != nil {
		insertData.IpWhitelist = normalizeIPWhitelist(*req.IpWhitelist)
	}
	if req.TotalQuota != nil {
		if *req.TotalQuota < 0 {
			return nil, common.NewBadRequestError("总额度不能小于 0")
		}
		insertData.TotalQuota = *req.TotalQuota
	}

	result, err := dao.ApiKeys.Ctx(ctx).Data(insertData).Insert()
	if err != nil {
		return nil, gerror.Wrapf(err, "创建密钥")
	}
	id, _ := result.LastInsertId()

	for _, modelName := range req.ModelNames {
		if modelName != "" {
			_, err = dao.ApiKeyModelScopes.Ctx(ctx).Insert(do.ApiKeyModelScopes{
				ApiKeyId:  id,
				ModelName: modelName,
			})
			if err != nil {
				g.Log().Warningf(ctx, "创建密钥模型范围失败: %v", err)
			}
		}
	}

	return &v1.TenantProjectApiKeyCreateRes{
		ID:        id,
		Name:      req.Name,
		Key:       rawKey,
		KeyPrefix: prefix,
	}, nil
}

// ProjectApiKeyDelete 删除项目密钥（owner/admin 权限）
func (s *sTenant) ProjectApiKeyDelete(ctx context.Context, req *v1.TenantProjectApiKeyDeleteReq) (*v1.TenantProjectApiKeyDeleteRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}

	tenantID := middleware.GetTenantID(ctx)

	// Verify the key belongs to the project and tenant
	var key *struct {
		ID        int64  `json:"id"`
		KeyPrefix string `json:"key_prefix"`
	}
	err := dao.ApiKeys.Ctx(ctx).
		Where("id", req.KeyId).
		Where("tenant_id", tenantID).
		Where("project_id", req.Id).
		Fields("id, key_prefix").
		Scan(&key)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, common.NewNotFoundError("密钥")
	}

	_, err = dao.ApiKeys.Ctx(ctx).
		Where("id", req.KeyId).
		Where("tenant_id", tenantID).
		Data(do.ApiKeys{
			Status: "revoked",
		}).Update()
	if err != nil {
		return nil, gerror.Wrapf(err, "删除密钥")
	}
	relay.InvalidateApiKey(ctx, key.KeyPrefix)
	return nil, nil
}

// ProjectUsageStats 获取项目用量统计（按日汇总，近30天）（owner/admin 权限）
func (s *sTenant) ProjectUsageStats(ctx context.Context, req *v1.TenantProjectUsageStatsReq) (*v1.TenantProjectUsageStatsRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}

	tenantID := middleware.GetTenantID(ctx)

	var project *struct {
		ID int64 `json:"id"`
	}
	err := dao.TntProjects.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Fields("id").
		Scan(&project)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, common.NewNotFoundError("项目")
	}

	// 获取项目关联的 API Key ID 列表
	keyIDs, _ := dao.ApiKeys.Ctx(ctx).
		Where("project_id", req.Id).
		Where("tenant_id", tenantID).
		Fields("id").
		Array()
	if len(keyIDs) == 0 {
		return &v1.TenantProjectUsageStatsRes{
			Data: map[string]any{
				"total_cost":          0,
				"total_requests":      0,
				"total_input_tokens":  0,
				"total_output_tokens": 0,
				"daily":               []map[string]any{},
				"models":              []map[string]any{},
			},
		}, nil
	}

	// 总用量
	var totalStats struct {
		TotalCost    float64 `json:"total_cost"`
		RequestCount int     `json:"request_count"`
		InputTokens  int64   `json:"input_tokens"`
		OutputTokens int64   `json:"output_tokens"`
	}
	dao.BilUsageLogs.Ctx(ctx).
		Where("tenant_id", tenantID).
		WhereIn("api_key_id", keyIDs).
		Fields("COALESCE(SUM(total_cost), 0) as total_cost, COUNT(*) as request_count, COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens").
		Scan(&totalStats)

	// 每日用量趋势（近30天）
	type dailyStatRow struct {
		Date         string  `json:"date"`
		RequestCount int     `json:"request_count"`
		TotalCost    float64 `json:"total_cost"`
		InputTokens  int64   `json:"input_tokens"`
		OutputTokens int64   `json:"output_tokens"`
	}
	var dailyStats []dailyStatRow
	dao.BilUsageLogs.Ctx(ctx).
		Where("tenant_id", tenantID).
		WhereIn("api_key_id", keyIDs).
		Where("created_at >= NOW() - INTERVAL '30 days'").
		Fields("DATE(created_at) as date, COUNT(*) as request_count, COALESCE(SUM(total_cost), 0) as total_cost, COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens").
		Group("DATE(created_at)").
		OrderAsc("date").
		Scan(&dailyStats)
	if dailyStats == nil {
		dailyStats = []dailyStatRow{}
	}

	// 模型用量分布（Top 10）
	type modelStatRow struct {
		ModelName    string  `json:"model_name"`
		RequestCount int     `json:"request_count"`
		TotalCost    float64 `json:"total_cost"`
	}
	var modelStats []modelStatRow
	dao.BilUsageLogs.Ctx(ctx).
		Where("tenant_id", tenantID).
		WhereIn("api_key_id", keyIDs).
		Fields("model_name, COUNT(*) as request_count, COALESCE(SUM(total_cost), 0) as total_cost").
		Group("model_name").
		OrderDesc("total_cost").
		Limit(10).
		Scan(&modelStats)
	if modelStats == nil {
		modelStats = []modelStatRow{}
	}

	return &v1.TenantProjectUsageStatsRes{
		Data: map[string]any{
			"total_cost":          totalStats.TotalCost,
			"total_requests":      totalStats.RequestCount,
			"total_input_tokens":  totalStats.InputTokens,
			"total_output_tokens": totalStats.OutputTokens,
			"daily":               dailyStats,
			"models":              modelStats,
		},
	}, nil
}

// ProjectUsageLogs 获取项目用量日志（分页）（owner/admin 权限）
func (s *sTenant) ProjectUsageLogs(ctx context.Context, req *v1.TenantProjectUsageLogsReq) (*v1.TenantProjectUsageLogsRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}

	tenantID := middleware.GetTenantID(ctx)
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	// 获取项目关联的 API Key ID 列表
	keyIDs, _ := dao.ApiKeys.Ctx(ctx).
		Where("project_id", req.Id).
		Where("tenant_id", tenantID).
		Fields("id").
		Array()
	if len(keyIDs) == 0 {
		return &v1.TenantProjectUsageLogsRes{
			List:     []map[string]any{},
			Total:    0,
			Page:     page,
			PageSize: pageSize,
		}, nil
	}

	var logs []usageLogRow
	var err error
	var total int
	err = dao.BilUsageLogs.Ctx(ctx).
		Where("tenant_id", tenantID).
		WhereIn("api_key_id", keyIDs).
		Fields("id, model_name, relay_mode, input_tokens, output_tokens, total_cost, latency_ms, status, error_message, created_at").
		OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&logs, &total, false)
	if err != nil {
		return nil, err
	}
	if logs == nil {
		logs = []usageLogRow{}
	}

	return &v1.TenantProjectUsageLogsRes{
		List:     convertUsageLogRowsToMaps(logs),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// convertProjectRowsToMaps converts project rows to []map[string]any for JSON serialization.
func convertProjectRowsToMaps(rows any) []map[string]any {
	result := make([]map[string]any, 0)
	switch v := rows.(type) {
	case []projectRow:
		for _, r := range v {
			result = append(result, map[string]any{
				"id":          r.Id,
				"name":        r.Name,
				"description": r.Description,
				"status":      r.Status,
				"budget":      r.Budget,
				"created_by":  r.CreatedBy,
				"created_at":  r.CreatedAt,
				"updated_at":  r.UpdatedAt,
			})
		}
	}
	return result
}

// convertApiKeyRowsToMaps converts api key rows to []map[string]any for JSON serialization.
func convertApiKeyRowsToMaps(ctx context.Context, rows any) []map[string]any {
	result := make([]map[string]any, 0)
	switch v := rows.(type) {
	case []apiKeyRow:
		// Batch query model counts
		keyIDs := make([]int64, len(v))
		for i, r := range v {
			keyIDs[i] = r.Id
		}
		modelCountMap := make(map[int64]int)
		if len(keyIDs) > 0 {
			type countRow struct {
				ApiKeyId int64 `json:"api_key_id"`
				Cnt      int   `json:"cnt"`
			}
			var countRows []countRow
			err := dao.ApiKeyModelScopes.Ctx(ctx).
				WhereIn("api_key_id", keyIDs).
				Fields("api_key_id, COUNT(*) as cnt").
				Group("api_key_id").
				Scan(&countRows)
			if err == nil {
				for _, cr := range countRows {
					modelCountMap[cr.ApiKeyId] = cr.Cnt
				}
			}
		}

		for _, r := range v {
			m := map[string]any{
				"id":                     r.Id,
				"name":                   r.Name,
				"key_prefix":             r.KeyPrefix,
				"scope":                  r.Scope,
				"status":                 r.Status,
				"rate_limit_qps":         r.RateLimitQps,
				"rate_limit_concurrency": r.RateLimitConcurrency,
				"ip_whitelist":           r.IpWhitelist,
				"total_quota":            r.TotalQuota,
				"used_quota":             r.UsedQuota,
				"model_count":            modelCountMap[r.Id],
			}
			if r.CreatedAt != nil {
				m["created_at"] = r.CreatedAt.String()
			}
			if r.ExpiresAt != nil {
				m["expires_at"] = r.ExpiresAt.String()
			}
			result = append(result, m)
		}
	}
	return result
}

// convertUsageLogRowsToMaps converts usage log rows to []map[string]any for JSON serialization.
func convertUsageLogRowsToMaps(rows any) []map[string]any {
	result := make([]map[string]any, 0)
	switch v := rows.(type) {
	case []usageLogRow:
		for _, r := range v {
			m := map[string]any{
				"id":            r.Id,
				"model_name":    r.ModelName,
				"relay_mode":    r.RelayMode,
				"input_tokens":  r.InputTokens,
				"output_tokens": r.OutputTokens,
				"total_cost":    r.TotalCost,
				"latency_ms":    r.LatencyMs,
				"status":        r.Status,
				"error_message": r.ErrorMessage,
			}
			if r.CreatedAt != nil {
				m["created_at"] = r.CreatedAt.String()
			}
			result = append(result, m)
		}
	}
	return result
}

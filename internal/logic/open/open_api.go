package open

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/internal/middleware"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/service"
	"github.com/qianfree/team-api/internal/utility/crypto"
)

type sOpen struct{}

var openMemberModelCache = common.NewCache("member_model", 60*time.Second)

func getProjectApiKeyPrefixes(ctx context.Context, tenantID, projectID int64) ([]string, error) {
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
		return nil, err
	}
	prefixes := make([]string, 0, len(keys))
	for _, key := range keys {
		if key.KeyPrefix != "" {
			prefixes = append(prefixes, key.KeyPrefix)
		}
	}
	return prefixes, nil
}

func invalidateApiKeyPrefixes(ctx context.Context, prefixes []string) {
	for _, prefix := range prefixes {
		relay.InvalidateApiKey(ctx, prefix)
	}
}

func New() *sOpen {
	return &sOpen{}
}

func init() {
	service.RegisterOpen(New())
}

// ============================================================
// 成员管理
// ============================================================

func (s *sOpen) OpenMemberList(ctx context.Context, req *v1.OpenMemberListReq) (*v1.OpenMemberListRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "members:read"); err != nil {
		return nil, err
	}

	page := req.Page
	pageSize := req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	m := dao.TntUsers.Ctx(ctx).Where("tenant_id", tenantID)
	if req.Keyword != "" {
		kw := "%" + strings.TrimSpace(req.Keyword) + "%"
		m = m.Where("username LIKE ? OR email LIKE ?", kw, kw)
	}
	if req.Role != "" {
		m = m.Where("role", req.Role)
	}
	if req.Status != "" {
		m = m.Where("status", req.Status)
	}

	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	var users []struct {
		Id        int64  `json:"id"`
		Username  string `json:"username"`
		Email     string `json:"email"`
		Role      string `json:"role"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
	}
	err = m.OrderDesc("id").Page(page, pageSize).Scan(&users)
	if err != nil {
		return nil, err
	}

	items := make([]v1.OpenMemberItem, len(users))
	for i, u := range users {
		items[i] = v1.OpenMemberItem{
			ID:        u.Id,
			Username:  u.Username,
			Email:     u.Email,
			Role:      u.Role,
			Status:    u.Status,
			CreatedAt: u.CreatedAt,
		}
	}

	return &v1.OpenMemberListRes{List: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *sOpen) OpenMemberCreate(ctx context.Context, req *v1.OpenMemberCreateReq) (*v1.OpenMemberCreateRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "members:write"); err != nil {
		return nil, err
	}

	count, _ := dao.TntUsers.Ctx(ctx).Where("tenant_id", tenantID).Where("email", req.Email).Count()
	if count > 0 {
		return nil, errOpen(consts.CodeUsernameExists, "邮箱已存在")
	}

	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	result, err := dao.TntUsers.Ctx(ctx).Data(do.TntUsers{
		TenantId:     tenantID,
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		Role:         req.Role,
		DisplayName:  req.DisplayName,
		Status:       "active",
	}).Insert()
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &v1.OpenMemberCreateRes{ID: id}, nil
}

func (s *sOpen) OpenMemberUpdate(ctx context.Context, req *v1.OpenMemberUpdateReq) (*v1.OpenMemberUpdateRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "members:write"); err != nil {
		return nil, err
	}

	data := do.TntUsers{}
	hasUpdate := false
	if req.Role != nil {
		data.Role = *req.Role
		hasUpdate = true
	}
	if req.DisplayName != nil {
		data.DisplayName = *req.DisplayName
		hasUpdate = true
	}
	if req.Status != nil {
		data.Status = *req.Status
		hasUpdate = true
	}

	if !hasUpdate {
		return nil, nil
	}

	_, err := dao.TntUsers.Ctx(ctx).Where("id", req.Id).Where("tenant_id", tenantID).OmitNil().Data(data).Update()
	return nil, err
}

func (s *sOpen) OpenMemberDelete(ctx context.Context, req *v1.OpenMemberDeleteReq) (*v1.OpenMemberDeleteRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "members:write"); err != nil {
		return nil, err
	}

	var user *struct {
		Role string `json:"role"`
	}
	err := dao.TntUsers.Ctx(ctx).Where("id", req.Id).Where("tenant_id", tenantID).Scan(&user)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errOpen(consts.CodeNotFound, "用户不存在")
	}
	if user.Role == "owner" {
		return nil, errOpen(consts.CodeForbidden, "不能删除所有者")
	}

	_, err = dao.TntUsers.Ctx(ctx).Where("id", req.Id).Where("tenant_id", tenantID).Delete()
	return nil, err
}

// ============================================================
// 成员额度 & 模型
// ============================================================

func (s *sOpen) OpenMemberQuota(ctx context.Context, req *v1.OpenMemberQuotaReq) (*v1.OpenMemberQuotaRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "members:read"); err != nil {
		return nil, err
	}

	var user *struct {
		QuotaType   string  `json:"quota_type"`
		QuotaLimit  float64 `json:"quota_limit"`
		QuotaUsed   float64 `json:"quota_used"`
		QuotaPeriod string  `json:"quota_period"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Fields("quota_type, quota_limit, quota_used, quota_period").
		Scan(&user)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errOpen(consts.CodeNotFound, "用户不存在")
	}

	return &v1.OpenMemberQuotaRes{
		QuotaType:  user.QuotaType,
		QuotaLimit: user.QuotaLimit,
		QuotaUsed:  user.QuotaUsed,
		Period:     user.QuotaPeriod,
	}, nil
}

func (s *sOpen) OpenMemberQuotaUpdate(ctx context.Context, req *v1.OpenMemberQuotaUpdateReq) (*v1.OpenMemberQuotaUpdateRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "members:write"); err != nil {
		return nil, err
	}

	if req.QuotaType == "periodic" && req.Period == "" {
		return nil, errOpen(consts.CodeBadRequest, "periodic 类型额度必须指定周期")
	}

	data := do.TntUsers{
		QuotaType: req.QuotaType,
		QuotaUsed: 0,
	}
	if req.QuotaType != "none" {
		data.QuotaLimit = req.QuotaLimit
	} else {
		data.QuotaLimit = 0
	}
	if req.QuotaType == "periodic" {
		data.QuotaPeriod = req.Period
		data.QuotaResetAt = gtime.Now()
	}

	_, err := dao.TntUsers.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Data(data).
		Update()
	return nil, err
}

func (s *sOpen) OpenMemberModels(ctx context.Context, req *v1.OpenMemberModelsReq) (*v1.OpenMemberModelsRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "members:read"); err != nil {
		return nil, err
	}

	var scopes []struct {
		ModelID string `json:"model_id"`
	}
	err := dao.TntMemberModelScopes.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("user_id", req.Id).
		Scan(&scopes)
	if err != nil {
		return nil, err
	}

	modelIDs := make([]string, 0, len(scopes))
	for _, s := range scopes {
		modelIDs = append(modelIDs, s.ModelID)
	}

	return &v1.OpenMemberModelsRes{List: modelIDs}, nil
}

func (s *sOpen) OpenMemberModelsUpdate(ctx context.Context, req *v1.OpenMemberModelsUpdateReq) (*v1.OpenMemberModelsUpdateRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "members:write"); err != nil {
		return nil, err
	}

	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		_, err := dao.TntMemberModelScopes.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("user_id", req.Id).
			Delete()
		if err != nil {
			return err
		}

		if len(req.ModelIDs) > 0 {
			for _, modelID := range req.ModelIDs {
				if modelID == 0 {
					continue
				}
				_, err = dao.TntMemberModelScopes.Ctx(ctx).Data(do.TntMemberModelScopes{
					TenantId: tenantID,
					UserId:   req.Id,
					ModelId:  modelID,
				}).Insert()
				if err != nil {
					return err
				}
			}
		} else {
			_, err = dao.TntMemberModelScopes.Ctx(ctx).Data(do.TntMemberModelScopes{
				TenantId: tenantID,
				UserId:   req.Id,
				ModelId:  -1,
			}).Insert()
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	cacheKey := fmt.Sprintf("%d:%d", tenantID, req.Id)
	openMemberModelCache.Delete(ctx, cacheKey)

	return nil, nil
}

// ============================================================
// API Key 管理
// ============================================================

func (s *sOpen) OpenKeyList(ctx context.Context, req *v1.OpenKeyListReq) (*v1.OpenKeyListRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "keys:read"); err != nil {
		return nil, err
	}

	page := req.Page
	pageSize := req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	m := dao.ApiKeys.Ctx(ctx).Where("tenant_id", tenantID)
	if req.Status != "" {
		m = m.Where("status", req.Status)
	}

	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	var keys []struct {
		Id        int64       `json:"id"`
		Name      string      `json:"name"`
		KeyPrefix string      `json:"key_prefix"`
		Status    string      `json:"status"`
		CreatedAt *gtime.Time `json:"created_at"`
	}
	err = m.OrderDesc("id").Page(page, pageSize).Scan(&keys)
	if err != nil {
		return nil, err
	}

	items := make([]v1.OpenKeyItem, len(keys))
	for i, k := range keys {
		items[i] = v1.OpenKeyItem{
			ID:     k.Id,
			Name:   k.Name,
			Key:    k.KeyPrefix + "***",
			Status: k.Status,
		}
		if k.CreatedAt != nil {
			items[i].CreatedAt = k.CreatedAt.Format("Y-m-d H:i:s")
		}
	}

	return &v1.OpenKeyListRes{List: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *sOpen) OpenKeyCreate(ctx context.Context, req *v1.OpenKeyCreateReq) (*v1.OpenKeyCreateRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "keys:write"); err != nil {
		return nil, err
	}

	rawKey, prefix, encryptedKey, err := relay.GenerateApiKey(ctx)
	if err != nil {
		return nil, err
	}

	quotaLimit := req.QuotaLimit
	if quotaLimit < 0 {
		quotaLimit = 0
	}
	quotaLimitDecimal := billing.NewFromFloat(quotaLimit)

	result, err := dao.ApiKeys.Ctx(ctx).Data(do.ApiKeys{
		TenantId:     tenantID,
		Name:         req.Name,
		EncryptedKey: encryptedKey,
		KeyPrefix:    prefix,
		Scope:        "full",
		Status:       "active",
		TotalQuota:   &quotaLimitDecimal,
		KeyType:      "project",
	}).Insert()
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()

	for _, modelName := range req.ModelScopes {
		if modelName != "" {
			if _, err = dao.ApiKeyModelScopes.Ctx(ctx).Insert(do.ApiKeyModelScopes{
				ApiKeyId:  id,
				ModelName: modelName,
			}); err != nil {
				return nil, gerror.Wrapf(err, "设置模型范围失败")
			}
		}
	}

	return &v1.OpenKeyCreateRes{ID: id, Key: rawKey}, nil
}

func (s *sOpen) OpenKeyDelete(ctx context.Context, req *v1.OpenKeyDeleteReq) (*v1.OpenKeyDeleteRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "keys:write"); err != nil {
		return nil, err
	}

	var key *struct {
		KeyPrefix string `json:"key_prefix"`
	}
	err := dao.ApiKeys.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Fields("key_prefix").
		Scan(&key)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, errOpen(consts.CodeNotFound, "密钥不存在")
	}

	_, err = dao.ApiKeys.Ctx(ctx).Where("id", req.Id).Where("tenant_id", tenantID).Delete()
	if err == nil {
		relay.InvalidateApiKey(ctx, key.KeyPrefix)
	}
	return nil, err
}

// ============================================================
// 用量查询
// ============================================================

func (s *sOpen) OpenUsageQuery(ctx context.Context, req *v1.OpenUsageQueryReq) (*v1.OpenUsageQueryRes, error) {
	if err := common.ValidateDateParam(req.StartDate, "开始日期"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateParam(req.EndDate, "结束日期"); err != nil {
		return nil, err
	}
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "usage:read"); err != nil {
		return nil, err
	}

	page := req.Page
	pageSize := req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	m := dao.BilUsageLogs.Ctx(ctx).Where("tenant_id", tenantID)
	m = m.Where("created_at >= ?", req.StartDate+" 00:00:00")
	m = m.Where("created_at <= ?", req.EndDate+" 23:59:59")

	selectFields := ""
	groupBy := ""
	switch req.GroupBy {
	case "model":
		selectFields = "model_name, COUNT(*) as request_count, SUM(input_tokens) as prompt_tokens, SUM(output_tokens) as completion_tokens, SUM(input_tokens+output_tokens) as total_tokens, SUM(actual_cost) as cost"
		groupBy = "model_name"
	case "key":
		selectFields = "key_prefix as key_name, COUNT(*) as request_count, SUM(input_tokens) as prompt_tokens, SUM(output_tokens) as completion_tokens, SUM(input_tokens+output_tokens) as total_tokens, SUM(actual_cost) as cost"
		groupBy = "key_prefix"
	default:
		selectFields = "DATE(created_at) as date, COUNT(*) as request_count, SUM(input_tokens) as prompt_tokens, SUM(output_tokens) as completion_tokens, SUM(input_tokens+output_tokens) as total_tokens, SUM(actual_cost) as cost"
		groupBy = "DATE(created_at)"
	}

	type usageRow struct {
		Date             string  `json:"date"`
		Model            string  `json:"model_name"`
		KeyName          string  `json:"key_name"`
		RequestCount     int64   `json:"request_count"`
		PromptTokens     int64   `json:"prompt_tokens"`
		CompletionTokens int64   `json:"completion_tokens"`
		TotalTokens      int64   `json:"total_tokens"`
		Cost             float64 `json:"cost"`
	}

	var rows []usageRow
	err := m.Fields(selectFields).Group(groupBy).OrderDesc(groupBy).Page(page, pageSize).Scan(&rows)
	if err != nil {
		return nil, err
	}

	items := make([]v1.OpenUsageItem, 0, len(rows))
	for _, r := range rows {
		item := v1.OpenUsageItem{
			RequestCount:     r.RequestCount,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
			TotalTokens:      r.TotalTokens,
			Cost:             fmt.Sprintf("%.6f", r.Cost),
		}
		switch req.GroupBy {
		case "model":
			item.Model = r.Model
		case "key":
			item.KeyName = r.KeyName
		default:
			item.Date = r.Date
		}
		items = append(items, item)
	}

	return &v1.OpenUsageQueryRes{List: items, Total: len(items), Page: page, PageSize: pageSize}, nil
}

// ============================================================
// 费用查询
// ============================================================

func (s *sOpen) OpenBillingQuery(ctx context.Context, req *v1.OpenBillingQueryReq) (*v1.OpenBillingQueryRes, error) {
	if err := common.ValidateDateParam(req.StartDate, "开始日期"); err != nil {
		return nil, err
	}
	if err := common.ValidateDateParam(req.EndDate, "结束日期"); err != nil {
		return nil, err
	}

	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "billing:read"); err != nil {
		return nil, err
	}

	page := req.Page
	pageSize := req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	m := dao.BilRecords.Ctx(ctx).Where("tenant_id", tenantID)
	m = m.Where("created_at >= ?", req.StartDate+" 00:00:00")
	m = m.Where("created_at <= ?", req.EndDate+" 23:59:59")

	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	type billingRow struct {
		Id        int64       `json:"id"`
		Type      string      `json:"type"`
		TotalCost float64     `json:"total_cost"`
		Balance   float64     `json:"balance"`
		ModelName string      `json:"model_name"`
		CreatedAt *gtime.Time `json:"created_at"`
	}

	var records []billingRow
	err = m.Fields("id, relay_mode as type, total_cost, 0 as balance, model_name, created_at").
		OrderDesc("created_at").
		Page(page, pageSize).
		Scan(&records)
	if err != nil {
		return nil, err
	}

	items := make([]v1.OpenBillingItem, 0, len(records))
	for _, r := range records {
		item := v1.OpenBillingItem{
			ID:          r.Id,
			Type:        r.Type,
			Amount:      fmt.Sprintf("%.6f", r.TotalCost),
			Description: r.ModelName,
		}
		if r.CreatedAt != nil {
			item.CreatedAt = r.CreatedAt.Format("Y-m-d H:i:s")
		}
		items = append(items, item)
	}

	return &v1.OpenBillingQueryRes{List: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// errOpen is a helper to create open platform errors with proper gerror codes.
func errOpen(code int, message string) error {
	return gerror.NewCode(gcode.New(code, message, nil), message)
}

// ============================================================
// 项目管理
// ============================================================

func (s *sOpen) OpenProjectList(ctx context.Context, req *v1.OpenProjectListReq) (*v1.OpenProjectListRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "projects:read"); err != nil {
		return nil, err
	}

	page := req.Page
	pageSize := req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	m := dao.TntProjects.Ctx(ctx).Where("tenant_id", tenantID)
	if req.Status != "" {
		m = m.Where("status", req.Status)
	}
	if req.Keyword != "" {
		kw := "%" + strings.TrimSpace(req.Keyword) + "%"
		m = m.Where("name LIKE ?", kw)
	}

	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	type projectRow struct {
		Id          int64       `json:"id"`
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Status      string      `json:"status"`
		Budget      *float64    `json:"budget"`
		CreatedAt   *gtime.Time `json:"created_at"`
		UpdatedAt   *gtime.Time `json:"updated_at"`
	}

	var projects []projectRow
	err = m.Fields("id, name, description, status, budget, created_at, updated_at").
		OrderDesc("id").Page(page, pageSize).Scan(&projects)
	if err != nil {
		return nil, err
	}

	items := make([]v1.OpenProjectItem, 0, len(projects))
	for _, p := range projects {
		item := v1.OpenProjectItem{
			ID:          p.Id,
			Name:        p.Name,
			Description: p.Description,
			Status:      p.Status,
		}
		if p.Budget != nil {
			item.Budget = fmt.Sprintf("%.6f", *p.Budget)
		} else {
			item.Budget = "unlimited"
		}
		if p.CreatedAt != nil {
			item.CreatedAt = p.CreatedAt.Format("Y-m-d H:i:s")
		}
		if p.UpdatedAt != nil {
			item.UpdatedAt = p.UpdatedAt.Format("Y-m-d H:i:s")
		}
		items = append(items, item)
	}

	return &v1.OpenProjectListRes{List: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *sOpen) OpenProjectCreate(ctx context.Context, req *v1.OpenProjectCreateReq) (*v1.OpenProjectCreateRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "projects:write"); err != nil {
		return nil, err
	}

	insertData := do.TntProjects{
		TenantId:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Status:      "active",
	}

	if req.Budget > 0 {
		budgetDecimal := billing.NewFromFloat(req.Budget)
		insertData.Budget = &budgetDecimal
	}

	result, err := dao.TntProjects.Ctx(ctx).Data(insertData).Insert()
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &v1.OpenProjectCreateRes{ID: id}, nil
}

func (s *sOpen) OpenProjectGet(ctx context.Context, req *v1.OpenProjectGetReq) (*v1.OpenProjectGetRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "projects:read"); err != nil {
		return nil, err
	}

	type projectRow struct {
		Id          int64       `json:"id"`
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Status      string      `json:"status"`
		Budget      *float64    `json:"budget"`
		CreatedAt   *gtime.Time `json:"created_at"`
		UpdatedAt   *gtime.Time `json:"updated_at"`
	}

	var p *projectRow
	err := dao.TntProjects.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Fields("id, name, description, status, budget, created_at, updated_at").
		Scan(&p)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errOpen(consts.CodeNotFound, "项目不存在")
	}

	// Key 统计
	type keyStatsRow struct {
		Total  int `json:"total"`
		Active int `json:"active"`
	}
	var keyStats keyStatsRow
	dao.ApiKeys.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("project_id", req.Id).
		Fields("COUNT(*) as total, COUNT(*) FILTER (WHERE status = 'active') as active").
		Scan(&keyStats)

	// 月度用量
	type monthUsageRow struct {
		TotalCost    float64 `json:"total_cost"`
		RequestCount int     `json:"request_count"`
	}
	var monthUsage monthUsageRow
	dao.BilUsageLogs.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("project_id", req.Id).
		Where("created_at >= date_trunc('month', NOW())").
		Fields("COALESCE(SUM(actual_cost), 0) as total_cost, COUNT(*) as request_count").
		Scan(&monthUsage)

	res := &v1.OpenProjectGetRes{
		ID:            p.Id,
		Name:          p.Name,
		Description:   p.Description,
		Status:        p.Status,
		ActiveKeys:    keyStats.Active,
		TotalKeys:     keyStats.Total,
		MonthCost:     fmt.Sprintf("%.6f", monthUsage.TotalCost),
		MonthRequests: int64(monthUsage.RequestCount),
	}
	if p.Budget != nil {
		res.Budget = fmt.Sprintf("%.6f", *p.Budget)
	} else {
		res.Budget = "unlimited"
	}
	if p.CreatedAt != nil {
		res.CreatedAt = p.CreatedAt.Format("Y-m-d H:i:s")
	}
	if p.UpdatedAt != nil {
		res.UpdatedAt = p.UpdatedAt.Format("Y-m-d H:i:s")
	}

	return res, nil
}

func (s *sOpen) OpenProjectUpdate(ctx context.Context, req *v1.OpenProjectUpdateReq) (*v1.OpenProjectUpdateRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "projects:write"); err != nil {
		return nil, err
	}

	project, err := s.getProjectOrError(ctx, req.Id, tenantID)
	if err != nil {
		return nil, err
	}
	if project.Status == "archived" {
		return nil, errOpen(consts.CodeBadRequest, "归档的项目不能直接编辑，请先取消归档")
	}

	data := do.TntProjects{}
	hasUpdate := false
	if req.Name != nil {
		data.Name = *req.Name
		hasUpdate = true
	}
	if req.Description != nil {
		data.Description = *req.Description
		hasUpdate = true
	}
	if req.Budget != nil {
		if *req.Budget > 0 {
			budgetDecimal := billing.NewFromFloat(*req.Budget)
			data.Budget = &budgetDecimal
		}
		hasUpdate = true
	}
	// 预算耗尽状态的项目，更新预算后自动恢复为 active
	if project.Status == "budget_exhausted" && req.Budget != nil && *req.Budget > 0 {
		data.Status = "active"
		hasUpdate = true
	}

	if !hasUpdate {
		return nil, nil
	}

	_, err = dao.TntProjects.Ctx(ctx).Where("id", req.Id).Data(data).Update()
	return nil, err
}

func (s *sOpen) OpenProjectArchive(ctx context.Context, req *v1.OpenProjectArchiveReq) (*v1.OpenProjectArchiveRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "projects:write"); err != nil {
		return nil, err
	}

	project, err := s.getProjectOrError(ctx, req.Id, tenantID)
	if err != nil {
		return nil, err
	}
	if project.Status == "archived" {
		return nil, errOpen(consts.CodeBadRequest, "项目已归档")
	}

	keyPrefixes, err := getProjectApiKeyPrefixes(ctx, tenantID, req.Id)
	if err != nil {
		return nil, err
	}

	// 吊销项目下所有 active Key
	_, err = dao.ApiKeys.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("project_id", req.Id).
		Where("status", "active").
		Data(do.ApiKeys{Status: "revoked"}).Update()
	if err != nil {
		return nil, err
	}

	_, err = dao.TntProjects.Ctx(ctx).Where("id", req.Id).
		Data(do.TntProjects{Status: "archived"}).Update()
	if err == nil {
		invalidateApiKeyPrefixes(ctx, keyPrefixes)
	}
	return nil, err
}

func (s *sOpen) OpenProjectUnarchive(ctx context.Context, req *v1.OpenProjectUnarchiveReq) (*v1.OpenProjectUnarchiveRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "projects:write"); err != nil {
		return nil, err
	}

	project, err := s.getProjectOrError(ctx, req.Id, tenantID)
	if err != nil {
		return nil, err
	}
	if project.Status != "archived" {
		return nil, errOpen(consts.CodeBadRequest, "只有归档状态的项目可以取消归档")
	}

	_, err = dao.TntProjects.Ctx(ctx).Where("id", req.Id).
		Data(do.TntProjects{Status: "active"}).Update()
	return nil, err
}

// ============================================================
// 项目 API Key 管理
// ============================================================

func (s *sOpen) OpenProjectKeyList(ctx context.Context, req *v1.OpenProjectKeyListReq) (*v1.OpenProjectKeyListRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "projects:read"); err != nil {
		return nil, err
	}

	if _, err := s.getProjectOrError(ctx, req.Id, tenantID); err != nil {
		return nil, err
	}

	page := req.Page
	pageSize := req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	m := dao.ApiKeys.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("project_id", req.Id)

	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	type keyRow struct {
		Id        int64       `json:"id"`
		Name      string      `json:"name"`
		KeyPrefix string      `json:"key_prefix"`
		Status    string      `json:"status"`
		CreatedAt *gtime.Time `json:"created_at"`
		ExpiresAt *gtime.Time `json:"expires_at"`
	}

	var keys []keyRow
	err = m.Fields("id, name, key_prefix, status, created_at, expires_at").
		OrderDesc("id").Page(page, pageSize).Scan(&keys)
	if err != nil {
		return nil, err
	}

	items := make([]v1.OpenProjectKeyItem, 0, len(keys))
	for _, k := range keys {
		item := v1.OpenProjectKeyItem{
			ID:        k.Id,
			Name:      k.Name,
			KeyPrefix: k.KeyPrefix,
			Status:    k.Status,
		}
		if k.CreatedAt != nil {
			item.CreatedAt = k.CreatedAt.Format("Y-m-d H:i:s")
		}
		if k.ExpiresAt != nil {
			item.ExpiresAt = k.ExpiresAt.Format("Y-m-d H:i:s")
		}
		items = append(items, item)
	}

	return &v1.OpenProjectKeyListRes{List: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *sOpen) OpenProjectKeyCreate(ctx context.Context, req *v1.OpenProjectKeyCreateReq) (*v1.OpenProjectKeyCreateRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "projects:write"); err != nil {
		return nil, err
	}

	project, err := s.getProjectOrError(ctx, req.Id, tenantID)
	if err != nil {
		return nil, err
	}
	if project.Status != "active" {
		return nil, errOpen(consts.CodeProjectNotActive, "项目状态不可用")
	}

	rawKey, prefix, encryptedKey, err := relay.GenerateApiKey(ctx)
	if err != nil {
		return nil, err
	}

	insertData := do.ApiKeys{
		TenantId:     tenantID,
		ProjectId:    req.Id,
		Name:         req.Name,
		EncryptedKey: encryptedKey,
		KeyPrefix:    prefix,
		Scope:        req.Scope,
		Status:       "active",
		KeyType:      "project",
	}

	if req.ExpiresAt != nil {
		insertData.ExpiresAt = req.ExpiresAt
	}

	result, err := dao.ApiKeys.Ctx(ctx).Data(insertData).Insert()
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()

	for _, modelName := range req.ModelNames {
		if modelName != "" {
			if _, err = dao.ApiKeyModelScopes.Ctx(ctx).Insert(do.ApiKeyModelScopes{
				ApiKeyId:  id,
				ModelName: modelName,
			}); err != nil {
				g.Log().Warningf(ctx, "创建密钥模型范围失败: %v", err)
			}
		}
	}

	return &v1.OpenProjectKeyCreateRes{
		ID:        id,
		Name:      req.Name,
		Key:       rawKey,
		KeyPrefix: prefix,
	}, nil
}

func (s *sOpen) OpenProjectKeyDelete(ctx context.Context, req *v1.OpenProjectKeyDeleteReq) (*v1.OpenProjectKeyDeleteRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "projects:write"); err != nil {
		return nil, err
	}

	// 验证密钥属于该项目和租户
	var key *struct {
		KeyPrefix string `json:"key_prefix"`
	}
	err := dao.ApiKeys.Ctx(ctx).
		Where("id", req.KeyId).
		Where("tenant_id", tenantID).
		Where("project_id", req.Id).
		Fields("key_prefix").
		Scan(&key)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, errOpen(consts.CodeNotFound, "密钥不存在")
	}

	_, err = dao.ApiKeys.Ctx(ctx).
		Where("id", req.KeyId).
		Where("tenant_id", tenantID).
		Data(do.ApiKeys{Status: "revoked"}).Update()
	if err == nil {
		relay.InvalidateApiKey(ctx, key.KeyPrefix)
	}
	return nil, err
}

// ============================================================
// 项目用量查询
// ============================================================

func (s *sOpen) OpenProjectUsageStats(ctx context.Context, req *v1.OpenProjectUsageStatsReq) (*v1.OpenProjectUsageStatsRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "projects:read"); err != nil {
		return nil, err
	}

	if _, err := s.getProjectOrError(ctx, req.Id, tenantID); err != nil {
		return nil, err
	}

	// 构建日期条件
	useCustomRange := req.StartDate != "" && req.EndDate != ""
	if useCustomRange {
		if err := common.ValidateDateParam(req.StartDate, "开始日期"); err != nil {
			return nil, err
		}
		if err := common.ValidateDateParam(req.EndDate, "结束日期"); err != nil {
			return nil, err
		}
	}

	dateCondition := func(m *gdb.Model) *gdb.Model {
		if useCustomRange {
			return m.Where("created_at >= ?", req.StartDate+" 00:00:00").
				Where("created_at <= ?", req.EndDate+" 23:59:59")
		}
		return m.Where("created_at >= NOW() - INTERVAL '30 days'")
	}

	// 汇总
	type totalRow struct {
		TotalCost    float64 `json:"total_cost"`
		RequestCount int     `json:"request_count"`
		InputTokens  int64   `json:"input_tokens"`
		OutputTokens int64   `json:"output_tokens"`
	}
	var totalStats totalRow
	dateCondition(dao.BilUsageLogs.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("project_id", req.Id).
		Fields("COALESCE(SUM(actual_cost), 0) as total_cost, COUNT(*) as request_count, COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens")).
		Scan(&totalStats)

	// 每日趋势
	type dailyRow struct {
		Date         string  `json:"date"`
		RequestCount int     `json:"request_count"`
		TotalCost    float64 `json:"total_cost"`
		InputTokens  int64   `json:"input_tokens"`
		OutputTokens int64   `json:"output_tokens"`
	}
	var dailyStats []dailyRow
	dateCondition(dao.BilUsageLogs.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("project_id", req.Id).
		Fields("DATE(created_at) as date, COUNT(*) as request_count, COALESCE(SUM(actual_cost), 0) as total_cost, COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens")).
		Group("DATE(created_at)").OrderAsc("date").Scan(&dailyStats)

	// 模型分布（Top 10）
	type modelRow struct {
		ModelName    string  `json:"model_name"`
		RequestCount int     `json:"request_count"`
		TotalCost    float64 `json:"total_cost"`
	}
	var modelStats []modelRow
	dateCondition(dao.BilUsageLogs.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("project_id", req.Id).
		Fields("model_name, COUNT(*) as request_count, COALESCE(SUM(actual_cost), 0) as total_cost")).
		Group("model_name").OrderDesc("total_cost").Limit(10).Scan(&modelStats)

	daily := make([]v1.OpenProjectDailyStat, 0, len(dailyStats))
	for _, d := range dailyStats {
		daily = append(daily, v1.OpenProjectDailyStat{
			Date:         d.Date,
			RequestCount: int64(d.RequestCount),
			TotalCost:    fmt.Sprintf("%.6f", d.TotalCost),
			InputTokens:  d.InputTokens,
			OutputTokens: d.OutputTokens,
		})
	}

	models := make([]v1.OpenProjectModelStat, 0, len(modelStats))
	for _, m := range modelStats {
		models = append(models, v1.OpenProjectModelStat{
			ModelName:    m.ModelName,
			RequestCount: int64(m.RequestCount),
			TotalCost:    fmt.Sprintf("%.6f", m.TotalCost),
		})
	}

	return &v1.OpenProjectUsageStatsRes{
		TotalCost:         fmt.Sprintf("%.6f", totalStats.TotalCost),
		TotalRequests:     int64(totalStats.RequestCount),
		TotalInputTokens:  totalStats.InputTokens,
		TotalOutputTokens: totalStats.OutputTokens,
		Daily:             daily,
		Models:            models,
	}, nil
}

func (s *sOpen) OpenProjectUsageLogs(ctx context.Context, req *v1.OpenProjectUsageLogsReq) (*v1.OpenProjectUsageLogsRes, error) {
	tenantID := middleware.GetOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, errOpen(consts.CodeUnauthorized, "未认证")
	}
	if err := middleware.CheckOpenPermission(ctx, "projects:read"); err != nil {
		return nil, err
	}

	if _, err := s.getProjectOrError(ctx, req.Id, tenantID); err != nil {
		return nil, err
	}

	page := req.Page
	pageSize := req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	m := dao.BilUsageLogs.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("project_id", req.Id)

	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	type logRow struct {
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

	var logs []logRow
	err = m.Fields("id, model_name, relay_mode, input_tokens, output_tokens, actual_cost as total_cost, latency_ms, status, error_message, created_at").
		OrderDesc("created_at").Page(page, pageSize).Scan(&logs)
	if err != nil {
		return nil, err
	}

	items := make([]v1.OpenProjectUsageLogItem, 0, len(logs))
	for _, l := range logs {
		item := v1.OpenProjectUsageLogItem{
			ID:           l.Id,
			ModelName:    l.ModelName,
			RelayMode:    l.RelayMode,
			InputTokens:  l.InputTokens,
			OutputTokens: l.OutputTokens,
			TotalCost:    fmt.Sprintf("%.6f", l.TotalCost),
			LatencyMs:    l.LatencyMs,
			Status:       l.Status,
			ErrorMessage: l.ErrorMessage,
		}
		if l.CreatedAt != nil {
			item.CreatedAt = l.CreatedAt.Format("Y-m-d H:i:s")
		}
		items = append(items, item)
	}

	return &v1.OpenProjectUsageLogsRes{List: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// getProjectOrError 验证项目归属并返回基本信息
func (s *sOpen) getProjectOrError(ctx context.Context, projectID, tenantID int64) (*struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}, error) {
	var project *struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	err := dao.TntProjects.Ctx(ctx).
		Where("id", projectID).
		Where("tenant_id", tenantID).
		Scan(&project)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, errOpen(consts.CodeNotFound, "项目不存在")
	}
	return project, nil
}

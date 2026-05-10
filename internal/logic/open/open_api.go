package open

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/relay"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/service"
	"github.com/qianfree/team-api/internal/utility/crypto"
)

type sOpen struct{}

func New() *sOpen {
	return &sOpen{}
}

func init() {
	service.RegisterOpen(New())
}

// ctxOpenTenantID extracts tenant_id from context (injected by OpenPlatformAuth middleware).
func ctxOpenTenantID(ctx context.Context) int64 {
	val := ctx.Value("openTenantId")
	if val != nil {
		if id, ok := val.(int64); ok {
			return id
		}
	}
	return 0
}

// ctxOpenAppID extracts app id from context.
func ctxOpenAppID(ctx context.Context) int64 {
	val := ctx.Value("openAppId")
	if val != nil {
		if id, ok := val.(int64); ok {
			return id
		}
	}
	return 0
}

// ============================================================
// 成员管理
// ============================================================

func (s *sOpen) OpenMemberList(ctx context.Context, req *v1.OpenMemberListReq) (*v1.OpenMemberListRes, error) {
	tenantID := ctxOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("unauthorized")
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
	tenantID := ctxOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}

	// Check duplicate email
	count, _ := dao.TntUsers.Ctx(ctx).Where("tenant_id", tenantID).Where("email", req.Email).Count()
	if count > 0 {
		return nil, fmt.Errorf("邮箱已存在")
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
	tenantID := ctxOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}

	data := g.Map{}
	if req.Role != nil {
		data["role"] = *req.Role
	}
	if req.DisplayName != nil {
		data["display_name"] = *req.DisplayName
	}
	if req.Status != nil {
		data["status"] = *req.Status
	}

	if len(data) == 0 {
		return nil, nil
	}

	_, err := dao.TntUsers.Ctx(ctx).Where("id", req.Id).Where("tenant_id", tenantID).Data(data).Update()
	return nil, err
}

func (s *sOpen) OpenMemberDelete(ctx context.Context, req *v1.OpenMemberDeleteReq) (*v1.OpenMemberDeleteRes, error) {
	tenantID := ctxOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}

	// Prevent deleting owner
	var user struct {
		Role string `json:"role"`
	}
	err := dao.TntUsers.Ctx(ctx).Where("id", req.Id).Where("tenant_id", tenantID).Scan(&user)
	if err != nil {
		return nil, err
	}
	if user.Role == "owner" {
		return nil, fmt.Errorf("不能删除所有者")
	}

	_, err = dao.TntUsers.Ctx(ctx).Where("id", req.Id).Where("tenant_id", tenantID).Delete()
	return nil, err
}

// ============================================================
// 成员额度 & 模型
// ============================================================

func (s *sOpen) OpenMemberQuota(ctx context.Context, req *v1.OpenMemberQuotaReq) (*v1.OpenMemberQuotaRes, error) {
	tenantID := ctxOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}

	var user struct {
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

	return &v1.OpenMemberQuotaRes{
		QuotaType:  user.QuotaType,
		QuotaLimit: user.QuotaLimit,
		QuotaUsed:  user.QuotaUsed,
		Period:     user.QuotaPeriod,
	}, nil
}

func (s *sOpen) OpenMemberQuotaUpdate(ctx context.Context, req *v1.OpenMemberQuotaUpdateReq) (*v1.OpenMemberQuotaUpdateRes, error) {
	tenantID := ctxOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}

	if req.QuotaType == "periodic" && req.Period == "" {
		return nil, fmt.Errorf("period is required for periodic quota")
	}

	data := g.Map{
		"quota_type":     req.QuotaType,
		"quota_limit":    req.QuotaLimit,
		"quota_used":     0,
		"quota_period":   nil,
		"quota_reset_at": nil,
	}
	if req.QuotaType == "periodic" {
		data["quota_period"] = req.Period
		data["quota_reset_at"] = gtime.Now()
	}
	if req.QuotaType == "none" {
		data["quota_limit"] = 0
	}

	_, err := dao.TntUsers.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Data(data).
		Update()
	return nil, err
}

func (s *sOpen) OpenMemberModels(ctx context.Context, req *v1.OpenMemberModelsReq) (*v1.OpenMemberModelsRes, error) {
	tenantID := ctxOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("unauthorized")
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
	tenantID := ctxOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}

	// Delete existing scopes
	_, err := dao.TntMemberModelScopes.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("user_id", req.Id).
		Delete()
	if err != nil {
		return nil, err
	}

	// Insert new scopes
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
			return nil, err
		}
	}

	return nil, nil
}

// ============================================================
// API Key 管理
// ============================================================

func (s *sOpen) OpenKeyList(ctx context.Context, req *v1.OpenKeyListReq) (*v1.OpenKeyListRes, error) {
	tenantID := ctxOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("unauthorized")
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
	tenantID := ctxOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}

	// Generate API key
	rawKey, prefix, encryptedKey, err := relay.GenerateApiKey(ctx)
	if err != nil {
		return nil, err
	}

	quotaLimit := req.QuotaLimit
	if quotaLimit == "" {
		quotaLimit = "0"
	}

	result, err := dao.ApiKeys.Ctx(ctx).Data(do.ApiKeys{
		TenantId:     tenantID,
		Name:         req.Name,
		EncryptedKey: encryptedKey,
		KeyPrefix:    prefix,
		Scope:        "full",
		Status:       "active",
		TotalQuota:   quotaLimit,
		KeyType:      "project",
	}).Insert()
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()

	// Set model scopes if provided
	for _, modelName := range req.ModelScopes {
		if modelName != "" {
			dao.ApiKeyModelScopes.Ctx(ctx).Insert(do.ApiKeyModelScopes{
				ApiKeyId:  id,
				ModelName: modelName,
			})
		}
	}

	return &v1.OpenKeyCreateRes{ID: id, Key: rawKey}, nil
}

func (s *sOpen) OpenKeyDelete(ctx context.Context, req *v1.OpenKeyDeleteReq) (*v1.OpenKeyDeleteRes, error) {
	tenantID := ctxOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("unauthorized")
	}

	_, err := dao.ApiKeys.Ctx(ctx).Where("id", req.Id).Where("tenant_id", tenantID).Delete()
	return nil, err
}

// ============================================================
// 用量查询
// ============================================================

func (s *sOpen) OpenUsageQuery(ctx context.Context, req *v1.OpenUsageQueryReq) (*v1.OpenUsageQueryRes, error) {
	tenantID := ctxOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("unauthorized")
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

	// Group by
	selectFields := ""
	groupBy := ""
	switch req.GroupBy {
	case "model":
		selectFields = "model_name, SUM(request_count) as request_count, SUM(prompt_tokens) as prompt_tokens, SUM(completion_tokens) as completion_tokens, SUM(total_tokens) as total_tokens, SUM(cost) as cost"
		groupBy = "model_name"
	case "key":
		selectFields = "key_prefix as key_name, SUM(request_count) as request_count, SUM(prompt_tokens) as prompt_tokens, SUM(completion_tokens) as completion_tokens, SUM(total_tokens) as total_tokens, SUM(cost) as cost"
		groupBy = "key_prefix"
	default:
		selectFields = "DATE(created_at) as date, SUM(request_count) as request_count, SUM(prompt_tokens) as prompt_tokens, SUM(completion_tokens) as completion_tokens, SUM(total_tokens) as total_tokens, SUM(cost) as cost"
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
	tenantID := ctxOpenTenantID(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("unauthorized")
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

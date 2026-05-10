package tenant

import (
	"context"
	"github.com/qianfree/team-api/internal/dao"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/consts"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/relay"
	do "github.com/qianfree/team-api/internal/model/do"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/utility/export"
)

// ApiKeyList 列出 API Keys，支持按类型过滤
func (s *sTenant) ApiKeyList(ctx context.Context, req *v1.TenantApiKeyListReq) (*v1.TenantApiKeyListRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)
	role := ctxUserRole(ctx)
	page, pageSize := lcommon.NormalizePagination(req.Page, req.PageSize)

	// member 只能查看个人密钥
	if role == "member" && req.KeyType == "project" {
		return &v1.TenantApiKeyListRes{
			List:     []map[string]any{},
			Total:    0,
			Page:     page,
			PageSize: pageSize,
		}, nil
	}

	query := dao.ApiKeys.Ctx(ctx).Where("tenant_id", tenantID)

	if req.KeyType == "project" {
		// 项目密钥：owner/admin 可看租户内所有项目密钥
		query = query.Where("key_type", "project")
		if req.ProjectID > 0 {
			query = query.Where("project_id", req.ProjectID)
		}
	} else {
		// 个人密钥（默认）：只看自己的
		query = query.Where("user_id", userID).Where("key_type", "personal")
	}

	type keyRow struct {
		Id                   int64      `json:"id"`
		Name                 string     `json:"name"`
		KeyPrefix            string     `json:"key_prefix"`
		Scope                string     `json:"scope"`
		Status               string     `json:"status"`
		KeyType              string     `json:"key_type"`
		ProjectId            *int64     `json:"project_id"`
		ExpiresAt            *time.Time `json:"expires_at"`
		RateLimitQps         *int       `json:"rate_limit_qps"`
		RateLimitConcurrency *int       `json:"rate_limit_concurrency"`
		TotalQuota           *float64   `json:"total_quota"`
		UsedQuota            *float64   `json:"used_quota"`
		CreatedAt            *time.Time `json:"created_at"`
		UpdatedAt            *time.Time `json:"updated_at"`
	}

	var keys []keyRow
	var err error
	var total int
	err = query.Fields("id, name, key_prefix, scope, status, key_type, project_id, expires_at, rate_limit_qps, rate_limit_concurrency, total_quota, used_quota, created_at, updated_at").
		OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&keys, &total, false)
	if err != nil {
		return nil, err
	}
	if keys == nil {
		keys = []keyRow{}
	}

	list := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		list = append(list, map[string]any{
			"id":                     k.Id,
			"name":                   k.Name,
			"key_prefix":             k.KeyPrefix,
			"scope":                  k.Scope,
			"status":                 k.Status,
			"key_type":               k.KeyType,
			"project_id":             k.ProjectId,
			"expires_at":             k.ExpiresAt,
			"rate_limit_qps":         k.RateLimitQps,
			"rate_limit_concurrency": k.RateLimitConcurrency,
			"total_quota":            k.TotalQuota,
			"used_quota":             k.UsedQuota,
			"created_at":             k.CreatedAt,
			"updated_at":             k.UpdatedAt,
		})
	}

	return &v1.TenantApiKeyListRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ApiKeyCreate 创建新的 API Key
func (s *sTenant) ApiKeyCreate(ctx context.Context, req *v1.TenantApiKeyCreateReq) (*v1.TenantApiKeyCreateRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)
	role := ctxUserRole(ctx)

	keyType := req.KeyType
	if keyType == "" {
		keyType = "personal"
	}
	if keyType != "personal" && keyType != "project" {
		return nil, lcommon.NewBusinessError(consts.CodeBadRequest, "无效的密钥类型")
	}

	// 项目密钥权限检查
	if keyType == "project" {
		if role != "owner" && role != "admin" {
			return nil, lcommon.NewBusinessError(consts.CodeProjectKeyForbidden, consts.MsgProjectKeyForbidden)
		}
		if req.ProjectID <= 0 {
			return nil, lcommon.NewBusinessError(consts.CodeBadRequest, "项目密钥必须指定关联项目")
		}
		// 验证项目存在且活跃
		var project struct {
			Id     int64  `json:"id"`
			Status string `json:"status"`
		}
		err := dao.TntProjects.Ctx(ctx).
			Where("id", req.ProjectID).
			Where("tenant_id", tenantID).
			Scan(&project)
		if err != nil {
			return nil, err
		}
		if project.Id == 0 {
			return nil, lcommon.NewBusinessError(consts.CodeProjectNotFound, consts.MsgProjectNotFound)
		}
		if project.Status != "active" {
			return nil, lcommon.NewBusinessError(consts.CodeProjectNotActive, consts.MsgProjectNotActive)
		}
	}

	rawKey, prefix, encryptedKey, err := relay.GenerateApiKey(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	data := do.ApiKeys{
		TenantId:     tenantID,
		UserId:       userID,
		Name:         req.Name,
		EncryptedKey: encryptedKey,
		KeyPrefix:    prefix,
		Scope:        req.Scope,
		Status:       "active",
	}

	if keyType == "project" {
		data.ProjectId = req.ProjectID
	}

	if req.ExpiresInDays > 0 {
		data.ExpiresAt = gtime.NewFromTime(now.AddDate(0, 0, req.ExpiresInDays))
	}

	result, err := dao.ApiKeys.Ctx(ctx).Insert(data)
	if err != nil {
		return nil, err
	}
	keyID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	for _, modelName := range req.ModelNames {
		if modelName != "" {
			dao.ApiKeyModelScopes.Ctx(ctx).Insert(do.ApiKeyModelScopes{
				ApiKeyId:  keyID,
				ModelName: modelName,
			})
		}
	}

	return &v1.TenantApiKeyCreateRes{
		Id:        keyID,
		Name:      req.Name,
		Key:       rawKey,
		KeyPrefix: prefix,
		Scope:     req.Scope,
		KeyType:   keyType,
	}, nil
}

// ApiKeyDelete 禁用 API Key
func (s *sTenant) ApiKeyDelete(ctx context.Context, req *v1.TenantApiKeyDeleteReq) (*v1.TenantApiKeyDeleteRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)
	role := ctxUserRole(ctx)
	keyID := req.Id

	// 先查询密钥信息以判断类型
	type keyInfo struct {
		KeyType   string `json:"key_type"`
		KeyPrefix string `json:"key_prefix"`
	}
	var info keyInfo
	err := dao.ApiKeys.Ctx(ctx).
		Where("id", keyID).
		Where("tenant_id", tenantID).
		Fields("key_type, key_prefix").
		Scan(&info)
	if err != nil {
		return nil, err
	}
	if info.KeyPrefix == "" {
		return nil, lcommon.NewNotFoundError("API key")
	}

	query := dao.ApiKeys.Ctx(ctx).
		Where("id", keyID).
		Where("tenant_id", tenantID)

	if info.KeyType == "project" {
		// 项目密钥：需要 owner/admin 权限
		if role != "owner" && role != "admin" {
			return nil, lcommon.NewBusinessError(consts.CodeProjectKeyForbidden, consts.MsgProjectKeyForbidden)
		}
		// 不限 user_id，按租户级操作
	} else {
		// 个人密钥：只能禁用自己的
		query = query.Where("user_id", userID)
	}

	result, err := query.Data(do.ApiKeys{
		Status: "disabled",
	}).Update()
	if err != nil {
		return nil, err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, lcommon.NewNotFoundError("API key")
	}

	if info.KeyPrefix != "" {
		g.Log().Infof(ctx, "API key %d disabled, cache invalidation needed for prefix %s", keyID, info.KeyPrefix)
	}

	return &v1.TenantApiKeyDeleteRes{}, nil
}

// ApiKeyUpdateScopes 更新 API Key 的模型 scope
func (s *sTenant) ApiKeyUpdateScopes(ctx context.Context, req *v1.TenantApiKeyUpdateScopesReq) (*v1.TenantApiKeyUpdateScopesRes, error) {
	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)
	role := ctxUserRole(ctx)
	keyID := req.Id

	// 先查询密钥信息以判断类型
	type keyInfo struct {
		KeyType string `json:"key_type"`
	}
	var info keyInfo
	err := dao.ApiKeys.Ctx(ctx).
		Where("id", keyID).
		Where("tenant_id", tenantID).
		Fields("key_type").
		Scan(&info)
	if err != nil {
		return nil, err
	}
	if info.KeyType == "" {
		return nil, lcommon.NewNotFoundError("API key")
	}

	// 权限检查
	if info.KeyType == "project" {
		if role != "owner" && role != "admin" {
			return nil, lcommon.NewBusinessError(consts.CodeProjectKeyForbidden, consts.MsgProjectKeyForbidden)
		}
	} else {
		// 个人密钥：只能更新自己的
		count, err := dao.ApiKeys.Ctx(ctx).
			Where("id", keyID).
			Where("tenant_id", tenantID).
			Where("user_id", userID).
			Count()
		if err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, lcommon.NewNotFoundError("API key")
		}
	}

	dao.ApiKeyModelScopes.Ctx(ctx).
		Where("api_key_id", keyID).
		Delete()

	for _, modelName := range req.ModelNames {
		if modelName != "" {
			dao.ApiKeyModelScopes.Ctx(ctx).Insert(do.ApiKeyModelScopes{
				ApiKeyId:  keyID,
				ModelName: modelName,
			})
		}
	}

	return &v1.TenantApiKeyUpdateScopesRes{}, nil
}

// listProjectApiKeys 列出指定项目的 API Keys
func listProjectApiKeys(ctx context.Context, tenantID, projectID int64, page, pageSize int) ([]map[string]any, int, error) {
	page, pageSize = lcommon.NormalizePagination(page, pageSize)

	// 验证项目存在且属于该租户
	var count int
	count, err := dao.TntProjects.Ctx(ctx).
		Where("id", projectID).
		Where("tenant_id", tenantID).
		Count()
	if err != nil {
		return nil, 0, err
	}
	if count == 0 {
		return nil, 0, lcommon.NewBusinessError(consts.CodeProjectNotFound, consts.MsgProjectNotFound)
	}

	query := dao.ApiKeys.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("project_id", projectID).
		Where("key_type", "project")

	type keyRow struct {
		Id                   int64      `json:"id"`
		Name                 string     `json:"name"`
		KeyPrefix            string     `json:"key_prefix"`
		Scope                string     `json:"scope"`
		Status               string     `json:"status"`
		ExpiresAt            *time.Time `json:"expires_at"`
		RateLimitQps         *int       `json:"rate_limit_qps"`
		RateLimitConcurrency *int       `json:"rate_limit_concurrency"`
		TotalQuota           *float64   `json:"total_quota"`
		UsedQuota            *float64   `json:"used_quota"`
		CreatedAt            *time.Time `json:"created_at"`
		UpdatedAt            *time.Time `json:"updated_at"`
	}

	var keys []keyRow
	var total int
	err = query.Fields("id, name, key_prefix, scope, status, expires_at, rate_limit_qps, rate_limit_concurrency, total_quota, used_quota, created_at, updated_at").
		OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&keys, &total, false)
	if err != nil {
		return nil, 0, err
	}
	if keys == nil {
		keys = []keyRow{}
	}

	list := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		list = append(list, map[string]any{
			"id":                     k.Id,
			"name":                   k.Name,
			"key_prefix":             k.KeyPrefix,
			"scope":                  k.Scope,
			"status":                 k.Status,
			"expires_at":             k.ExpiresAt,
			"rate_limit_qps":         k.RateLimitQps,
			"rate_limit_concurrency": k.RateLimitConcurrency,
			"total_quota":            k.TotalQuota,
			"used_quota":             k.UsedQuota,
			"created_at":             k.CreatedAt,
			"updated_at":             k.UpdatedAt,
		})
	}

	return list, total, nil
}

// ExportApiKeys exports the tenant API key list as CSV or Excel.
func (s *sTenant) ExportApiKeys(ctx context.Context, req *v1.TenantApiKeyExportReq) (*v1.TenantApiKeyExportRes, error) {
	r := g.RequestFromCtx(ctx)
	format := req.Format
	if format == "" {
		format = "csv"
	}

	tenantID := ctxTenantID(ctx)
	userID := ctxUserID(ctx)

	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "name", Header: "名称"},
		{Field: "key_prefix", Header: "Key前缀"},
		{Field: "key_type", Header: "类型"},
		{Field: "scope", Header: "范围"},
		{Field: "status", Header: "状态"},
		{Field: "expires_at", Header: "过期时间"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   format,
		Filename: "API密钥_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	buildQuery := func() *gdb.Model {
		query := dao.ApiKeys.Ctx(ctx).Where("tenant_id", tenantID)
		if req.KeyType == "project" {
			query = query.Where("key_type", "project")
			if req.ProjectID > 0 {
				query = query.Where("project_id", req.ProjectID)
			}
		} else {
			query = query.Where("user_id", userID).Where("key_type", "personal")
		}
		return query
	}

	if format == "xlsx" {
		type keyRow struct {
			Id        int64      `json:"id"`
			Name      string     `json:"name"`
			KeyPrefix string     `json:"key_prefix"`
			KeyType   string     `json:"key_type"`
			Scope     string     `json:"scope"`
			Status    string     `json:"status"`
			ExpiresAt *time.Time `json:"expires_at"`
			CreatedAt *time.Time `json:"created_at"`
		}

		var keys []keyRow
		err := buildQuery().
			Fields("id, name, key_prefix, key_type, scope, status, expires_at, created_at").
			OrderDesc("created_at").
			Scan(&keys)
		if err != nil {
			return nil, err
		}

		data := make([]map[string]any, 0, len(keys))
		for _, k := range keys {
			data = append(data, map[string]any{
				"id":         k.Id,
				"name":       k.Name,
				"key_prefix": k.KeyPrefix,
				"key_type":   k.KeyType,
				"scope":      k.Scope,
				"status":     k.Status,
				"expires_at": k.ExpiresAt,
				"created_at": k.CreatedAt,
			})
		}
		return nil, export.WriteExcel(r, config, data)
	}

	return nil, export.StreamCSV(r, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			type keyRow struct {
				Id        int64      `json:"id"`
				Name      string     `json:"name"`
				KeyPrefix string     `json:"key_prefix"`
				KeyType   string     `json:"key_type"`
				Scope     string     `json:"scope"`
				Status    string     `json:"status"`
				ExpiresAt *time.Time `json:"expires_at"`
				CreatedAt *time.Time `json:"created_at"`
			}

			var keys []keyRow
			err := buildQuery().
				Fields("id, name, key_prefix, key_type, scope, status, expires_at, created_at").
				OrderDesc("created_at").
				Limit(1000).Offset(offset).
				Scan(&keys)
			if err != nil {
				return
			}
			for _, k := range keys {
				if !yield(map[string]any{
					"id":         k.Id,
					"name":       k.Name,
					"key_prefix": k.KeyPrefix,
					"key_type":   k.KeyType,
					"scope":      k.Scope,
					"status":     k.Status,
					"expires_at": k.ExpiresAt,
					"created_at": k.CreatedAt,
				}) {
					return
				}
			}
			if len(keys) < 1000 {
				break
			}
			offset += 1000
		}
	})
}

package tenant

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/consts"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/relay"
	"github.com/qianfree/team-api/internal/middleware"
	do "github.com/qianfree/team-api/internal/model/do"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	uc "github.com/qianfree/team-api/internal/utility/crypto"
	"github.com/qianfree/team-api/internal/utility/export"
)

// ApiKeyList 列出 API Keys，支持按类型过滤
func (s *sTenant) ApiKeyList(ctx context.Context, req *v1.TenantApiKeyListReq) (*v1.TenantApiKeyListRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)
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
		query = query.Where("key_type", "project")
		if req.ProjectID > 0 {
			query = query.Where("project_id", req.ProjectID)
		}
	} else {
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

	// 批量查询所有 key 的模型数量（修复 N+1）
	keyIDs := make([]int64, 0, len(keys))
	for _, k := range keys {
		keyIDs = append(keyIDs, k.Id)
	}
	modelCountMap := make(map[int64]int, len(keys))
	if len(keyIDs) > 0 {
		type countRow struct {
			ApiKeyId int64 `json:"api_key_id"`
			Cnt      int   `json:"cnt"`
		}
		var counts []countRow
		err = dao.ApiKeyModelScopes.Ctx(ctx).
			Fields("api_key_id, COUNT(*) AS cnt").
			WhereIn("api_key_id", keyIDs).
			Group("api_key_id").
			Scan(&counts)
		if err == nil {
			for _, c := range counts {
				modelCountMap[c.ApiKeyId] = c.Cnt
			}
		}
	}

	list := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		list = append(list, map[string]any{
			"id":                     k.Id,
			"name":                   k.Name,
			"key_prefix":             k.KeyPrefix,
			"scope":                  k.Scope,
			"model_count":            modelCountMap[k.Id],
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
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)

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
		var project *struct {
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
		if project == nil {
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

	if req.ExpiresAt != nil {
		data.ExpiresAt = req.ExpiresAt
	}

	result, err := dao.ApiKeys.Ctx(ctx).Insert(data)
	if err != nil {
		return nil, err
	}
	keyID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	// 写入模型范围，检查每次插入的错误
	for _, modelName := range req.ModelNames {
		if modelName != "" {
			if _, err := dao.ApiKeyModelScopes.Ctx(ctx).Insert(do.ApiKeyModelScopes{
				ApiKeyId:  keyID,
				ModelName: modelName,
			}); err != nil {
				return nil, lcommon.NewBusinessError(consts.CodeBadRequest, fmt.Sprintf("写入模型范围 %s 失败", modelName))
			}
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
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)
	keyID := req.Id

	// 先查询密钥信息以判断类型
	type keyInfo struct {
		KeyType   string `json:"key_type"`
		KeyPrefix string `json:"key_prefix"`
	}
	var info *keyInfo
	err := dao.ApiKeys.Ctx(ctx).
		Where("id", keyID).
		Where("tenant_id", tenantID).
		Fields("key_type, key_prefix").
		Scan(&info)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, lcommon.NewNotFoundError("API key")
	}

	query := dao.ApiKeys.Ctx(ctx).
		Where("id", keyID).
		Where("tenant_id", tenantID)

	if info.KeyType == "project" {
		if role != "owner" && role != "admin" {
			return nil, lcommon.NewBusinessError(consts.CodeProjectKeyForbidden, consts.MsgProjectKeyForbidden)
		}
	} else {
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

// ApiKeyUpdate 更新 API Key 的可编辑字段
func (s *sTenant) ApiKeyUpdate(ctx context.Context, req *v1.TenantApiKeyUpdateReq) (*v1.TenantApiKeyUpdateRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)
	keyID := req.Id

	// 查询密钥信息
	type keyInfo struct {
		KeyType   string  `json:"key_type"`
		KeyPrefix string  `json:"key_prefix"`
		Status    string  `json:"status"`
		UsedQuota float64 `json:"used_quota"`
	}
	var info *keyInfo
	err := dao.ApiKeys.Ctx(ctx).
		Where("id", keyID).
		Where("tenant_id", tenantID).
		Fields("key_type, key_prefix, status, used_quota").
		Scan(&info)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, lcommon.NewNotFoundError("API key")
	}

	// 权限检查
	if info.KeyType == "project" {
		if role != "owner" && role != "admin" {
			return nil, lcommon.NewBusinessError(consts.CodeProjectKeyForbidden, consts.MsgProjectKeyForbidden)
		}
	} else {
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

	// 使用 DO 对象构建更新数据
	data := do.ApiKeys{}
	hasUpdate := false

	if req.Name != "" {
		data.Name = req.Name
		hasUpdate = true
	}
	if req.Scope != "" {
		data.Scope = req.Scope
		hasUpdate = true
	}
	if req.Status != "" {
		if req.Status != "active" && req.Status != "disabled" {
			return nil, lcommon.NewBusinessError(consts.CodeBadRequest, "无效的状态值")
		}
		// 从 disabled 恢复为 active 时，检查是否已过期
		if req.Status == "active" && info.Status == "disabled" {
			type expireCheck struct {
				ExpiresAt *gtime.Time `json:"expires_at"`
			}
			var ec *expireCheck
			err := dao.ApiKeys.Ctx(ctx).
				Where("id", keyID).
				Fields("expires_at").
				Scan(&ec)
			if err != nil {
				return nil, err
			}
			if ec != nil && ec.ExpiresAt != nil && ec.ExpiresAt.Before(gtime.Now()) {
				return nil, lcommon.NewBusinessError(consts.CodeBadRequest, "密钥已过期，无法重新启用")
			}
		}
		data.Status = req.Status
		hasUpdate = true
	}
	if req.ExpiresAt != nil {
		data.ExpiresAt = req.ExpiresAt
		hasUpdate = true
	}
	if req.RateLimitQps != nil {
		data.RateLimitQps = *req.RateLimitQps
		hasUpdate = true
	}
	if req.RateLimitConcurrency != nil {
		data.RateLimitConcurrency = *req.RateLimitConcurrency
		hasUpdate = true
	}
	if req.TotalQuota != nil {
		if *req.TotalQuota > 0 && *req.TotalQuota < info.UsedQuota {
			return nil, lcommon.NewBusinessError(consts.CodeBadRequest, "总额度不能小于已用额度")
		}
		data.TotalQuota = *req.TotalQuota
		hasUpdate = true
	}

	if hasUpdate {
		_, err = dao.ApiKeys.Ctx(ctx).
			Where("id", keyID).
			Where("tenant_id", tenantID).
			Data(data).
			Update()
		if err != nil {
			return nil, err
		}
	}

	// 更新模型范围（事务内先删后插）
	if req.ModelNames != nil {
		err = updateApiKeyModelScopes(ctx, keyID, req.ModelNames)
		if err != nil {
			return nil, err
		}
	}

	if info.KeyPrefix != "" {
		g.Log().Infof(ctx, "API key %d updated, cache invalidation needed for prefix %s", keyID, info.KeyPrefix)
	}

	return &v1.TenantApiKeyUpdateRes{}, nil
}

// ApiKeyUpdateScopes 更新 API Key 的模型 scope
func (s *sTenant) ApiKeyUpdateScopes(ctx context.Context, req *v1.TenantApiKeyUpdateScopesReq) (*v1.TenantApiKeyUpdateScopesRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)
	keyID := req.Id

	// 先查询密钥信息以判断类型
	type keyInfo struct {
		KeyType string `json:"key_type"`
	}
	var info *keyInfo
	err := dao.ApiKeys.Ctx(ctx).
		Where("id", keyID).
		Where("tenant_id", tenantID).
		Fields("key_type").
		Scan(&info)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, lcommon.NewNotFoundError("API key")
	}

	// 权限检查
	if info.KeyType == "project" {
		if role != "owner" && role != "admin" {
			return nil, lcommon.NewBusinessError(consts.CodeProjectKeyForbidden, consts.MsgProjectKeyForbidden)
		}
	} else {
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

	err = updateApiKeyModelScopes(ctx, keyID, req.ModelNames)
	if err != nil {
		return nil, err
	}

	return &v1.TenantApiKeyUpdateScopesRes{}, nil
}

// updateApiKeyModelScopes 在事务内更新模型范围（先删后插）
func updateApiKeyModelScopes(ctx context.Context, keyID int64, modelNames []string) error {
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Ctx(ctx).Model("api_key_model_scopes").
			Where("api_key_id", keyID).
			Delete()
		if err != nil {
			return err
		}

		for _, modelName := range modelNames {
			if modelName != "" {
				if _, err := tx.Ctx(ctx).Model("api_key_model_scopes").Insert(do.ApiKeyModelScopes{
					ApiKeyId:  keyID,
					ModelName: modelName,
				}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// ApiKeyModelScopes 查询 API Key 的模型范围
func (s *sTenant) ApiKeyModelScopes(ctx context.Context, req *v1.TenantApiKeyModelScopesReq) (*v1.TenantApiKeyModelScopesRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	// 验证 key 属于该租户
	count, err := dao.ApiKeys.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Count()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, lcommon.NewNotFoundError("API key")
	}

	type scopeRow struct {
		ModelName string `json:"model_name"`
	}
	var rows []scopeRow
	err = dao.ApiKeyModelScopes.Ctx(ctx).
		Where("api_key_id", req.Id).
		Fields("model_name").
		Order("model_name").
		Scan(&rows)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, r.ModelName)
	}

	return &v1.TenantApiKeyModelScopesRes{ModelNames: names}, nil
}

// listProjectApiKeys 列出指定项目的 API Keys
func listProjectApiKeys(ctx context.Context, tenantID, projectID int64, page, pageSize int) ([]map[string]any, int, error) {
	page, pageSize = lcommon.NormalizePagination(page, pageSize)

	// 验证项目存在且属于该租户
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
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)

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
		Format:   req.Format,
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

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
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

// ApiKeyReveal 获取 API Key 明文值（用于 Playground 等场景）
func (s *sTenant) ApiKeyReveal(ctx context.Context, req *v1.TenantApiKeyRevealReq) (*v1.TenantApiKeyRevealRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)

	type keyInfo struct {
		KeyType      string `json:"key_type"`
		KeyPrefix    string `json:"key_prefix"`
		Status       string `json:"status"`
		EncryptedKey string `json:"encrypted_key"`
		UserId       int64  `json:"user_id"`
	}
	var info *keyInfo
	err := dao.ApiKeys.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Fields("key_type, key_prefix, status, encrypted_key, user_id").
		Scan(&info)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, lcommon.NewNotFoundError("API key")
	}

	// 权限校验
	if info.KeyType == "project" {
		if role != "owner" && role != "admin" {
			return nil, lcommon.NewBusinessError(consts.CodeProjectKeyForbidden, consts.MsgProjectKeyForbidden)
		}
	} else {
		// 个人密钥：只有本人可以查看
		if info.UserId != userID {
			return nil, lcommon.NewNotFoundError("API key")
		}
	}

	// 解密
	encKey := relay.GetEncryptionKey()
	plainKey, err := uc.DecryptString(encKey, info.EncryptedKey)
	if err != nil {
		return nil, lcommon.NewBusinessError(consts.CodeBadRequest, "密钥解密失败")
	}

	return &v1.TenantApiKeyRevealRes{
		Key:       plainKey,
		KeyPrefix: info.KeyPrefix,
		Status:    info.Status,
	}, nil
}

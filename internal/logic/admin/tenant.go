package admin

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"
	"github.com/qianfree/team-api/internal/utility/crypto"
	"github.com/qianfree/team-api/internal/utility/export"
)

// TenantSelect returns a lightweight paginated tenant list for dropdown selectors.
func (s *sAdmin) TenantSelect(ctx context.Context, req *v1.TenantSelectReq) (*v1.TenantSelectRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	m := dao.TntTenants.Ctx(ctx)
	if req.Keyword != "" {
		keyword := "%" + strings.TrimSpace(req.Keyword) + "%"
		m = m.Where("name LIKE ? OR code LIKE ?", keyword, keyword)
	}

	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	m = dao.TntTenants.Ctx(ctx)
	if req.Keyword != "" {
		keyword := "%" + strings.TrimSpace(req.Keyword) + "%"
		m = m.Where("name LIKE ? OR code LIKE ?", keyword, keyword)
	}

	var tenants []struct {
		Id   int64  `json:"id"`
		Name string `json:"name"`
		Code string `json:"code"`
	}
	err = m.Fields("id, name, code").OrderAsc("id").
		Page(page, pageSize).
		Scan(&tenants)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	items := make([]v1.TenantSelectItem, len(tenants))
	for i, t := range tenants {
		items[i] = v1.TenantSelectItem{
			ID:   t.Id,
			Name: t.Name,
			Code: t.Code,
		}
	}

	return &v1.TenantSelectRes{
		List:     items,
		Total:    int(total),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// CreateTenant creates a new tenant with its owner user and wallet.
func (s *sAdmin) CreateTenant(ctx context.Context, req *v1.TenantCreateReq) (*v1.TenantCreateRes, error) {
	tenantCode := strings.TrimSpace(strings.ToLower(req.TenantCode))
	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(strings.ToLower(req.Email))

	// Validate username format
	if err := common.ValidateUsername(username); err != nil {
		return nil, common.NewBusinessError(consts.CodeInvalidUsername, err.Error())
	}

	count, err := dao.TntTenants.Ctx(ctx).
		Where("code", tenantCode).Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewBusinessError(consts.CodeTenantCodeExists, consts.MsgTenantCodeExists)
	}

	count, err = dao.TntUsers.Ctx(ctx).
		Where("email", email).Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewBadRequestError("邮箱已被使用")
	}

	if err := common.ValidatePassword(req.Password); err != nil {
		return nil, common.NewBusinessError(consts.CodePasswordTooWeak, consts.MsgPasswordTooWeak)
	}

	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// max_members 和 max_concurrency 默认为 nil（跟随等级配置）
	var maxMembersVal *int
	if req.MaxMembers != nil && *req.MaxMembers >= 1 {
		maxMembersVal = req.MaxMembers
	}
	var maxConcurrencyVal *int
	if req.MaxConcurrency != nil {
		maxConcurrencyVal = req.MaxConcurrency
	}

	var tenantID int64

	err = dao.TntTenants.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tenantResult, err := tx.Model("tnt_tenants").Ctx(ctx).Data(do.TntTenants{
			Name:           strings.TrimSpace(req.TenantName),
			Code:           tenantCode,
			MaxMembers:     maxMembersVal,
			MaxConcurrency: maxConcurrencyVal,
			Level:          1,
			Settings:       "{}",
		}).Insert()
		if err != nil {
			return gerror.Wrapf(err, "create tenant")
		}
		tenantID, err = tenantResult.LastInsertId()
		if err != nil {
			return gerror.Wrapf(err, "get tenant id")
		}

		userResult, err := tx.Model("tnt_users").Ctx(ctx).Data(do.TntUsers{
			TenantId:     tenantID,
			Username:     username,
			Email:        email,
			PasswordHash: passwordHash,
			DisplayName:  username,
			Role:         "owner",
		}).Insert()
		if err != nil {
			return gerror.Wrapf(err, "create owner user")
		}
		ownerUserID, err := userResult.LastInsertId()
		if err != nil {
			return gerror.Wrapf(err, "get owner user id")
		}

		_, err = tx.Model("tnt_tenants").Ctx(ctx).
			Where("id", tenantID).
			Data(do.TntTenants{
				OwnerUserId: ownerUserID,
			}).Update()
		if err != nil {
			return gerror.Wrapf(err, "set tenant owner")
		}

		_, err = tx.Model("bil_wallets").Ctx(ctx).Data(do.BilWallets{
			TenantId:      tenantID,
			Balance:       0,
			FrozenBalance: 0,
			Currency:      "USD",
		}).Insert()
		if err != nil {
			return gerror.Wrapf(err, "create wallet")
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &v1.TenantCreateRes{Id: tenantID}, nil
}

// ListTenants returns a paginated list of tenants.
func (s *sAdmin) ListTenants(ctx context.Context, req *v1.TenantListReq) (*v1.TenantListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	m := dao.TntTenants.Ctx(ctx)

	if req.Keyword != "" {
		keyword := "%" + strings.TrimSpace(req.Keyword) + "%"
		m = m.Where("name LIKE ? OR code LIKE ?", keyword, keyword)
	}
	if req.Status != "" {
		m = m.Where("status", req.Status)
	}

	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	// Rebuild model for data query
	m = dao.TntTenants.Ctx(ctx)
	if req.Keyword != "" {
		keyword := "%" + strings.TrimSpace(req.Keyword) + "%"
		m = m.Where("name LIKE ? OR code LIKE ?", keyword, keyword)
	}
	if req.Status != "" {
		m = m.Where("status", req.Status)
	}

	var tenants []struct {
		Id                  int64       `json:"id"`
		Name                string      `json:"name"`
		Code                string      `json:"code"`
		LogoURL             string      `json:"logo_url"`
		OwnerUserID         int64       `json:"owner_user_id"`
		Status              string      `json:"status"`
		MaxMembers          *int        `json:"max_members"`
		MaxConcurrency      *int        `json:"max_concurrency"`
		DefaultChannelScope string      `json:"default_channel_scope"`
		Settings            string      `json:"settings"`
		Level               int         `json:"level"`
		CreatedAt           *gtime.Time `json:"created_at"`
		UpdatedAt           *gtime.Time `json:"updated_at"`
	}
	err = m.OrderDesc("id").
		Page(page, pageSize).
		Scan(&tenants)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	// 批量获取等级配置（避免 N+1）
	var levelConfigs []*entity.TntTenantLevelConfigs
	dao.TntTenantLevelConfigs.Ctx(ctx).OrderAsc("level").Scan(&levelConfigs)
	levelNameMap := make(map[int]string, len(levelConfigs))
	levelMaxMembersMap := make(map[int]int, len(levelConfigs))
	levelMaxConcMap := make(map[int]int, len(levelConfigs))
	for _, lc := range levelConfigs {
		levelNameMap[lc.Level] = lc.Name
		levelMaxMembersMap[lc.Level] = lc.MaxMembers
		levelMaxConcMap[lc.Level] = lc.MaxConcurrency
	}

	items := make([]v1.TenantItem, len(tenants))
	for i, t := range tenants {
		item := v1.TenantItem{
			ID:                  t.Id,
			Name:                t.Name,
			Code:                t.Code,
			LogoURL:             t.LogoURL,
			OwnerUserID:         t.OwnerUserID,
			Status:              t.Status,
			MaxMembers:          t.MaxMembers,
			MaxConcurrency:      t.MaxConcurrency,
			DefaultChannelScope: t.DefaultChannelScope,
			Level:               t.Level,
			LevelName:           levelNameMap[t.Level],
			CreatedAt:           t.CreatedAt.String(),
			UpdatedAt:           t.UpdatedAt.String(),
		}

		// 计算实际生效值
		if t.MaxMembers != nil {
			item.EffectiveMaxMembers = *t.MaxMembers
		} else if v, ok := levelMaxMembersMap[t.Level]; ok {
			item.EffectiveMaxMembers = v
		} else {
			item.EffectiveMaxMembers = 10
		}
		if t.MaxConcurrency != nil {
			item.EffectiveMaxConcurrency = *t.MaxConcurrency
		} else if v, ok := levelMaxConcMap[t.Level]; ok {
			item.EffectiveMaxConcurrency = v
		} else {
			item.EffectiveMaxConcurrency = 0
		}

		// Get owner name
		var owner *struct {
			DisplayName string `json:"display_name"`
		}
		_ = dao.TntUsers.Ctx(ctx).
			Where("id", t.OwnerUserID).Scan(&owner)
		if owner != nil {
			item.OwnerName = owner.DisplayName
		}

		// Get member count
		memberCount, _ := dao.TntUsers.Ctx(ctx).
			Where("tenant_id", t.Id).Count()
		item.MemberCount = memberCount

		// Get wallet balance
		var wallet *struct {
			Balance string `json:"balance"`
		}
		_ = dao.BilWallets.Ctx(ctx).
			Where("tenant_id", t.Id).Scan(&wallet)
		if wallet != nil && wallet.Balance != "" {
			item.WalletBalance = wallet.Balance
		} else {
			item.WalletBalance = "0"
		}

		items[i] = item
	}

	return &v1.TenantListRes{
		List:     items,
		Total:    int(total),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetTenant returns detail of a single tenant.
func (s *sAdmin) GetTenant(ctx context.Context, req *v1.TenantGetReq) (*v1.TenantGetRes, error) {
	var tenant *struct {
		Id                  int64       `json:"id"`
		Name                string      `json:"name"`
		Code                string      `json:"code"`
		LogoURL             string      `json:"logo_url"`
		OwnerUserID         int64       `json:"owner_user_id"`
		Status              string      `json:"status"`
		MaxMembers          *int        `json:"max_members"`
		MaxConcurrency      *int        `json:"max_concurrency"`
		DefaultChannelScope string      `json:"default_channel_scope"`
		Settings            string      `json:"settings"`
		Level               int         `json:"level"`
		CreatedAt           *gtime.Time `json:"created_at"`
		UpdatedAt           *gtime.Time `json:"updated_at"`
	}
	err := dao.TntTenants.Ctx(ctx).
		Where("id", req.Id).Scan(&tenant)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, common.NewNotFoundError("租户")
	}

	// Get owner name
	var owner *struct {
		DisplayName string `json:"display_name"`
	}
	_ = dao.TntUsers.Ctx(ctx).
		Where("id", tenant.OwnerUserID).Scan(&owner)

	// Get member count
	memberCount, _ := dao.TntUsers.Ctx(ctx).
		Where("tenant_id", req.Id).Count()

	// Get wallet balance
	var wallet *struct {
		Balance string `json:"balance"`
	}
	_ = dao.BilWallets.Ctx(ctx).
		Where("tenant_id", req.Id).Scan(&wallet)

	walletBalance := "0"
	if wallet != nil && wallet.Balance != "" {
		walletBalance = wallet.Balance
	}
	ownerName := ""
	if owner != nil {
		ownerName = owner.DisplayName
	}

	// Get level name
	levelName := ""
	var levelConfig *entity.TntTenantLevelConfigs
	_ = dao.TntTenantLevelConfigs.Ctx(ctx).Where("level", tenant.Level).Scan(&levelConfig)
	if levelConfig != nil {
		levelName = levelConfig.Name
	}

	// 计算实际生效值
	effectiveMaxMembers := 10
	effectiveMaxConc := 0
	if tenant.MaxMembers != nil {
		effectiveMaxMembers = *tenant.MaxMembers
	} else if levelConfig != nil {
		effectiveMaxMembers = levelConfig.MaxMembers
	}
	if tenant.MaxConcurrency != nil {
		effectiveMaxConc = *tenant.MaxConcurrency
	} else if levelConfig != nil {
		effectiveMaxConc = levelConfig.MaxConcurrency
	}

	return &v1.TenantGetRes{
		TenantItem: v1.TenantItem{
			ID:                      tenant.Id,
			Name:                    tenant.Name,
			Code:                    tenant.Code,
			LogoURL:                 tenant.LogoURL,
			OwnerUserID:             tenant.OwnerUserID,
			OwnerName:               ownerName,
			Status:                  tenant.Status,
			MaxMembers:              tenant.MaxMembers,
			MaxConcurrency:          tenant.MaxConcurrency,
			EffectiveMaxMembers:     effectiveMaxMembers,
			EffectiveMaxConcurrency: effectiveMaxConc,
			DefaultChannelScope:     tenant.DefaultChannelScope,
			MemberCount:             memberCount,
			WalletBalance:           walletBalance,
			Level:                   tenant.Level,
			LevelName:               levelName,
			CreatedAt:               tenant.CreatedAt.String(),
			UpdatedAt:               tenant.UpdatedAt.String(),
		},
		Settings: tenant.Settings,
	}, nil
}

// UpdateTenantStatus updates a tenant's status.
func (s *sAdmin) UpdateTenantStatus(ctx context.Context, req *v1.TenantUpdateStatusReq) (*v1.TenantUpdateStatusRes, error) {
	if req.Status != "active" && req.Status != "suspended" && req.Status != "closed" {
		return nil, common.NewBadRequestError("状态值无效")
	}

	// Check tenant exists
	var tenant *struct {
		Id int64 `json:"id"`
	}
	err := dao.TntTenants.Ctx(ctx).
		Where("id", req.Id).Scan(&tenant)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, common.NewNotFoundError("租户")
	}

	_, err = dao.TntTenants.Ctx(ctx).Where("id", req.Id).Update(do.TntTenants{
		Status: req.Status,
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// UpdateTenant updates tenant information.
func (s *sAdmin) UpdateTenant(ctx context.Context, req *v1.TenantUpdateReq) (*v1.TenantUpdateRes, error) {
	data := do.TntTenants{}

	if req.Name != "" {
		data.Name = req.Name
	}
	if req.MaxMembers != nil {
		if *req.MaxMembers < 1 {
			return nil, common.NewBadRequestError("最大成员数不能小于1")
		}
		data.MaxMembers = *req.MaxMembers
	}
	if req.MaxConcurrency != nil {
		data.MaxConcurrency = *req.MaxConcurrency
	}

	// 管理员手动调整等级：仅更新等级，不自动填充成员数和并发数
	// 成员数和并发数为 NULL 时自动跟随等级配置
	if req.Level != nil {
		var config *entity.TntTenantLevelConfigs
		err := dao.TntTenantLevelConfigs.Ctx(ctx).Where("level", *req.Level).Scan(&config)
		if err = common.IgnoreScanNoRows(err); err != nil || config == nil {
			return nil, common.NewBadRequestError("等级配置不存在")
		}
		data.Level = *req.Level
	}

	_, err := dao.TntTenants.Ctx(ctx).Where("id", req.Id).Update(data)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// UpdateTenantChannelScope 更新租户默认渠道范围
func (s *sAdmin) UpdateTenantChannelScope(ctx context.Context, req *v1.TenantChannelScopeUpdateReq) (*v1.TenantChannelScopeUpdateRes, error) {
	var scopeValue any
	if req.DefaultChannelScope == nil || *req.DefaultChannelScope == "" || *req.DefaultChannelScope == "all" {
		scopeValue = nil
	} else if json.Valid([]byte(*req.DefaultChannelScope)) {
		scopeValue = *req.DefaultChannelScope
	} else {
		return nil, gerror.New("default_channel_scope 必须是有效的 JSON（如 [1,5,12]）或 null")
	}

	_, err := dao.TntTenants.Ctx(ctx).Where("id", req.Id).Data(do.TntTenants{
		DefaultChannelScope: scopeValue,
	}).Update()
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// ExportTenants exports tenant list to CSV or Excel.
func (s *sAdmin) ExportTenants(ctx context.Context, req *v1.TenantExportReq) (*v1.TenantExportRes, error) {
	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "name", Header: "名称"},
		{Field: "code", Header: "代码"},
		{Field: "owner_name", Header: "所有者"},
		{Field: "status", Header: "状态"},
		{Field: "member_count", Header: "成员数"},
		{Field: "wallet_balance", Header: "钱包余额"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   req.Format,
		Filename: "租户_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	fetchTenantRow := func(t struct {
		Id          int64       `json:"id"`
		Name        string      `json:"name"`
		Code        string      `json:"code"`
		OwnerUserID int64       `json:"owner_user_id"`
		Status      string      `json:"status"`
		CreatedAt   *gtime.Time `json:"created_at"`
	}) map[string]any {
		var owner *struct {
			DisplayName string `json:"display_name"`
		}
		_ = dao.TntUsers.Ctx(ctx).Where("id", t.OwnerUserID).Scan(&owner)

		memberCount, _ := dao.TntUsers.Ctx(ctx).Where("tenant_id", t.Id).Count()

		var wallet *struct {
			Balance string `json:"balance"`
		}
		_ = dao.BilWallets.Ctx(ctx).Where("tenant_id", t.Id).Scan(&wallet)
		walletBalance := "0"
		if wallet != nil && wallet.Balance != "" {
			walletBalance = wallet.Balance
		}

		ownerName := ""
		if owner != nil {
			ownerName = owner.DisplayName
		}

		return map[string]any{
			"id":             t.Id,
			"name":           t.Name,
			"code":           t.Code,
			"owner_name":     ownerName,
			"status":         t.Status,
			"member_count":   memberCount,
			"wallet_balance": walletBalance,
			"created_at":     t.CreatedAt.String(),
		}
	}

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			m := dao.TntTenants.Ctx(ctx)
			if req.Keyword != "" {
				keyword := "%" + strings.TrimSpace(req.Keyword) + "%"
				m = m.Where("name LIKE ? OR code LIKE ?", keyword, keyword)
			}
			if req.Status != "" {
				m = m.Where("status", req.Status)
			}
			var batch []struct {
				Id          int64       `json:"id"`
				Name        string      `json:"name"`
				Code        string      `json:"code"`
				OwnerUserID int64       `json:"owner_user_id"`
				Status      string      `json:"status"`
				CreatedAt   *gtime.Time `json:"created_at"`
			}
			if err := m.Fields("id, name, code, owner_user_id, status, created_at").OrderDesc("id").Limit(1000).Offset(offset).Scan(&batch); err != nil {
				return
			}
			for _, t := range batch {
				if !yield(fetchTenantRow(t)) {
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

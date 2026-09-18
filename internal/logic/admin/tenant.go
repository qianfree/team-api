package admin

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"
	"github.com/qianfree/team-api/internal/utility/crypto"
	"github.com/qianfree/team-api/internal/utility/export"
)

// 租户标签约束：数量与单项长度上限（字符数，非字节数）
const (
	tenantTagsMaxCount   = 10
	tenantTagsMaxRunes   = 30
	tenantRemarkMaxRunes = 1000
)

// marshalTagFilter 把单个标签序列化为 JSONB 包含查询参数（tags @> '["vip"]'）。
// 序列化不会失败（输入为字符串），错误时返回空数组条件（匹配不到任何行，安全降级）。
func marshalTagFilter(tag string) string {
	b, err := json.Marshal([]string{strings.TrimSpace(tag)})
	if err != nil {
		return "[]"
	}
	return string(b)
}

// parseTenantTags 解析租户标签 JSONB（空/非法输入返回空切片，不报错——展示路径容错）
func parseTenantTags(raw string) []string {
	if raw == "" {
		return []string{}
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return []string{}
	}
	return tags
}

// marshalTenantTags 校验并序列化租户标签为 JSONB 字符串：
// 去首尾空白、跳过空项、去重，≤ tenantTagsMaxCount 个、每项 ≤ tenantTagsMaxRunes 字符。
func marshalTenantTags(tags []string) (string, error) {
	cleaned := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if utf8.RuneCountInString(tag) > tenantTagsMaxRunes {
			return "", common.NewBadRequestError("单个标签最长 30 字符")
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		cleaned = append(cleaned, tag)
	}
	if len(cleaned) > tenantTagsMaxCount {
		return "", common.NewBadRequestError("标签数量最多 10 个")
	}
	b, err := json.Marshal(cleaned)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

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

	tenantName := strings.TrimSpace(req.TenantName)
	if err := common.ValidateTenantName(tenantName); err != nil {
		return nil, common.NewBusinessError(consts.CodeInvalidTenantName, err.Error())
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

	// 标签与备注：可选字段，校验规则与更新接口一致；空值落库为列默认（[] / ''）
	tagsJSON, err := marshalTenantTags(req.Tags)
	if err != nil {
		return nil, err
	}
	remarkVal := ""
	if req.Remark != nil {
		remarkVal = strings.TrimSpace(*req.Remark)
		if utf8.RuneCountInString(remarkVal) > tenantRemarkMaxRunes {
			return nil, common.NewBadRequestError("备注最长 1000 字符")
		}
	}

	var tenantID int64

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tenantResult, err := dao.TntTenants.Ctx(ctx).Data(do.TntTenants{
			Name:           tenantName,
			Code:           tenantCode,
			MaxMembers:     maxMembersVal,
			MaxConcurrency: maxConcurrencyVal,
			Level:          1,
			Settings:       "{}",
			Tags:           tagsJSON,
			Remark:         remarkVal,
		}).Insert()
		if err != nil {
			return gerror.Wrapf(err, "create tenant")
		}
		tenantID, err = tenantResult.LastInsertId()
		if err != nil {
			return gerror.Wrapf(err, "get tenant id")
		}

		userResult, err := dao.TntUsers.Ctx(ctx).Data(do.TntUsers{
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

		_, err = dao.TntTenants.Ctx(ctx).
			Where("id", tenantID).
			Data(do.TntTenants{
				OwnerUserId: ownerUserID,
			}).Update()
		if err != nil {
			return gerror.Wrapf(err, "set tenant owner")
		}

		warningThreshold := billing.Zero // 预警阈值默认 0 = 关闭，用户可自行开启
		_, err = dao.BilWallets.Ctx(ctx).Data(do.BilWallets{
			TenantId:         tenantID,
			Balance:          0,
			FrozenBalance:    0,
			WarningThreshold: &warningThreshold,
			Currency:         billing.Currency(ctx),
		}).Insert()
		if err != nil {
			return gerror.Wrapf(err, "create wallet")
		}

		// Assign default model groups
		var defaultGroups []struct {
			Id int64
		}
		if err := dao.MdlModelGroups.Ctx(ctx).
			Where("is_default", true).Where("status", "active").
			Fields("id").Scan(&defaultGroups); err == nil && len(defaultGroups) > 0 {
			insertData := make([]do.MdlTenantGroups, 0, len(defaultGroups))
			for _, dg := range defaultGroups {
				insertData = append(insertData, do.MdlTenantGroups{
					TenantId: tenantID,
					GroupId:  dg.Id,
				})
			}
			if _, err := dao.MdlTenantGroups.Ctx(ctx).
				Batch(len(insertData)).Insert(insertData); err != nil {
				g.Log().Warningf(ctx, "assign default model groups failed: %v", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &v1.TenantCreateRes{Id: tenantID}, nil
}

// batchTenantAggregates 批量查询一批租户的 owner 名称、成员数、钱包余额，
// 用三条聚合查询替代逐行 N+1 查询。
// 返回：ownerNames(ownerUserID→名称)、memberCounts(tenantID→成员数)、walletBalances(tenantID→余额)。
func batchTenantAggregates(ctx context.Context, tenantIDs, ownerIDs []int64) (
	ownerNames map[int64]string,
	memberCounts map[int64]int,
	walletBalances map[int64]string,
	walletTotalConsumed map[int64]string,
) {
	ownerNames = make(map[int64]string, len(ownerIDs))
	memberCounts = make(map[int64]int, len(tenantIDs))
	walletBalances = make(map[int64]string, len(tenantIDs))
	walletTotalConsumed = make(map[int64]string, len(tenantIDs))

	// owner 名称：按 owner_user_id 批量取
	if len(ownerIDs) > 0 {
		var owners []struct {
			Id          int64  `json:"id"`
			DisplayName string `json:"display_name"`
		}
		_ = dao.TntUsers.Ctx(ctx).Fields("id, display_name").WhereIn("id", ownerIDs).Scan(&owners)
		for _, o := range owners {
			ownerNames[o.Id] = o.DisplayName
		}
	}

	if len(tenantIDs) > 0 {
		// 成员数：按 tenant_id 分组统计
		var counts []struct {
			TenantID int64 `json:"tenant_id"`
			Count    int   `json:"count"`
		}
		_ = dao.TntUsers.Ctx(ctx).Fields("tenant_id, COUNT(*) AS count").
			WhereIn("tenant_id", tenantIDs).Group("tenant_id").Scan(&counts)
		for _, c := range counts {
			memberCounts[c.TenantID] = c.Count
		}

		// 钱包余额 / 累计消费：按 tenant_id 批量取（DB 物化副本，累计消费随物化器 ≤5s 滞后）
		var wallets []struct {
			TenantID      int64  `json:"tenant_id"`
			Balance       string `json:"balance"`
			TotalConsumed string `json:"total_consumed"`
		}
		_ = dao.BilWallets.Ctx(ctx).Fields("tenant_id, balance, total_consumed").
			WhereIn("tenant_id", tenantIDs).Scan(&wallets)
		for _, w := range wallets {
			walletBalances[w.TenantID] = w.Balance
			walletTotalConsumed[w.TenantID] = w.TotalConsumed
		}
	}

	return
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
	if req.Tag != "" {
		m = m.Where("tags @> ?::jsonb", marshalTagFilter(req.Tag))
	}
	if req.Level != nil {
		m = m.Where("level", *req.Level)
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
	if req.Tag != "" {
		m = m.Where("tags @> ?::jsonb", marshalTagFilter(req.Tag))
	}
	if req.Level != nil {
		m = m.Where("level", *req.Level)
	}

	// 扫描结构体必须带 orm 标签：gf 的 Scan 会用「结构体字段名」自动生成 SELECT 字段列表，
	// 无 orm 标签时靠运行时查表结构（TableFields）做 Id→id 映射；该元数据查询失败会被框架静默吞掉，
	// 帕斯卡字段名将原样进 SQL，PostgreSQL 引号列名大小写敏感，报 column "Id" does not exist。
	var tenants []struct {
		Id                  int64       `json:"id" orm:"id"`
		Name                string      `json:"name" orm:"name"`
		Code                string      `json:"code" orm:"code"`
		LogoURL             string      `json:"logo_url" orm:"logo_url"`
		OwnerUserID         int64       `json:"owner_user_id" orm:"owner_user_id"`
		Status              string      `json:"status" orm:"status"`
		MaxMembers          *int        `json:"max_members" orm:"max_members"`
		MaxConcurrency      *int        `json:"max_concurrency" orm:"max_concurrency"`
		DefaultChannelScope string      `json:"default_channel_scope" orm:"default_channel_scope"`
		Settings            string      `json:"settings" orm:"settings"`
		Level               int         `json:"level" orm:"level"`
		Tags                string      `json:"tags" orm:"tags"`
		Remark              string      `json:"remark" orm:"remark"`
		CreatedAt           *gtime.Time `json:"created_at" orm:"created_at"`
		UpdatedAt           *gtime.Time `json:"updated_at" orm:"updated_at"`
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

	// 批量查询 owner 名称 / 成员数 / 钱包余额（避免逐行 N+1）
	tenantIDs := make([]int64, len(tenants))
	ownerIDs := make([]int64, len(tenants))
	for i, t := range tenants {
		tenantIDs[i] = t.Id
		ownerIDs[i] = t.OwnerUserID
	}
	ownerNameMap, memberCountMap, walletBalanceMap, walletConsumedMap := batchTenantAggregates(ctx, tenantIDs, ownerIDs)

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
			Tags:                parseTenantTags(t.Tags),
			Remark:              t.Remark,
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

		// 从批量结果中取 owner 名称 / 成员数 / 钱包余额 / 累计消费
		item.OwnerName = ownerNameMap[t.OwnerUserID]
		item.MemberCount = memberCountMap[t.Id]
		if bal, ok := walletBalanceMap[t.Id]; ok && bal != "" {
			item.WalletBalance = bal
		} else {
			item.WalletBalance = "0"
		}
		if tc, ok := walletConsumedMap[t.Id]; ok && tc != "" {
			item.TotalConsumed = tc
		} else {
			item.TotalConsumed = "0"
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
		Tags                string      `json:"tags"`
		Remark              string      `json:"remark"`
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

	// Get wallet balance / 累计消费：优先读 Redis 权威值（调整余额/入账/结算后详情页立即可见），DB 物化值兜底
	walletBalance := "0"
	totalConsumed := "0"
	if w, err := billing.GetWallet(ctx, req.Id); err == nil {
		walletBalance = strconv.FormatFloat(w.Balance, 'f', -1, 64)
		totalConsumed = strconv.FormatFloat(w.TotalConsumed, 'f', -1, 64)
	} else {
		var wallet *struct {
			Balance       string `json:"balance"`
			TotalConsumed string `json:"total_consumed"`
		}
		_ = dao.BilWallets.Ctx(ctx).
			Where("tenant_id", req.Id).Scan(&wallet)

		if wallet != nil {
			if wallet.Balance != "" {
				walletBalance = wallet.Balance
			}
			if wallet.TotalConsumed != "" {
				totalConsumed = wallet.TotalConsumed
			}
		}
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
			Tags:                    parseTenantTags(tenant.Tags),
			Remark:                  tenant.Remark,
			TotalConsumed:           totalConsumed,
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
		tenantName := strings.TrimSpace(req.Name)
		if err := common.ValidateTenantName(tenantName); err != nil {
			return nil, common.NewBusinessError(consts.CodeInvalidTenantName, err.Error())
		}
		data.Name = tenantName
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

	// 标签：nil 表示未传（不更新）；空数组表示清空（序列化为 []）
	if req.Tags != nil {
		tagsJSON, err := marshalTenantTags(req.Tags)
		if err != nil {
			return nil, err
		}
		data.Tags = tagsJSON
	}
	if req.Remark != nil {
		remark := strings.TrimSpace(*req.Remark)
		if utf8.RuneCountInString(remark) > tenantRemarkMaxRunes {
			return nil, common.NewBadRequestError("备注最长 1000 字符")
		}
		data.Remark = remark
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
		{Field: "tags", Header: "标签"},
		{Field: "remark", Header: "备注"},
		{Field: "member_count", Header: "成员数"},
		{Field: "wallet_balance", Header: "钱包余额"},
		{Field: "total_consumed", Header: "累计消费"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   req.Format,
		Filename: "租户_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
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
			if req.Tag != "" {
				m = m.Where("tags @> ?::jsonb", marshalTagFilter(req.Tag))
			}
			if req.Level != nil {
				m = m.Where("level", *req.Level)
			}
			var batch []struct {
				Id          int64       `json:"id"`
				Name        string      `json:"name"`
				Code        string      `json:"code"`
				OwnerUserID int64       `json:"owner_user_id"`
				Status      string      `json:"status"`
				Tags        string      `json:"tags"`
				Remark      string      `json:"remark"`
				CreatedAt   *gtime.Time `json:"created_at"`
			}
			if err := m.Fields("id, name, code, owner_user_id, status, tags, remark, created_at").OrderDesc("id").Limit(1000).Offset(offset).Scan(&batch); err != nil {
				return
			}

			// 批量查询本页的 owner 名称 / 成员数 / 钱包余额 / 累计消费（避免逐行 N+1）
			tenantIDs := make([]int64, len(batch))
			ownerIDs := make([]int64, len(batch))
			for i, t := range batch {
				tenantIDs[i] = t.Id
				ownerIDs[i] = t.OwnerUserID
			}
			ownerNameMap, memberCountMap, walletBalanceMap, walletConsumedMap := batchTenantAggregates(ctx, tenantIDs, ownerIDs)

			for _, t := range batch {
				walletBalance := "0"
				if bal, ok := walletBalanceMap[t.Id]; ok && bal != "" {
					walletBalance = bal
				}
				totalConsumed := "0"
				if tc, ok := walletConsumedMap[t.Id]; ok && tc != "" {
					totalConsumed = tc
				}
				row := map[string]any{
					"id":             t.Id,
					"name":           t.Name,
					"code":           t.Code,
					"owner_name":     ownerNameMap[t.OwnerUserID],
					"status":         t.Status,
					"tags":           strings.Join(parseTenantTags(t.Tags), ","),
					"remark":         t.Remark,
					"member_count":   memberCountMap[t.Id],
					"wallet_balance": walletBalance,
					"total_consumed": totalConsumed,
					"created_at":     t.CreatedAt.String(),
				}
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

// TenantApiKeySelect 租户 API Key 下拉选择列表（轻量，不返回密钥原值）。
// 供渠道调试目标过滤等管理端选择器使用；user_id > 0 时按创建者过滤。
// 注意：Fields 不能挂在 Count 之前的模型上——gdb 会生成 COUNT(col1, col2, ...)，
// PostgreSQL 的 count 只接受单参数，会直接报错。
func (s *sAdmin) TenantApiKeySelect(ctx context.Context, req *v1.TenantApiKeySelectReq) (*v1.TenantApiKeySelectRes, error) {
	m := dao.ApiKeys.Ctx(ctx).
		Where("tenant_id", req.TenantID).
		OrderDesc("id")
	if req.UserID > 0 {
		m = m.Where("user_id", req.UserID)
	}

	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	var keys []struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		KeyPrefix string `json:"key_prefix"`
		UserID    int64  `json:"user_id"`
		Status    string `json:"status"`
	}
	if err := m.Fields("id, name, key_prefix, user_id, status").Page(req.Page, req.PageSize).Scan(&keys); err != nil {
		return nil, err
	}

	list := make([]v1.TenantApiKeySelectItem, len(keys))
	for i, k := range keys {
		list[i] = v1.TenantApiKeySelectItem{
			ID:        k.ID,
			Name:      k.Name,
			KeyPrefix: k.KeyPrefix,
			UserID:    k.UserID,
			Status:    k.Status,
		}
	}
	return &v1.TenantApiKeySelectRes{List: list, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

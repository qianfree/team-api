package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/relay"
	uc "github.com/qianfree/team-api/internal/utility/crypto"
	"github.com/qianfree/team-api/internal/utility/export"
)

// providerTypeNames 供应商类型名称映射
var providerTypeNames = map[int]string{
	1:  "OpenAI",
	2:  "Claude",
	3:  "Gemini",
	4:  "Ali",
	5:  "Baidu",
	6:  "Tencent",
	7:  "Zhipu",
	8:  "DeepSeek",
	9:  "Moonshot",
	10: "Volcengine",
	11: "AWS Bedrock",
	12: "Azure OpenAI",
	13: "Vertex AI",
	14: "Cohere",
	15: "Mistral",
	16: "xAI",
}

// ListChannels 获取渠道列表
func (s *sAdmin) ListChannels(ctx context.Context, req *v1.ChannelListReq) (*v1.ChannelListRes, error) {
	query := dao.ChnChannels.Ctx(ctx).
		LeftJoin("chn_health_scores h ON chn_channels.id = h.channel_id")

	if req.Type > 0 {
		query = query.Where("chn_channels.type", req.Type)
	}
	if req.Status != "" {
		query = query.Where("chn_channels.status", req.Status)
	}
	if req.Search != "" {
		query = query.Where("chn_channels.name LIKE ? OR chn_channels.remark LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
	}

	var total int
	var channels []struct {
		ID                       int64       `json:"id"`
		Name                     string      `json:"name"`
		Type                     int         `json:"type"`
		BaseURL                  string      `json:"base_url"`
		Status                   string      `json:"status"`
		Priority                 int         `json:"priority"`
		Weight                   int         `json:"weight"`
		TestModel                string      `json:"test_model"`
		Remark                   string      `json:"remark"`
		IsVIP                    bool        `json:"is_vip"`
		SharingThreshold         *float64    `json:"sharing_threshold"`
		PreemptionThreshold      *float64    `json:"preemption_threshold"`
		BorrowingCooldownSeconds *int        `json:"borrowing_cooldown_seconds"`
		CreatedAt                *gtime.Time `json:"created_at"`
		HealthScore              *float64    `json:"health_score"`
		Settings                 string      `json:"settings"`
	}

	err := query.Fields("chn_channels.id, chn_channels.name, chn_channels.type, chn_channels.base_url, chn_channels.status, chn_channels.priority, chn_channels.weight, chn_channels.test_model, chn_channels.remark, chn_channels.is_vip, chn_channels.sharing_threshold, chn_channels.preemption_threshold, chn_channels.borrowing_cooldown_seconds, chn_channels.created_at, h.health_score, chn_channels.settings").
		OrderDesc("chn_channels.priority").
		Page(req.Page, req.PageSize).
		ScanAndCount(&channels, &total, false)
	if err != nil {
		return nil, err
	}

	list := make([]v1.ChannelItem, 0, len(channels))
	for _, ch := range channels {
		typeName := providerTypeNames[ch.Type]
		if typeName == "" {
			typeName = fmt.Sprintf("Unknown(%d)", ch.Type)
		}

		settings := relay.ParseChannelSettings(ch.Settings)

		list = append(list, v1.ChannelItem{
			ID:                       ch.ID,
			Name:                     ch.Name,
			Type:                     ch.Type,
			TypeName:                 typeName,
			BaseURL:                  ch.BaseURL,
			Status:                   ch.Status,
			Priority:                 ch.Priority,
			Weight:                   ch.Weight,
			TestModel:                ch.TestModel,
			Remark:                   ch.Remark,
			IsVIP:                    ch.IsVIP,
			UseProxy:                 settings.UseProxy,
			SharingThreshold:         ch.SharingThreshold,
			PreemptionThreshold:      ch.PreemptionThreshold,
			BorrowingCooldownSeconds: ch.BorrowingCooldownSeconds,
			CreatedAt:                ch.CreatedAt.String(),
			HealthScore:              ch.HealthScore,
		})
	}

	return &v1.ChannelListRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// CloneChannel 克隆渠道
func (s *sAdmin) CloneChannel(ctx context.Context, req *v1.ChannelCloneReq) (*v1.ChannelCloneRes, error) {
	// 查询源渠道
	var src *struct {
		ID                       int64   `json:"id"`
		Name                     string  `json:"name"`
		Type                     int     `json:"type"`
		BaseURL                  string  `json:"base_url"`
		Priority                 int     `json:"priority"`
		Weight                   int     `json:"weight"`
		TestModel                string  `json:"test_model"`
		Remark                   string  `json:"remark"`
		Settings                 string  `json:"settings"`
		IsVIP                    bool    `json:"is_vip"`
		SharingThreshold         float64 `json:"sharing_threshold"`
		PreemptionThreshold      float64 `json:"preemption_threshold"`
		BorrowingCooldownSeconds int     `json:"borrowing_cooldown_seconds"`
	}
	err := dao.ChnChannels.Ctx(ctx).
		Fields("id, name, type, base_url, priority, weight, test_model, remark, settings, is_vip, sharing_threshold, preemption_threshold, borrowing_cooldown_seconds").
		Where("id", req.ID).
		Scan(&src)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, common.NewBusinessError(404, "源渠道不存在")
	}

	// 确定名称
	name := req.Name
	if name == "" {
		name = src.Name + " (副本)"
	}

	// 创建新渠道（复制配置）
	newID, err := dao.ChnChannels.Ctx(ctx).InsertAndGetId(do.ChnChannels{
		Name:                     name,
		Type:                     src.Type,
		BaseUrl:                  src.BaseURL,
		Status:                   "active",
		Priority:                 src.Priority,
		Weight:                   src.Weight,
		TestModel:                src.TestModel,
		Remark:                   src.Remark,
		Settings:                 src.Settings,
		IsVip:                    src.IsVIP,
		SharingThreshold:         src.SharingThreshold,
		PreemptionThreshold:      src.PreemptionThreshold,
		BorrowingCooldownSeconds: src.BorrowingCooldownSeconds,
	})
	if err != nil {
		return nil, err
	}

	// 创建 Key
	encKey := relay.GetEncryptionKey()
	encrypted, err := uc.EncryptString(encKey, req.ApiKey)
	if err != nil {
		return nil, gerror.Wrapf(err, "encrypt api key failed")
	}
	_, err = dao.ChnChannelKeys.Ctx(ctx).Insert(do.ChnChannelKeys{
		ChannelId:    newID,
		Name:         "default",
		EncryptedKey: encrypted,
		Status:       "active",
	})
	if err != nil {
		return nil, err
	}

	// 克隆 abilities
	var abilities []struct {
		ModelName     string `json:"model_name"`
		UpstreamModel string `json:"upstream_model"`
		Enabled       bool   `json:"enabled"`
	}
	err = dao.ChnAbilities.Ctx(ctx).
		Fields("model_name, upstream_model, enabled").
		Where("channel_id", req.ID).
		Scan(&abilities)
	if err != nil {
		return nil, err
	}
	for _, ab := range abilities {
		_, err := dao.ChnAbilities.Ctx(ctx).Insert(do.ChnAbilities{
			ChannelId:     newID,
			ModelName:     ab.ModelName,
			UpstreamModel: ab.UpstreamModel,
			Enabled:       ab.Enabled,
		})
		if err != nil {
			g.Log().Warningf(ctx, "clone ability %s for channel %d failed: %v", ab.ModelName, newID, err)
		}
	}

	// 初始化健康分
	if err := relay.InitHealthScore(ctx, newID); err != nil {
		g.Log().Warningf(ctx, "init health score for channel %d failed: %v", newID, err)
	}

	return &v1.ChannelCloneRes{ID: newID}, nil
}

// CreateChannel 创建渠道
func (s *sAdmin) CreateChannel(ctx context.Context, req *v1.ChannelCreateReq) (*v1.ChannelCreateRes, error) {
	// 自动填充默认 Base URL
	baseURL := req.BaseURL
	if baseURL == "" {
		baseURL = defaultProviderURL(req.Type)
	}

	// Build settings JSON
	settingsJSON := "{}"
	if req.UseProxy {
		settingsJSON = `{"use_proxy":true}`
	}

	id, err := dao.ChnChannels.Ctx(ctx).InsertAndGetId(do.ChnChannels{
		Name:                     req.Name,
		Type:                     req.Type,
		BaseUrl:                  baseURL,
		Status:                   "active",
		Priority:                 req.Priority,
		Weight:                   req.Weight,
		TestModel:                req.TestModel,
		Remark:                   req.Remark,
		Settings:                 settingsJSON,
		IsVip:                    req.IsVIP,
		SharingThreshold:         req.SharingThreshold,
		PreemptionThreshold:      req.PreemptionThreshold,
		BorrowingCooldownSeconds: req.BorrowingCooldownSeconds,
	})
	if err != nil {
		return nil, err
	}

	// 加密并创建 Key
	encKey := relay.GetEncryptionKey()
	encrypted, err := uc.EncryptString(encKey, req.ApiKey)
	if err != nil {
		return nil, gerror.Wrapf(err, "encrypt api key failed")
	}
	_, err = dao.ChnChannelKeys.Ctx(ctx).Insert(do.ChnChannelKeys{
		ChannelId:    id,
		Name:         "default",
		EncryptedKey: encrypted,
		Status:       "active",
	})
	if err != nil {
		return nil, err
	}

	// 初始化健康度记录
	if err := relay.InitHealthScore(ctx, id); err != nil {
		g.Log().Warningf(ctx, "init health score for channel %d failed: %v", id, err)
	}

	return &v1.ChannelCreateRes{ID: id}, nil
}

// UpdateChannel 更新渠道
func (s *sAdmin) UpdateChannel(ctx context.Context, req *v1.ChannelUpdateReq) (*v1.ChannelUpdateRes, error) {
	data := do.ChnChannels{}
	if req.Name != "" {
		data.Name = req.Name
	}
	if req.BaseURL != "" {
		data.BaseUrl = req.BaseURL
	}
	data.Priority = req.Priority
	data.Weight = req.Weight
	if req.TestModel != "" {
		data.TestModel = req.TestModel
	}
	if req.Remark != "" {
		data.Remark = req.Remark
	}
	if req.Status != "" {
		data.Status = req.Status
	}
	if req.IsVIP != nil {
		data.IsVip = *req.IsVIP
	}
	if req.SharingThreshold != nil {
		data.SharingThreshold = *req.SharingThreshold
	}
	if req.PreemptionThreshold != nil {
		data.PreemptionThreshold = *req.PreemptionThreshold
	}
	if req.BorrowingCooldownSeconds != nil {
		data.BorrowingCooldownSeconds = *req.BorrowingCooldownSeconds
	}

	// Update use_proxy in settings JSONB
	if req.UseProxy != nil {
		var currentSettings string
		_ = dao.ChnChannels.Ctx(ctx).Where("id", req.ID).Fields("settings").Scan(&currentSettings)
		settings := relay.ParseChannelSettings(currentSettings)
		settings.UseProxy = *req.UseProxy
		if settingsJSON, err := json.Marshal(settings); err == nil {
			data.Settings = string(settingsJSON)
		}
	}

	_, err := dao.ChnChannels.Ctx(ctx).
		Where("id", req.ID).
		Data(data).
		Update()
	if err != nil {
		return nil, err
	}

	// 更新 Key（如果提供了新的 API Key）
	if req.ApiKey != nil && *req.ApiKey != "" {
		encKey := relay.GetEncryptionKey()
		encrypted, err := uc.EncryptString(encKey, *req.ApiKey)
		if err != nil {
			return nil, gerror.Wrapf(err, "encrypt api key failed")
		}
		_, err = dao.ChnChannelKeys.Ctx(ctx).
			Where("channel_id", req.ID).
			Data(do.ChnChannelKeys{
				EncryptedKey: encrypted,
				KeyType:      "apikey",
			}).
			Update()
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// DeleteChannel 删除渠道
func (s *sAdmin) DeleteChannel(ctx context.Context, req *v1.ChannelDeleteReq) (*v1.ChannelDeleteRes, error) {
	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Model("chn_abilities").Ctx(ctx).Where("channel_id", req.ID).Delete(); err != nil {
			return err
		}
		if _, err := tx.Model("chn_channel_keys").Ctx(ctx).Where("channel_id", req.ID).Delete(); err != nil {
			return err
		}
		if _, err := tx.Model("chn_health_scores").Ctx(ctx).Where("channel_id", req.ID).Delete(); err != nil {
			return err
		}
		if _, err := tx.Model("chn_channels").Ctx(ctx).Where("id", req.ID).Delete(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// GetChannelDetail 获取渠道详情
func (s *sAdmin) GetChannelDetail(ctx context.Context, req *v1.ChannelDetailReq) (*v1.ChannelDetailRes, error) {
	var ch *struct {
		ID                       int64       `json:"id"`
		Name                     string      `json:"name"`
		Type                     int         `json:"type"`
		BaseURL                  string      `json:"base_url"`
		Status                   string      `json:"status"`
		Priority                 int         `json:"priority"`
		Weight                   int         `json:"weight"`
		TestModel                string      `json:"test_model"`
		Remark                   string      `json:"remark"`
		IsVIP                    bool        `json:"is_vip"`
		Settings                 string      `json:"settings"`
		SharingThreshold         *float64    `json:"sharing_threshold"`
		PreemptionThreshold      *float64    `json:"preemption_threshold"`
		BorrowingCooldownSeconds *int        `json:"borrowing_cooldown_seconds"`
		CreatedAt                *gtime.Time `json:"created_at"`
		UpdatedAt                *gtime.Time `json:"updated_at"`
		HealthScore              *float64    `json:"health_score"`
	}

	err := dao.ChnChannels.Ctx(ctx).
		LeftJoin("chn_health_scores h ON chn_channels.id = h.channel_id").
		Fields("chn_channels.id, chn_channels.name, chn_channels.type, chn_channels.base_url, chn_channels.status, chn_channels.priority, chn_channels.weight, chn_channels.test_model, chn_channels.remark, chn_channels.is_vip, chn_channels.settings, chn_channels.sharing_threshold, chn_channels.preemption_threshold, chn_channels.borrowing_cooldown_seconds, chn_channels.created_at, chn_channels.updated_at, h.health_score").
		Where("chn_channels.id", req.ID).
		Scan(&ch)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, common.NewBusinessError(404, "渠道不存在")
	}

	typeName := providerTypeNames[ch.Type]
	if typeName == "" {
		typeName = fmt.Sprintf("Unknown(%d)", ch.Type)
	}

	// 查询 Key 信息
	var keyInfo *struct {
		KeyType        string      `json:"key_type"`
		Status         string      `json:"status"`
		Name           string      `json:"name"`
		TokenExpiresAt *gtime.Time `json:"token_expires_at"`
	}
	err = dao.ChnChannelKeys.Ctx(ctx).
		Where("channel_id", req.ID).
		Fields("key_type, status, name, token_expires_at").
		Scan(&keyInfo)
	if err != nil {
		return nil, err
	}
	if keyInfo == nil {
		return nil, common.NewNotFoundError("密钥")
	}

	settings := relay.ParseChannelSettings(ch.Settings)

	res := &v1.ChannelDetailRes{
		ID:                       ch.ID,
		Name:                     ch.Name,
		Type:                     ch.Type,
		TypeName:                 typeName,
		BaseURL:                  ch.BaseURL,
		Status:                   ch.Status,
		Priority:                 ch.Priority,
		Weight:                   ch.Weight,
		TestModel:                ch.TestModel,
		Remark:                   ch.Remark,
		IsVIP:                    ch.IsVIP,
		UseProxy:                 settings.UseProxy,
		SharingThreshold:         ch.SharingThreshold,
		PreemptionThreshold:      ch.PreemptionThreshold,
		BorrowingCooldownSeconds: ch.BorrowingCooldownSeconds,
		CreatedAt:                ch.CreatedAt.String(),
		UpdatedAt:                ch.UpdatedAt.String(),
		HealthScore:              ch.HealthScore,
		KeyType:                  keyInfo.KeyType,
		KeyStatus:                keyInfo.Status,
		KeyName:                  keyInfo.Name,
	}
	if res.KeyType == "" {
		res.KeyType = "apikey"
	}
	if keyInfo.TokenExpiresAt != nil {
		res.TokenExpiresAt = keyInfo.TokenExpiresAt.String()
	}

	return res, nil
}

// AddChannelKey 添加渠道 API Key（已废弃：每渠道仅支持一个 Key）
func (s *sAdmin) AddChannelKey(ctx context.Context, req *v1.ChannelKeyCreateReq) (*v1.ChannelKeyCreateRes, error) {
	return nil, common.NewBusinessError(400, "每渠道仅支持一个 Key，请通过编辑渠道更新 Key")
}

// DeleteChannelKey 删除渠道 API Key
func (s *sAdmin) DeleteChannelKey(ctx context.Context, req *v1.ChannelKeyDeleteReq) (*v1.ChannelKeyDeleteRes, error) {
	_, err := dao.ChnChannelKeys.Ctx(ctx).
		Where("id", req.KeyID).
		Where("channel_id", req.ChannelID).
		Delete()
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// SetChannelAbilities 设置渠道模型能力
func (s *sAdmin) SetChannelAbilities(ctx context.Context, req *v1.ChannelAbilityBatchReq) (*v1.ChannelAbilityBatchRes, error) {
	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Model("chn_abilities").Ctx(ctx).Where("channel_id", req.ChannelID).Delete(); err != nil {
			return err
		}
		for _, ab := range req.Abilities {
			if _, err := tx.Model("chn_abilities").Ctx(ctx).Insert(do.ChnAbilities{
				ChannelId:     req.ChannelID,
				ModelName:     ab.ModelName,
				UpstreamModel: ab.UpstreamModel,
				Enabled:       ab.Enabled,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// GetChannelKeys 获取渠道 Key 列表
func (s *sAdmin) GetChannelKeys(ctx context.Context, req *v1.ChannelKeyListReq) (*v1.ChannelKeyListRes, error) {
	var keys []struct {
		ID             int64       `json:"id"`
		Name           string      `json:"name"`
		Status         string      `json:"status"`
		KeyType        string      `json:"key_type"`
		TokenExpiresAt *gtime.Time `json:"token_expires_at"`
		LastUsedAt     *gtime.Time `json:"last_used_at"`
		LastError      string      `json:"last_error"`
		CreatedAt      *gtime.Time `json:"created_at"`
	}

	err := dao.ChnChannelKeys.Ctx(ctx).
		Where("channel_id", req.ChannelID).
		Fields("id, name, status, key_type, token_expires_at, last_used_at, last_error, created_at").
		OrderDesc("id").
		Scan(&keys)
	if err != nil {
		return nil, err
	}

	list := make([]v1.ChannelKeyItem, len(keys))
	for i, k := range keys {
		item := v1.ChannelKeyItem{
			ID:        k.ID,
			Name:      k.Name,
			Status:    k.Status,
			KeyType:   k.KeyType,
			CreatedAt: k.CreatedAt.String(),
		}
		if item.KeyType == "" {
			item.KeyType = "apikey"
		}
		if k.TokenExpiresAt != nil {
			item.TokenExpiresAt = k.TokenExpiresAt.String()
		}
		list[i] = item
	}

	return &v1.ChannelKeyListRes{List: list}, nil
}

// GetChannelAbilities 获取渠道模型能力列表
func (s *sAdmin) GetChannelAbilities(ctx context.Context, req *v1.ChannelAbilitiesGetReq) (*v1.ChannelAbilitiesGetRes, error) {
	var abilities []struct {
		ID            int64  `json:"id"`
		ModelName     string `json:"model_name"`
		UpstreamModel string `json:"upstream_model"`
		Enabled       bool   `json:"enabled"`
	}

	err := dao.ChnAbilities.Ctx(ctx).
		Where("channel_id", req.ChannelID).
		OrderAsc("model_name").
		Scan(&abilities)
	if err != nil {
		return nil, err
	}

	list := make([]v1.AbilityItem, len(abilities))
	for i, a := range abilities {
		list[i] = v1.AbilityItem{
			ID:            a.ID,
			ModelName:     a.ModelName,
			UpstreamModel: a.UpstreamModel,
			Enabled:       a.Enabled,
		}
	}

	return &v1.ChannelAbilitiesGetRes{List: list}, nil
}

// GetProviderDefaultURLs 获取供应商默认 API 地址
func (s *sAdmin) GetProviderDefaultURLs(ctx context.Context, _ *v1.ProviderDefaultURLReq) (*v1.ProviderDefaultURLRes, error) {
	return &v1.ProviderDefaultURLRes{URLs: defaultProviderURLs}, nil
}

// defaultProviderURLs 供应商默认 API 地址
var defaultProviderURLs = map[int]string{
	1:  "https://api.openai.com",
	2:  "https://api.anthropic.com",
	3:  "https://generativelanguage.googleapis.com",
	4:  "https://dashscope.aliyuncs.com/compatible-mode",
	5:  "https://aip.baidubce.com",
	6:  "https://hunyuan.tencentcloudapi.com",
	7:  "https://open.bigmodel.cn",
	8:  "https://api.deepseek.com",
	9:  "https://api.moonshot.cn",
	10: "https://ark.cn-beijing.volces.com/api",
	11: "https://bedrock-runtime.us-east-1.amazonaws.com",
	12: "", // Azure 需要自行填写，格式 https://{resource}.openai.azure.com
	13: "https://us-central1-aiplatform.googleapis.com",
	14: "https://api.cohere.com",
	15: "https://api.mistral.ai",
	16: "https://api.x.ai",
}

// defaultProviderURL 返回供应商类型的默认 API 地址
func defaultProviderURL(t int) string {
	return defaultProviderURLs[t]
}

// GetChannelHealthTrend 获取渠道健康趋势数据
func (s *sAdmin) GetChannelHealthTrend(ctx context.Context, req *v1.ChannelHealthTrendReq) (*v1.ChannelHealthTrendRes, error) {
	var points []v1.HealthTrendPoint
	err := dao.ChnHealthSnapshots.Ctx(ctx).
		Fields("snapshot_at, health_score, success_rate, latency_ms, stability_score, consecutive_failures").
		Where("channel_id", req.ID).
		Where("snapshot_at >= ?", gtime.Now().Add(-time.Duration(req.Hours)*time.Hour)).
		OrderAsc("snapshot_at").
		Scan(&points)
	if err != nil {
		return nil, err
	}

	if points == nil {
		points = []v1.HealthTrendPoint{}
	}

	return &v1.ChannelHealthTrendRes{Points: points}, nil
}

// ExportChannels exports channel list to CSV or Excel.
func (s *sAdmin) ExportChannels(ctx context.Context, req *v1.ChannelExportReq) (*v1.ChannelExportRes, error) {
	channelFields := "chn_channels.id, chn_channels.name, chn_channels.type, chn_channels.status, chn_channels.priority, chn_channels.weight, chn_channels.created_at, h.health_score"

	config := export.Config{
		Format:   req.Format,
		Filename: "渠道_" + gtime.Now().Format("Ymd_His"),
		Columns: []export.Column{
			{Field: "id", Header: "ID"},
			{Field: "name", Header: "名称"},
			{Field: "type_name", Header: "供应商类型"},
			{Field: "status", Header: "状态"},
			{Field: "priority", Header: "优先级"},
			{Field: "weight", Header: "权重"},
			{Field: "health_score", Header: "健康分数"},
			{Field: "created_at", Header: "创建时间"},
		},
	}

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			query := dao.ChnChannels.Ctx(ctx).
				LeftJoin("chn_health_scores h ON chn_channels.id = h.channel_id")
			if req.Type > 0 {
				query = query.Where("chn_channels.type", req.Type)
			}
			if req.Status != "" {
				query = query.Where("chn_channels.status", req.Status)
			}
			if req.Search != "" {
				query = query.Where("chn_channels.name LIKE ? OR chn_channels.remark LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
			}
			var batch []struct {
				ID          int64       `json:"id"`
				Name        string      `json:"name"`
				Type        int         `json:"type"`
				Status      string      `json:"status"`
				Priority    int         `json:"priority"`
				Weight      int         `json:"weight"`
				CreatedAt   *gtime.Time `json:"created_at"`
				HealthScore *float64    `json:"health_score"`
			}
			if err := query.Fields(channelFields).OrderDesc("chn_channels.priority").Limit(1000).Offset(offset).Scan(&batch); err != nil {
				g.Log().Errorf(ctx, "ExportChannels: query batch at offset %d failed: %v", offset, err)
				return
			}
			for _, ch := range batch {
				typeName := providerTypeNames[ch.Type]
				if typeName == "" {
					typeName = fmt.Sprintf("Unknown(%d)", ch.Type)
				}
				healthScore := ""
				if ch.HealthScore != nil {
					healthScore = fmt.Sprintf("%.2f", *ch.HealthScore)
				}
				if !yield(map[string]any{
					"id":           ch.ID,
					"name":         ch.Name,
					"type_name":    typeName,
					"status":       ch.Status,
					"priority":     ch.Priority,
					"weight":       ch.Weight,
					"health_score": healthScore,
					"created_at":   ch.CreatedAt.String(),
				}) {
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

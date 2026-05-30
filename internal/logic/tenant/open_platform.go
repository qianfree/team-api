package tenant

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"
	"github.com/qianfree/team-api/internal/utility/crypto"
)

// ============================================================
// 开放平台应用管理
// ============================================================

func (s *sTenant) OpenAppList(ctx context.Context, req *v1.OpenAppListReq) (*v1.OpenAppListRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	page := req.Page
	pageSize := req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	m := dao.OpnApps.Ctx(ctx).Where("tenant_id", tenantID)
	if req.Keyword != "" {
		m = m.Where("name LIKE ?", "%"+req.Keyword+"%")
	}

	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	var apps []entity.OpnApps
	err = m.OrderDesc("created_at").Page(page, pageSize).Scan(&apps)
	if err != nil {
		return nil, err
	}

	items := make([]v1.OpenAppItem, len(apps))
	for i, app := range apps {
		var perms []string
		_ = json.Unmarshal([]byte(app.Permissions), &perms)
		items[i] = v1.OpenAppItem{
			ID:          app.Id,
			Name:        app.Name,
			Description: app.Description,
			AppID:       app.AppId,
			Permissions: perms,
			Status:      app.Status,
			IsSandbox:   app.IsSandbox,
			RateLimit:   app.RateLimit,
		}
		if app.LastUsedAt != nil {
			items[i].LastUsedAt = app.LastUsedAt.Format("Y-m-d H:i:s")
		}
		if app.CreatedAt != nil {
			items[i].CreatedAt = app.CreatedAt.Format("Y-m-d H:i:s")
		}
	}

	return &v1.OpenAppListRes{List: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *sTenant) OpenAppCreate(ctx context.Context, req *v1.OpenAppCreateReq) (*v1.OpenAppCreateRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)

	// Generate app_id (opn_xxxxxxxx)
	appIDBytes := make([]byte, 12)
	if _, err := rand.Read(appIDBytes); err != nil {
		return nil, err
	}
	appID := "opn_" + hex.EncodeToString(appIDBytes)

	// Generate app_secret (sk-opn-xxxxxxxxxxxxxxxx)
	secretBytes := make([]byte, 24)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, err
	}
	appSecret := "sk-opn-" + hex.EncodeToString(secretBytes)

	// Hash secret for storage
	secretHash, err := crypto.HashPassword(appSecret)
	if err != nil {
		return nil, err
	}

	// Store raw secret in Redis (encrypted) for HMAC verification
	encKey := getEncKey(ctx)
	encryptedSecret, err := crypto.EncryptString(encKey, appSecret)
	if err != nil {
		return nil, err
	}

	permsJSON, _ := json.Marshal(req.Permissions)
	ipJSON, _ := json.Marshal(req.IPWhitelist)
	if req.IPWhitelist == nil {
		ipJSON = []byte("[]")
	}

	rateLimit := req.RateLimit
	if rateLimit <= 0 {
		rateLimit = 60
	}

	result, err := dao.OpnApps.Ctx(ctx).Data(do.OpnApps{
		TenantId:      tenantID,
		Name:          req.Name,
		Description:   req.Description,
		AppId:         appID,
		AppSecretHash: secretHash,
		Permissions:   string(permsJSON),
		IpWhitelist:   string(ipJSON),
		RateLimit:     rateLimit,
		Status:        "active",
	}).Insert()
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()

	// Store encrypted secret in Redis
	_, err = g.Redis().Do(ctx, "SET", fmt.Sprintf("open:secret:%d", id), encryptedSecret)
	if err != nil {
		return nil, err
	}

	return &v1.OpenAppCreateRes{ID: id, AppID: appID, AppSecret: appSecret}, nil
}

func (s *sTenant) OpenAppUpdate(ctx context.Context, req *v1.OpenAppUpdateReq) (*v1.OpenAppUpdateRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	data := g.Map{}

	if req.Name != nil {
		data["name"] = *req.Name
	}
	if req.Description != nil {
		data["description"] = *req.Description
	}
	if req.Permissions != nil {
		permsJSON, _ := json.Marshal(req.Permissions)
		data["permissions"] = string(permsJSON)
	}
	if req.IPWhitelist != nil {
		ipJSON, _ := json.Marshal(req.IPWhitelist)
		data["ip_whitelist"] = string(ipJSON)
	}
	if req.RateLimit != nil {
		data["rate_limit"] = *req.RateLimit
	}

	if len(data) == 0 {
		return nil, nil
	}

	_, err := dao.OpnApps.Ctx(ctx).Where("id", req.Id).Where("tenant_id", tenantID).Data(data).Update()
	return nil, err
}

func (s *sTenant) OpenAppDelete(ctx context.Context, req *v1.OpenAppDeleteReq) (*v1.OpenAppDeleteRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	_, err := dao.OpnApps.Ctx(ctx).Where("id", req.Id).Where("tenant_id", tenantID).Delete()
	if err != nil {
		return nil, err
	}
	// Clean up secret from Redis
	_, _ = g.Redis().Do(ctx, "DEL", fmt.Sprintf("open:secret:%d", req.Id))
	return nil, nil
}

func (s *sTenant) OpenAppResetSecret(ctx context.Context, req *v1.OpenAppResetSecretReq) (*v1.OpenAppResetSecretRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)

	// Generate new secret
	secretBytes := make([]byte, 24)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, err
	}
	appSecret := "sk-opn-" + hex.EncodeToString(secretBytes)

	secretHash, err := crypto.HashPassword(appSecret)
	if err != nil {
		return nil, err
	}

	_, err = dao.OpnApps.Ctx(ctx).Where("id", req.Id).Where("tenant_id", tenantID).Data(do.OpnApps{
		AppSecretHash: secretHash,
	}).Update()
	if err != nil {
		return nil, err
	}

	// Update Redis
	encKey := getEncKey(ctx)
	encryptedSecret, err := crypto.EncryptString(encKey, appSecret)
	if err != nil {
		return nil, err
	}
	_, err = g.Redis().Do(ctx, "SET", fmt.Sprintf("open:secret:%d", req.Id), encryptedSecret)
	if err != nil {
		return nil, err
	}

	return &v1.OpenAppResetSecretRes{AppSecret: appSecret}, nil
}

func (s *sTenant) OpenAppToggleStatus(ctx context.Context, req *v1.OpenAppToggleStatusReq) (*v1.OpenAppToggleStatusRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	_, err := dao.OpnApps.Ctx(ctx).Where("id", req.Id).Where("tenant_id", tenantID).Data(do.OpnApps{
		Status: req.Status,
	}).Update()
	return nil, err
}

// ============================================================
// Webhook 配置管理
// ============================================================

func (s *sTenant) WebhookConfigList(ctx context.Context, _ *v1.WebhookConfigListReq) (*v1.WebhookConfigListRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	var configs []entity.OpnWebhookConfigs
	err := dao.OpnWebhookConfigs.Ctx(ctx).Where("tenant_id", tenantID).OrderDesc("created_at").Scan(&configs)
	if err != nil {
		return nil, err
	}

	items := make([]v1.WebhookConfigItem, len(configs))
	for i, c := range configs {
		var events []string
		_ = json.Unmarshal([]byte(c.Events), &events)
		items[i] = v1.WebhookConfigItem{
			ID:                     c.Id,
			Name:                   c.Name,
			URL:                    c.Url,
			Events:                 events,
			IsActive:               c.IsActive,
			ConsecutiveFailures:    c.ConsecutiveFailures,
			MaxConsecutiveFailures: c.MaxConsecutiveFailures,
		}
		if c.LastDeliveryAt != nil {
			items[i].LastDeliveryAt = c.LastDeliveryAt.Format("Y-m-d H:i:s")
		}
		if c.CreatedAt != nil {
			items[i].CreatedAt = c.CreatedAt.Format("Y-m-d H:i:s")
		}
	}

	return &v1.WebhookConfigListRes{List: items}, nil
}

func (s *sTenant) WebhookConfigCreate(ctx context.Context, req *v1.WebhookConfigCreateReq) (*v1.WebhookConfigCreateRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)

	// Generate signing key
	keyBytes := make([]byte, 24)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, err
	}
	secretKey := "whk_" + hex.EncodeToString(keyBytes)

	eventsJSON, _ := json.Marshal(req.Events)
	maxFails := req.MaxConsecutiveFailures
	if maxFails <= 0 {
		maxFails = 10
	}

	result, err := dao.OpnWebhookConfigs.Ctx(ctx).Data(do.OpnWebhookConfigs{
		TenantId:               tenantID,
		Name:                   req.Name,
		Url:                    req.URL,
		SecretKey:              secretKey,
		Events:                 string(eventsJSON),
		IsActive:               true,
		MaxConsecutiveFailures: maxFails,
	}).Insert()
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &v1.WebhookConfigCreateRes{ID: id, SecretKey: secretKey}, nil
}

func (s *sTenant) WebhookConfigUpdate(ctx context.Context, req *v1.WebhookConfigUpdateReq) (*v1.WebhookConfigUpdateRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	data := g.Map{}

	if req.Name != nil {
		data["name"] = *req.Name
	}
	if req.URL != nil {
		data["url"] = *req.URL
	}
	if req.Events != nil {
		eventsJSON, _ := json.Marshal(req.Events)
		data["events"] = string(eventsJSON)
	}
	if req.IsActive != nil {
		data["is_active"] = *req.IsActive
	}
	if req.MaxConsecutiveFailures != nil {
		data["max_consecutive_failures"] = *req.MaxConsecutiveFailures
	}

	if len(data) == 0 {
		return nil, nil
	}

	_, err := dao.OpnWebhookConfigs.Ctx(ctx).Where("id", req.Id).Where("tenant_id", tenantID).Data(data).Update()
	return nil, err
}

func (s *sTenant) WebhookConfigDelete(ctx context.Context, req *v1.WebhookConfigDeleteReq) (*v1.WebhookConfigDeleteRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	_, err := dao.OpnWebhookConfigs.Ctx(ctx).Where("id", req.Id).Where("tenant_id", tenantID).Delete()
	return nil, err
}

func (s *sTenant) WebhookDeliveryLogs(ctx context.Context, req *v1.WebhookDeliveryLogsReq) (*v1.WebhookDeliveryLogsRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	page := req.Page
	pageSize := req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	m := dao.OpnWebhookDeliveryLogs.Ctx(ctx).Where("tenant_id", tenantID).Where("webhook_config_id", req.Id)
	if req.Status != "" {
		// Filter by event status through join
		m = m.LeftJoin("opn_webhook_events e", "e.id = opn_webhook_delivery_logs.event_id").Where("e.status", req.Status)
	}

	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	var logs []entity.OpnWebhookDeliveryLogs
	err = m.OrderDesc("created_at").Page(page, pageSize).Scan(&logs)
	if err != nil {
		return nil, err
	}

	items := make([]v1.WebhookDeliveryLogItem, len(logs))
	for i, l := range logs {
		items[i] = v1.WebhookDeliveryLogItem{
			ID:             l.Id,
			EventID:        l.EventId,
			Attempt:        l.Attempt,
			ResponseStatus: l.ResponseStatus,
			ResponseTimeMs: l.ResponseTimeMs,
			ErrorMessage:   l.ErrorMessage,
		}
		if l.CreatedAt != nil {
			items[i].CreatedAt = l.CreatedAt.Format("Y-m-d H:i:s")
		}
	}

	return &v1.WebhookDeliveryLogsRes{List: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *sTenant) WebhookRetry(ctx context.Context, req *v1.WebhookRetryReq) (*v1.WebhookRetryRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)

	result, err := dao.OpnWebhookEvents.Ctx(ctx).
		Where("id", req.EventId).
		Where("tenant_id", tenantID).
		Where("status IN (?)", g.Slice{"failed", "pending"}).
		Data(do.OpnWebhookEvents{
			Status:      "pending",
			NextRetryAt: gtime.Now(),
			Attempts:    0,
		}).Update()
	if err != nil {
		return nil, err
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		NotifyNewEvent(req.EventId)
	}
	return nil, nil
}

// ============================================================
// Webhook 事件投递（后台任务）
// ============================================================

// PublishWebhookEvent publishes a webhook event for a tenant.
func PublishWebhookEvent(ctx context.Context, tenantID int64, eventType string, payload any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Find all active webhook configs that subscribe to this event type
	var configs []entity.OpnWebhookConfigs
	err = dao.OpnWebhookConfigs.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("is_active", true).
		Where("events::jsonb @> ?", fmt.Sprintf(`["%s"]`, eventType)).
		Scan(&configs)
	if err != nil {
		return err
	}

	for _, config := range configs {
		// Generate event ID
		evtBytes := make([]byte, 12)
		if _, err := rand.Read(evtBytes); err != nil {
			continue
		}
		eventID := "evt_" + hex.EncodeToString(evtBytes)

		result, insertErr := dao.OpnWebhookEvents.Ctx(ctx).Data(do.OpnWebhookEvents{
			TenantId:        tenantID,
			WebhookConfigId: config.Id,
			EventId:         eventID,
			EventType:       eventType,
			Payload:         string(payloadJSON),
			Status:          "pending",
			NextRetryAt:     gtime.Now(),
		}).Insert()
		if insertErr != nil {
			g.Log().Error(ctx, "publish webhook event failed:", insertErr)
			continue
		}

		id, _ := result.LastInsertId()
		NotifyNewEvent(id)
	}

	return nil
}

// ComputeWebhookSignature computes the HMAC-SHA256 signature for a webhook payload.
func ComputeWebhookSignature(secret string, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + string(body)))
	return hex.EncodeToString(mac.Sum(nil))
}

package tenant

import (
	"context"
	"encoding/json"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/tenant/v1"

	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
	"github.com/qianfree/team-api/internal/utility/export"
)

// Notifications 获取用户的通知列表（个人消息 + 广播消息）
func (s *sTenant) Notifications(ctx context.Context, req *v1.TenantNotificationsReq) (*v1.TenantNotificationsRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)

	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	// Personal: user_id = userID
	// Broadcast: user_id IS NULL AND is_broadcast = 1 AND role has access
	//   - target_roles IS NULL: visible to all roles
	//   - target_roles contains current role: visible
	//   - otherwise: hidden
	query := dao.NtfMessages.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("(user_id = ? OR (user_id IS NULL AND is_broadcast = 1 AND (target_roles IS NULL OR ? = ANY(string_to_array(target_roles, ',')))))", userID, role)

	items := make([]*v1.TenantNotificationItem, 0)
	var total int
	err := query.OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&items, &total, false)
	if err != nil {
		return nil, err
	}

	// Enrich items with read status for broadcasts (batch query to avoid N+1)
	broadcastIDs := make([]int64, 0)
	for _, item := range items {
		if item.IsBroadcast == 1 {
			broadcastIDs = append(broadcastIDs, item.Id)
		}
	}
	if len(broadcastIDs) > 0 {
		readStatuses := make([]struct {
			MessageId int64 `json:"message_id"`
		}, 0)
		err = dao.NtfReadStatus.Ctx(ctx).
			Where("user_id", userID).
			WhereIn("message_id", broadcastIDs).
			Fields("message_id").
			Scan(&readStatuses)
		if err != nil {
			return nil, err
		}
		readMap := make(map[int64]bool, len(readStatuses))
		for _, rs := range readStatuses {
			readMap[rs.MessageId] = true
		}
		for i, item := range items {
			if item.IsBroadcast == 1 {
				if readMap[item.Id] {
					items[i].IsRead = 1
				} else {
					items[i].IsRead = 0
				}
			}
		}
	}

	return &v1.TenantNotificationsRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// UnreadCount 获取未读消息数量
func (s *sTenant) UnreadCount(ctx context.Context, req *v1.TenantUnreadCountReq) (*v1.TenantUnreadCountRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)

	// Count personal unread messages
	personalUnread, err := dao.NtfMessages.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("user_id", userID).
		Where("is_read", 0).
		Count()
	if err != nil {
		return nil, err
	}

	// Count broadcast messages that the user has NOT read AND has access to
	broadcastUnread, err := dao.NtfMessages.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("user_id IS NULL").
		Where("is_broadcast", 1).
		Where("(target_roles IS NULL OR ? = ANY(string_to_array(target_roles, ',')))", role).
		Where("id NOT IN (?)",
			dao.NtfReadStatus.Ctx(ctx).
				Where("user_id", userID).
				Fields("message_id"),
		).
		Count()
	if err != nil {
		return nil, err
	}

	totalUnread := personalUnread + broadcastUnread

	return &v1.TenantUnreadCountRes{
		UnreadCount: totalUnread,
	}, nil
}

// MarkRead 标记消息为已读
func (s *sTenant) MarkRead(ctx context.Context, req *v1.TenantMarkReadReq) (*v1.TenantMarkReadRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)

	// Verify the message belongs to this tenant
	var msg *struct {
		ID          int64  `json:"id"`
		IsBroadcast int    `json:"is_broadcast"`
		UserID      *int64 `json:"user_id"`
		IsRead      int    `json:"is_read"`
		TargetRoles string `json:"target_roles"`
	}
	err := dao.NtfMessages.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Scan(&msg)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, common.NewNotFoundError("消息")
	}

	// Check role access for broadcast messages
	if msg.IsBroadcast == 1 && msg.TargetRoles != "" {
		role := middleware.GetUserRole(ctx)
		if !containsRole(msg.TargetRoles, role) {
			return nil, common.NewForbiddenError("无权访问该消息")
		}
	}

	if msg.IsBroadcast == 1 {
		// For broadcast messages, add a read_status record
		_, err = dao.NtfReadStatus.Ctx(ctx).Insert(do.NtfReadStatus{
			MessageId: req.Id,
			UserId:    userID,
			ReadAt:    gtime.Now(),
		})
		if err != nil {
			// Ignore duplicate key error (already marked as read)
			return &v1.TenantMarkReadRes{}, nil
		}
	} else {
		// For personal messages, update is_read flag
		if msg.UserID == nil || *msg.UserID != userID {
			return nil, common.NewForbiddenError("该消息不属于当前用户")
		}
		if msg.IsRead == 1 {
			return &v1.TenantMarkReadRes{}, nil // Already read
		}
		_, err = dao.NtfMessages.Ctx(ctx).
			Where("id", req.Id).
			Where("tenant_id", tenantID).
			Data(do.NtfMessages{
				IsRead: 1,
			}).
			Update()
		if err != nil {
			return nil, err
		}
	}

	return &v1.TenantMarkReadRes{}, nil
}

// MarkAllRead 标记所有未读消息为已读
func (s *sTenant) MarkAllRead(ctx context.Context, req *v1.TenantMarkAllReadReq) (*v1.TenantMarkAllReadRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)

	// Mark all personal unread messages as read
	_, err := dao.NtfMessages.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("user_id", userID).
		Where("is_read", 0).
		Data(do.NtfMessages{
			IsRead: 1,
		}).
		Update()
	if err != nil {
		return nil, gerror.Wrapf(err, "标记个人消息已读失败")
	}

	// For broadcast messages visible to this role, add read_status records for all unread broadcasts
	broadcastIDs := make([]struct {
		ID int64 `json:"id"`
	}, 0)
	err = dao.NtfMessages.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("user_id IS NULL").
		Where("is_broadcast", 1).
		Where("(target_roles IS NULL OR ? = ANY(string_to_array(target_roles, ',')))", role).
		Where("id NOT IN (?)",
			dao.NtfReadStatus.Ctx(ctx).
				Where("user_id", userID).
				Fields("message_id"),
		).
		Fields("id").
		Scan(&broadcastIDs)
	if err != nil {
		return nil, gerror.Wrapf(err, "查询未读广播消息失败")
	}

	// Batch insert read_status records
	if len(broadcastIDs) > 0 {
		batch := make([]do.NtfReadStatus, 0, len(broadcastIDs))
		for _, b := range broadcastIDs {
			batch = append(batch, do.NtfReadStatus{
				MessageId: b.ID,
				UserId:    userID,
				ReadAt:    gtime.Now(),
			})
		}
		_, err = dao.NtfReadStatus.Ctx(ctx).Batch(len(batch)).Data(batch).Insert()
		if err != nil {
			return nil, gerror.Wrapf(err, "标记广播消息已读失败")
		}
	}

	return &v1.TenantMarkAllReadRes{}, nil
}

// DeleteNotification 删除已读的个人消息（广播消息不允许用户删除）
func (s *sTenant) DeleteNotification(ctx context.Context, req *v1.TenantNotificationDeleteReq) (*v1.TenantNotificationDeleteRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)

	var msg *struct {
		ID          int64  `json:"id"`
		IsBroadcast int    `json:"is_broadcast"`
		UserID      *int64 `json:"user_id"`
		IsRead      int    `json:"is_read"`
	}
	err := dao.NtfMessages.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Scan(&msg)
	if err != nil {
		return nil, err
	}
	if msg.ID == 0 {
		return nil, common.NewNotFoundError("消息")
	}
	if msg.IsBroadcast == 1 {
		return nil, common.NewBadRequestError("广播消息不支持删除")
	}
	if msg.UserID == nil || *msg.UserID != userID {
		return nil, common.NewForbiddenError("无权删除该消息")
	}
	if msg.IsRead == 0 {
		return nil, common.NewBadRequestError("只能删除已读消息")
	}

	_, err = dao.NtfMessages.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Delete()
	if err != nil {
		return nil, err
	}

	return &v1.TenantNotificationDeleteRes{}, nil
}

// NotificationPreferencesGet 获取合并后的通知偏好（组织 + 用户）
func (s *sTenant) NotificationPreferencesGet(ctx context.Context, req *v1.TenantNotificationPreferencesGetReq) (*v1.TenantNotificationPreferencesGetRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)

	orgPrefs := loadPreferences(ctx, tenantID, 0, "org")
	userPrefs := loadPreferences(ctx, tenantID, userID, "user")

	return &v1.TenantNotificationPreferencesGetRes{
		OrgPreferences:  orgPrefs,
		UserPreferences: userPrefs,
		Merged:          mergePreferences(orgPrefs, userPrefs),
	}, nil
}

// NotificationPreferencesUpdate 更新通知偏好
func (s *sTenant) NotificationPreferencesUpdate(ctx context.Context, req *v1.TenantNotificationPreferencesUpdateReq) (*v1.TenantNotificationPreferencesUpdateRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)

	if req.Scope != "user" && req.Scope != "org" {
		return nil, common.NewBadRequestError("scope 必须是 'user' 或 'org'")
	}

	// Safety categories cannot be fully disabled
	if req.Scope == "org" && middleware.GetUserRole(ctx) != "owner" {
		return nil, common.NewForbiddenError("仅组织所有者可修改组织级通知偏好")
	}
	validateSafetyCategories(req.Preferences)

	prefsJSON, err := json.Marshal(req.Preferences)
	if err != nil {
		return nil, gerror.Wrapf(err, "序列化偏好设置失败")
	}

	// Check if preference record exists
	var existing *struct {
		ID int64 `json:"id"`
	}
	query := dao.NtfPreferences.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("scope", req.Scope)
	if req.Scope == "user" {
		query = query.Where("user_id", userID)
	} else {
		query = query.Where("user_id IS NULL")
	}
	err = query.Fields("id").Scan(&existing)
	if err != nil {
		return nil, err
	}

	if existing != nil && existing.ID > 0 {
		// Update existing
		_, err = dao.NtfPreferences.Ctx(ctx).
			Where("id", existing.ID).
			Data(do.NtfPreferences{
				Preferences: string(prefsJSON),
			}).Update()
		if err != nil {
			return nil, err
		}
	} else {
		// Insert new
		data := do.NtfPreferences{
			TenantId:    tenantID,
			Scope:       req.Scope,
			Preferences: string(prefsJSON),
		}
		if req.Scope == "user" {
			data.UserId = userID
		} else {
			data.UserId = nil
		}
		_, err = dao.NtfPreferences.Ctx(ctx).Insert(data)
		if err != nil {
			return nil, err
		}
	}

	return &v1.TenantNotificationPreferencesUpdateRes{}, nil
}

// Announcements 获取已发布的公告（未过期）
func (s *sTenant) Announcements(ctx context.Context, req *v1.TenantAnnouncementsReq) (*v1.TenantAnnouncementsRes, error) {
	items := make([]*v1.TenantAnnouncementItem, 0)
	err := dao.NtfAnnouncements.Ctx(ctx).
		Where("status", "published").
		Where("(effective_at IS NULL OR effective_at <= NOW())").
		Where("(expires_at IS NULL OR expires_at > NOW())").
		OrderDesc("is_pinned").
		OrderDesc("created_at").
		Limit(50).
		Scan(&items)
	if err != nil {
		return nil, err
	}
	return &v1.TenantAnnouncementsRes{List: items}, nil
}

// -- internal helpers --

// loadPreferences loads notification preferences for a given scope.
func loadPreferences(ctx context.Context, tenantID, userID int64, scope string) map[string]any {
	type prefRow struct {
		Preferences string `json:"preferences"`
	}
	var row *prefRow
	query := dao.NtfPreferences.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("scope", scope)
	if scope == "user" && userID > 0 {
		query = query.Where("user_id", userID)
	} else {
		query = query.Where("user_id IS NULL")
	}
	if err := query.Fields("preferences").Scan(&row); err != nil {
		return getDefaultPreferences()
	}

	if row == nil || row.Preferences == "" || row.Preferences == "{}" {
		return getDefaultPreferences()
	}

	var prefs map[string]any
	if err := json.Unmarshal([]byte(row.Preferences), &prefs); err != nil {
		return getDefaultPreferences()
	}
	return prefs
}

// getDefaultPreferences returns the default notification preferences.
func getDefaultPreferences() map[string]any {
	return map[string]any{
		"billing": map[string]any{
			"email":  true,
			"in_app": true,
		},
		"system": map[string]any{
			"email":  false,
			"in_app": true,
		},
		"security": map[string]any{
			"email":  true,
			"in_app": true,
		},
		"invitation": map[string]any{
			"email":  true,
			"in_app": true,
		},
	}
}

// mergePreferences merges org-level and user-level preferences.
// User-level takes precedence over org-level.
func mergePreferences(org, user map[string]any) map[string]any {
	merged := make(map[string]any)

	// Start with org preferences
	for k, v := range org {
		merged[k] = v
	}

	// Overlay user preferences (takes precedence)
	for k, v := range user {
		if userMap, ok := v.(map[string]any); ok {
			if orgVal, exists := merged[k]; exists {
				if orgMap, ok := orgVal.(map[string]any); ok {
					// Merge the nested maps
					combined := make(map[string]any)
					for orgKey, orgValItem := range orgMap {
						combined[orgKey] = orgValItem
					}
					for uk, uv := range userMap {
						combined[uk] = uv
					}
					merged[k] = combined
					continue
				}
			}
		}
		merged[k] = v
	}

	return merged
}

// validateSafetyCategories ensures security notifications cannot be fully disabled.
func validateSafetyCategories(prefs map[string]any) {
	sec, ok := prefs["security"]
	if !ok {
		return
	}
	secMap, ok := sec.(map[string]any)
	if !ok {
		return
	}

	// Ensure at least one channel is enabled for security
	emailEnabled, _ := secMap["email"].(bool)
	inAppEnabled, _ := secMap["in_app"].(bool)

	if !emailEnabled && !inAppEnabled {
		// Force both channels on for security
		secMap["email"] = true
		secMap["in_app"] = true
		prefs["security"] = secMap
	}
}

// toInt converts various numeric types to int.
func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case json.Number:
		n, _ := val.Int64()
		return int(n)
	default:
		return 0
	}
}

// containsRole checks if a comma-separated role list contains the given role.
func containsRole(targetRoles, role string) bool {
	for _, r := range splitString(targetRoles, ",") {
		if r == role {
			return true
		}
	}
	return false
}

// splitString splits a string by sep, trimming whitespace from each part.
func splitString(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ExportNotifications exports the tenant notification list as CSV or Excel.
func (s *sTenant) ExportNotifications(ctx context.Context, req *v1.TenantNotificationsExportReq) (*v1.TenantNotificationsExportRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	userID := middleware.GetUserID(ctx)
	role := middleware.GetUserRole(ctx)

	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "type", Header: "类型"},
		{Field: "title", Header: "标题"},
		{Field: "channel", Header: "渠道"},
		{Field: "is_read", Header: "已读"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   req.Format,
		Filename: "通知列表_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			items := make([]*v1.TenantNotificationItem, 0)
			err := dao.NtfMessages.Ctx(ctx).
				Where("tenant_id", tenantID).
				Where("(user_id = ? OR (user_id IS NULL AND is_broadcast = 1 AND (target_roles IS NULL OR ? = ANY(string_to_array(target_roles, ',')))))", userID, role).
				OrderDesc("created_at").
				Limit(1000).Offset(offset).
				Scan(&items)
			if err != nil {
				return
			}
			for _, item := range items {
				isRead := 0
				if item.IsBroadcast == 1 {
					readCount, _ := dao.NtfReadStatus.Ctx(ctx).
						Where("message_id", item.Id).
						Where("user_id", userID).
						Count()
					if readCount > 0 {
						isRead = 1
					}
				} else {
					isRead = item.IsRead
				}
				if !yield(map[string]any{
					"id":         item.Id,
					"type":       item.Type,
					"title":      item.Title,
					"channel":    item.Channel,
					"is_read":    isRead,
					"created_at": item.CreatedAt,
				}) {
					return
				}
			}
			if len(items) < 1000 {
				break
			}
			offset += 1000
		}
	})
}

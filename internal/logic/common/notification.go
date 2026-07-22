package common

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// NotificationEngine provides unified notification sending across email and in-app channels.
type NotificationEngine struct{}

// NewNotificationEngine creates a new NotificationEngine instance.
func NewNotificationEngine() *NotificationEngine {
	return &NotificationEngine{}
}

// SendNotification sends a notification to a specific user within a tenant.
// It loads the template by code, renders it, checks preferences, then dispatches
// to both email and in-app channels as appropriate.
func (e *NotificationEngine) SendNotification(ctx context.Context, tenantID, userID int64, templateCode string, variables map[string]any) error {
	// 1. Load template from ntf_templates
	tpl, err := e.loadTemplate(ctx, templateCode)
	if err != nil {
		return gerror.Wrapf(err, "load template %s", templateCode)
	}

	// 2. Render subject and body with variables
	subject, err := renderTemplate(tpl.Subject, variables)
	if err != nil {
		return gerror.Wrapf(err, "render subject for template %s", templateCode)
	}
	body, err := renderTemplate(tpl.BodyTemplate, variables)
	if err != nil {
		return gerror.Wrapf(err, "render body for template %s", templateCode)
	}

	// 3. Determine notification category from template metadata
	category := e.templateCategory(tpl.Channel, templateCode)

	// 4. Check user notification preferences
	prefs := e.getUserPreferences(ctx, tenantID, userID)
	sendEmail := e.shouldSendChannel(prefs, category, "email")
	sendInApp := e.shouldSendChannel(prefs, category, "in_app")

	// 5. Send via email channel
	if sendEmail {
		email, err := e.getUserEmail(ctx, tenantID, userID)
		if err != nil {
			g.Log().Warningf(ctx, "notification engine: get user email failed: tenant=%d user=%d err=%v", tenantID, userID, err)
		} else if email != "" {
			sender, err := e.createEmailSender(ctx)
			if err != nil {
				g.Log().Warningf(ctx, "notification engine: create email sender failed: %v", err)
			} else {
				msg := &EmailMessage{
					To:           email,
					Subject:      subject,
					BodyHTML:     body,
					TemplateCode: templateCode,
					Variables:    variables,
					TenantID:     tenantID,
					UserID:       userID,
				}
				if err := sender.Send(ctx, msg); err != nil {
					g.Log().Warningf(ctx, "notification engine: send email failed: %v", err)
				}
			}
		}
	}

	// 6. Create in-app message
	if sendInApp {
		_, insertErr := dao.NtfMessages.Ctx(ctx).Insert(do.NtfMessages{
			TenantId:    tenantID,
			UserId:      userID,
			Type:        category,
			Title:       subject,
			Content:     body,
			Channel:     "in_app",
			IsRead:      0,
			IsBroadcast: 0,
			Metadata:    e.buildMetadata(templateCode, variables),
		})
		if insertErr != nil {
			g.Log().Warningf(ctx, "notification engine: create in-app message failed: %v", insertErr)

		}
	}

	// 7. Log to ntf_send_log
	e.logNotification(ctx, tenantID, userID, templateCode, subject, body)

	return nil
}

// SendBroadcast sends a notification to all members of a tenant.
// It creates a single broadcast message with user_id=NULL and records it as is_broadcast=1.
// targetRoles: comma-separated roles (e.g. "owner,admin"). Empty string means visible to all roles.
func (e *NotificationEngine) SendBroadcast(ctx context.Context, tenantID int64, templateCode string, variables map[string]any, targetRoles string) error {
	// 1. Load template
	tpl, err := e.loadTemplate(ctx, templateCode)
	if err != nil {
		return gerror.Wrapf(err, "load template %s", templateCode)
	}

	// 2. Render subject and body
	subject, err := renderTemplate(tpl.Subject, variables)
	if err != nil {
		return gerror.Wrapf(err, "render subject for template %s", templateCode)
	}
	body, err := renderTemplate(tpl.BodyTemplate, variables)
	if err != nil {
		return gerror.Wrapf(err, "render body for template %s", templateCode)
	}

	category := e.templateCategory(tpl.Channel, templateCode)

	// 3. Create broadcast in-app message (user_id=NULL)
	_, err = dao.NtfMessages.Ctx(ctx).Insert(do.NtfMessages{
		TenantId:    tenantID,
		UserId:      nil,
		Type:        category,
		Title:       subject,
		Content:     body,
		Channel:     "in_app",
		IsRead:      0,
		IsBroadcast: 1,
		Metadata:    e.buildMetadata(templateCode, variables),
		TargetRoles: targetRoles,
	})
	if err != nil {
		return gerror.Wrapf(err, "create broadcast message")
	}

	// 4. Send email to all tenant members (async, best-effort)
	go e.broadcastEmails(ctx, tenantID, subject, body, templateCode, variables)

	// 5. Log
	e.logNotification(ctx, tenantID, 0, templateCode, subject, body)

	return nil
}

// SendToAllTenants sends a broadcast notification to all active tenants.
// It retrieves all active tenant IDs and sends a broadcast to each.
// targetRoles: comma-separated roles. Empty string means visible to all roles.
func (e *NotificationEngine) SendToAllTenants(ctx context.Context, templateCode string, variables map[string]any, targetRoles string) error {
	tenants := make([]struct {
		ID int64 `json:"id"`
	}, 0)
	err := dao.TntTenants.Ctx(ctx).
		Where("status IN(?)", g.Slice{"active", "trial", "free"}).
		Fields("id").
		Scan(&tenants)
	if err != nil {
		return gerror.Wrapf(err, "query active tenants")
	}

	var lastErr error
	for _, t := range tenants {
		if err := e.SendBroadcast(ctx, t.ID, templateCode, variables, targetRoles); err != nil {
			g.Log().Warningf(ctx, "notification engine: send to tenant %d failed: %v", t.ID, err)
			lastErr = err
		}
	}

	return lastErr
}

// SendMessage creates a direct in-app message without a template.
func (e *NotificationEngine) SendMessage(ctx context.Context, tenantID, userID int64, msgType, title, content string) error {
	_, err := dao.NtfMessages.Ctx(ctx).Insert(do.NtfMessages{
		TenantId:    tenantID,
		UserId:      userID,
		Type:        msgType,
		Title:       title,
		Content:     content,
		Channel:     "in_app",
		IsRead:      0,
		IsBroadcast: 0,
		Metadata:    nil,
	})
	return err
}

// SendBroadcastMessage creates a broadcast in-app message without a template.
// targetRoles: comma-separated roles. Empty string means visible to all roles.
func (e *NotificationEngine) SendBroadcastMessage(ctx context.Context, tenantID int64, msgType, title, content, targetRoles string) error {
	// Empty targetRoles must be nil so DB stores NULL (visible to all roles).
	// Empty string "" would fail the tenant query: target_roles IS NULL OR ...
	var targetRolesVal any
	if targetRoles != "" {
		targetRolesVal = targetRoles
	}

	_, err := dao.NtfMessages.Ctx(ctx).Insert(do.NtfMessages{
		TenantId:    tenantID,
		UserId:      nil,
		Type:        msgType,
		Title:       title,
		Content:     content,
		Channel:     "in_app",
		IsRead:      0,
		IsBroadcast: 1,
		Metadata:    nil,
		TargetRoles: targetRolesVal,
	})
	return err
}

// -- internal helpers --

// notificationTemplate holds loaded template data.
type notificationTemplate struct {
	Subject      string `json:"subject"`
	BodyTemplate string `json:"body_template"`
	Channel      string `json:"channel"`
	Status       string `json:"status"`
}

// loadTemplate loads a notification template by code from ntf_templates.
func (e *NotificationEngine) loadTemplate(ctx context.Context, code string) (*notificationTemplate, error) {
	var tpl notificationTemplate
	err := dao.NtfTemplates.Ctx(ctx).
		Where("code", code).
		Where("status", "active").
		Scan(&tpl)
	if err != nil {
		return nil, err
	}
	if tpl.BodyTemplate == "" {
		return nil, gerror.Newf("template %s not found or inactive", code)
	}
	return &tpl, nil
}

// templateCategory extracts a notification category from the template channel or code.
func (e *NotificationEngine) templateCategory(channel, templateCode string) string {
	// Derive category from template code prefix
	switch {
	case containsAny(templateCode, "billing", "payment", "invoice", "balance", "quota"):
		return "billing"
	case containsAny(templateCode, "security", "password", "login", "2fa"):
		return "security"
	case containsAny(templateCode, "invitation", "invite", "member"):
		return "invitation"
	default:
		return "system"
	}
}

// getUserPreferences loads the merged notification preferences for a user.
// Merges org-level and user-level preferences; user-level takes precedence.
func (e *NotificationEngine) getUserPreferences(ctx context.Context, tenantID, userID int64) map[string]any {
	// Start with org-level preferences
	orgPrefs := e.loadPreferencesByScope(ctx, tenantID, 0, "org")
	userPrefs := e.loadPreferencesByScope(ctx, tenantID, userID, "user")

	// Merge: start with org, overlay user preferences
	merged := make(map[string]any)
	if orgPrefs != nil {
		for k, v := range orgPrefs {
			merged[k] = v
		}
	}
	if userPrefs != nil {
		for k, v := range userPrefs {
			merged[k] = v
		}
	}

	return merged
}

// loadPreferencesByScope loads preferences for a specific scope.
func (e *NotificationEngine) loadPreferencesByScope(ctx context.Context, tenantID, userID int64, scope string) map[string]any {
	type prefRow struct {
		Preferences string `json:"preferences"`
	}
	var row prefRow
	query := dao.NtfPreferences.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("scope", scope)
	if userID > 0 {
		query = query.Where("user_id", userID)
	} else {
		query = query.Where("user_id IS NULL")
	}
	query.Fields("preferences").Scan(&row)

	if row.Preferences == "" || row.Preferences == "{}" {
		return nil
	}

	var prefs map[string]any
	if err := json.Unmarshal([]byte(row.Preferences), &prefs); err != nil {
		return nil
	}
	return prefs
}

// shouldSendChannel checks if a given channel is enabled for a category.
// Default is true if no preference is set.
func (e *NotificationEngine) shouldSendChannel(prefs map[string]any, category, channel string) bool {
	if prefs == nil {
		return true
	}

	catPrefs, ok := prefs[category]
	if !ok {
		return true
	}
	catMap, ok := catPrefs.(map[string]any)
	if !ok {
		return true
	}

	enabled, ok := catMap[channel]
	if !ok {
		return true
	}

	// Safety category cannot be fully disabled
	if category == "security" {
		return true
	}

	if boolVal, ok := enabled.(bool); ok {
		return boolVal
	}
	return true
}

// getUserEmail retrieves the email address of a user.
func (e *NotificationEngine) getUserEmail(ctx context.Context, tenantID, userID int64) (string, error) {
	type userRow struct {
		Email string `json:"email"`
	}
	var user userRow
	err := dao.TntUsers.Ctx(ctx).
		Where("id", userID).
		Fields("email").
		Scan(&user)
	if err != nil {
		return "", err
	}
	return user.Email, nil
}

// createEmailSender creates a BasicEmailSender from configuration.
func (e *NotificationEngine) createEmailSender(ctx context.Context) (*BasicEmailSender, error) {
	cfg, err := EmailConfigFromOptions(ctx)
	if err != nil {
		return nil, err
	}
	return NewEmailSender(cfg), nil
}

// broadcastEmails sends emails to all members of a tenant asynchronously.
func (e *NotificationEngine) broadcastEmails(ctx context.Context, tenantID int64, subject, body, templateCode string, variables map[string]any) {
	sender, err := e.createEmailSender(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "notification engine broadcast: create email sender failed: %v", err)
		return
	}

	// Get all member emails
	members := make([]struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
	}, 0)
	err = dao.TntUsers.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("status", "active").
		Fields("id, email").
		Scan(&members)
	if err != nil {
		g.Log().Warningf(ctx, "notification engine broadcast: query members failed: %v", err)
		return
	}

	for _, m := range members {
		if m.Email == "" || !IsValidEmail(m.Email) {
			continue
		}
		msg := &EmailMessage{
			To:           m.Email,
			Subject:      subject,
			BodyHTML:     body,
			TemplateCode: templateCode,
			Variables:    variables,
			TenantID:     tenantID,
			UserID:       m.ID,
		}
		if err := sender.Send(ctx, msg); err != nil {
			g.Log().Warningf(ctx, "notification engine broadcast: send to user %d failed: %v", m.ID, err)
		}
	}
}

// buildMetadata constructs a JSONB metadata value for the message.
func (e *NotificationEngine) buildMetadata(templateCode string, variables map[string]any) string {
	meta := map[string]any{
		"template_code": templateCode,
	}
	if variables != nil {
		meta["variables"] = variables
	}
	data, _ := json.Marshal(meta)
	return string(data)
}

// logNotification records the notification dispatch in ntf_send_log.
func (e *NotificationEngine) logNotification(ctx context.Context, tenantID, userID int64, templateCode, subject, body string) {
	recipient := fmt.Sprintf("in_app:tenant:%d", tenantID)
	if userID > 0 {
		recipient = fmt.Sprintf("in_app:tenant:%d:user:%d", tenantID, userID)
	}

	_, err := dao.NtfSendLog.Ctx(ctx).Insert(do.NtfSendLog{
		TenantId:     tenantID,
		UserId:       userID,
		TemplateCode: templateCode,
		Channel:      "in_app",
		Recipient:    recipient,
		Subject:      subject,
		Body:         body,
		Status:       "sent",
		SentAt:       gtime.Now(),
		RetryCount:   0,
	})
	if err != nil {
		g.Log().Warningf(ctx, "notification engine: log notification failed: %v", err)
	}
}

// containsAny checks if the haystack contains any of the given substrings。
func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if n != "" && strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

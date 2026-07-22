package middleware

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/gconv"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
)

// sensitiveFields are field names that should be masked in operation logs.
var sensitiveFields = map[string]bool{
	"password":      true,
	"password_hash": true,
	"old_password":  true,
	"new_password":  true,
	"token":         true,
	"access_token":  true,
	"refresh_token": true,
	"secret":        true,
	"api_key":       true,
	"encrypted_key": true,
	// 存储配置凭证（系统设置更新 / 存储连通性测试的请求体中携带）
	"storage_access_key_id":     true,
	"storage_access_key_secret": true,
}

// resourcePluralToSingular maps URL path resource names to singular form.
var resourcePluralToSingular = map[string]string{
	"channels":      "channel",
	"models":        "model",
	"tenants":       "tenant",
	"users":         "user",
	"keys":          "key",
	"abilities":     "ability",
	"plans":         "plan",
	"orders":        "order",
	"redemptions":   "redemption",
	"promo-codes":   "promo_code",
	"sessions":      "session",
	"members":       "member",
	"announcements": "announcement",
	"tickets":       "ticket",
	"templates":     "template",
	"messages":      "message",
	"settings":      "settings",
	"permissions":   "permissions",
	"data-scopes":   "data_scopes",
	"wallet":        "wallet",
	"audit":         "audit",
	"auth":          "auth",
	"notifications": "notification",
	"pricing-tiers": "pricing_tier",
	"channel-scope": "channel_scope",
}

// actionVerbMap maps HTTP methods to action prefixes.
var actionVerbMap = map[string]string{
	"POST":   "create",
	"PUT":    "update",
	"DELETE": "delete",
	"PATCH":  "update",
}

// OperationLog records admin operations for audit trail.
func OperationLog(r *ghttp.Request) {
	// Only log write operations
	method := r.Method
	if method != "POST" && method != "PUT" && method != "DELETE" {
		r.Middleware.Next()
		return
	}

	// Skip body capture for multipart/form-data (file uploads)
	// r.GetBodyString() consumes the body stream and breaks multipart parsing
	var body string
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		body = r.GetBodyString()
	}

	r.Middleware.Next()

	// Only log successful operations
	if r.Response.Status >= 400 {
		return
	}

	// Get auth info
	userID := GetUserID(r.Context())
	userType := GetUserType(r.Context())

	if userID == 0 {
		return
	}

	// Parse resource info from URL path
	resourceType, resourceID := parseResourceFromPath(r.URL.Path)
	action := buildAction(method, r.URL.Path, resourceType)

	// Mask sensitive fields in request body
	maskedBody := maskSensitiveFields(body)

	// Build detail JSONB
	detail := map[string]any{
		"path":        r.URL.Path,
		"method":      method,
		"status_code": r.Response.Status,
		"user_agent":  r.Header.Get("User-Agent"),
	}
	if maskedBody != "" {
		detail["request_body"] = maskedBody
	}
	if requestID := r.GetCtxVar("RequestId"); requestID != nil {
		detail["request_id"] = requestID.String()
	}
	detailJSON, _ := json.Marshal(detail)

	// Build log data using DO struct
	logData := do.AudOperationLogs{
		UserId:       userID,
		UserType:     userType,
		Action:       action,
		ResourceType: resourceType,
		IpAddress:    r.GetClientIp(),
		Detail:       string(detailJSON),
	}

	if resourceID > 0 {
		logData.ResourceId = resourceID
	}

	tenantID := GetTenantID(r.Context())
	if tenantID > 0 {
		logData.TenantId = tenantID
	}

	if maskedBody != "" {
		var changesJSON map[string]any
		if err := json.Unmarshal([]byte(maskedBody), &changesJSON); err == nil {
			wrapper := map[string]any{
				"updated_fields": changesJSON,
			}
			if wrapped, err := json.Marshal(wrapper); err == nil {
				logData.ChangesJson = string(wrapped)
			}
		}
	}

	go func() {
		ctx := context.Background()
		_, err := lcommon.AuditModelCtx(ctx, "aud_operation_logs").Data(logData).Insert()
		if err != nil {
			g.Log().Errorf(ctx, "write operation log: %v", err)
		}
	}()
}

// parseResourceFromPath extracts resource type and ID from URL path.
// e.g. /api/admin/channels/123/keys → ("channel", 123)
// e.g. /api/admin/models → ("model", 0)
// e.g. /api/admin/auth/logout → ("auth", 0)
func parseResourceFromPath(path string) (resourceType string, resourceID int64) {
	// Normalize: remove /api/ prefix and split
	path = strings.TrimPrefix(path, "/api/")
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Skip prefix like "admin" or "tenant"
	idx := 0
	if len(parts) > 0 && (parts[0] == "admin" || parts[0] == "tenant") {
		idx = 1
	}

	// Look for known resource names
	for i := idx; i < len(parts); i++ {
		if singular, ok := resourcePluralToSingular[parts[i]]; ok {
			resourceType = singular
			// Check if next part is a numeric ID
			if i+1 < len(parts) {
				if id := gconv.Int64(parts[i+1]); id > 0 {
					resourceID = id
				}
			}
			return
		}
	}

	// Fallback: use last meaningful path segment
	if len(parts) > idx {
		resourceType = parts[len(parts)-1]
		// Check if the second-to-last is numeric → use the segment before it
		if len(parts) >= 2 {
			if gconv.Int64(parts[len(parts)-1]) > 0 {
				resourceType = parts[len(parts)-2]
			}
		}
	}
	return
}

// buildAction creates a semantic action name from HTTP method and path.
// e.g. POST + /api/admin/channels → "create_channel"
// e.g. PUT + /api/admin/channels/123 → "update_channel"
// e.g. DELETE + /api/admin/channels/123 → "delete_channel"
func buildAction(method, path, resourceType string) string {
	verb, ok := actionVerbMap[strings.ToUpper(method)]
	if !ok {
		verb = strings.ToLower(method)
	}

	if resourceType != "" {
		// Special cases for non-CRUD endpoints
		path = strings.TrimPrefix(path, "/api/")
		parts := strings.Split(strings.Trim(path, "/"), "/")

		// Check for special action suffixes
		if len(parts) >= 2 {
			last := parts[len(parts)-1]
			secondLast := parts[len(parts)-2]

			// If last part is not a number and not a resource name, it's a custom action
			_, isResource := resourcePluralToSingular[last]

			if gconv.Int64(last) == 0 && !isResource {
				// Custom action: e.g. /channels/123/test → "test_channel"
				// or /auth/logout → "logout_auth"
				return last + "_" + resourceType
			}

			// If secondLast is a number and last is not a resource, custom action on specific resource
			if gconv.Int64(secondLast) > 0 && !isResource {
				return last + "_" + resourceType
			}
		}

		return verb + "_" + resourceType
	}

	return strings.ToLower(method)
}

// maskSensitiveFields masks sensitive field values in JSON body.
func maskSensitiveFields(body string) string {
	if body == "" {
		return body
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return body // Not JSON, return as-is
	}

	maskSensitiveMap(data)

	masked, err := json.Marshal(data)
	if err != nil {
		return body
	}
	return string(masked)
}

// maskSensitiveMap recursively masks sensitive fields in a map.
func maskSensitiveMap(data map[string]any) {
	for key, val := range data {
		if isSensitive(key) {
			data[key] = "******"
			continue
		}
		if nested, ok := val.(map[string]any); ok {
			maskSensitiveMap(nested)
		}
	}
}

// isSensitive checks if a field name is sensitive.
func isSensitive(field string) bool {
	field = strings.ToLower(field)
	return sensitiveFields[field]
}

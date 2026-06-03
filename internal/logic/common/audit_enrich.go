package common

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
)

// EnrichAuditRecords 为审计记录批量补充关联信息（用户名、租户名、API Key 名、项目名）。
// records 中的每条记录应包含 tenant_id、user_id、api_key_id、project_id 字段，
// 函数会从主库批量查询关联信息并回填 tenant_name、username、api_key_name、project_name 字段。
func EnrichAuditRecords(ctx context.Context, records []map[string]any) {
	if len(records) == 0 {
		return
	}

	// 收集所有需要查询的 ID
	tenantIDs := collectInt64Fields(records, "tenant_id")
	userKeys := collectUserKeys(records)
	apiKeyIDs := collectInt64Fields(records, "api_key_id")
	projectIDs := collectInt64Fields(records, "project_id")

	// 批量查询关联信息（从主库）
	tenantMap := BatchQueryTenantNames(ctx, tenantIDs)
	userMap := BatchQueryUserNames(ctx, userKeys)
	apiKeyMap := BatchQueryApiKeyNames(ctx, apiKeyIDs)
	projectMap := BatchQueryProjectNames(ctx, projectIDs)

	// 回填关联信息
	for _, record := range records {
		if tenantID, ok := getInt64Field(record, "tenant_id"); ok && tenantID > 0 {
			record["tenant_name"] = tenantMap[tenantID]
		}
		if userKey, ok := getUserKey(record); ok {
			record["username"] = userMap[userKey]
		}
		if apiKeyID, ok := getInt64Field(record, "api_key_id"); ok && apiKeyID > 0 {
			record["api_key_name"] = apiKeyMap[apiKeyID]
		}
		if projectID, ok := getInt64Field(record, "project_id"); ok && projectID > 0 {
			record["project_name"] = projectMap[projectID]
		}
	}
}

// BatchQueryUserNames 批量查询用户名。
// key 格式为 "tenantID:userID"，返回 map[key]username。
func BatchQueryUserNames(ctx context.Context, userKeys []string) map[string]string {
	if len(userKeys) == 0 {
		return nil
	}
	result := make(map[string]string, len(userKeys))
	// 去重
	seen := make(map[string]bool, len(userKeys))
	unique := make([]string, 0, len(userKeys))
	for _, k := range userKeys {
		if !seen[k] {
			seen[k] = true
			unique = append(unique, k)
		}
	}

	// 拆分 tenantID 和 userID，按 tenantID 分组批量查询
	tenantUsers := make(map[int64][]int64) // tenantID -> []userID
	userKeyToID := make(map[string]int64)  // "tenantID:userID" -> userID
	for _, key := range unique {
		var tenantID, userID int64
		fmt.Sscanf(key, "%d:%d", &tenantID, &userID)
		if tenantID > 0 && userID > 0 {
			tenantUsers[tenantID] = append(tenantUsers[tenantID], userID)
			userKeyToID[key] = userID
		}
	}

	// 按 tenantID 分组查询
	for tenantID, userIDs := range tenantUsers {
		type row struct {
			Id       int64  `json:"id"`
			Username string `json:"username"`
		}
		var rows []row
		err := g.DB().Ctx(ctx).Model("tnt_users").
			Where("tenant_id", tenantID).
			WhereIn("id", userIDs).
			Fields("id, username").
			Scan(&rows)
		if err != nil {
			continue
		}
		for _, r := range rows {
			mapKey := fmt.Sprintf("%d:%d", tenantID, r.Id)
			result[mapKey] = r.Username
		}
	}

	return result
}

// BatchQueryTenantNames 批量查询租户名。
func BatchQueryTenantNames(ctx context.Context, tenantIDs []int64) map[int64]string {
	if len(tenantIDs) == 0 {
		return nil
	}
	result := make(map[int64]string, len(tenantIDs))
	// 去重
	seen := make(map[int64]bool, len(tenantIDs))
	unique := make([]int64, 0, len(tenantIDs))
	for _, id := range tenantIDs {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}

	type row struct {
		Id   int64  `json:"id"`
		Name string `json:"name"`
	}
	var rows []row
	err := g.DB().Ctx(ctx).Model("tnt_tenants").
		WhereIn("id", unique).
		Fields("id, name").
		Scan(&rows)
	if err != nil {
		return result
	}
	for _, r := range rows {
		result[r.Id] = r.Name
	}
	return result
}

// BatchQueryApiKeyNames 批量查询 API Key 名。
func BatchQueryApiKeyNames(ctx context.Context, apiKeyIDs []int64) map[int64]string {
	if len(apiKeyIDs) == 0 {
		return nil
	}
	result := make(map[int64]string, len(apiKeyIDs))
	// 去重
	seen := make(map[int64]bool, len(apiKeyIDs))
	unique := make([]int64, 0, len(apiKeyIDs))
	for _, id := range apiKeyIDs {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}

	type row struct {
		Id   int64  `json:"id"`
		Name string `json:"name"`
	}
	var rows []row
	err := g.DB().Ctx(ctx).Model("api_keys").
		WhereIn("id", unique).
		Fields("id, name").
		Scan(&rows)
	if err != nil {
		return result
	}
	for _, r := range rows {
		result[r.Id] = r.Name
	}
	return result
}

// BatchQueryProjectNames 批量查询项目名。
func BatchQueryProjectNames(ctx context.Context, projectIDs []int64) map[int64]string {
	if len(projectIDs) == 0 {
		return nil
	}
	result := make(map[int64]string, len(projectIDs))
	// 去重
	seen := make(map[int64]bool, len(projectIDs))
	unique := make([]int64, 0, len(projectIDs))
	for _, id := range projectIDs {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}

	type row struct {
		Id   int64  `json:"id"`
		Name string `json:"name"`
	}
	var rows []row
	err := g.DB().Ctx(ctx).Model("tnt_projects").
		WhereIn("id", unique).
		Fields("id, name").
		Scan(&rows)
	if err != nil {
		return result
	}
	for _, r := range rows {
		result[r.Id] = r.Name
	}
	return result
}

// collectInt64Fields 从记录集合中收集指定字段的所有非零 int64 值（去重）。
func collectInt64Fields(records []map[string]any, field string) []int64 {
	seen := make(map[int64]bool)
	result := make([]int64, 0)
	for _, record := range records {
		if v, ok := getInt64Field(record, field); ok && v > 0 && !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// collectUserKeys 收集 "tenantID:userID" 格式的 key（去重）。
func collectUserKeys(records []map[string]any) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, record := range records {
		if key, ok := getUserKey(record); ok && !seen[key] {
			seen[key] = true
			result = append(result, key)
		}
	}
	return result
}

// getInt64Field 从 map 中安全获取 int64 字段值。
func getInt64Field(record map[string]any, field string) (int64, bool) {
	v, ok := record[field]
	if !ok || v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case int64:
		return val, true
	case int:
		return int64(val), true
	case float64:
		return int64(val), true
	default:
		return 0, false
	}
}

// getUserKey 从记录中构建 "tenantID:userID" 格式的 key。
func getUserKey(record map[string]any) (string, bool) {
	tenantID, ok1 := getInt64Field(record, "tenant_id")
	userID, ok2 := getInt64Field(record, "user_id")
	if !ok1 || !ok2 || tenantID == 0 || userID == 0 {
		return "", false
	}
	return fmt.Sprintf("%d:%d", tenantID, userID), true
}

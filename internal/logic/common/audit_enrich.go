package common

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// 关联名称字典缓存：审计 / 日志列表回填 tenant_name / username / api_key_name /
// project_name 用。
//
// 这四类都是「改一次、读十万次」的字典数据，而列表页每翻一页都要回填一次，
// 命中缓存后名称回填不再产生任何 DB 查询（原先每页固定 3 次 + 每租户 1 次查询）。
//
// TTL 300s 与项目既定的「用户信息 300s」缓存策略一致：改名最多滞后一个 TTL。
// 列表展示的是历史日志，短暂滞后可接受，因此不做写侧的主动失效。
var nameDictCache = NewCache("namedict", 300*time.Second)

// 字典种类前缀，与各自 key 拼装函数一起构成缓存 key。
const (
	nameDictTenant  = "tenant"
	nameDictUser    = "user"
	nameDictApiKey  = "apikey"
	nameDictProject = "project"
)

// resolveNameDict 批量解析「字典标识 → 名称」：先按标识查缓存（一次 MGET 批量取回），
// 仅对未命中的标识调用 load 查库，查回的名称回填缓存。
//
// keyOf 把标识编码成缓存 key；load 只接收未命中的标识，且返回的 map 必须只以入参标识为键
// （不能自行规范化），否则回填的 key 与调用方后续的查询 key 对不上。
//
// 库里查不到的标识不回填缓存 —— 否则「刚创建、事务尚未提交」的读穿会被固化成
// 一个长期空值，之后建好也读到空。
// load 出错时降级为「只返回缓存命中部分」，调用方拿到零值名称，不阻断列表查询。
// ids 需调用方先去重。
func resolveNameDict[K comparable](
	ctx context.Context,
	label string,
	ids []K,
	keyOf func(K) string,
	load func(ctx context.Context, missing []K) (map[K]string, error),
) map[K]string {
	if len(ids) == 0 {
		return nil
	}

	cacheKeys := make([]string, len(ids))
	for i, id := range ids {
		cacheKeys[i] = keyOf(id)
	}

	cached := nameDictCache.GetMany(ctx, cacheKeys)
	result := make(map[K]string, len(ids))
	missing := make([]K, 0, len(ids))
	for i, id := range ids {
		if name, ok := cached[cacheKeys[i]].(string); ok && name != "" {
			result[id] = name
			continue
		}
		missing = append(missing, id)
	}
	if len(missing) == 0 {
		return result
	}

	loaded, err := load(ctx, missing)
	if err != nil {
		g.Log().Warningf(ctx, "[NameDict] 查询 %s 名称失败: %v", label, err)
		return result
	}
	if len(loaded) == 0 {
		return result
	}

	backfill := make(map[string]any, len(loaded))
	for id, name := range loaded {
		result[id] = name
		backfill[keyOf(id)] = name
	}
	nameDictCache.SetMany(ctx, backfill)
	return result
}

// idNameDictKey int64 主键类的字典缓存 key，形如 "tenant:12"。
func idNameDictKey(kind string, id int64) string {
	return kind + ":" + strconv.FormatInt(id, 10)
}

// resolveInt64NameDict 租户 / API Key / 项目三类同构字典的解析入口（主键均为 int64）。
func resolveInt64NameDict(
	ctx context.Context,
	kind string,
	ids []int64,
	load func(ctx context.Context, missing []int64) (map[int64]string, error),
) map[int64]string {
	return resolveNameDict(ctx, kind, ids, func(id int64) string {
		return idNameDictKey(kind, id)
	}, load)
}

// nameDictUserKey 用户字典缓存 key，形如 "user:3:45"。
// 租户维度必须编入 key：只按 user_id 缓存的话，A 租户的用户名会在 B 租户的记录上命中，
// 等于绕过了「按 ID 取对象用双键校验」的隔离约定（租户控制台也会回填这些名称）。
func nameDictUserKey(tenantID, userID int64) string {
	return nameDictUser + ":" + pairKey(tenantID, userID)
}

// pairKey 拼装 "tenantID:userID"。
func pairKey(tenantID, userID int64) string {
	return strconv.FormatInt(tenantID, 10) + ":" + strconv.FormatInt(userID, 10)
}

// uniqueInt64s 去重并保持首次出现的顺序（丢弃非正值）。
func uniqueInt64s(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

// parseTenantUserKey 拆解 "tenantID:userID" 复合 key。
func parseTenantUserKey(key string) (tenantID, userID int64, ok bool) {
	head, tail, found := strings.Cut(key, ":")
	if !found {
		return 0, 0, false
	}
	tenantID, err := strconv.ParseInt(head, 10, 64)
	if err != nil || tenantID <= 0 {
		return 0, 0, false
	}
	userID, err = strconv.ParseInt(tail, 10, 64)
	if err != nil || userID <= 0 {
		return 0, 0, false
	}
	return tenantID, userID, true
}

// EnrichAuditRecords 为审计记录批量补充关联信息（用户名、租户名、API Key 名、项目名）。
// records 中的每条记录应包含 tenant_id、user_id、api_key_id、project_id 字段，
// 函数会从主库批量查询关联信息并回填 tenant_name、username、api_key_name、project_name 字段。
// 四类名称各自最多一次查库（且仅在缓存未命中时），与记录条数无关。
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
// 入参 key 格式为 "tenantID:userID"；返回 map[key]username。
func BatchQueryUserNames(ctx context.Context, userKeys []string) map[string]string {
	// 先滤掉非法键：它们既不该进缓存也不该查库
	keys := make([]string, 0, len(userKeys))
	seen := make(map[string]bool, len(userKeys))
	for _, key := range userKeys {
		if _, _, ok := parseTenantUserKey(key); !ok {
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil
	}

	return resolveNameDict(ctx, nameDictUser, keys,
		func(key string) string {
			// keys 已过滤，必定可解析（忽略 ok）
			tenantID, userID, _ := parseTenantUserKey(key)
			return nameDictUserKey(tenantID, userID)
		},
		loadUserNames)
}

// loadUserNames 按 (tenant_id, user_id) 精确配对批量查询用户名。
// 先用 tenant_id IN (...) AND id IN (...) 缩小范围，再在应用层按配对过滤 ——
// 不用「只按 id 查」，否则 user_id 与 tenant_id 不匹配的脏记录会把其他租户的用户名带出来，
// 与「按 ID 取对象用双键校验」的隔离约定相悖。
func loadUserNames(ctx context.Context, keys []string) (map[string]string, error) {
	type pair struct{ tenantID, userID int64 }

	pairs := make([]pair, 0, len(keys))
	tenantIDs := make([]int64, 0, len(keys))
	userIDs := make([]int64, 0, len(keys))
	for _, key := range keys {
		tenantID, userID, ok := parseTenantUserKey(key)
		if !ok {
			continue
		}
		pairs = append(pairs, pair{tenantID, userID})
		tenantIDs = append(tenantIDs, tenantID)
		userIDs = append(userIDs, userID)
	}
	if len(pairs) == 0 {
		return nil, nil
	}

	type row struct {
		Id       int64  `json:"id"`
		TenantId int64  `json:"tenant_id"`
		Username string `json:"username"`
	}
	var rows []row
	err := g.DB().Ctx(ctx).Model("tnt_users").
		WhereIn("tenant_id", uniqueInt64s(tenantIDs)).
		WhereIn("id", uniqueInt64s(userIDs)).
		Fields("id, tenant_id, username").
		Scan(&rows)
	if err != nil {
		return nil, err
	}

	nameByPair := make(map[string]string, len(rows))
	for _, r := range rows {
		nameByPair[pairKey(r.TenantId, r.Id)] = r.Username
	}

	// 以入参 key 原样为返回键（回填缓存时 keyOf 也据此重算），不做规范化改写
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		tenantID, userID, ok := parseTenantUserKey(key)
		if !ok {
			continue
		}
		if name, found := nameByPair[pairKey(tenantID, userID)]; found {
			result[key] = name
		}
	}
	return result, nil
}

// BatchQueryTenantNames 批量查询租户名。
func BatchQueryTenantNames(ctx context.Context, tenantIDs []int64) map[int64]string {
	ids := uniqueInt64s(tenantIDs)
	if len(ids) == 0 {
		return nil
	}
	return resolveInt64NameDict(ctx, nameDictTenant, ids, loadTenantNames)
}

// loadTenantNames 从主库按主键批量查询租户名。
func loadTenantNames(ctx context.Context, tenantIDs []int64) (map[int64]string, error) {
	type row struct {
		Id   int64  `json:"id"`
		Name string `json:"name"`
	}
	var rows []row
	err := g.DB().Ctx(ctx).Model("tnt_tenants").
		WhereIn("id", tenantIDs).
		Fields("id, name").
		Scan(&rows)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]string, len(rows))
	for _, r := range rows {
		result[r.Id] = r.Name
	}
	return result, nil
}

// BatchQueryApiKeyNames 批量查询 API Key 名。
func BatchQueryApiKeyNames(ctx context.Context, apiKeyIDs []int64) map[int64]string {
	ids := uniqueInt64s(apiKeyIDs)
	if len(ids) == 0 {
		return nil
	}
	return resolveInt64NameDict(ctx, nameDictApiKey, ids, loadApiKeyNames)
}

// loadApiKeyNames 从主库按主键批量查询 API Key 名。
func loadApiKeyNames(ctx context.Context, apiKeyIDs []int64) (map[int64]string, error) {
	type row struct {
		Id   int64  `json:"id"`
		Name string `json:"name"`
	}
	var rows []row
	err := g.DB().Ctx(ctx).Model("api_keys").
		WhereIn("id", apiKeyIDs).
		Fields("id, name").
		Scan(&rows)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]string, len(rows))
	for _, r := range rows {
		result[r.Id] = r.Name
	}
	return result, nil
}

// BatchQueryProjectNames 批量查询项目名。
func BatchQueryProjectNames(ctx context.Context, projectIDs []int64) map[int64]string {
	ids := uniqueInt64s(projectIDs)
	if len(ids) == 0 {
		return nil
	}
	return resolveInt64NameDict(ctx, nameDictProject, ids, loadProjectNames)
}

// loadProjectNames 从主库按主键批量查询项目名。
func loadProjectNames(ctx context.Context, projectIDs []int64) (map[int64]string, error) {
	type row struct {
		Id   int64  `json:"id"`
		Name string `json:"name"`
	}
	var rows []row
	err := g.DB().Ctx(ctx).Model("tnt_projects").
		WhereIn("id", projectIDs).
		Fields("id, name").
		Scan(&rows)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]string, len(rows))
	for _, r := range rows {
		result[r.Id] = r.Name
	}
	return result, nil
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
	return pairKey(tenantID, userID), true
}

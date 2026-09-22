package common

import (
	"reflect"
	"testing"
)

// 说明：resolveNameDict / BatchQuery* 的缓存路径依赖 Redis（g.Redis() 在未配置时 panic，
// 单测环境无 Redis 配置），故此处只覆盖不触达缓存的纯逻辑：
// key 拼装、复合 key 解析、ID 去重。缓存命中/回填行为由集成环境验证。

func TestIdNameDictKey(t *testing.T) {
	cases := []struct {
		kind string
		id   int64
		want string
	}{
		{nameDictTenant, 12, "tenant:12"},
		{nameDictApiKey, 7, "apikey:7"},
		{nameDictProject, 0, "project:0"},
		{nameDictTenant, -3, "tenant:-3"},
	}
	for _, c := range cases {
		if got := idNameDictKey(c.kind, c.id); got != c.want {
			t.Errorf("idNameDictKey(%q, %d) = %q, want %q", c.kind, c.id, got, c.want)
		}
	}
}

// TestNameDictUserKeyIsTenantScoped 用户字典 key 必须带上租户维度：
// 只按 user_id 缓存会让 A 租户的用户名在 B 租户的记录上命中（跨租户名称泄漏）。
func TestNameDictUserKeyIsTenantScoped(t *testing.T) {
	if got := nameDictUserKey(3, 45); got != "user:3:45" {
		t.Fatalf("nameDictUserKey(3, 45) = %q, want %q", got, "user:3:45")
	}
	sameUserDifferentTenant := nameDictUserKey(5, 45)
	if nameDictUserKey(3, 45) == sameUserDifferentTenant {
		t.Fatalf("同一 user_id 在不同租户下必须得到不同缓存 key，均得到 %q", sameUserDifferentTenant)
	}
}

// TestPairKeyMatchesUserKeySuffix loadUserNames 用 pairKey 建索引、nameDictUserKey 拼缓存 key，
// 两者必须共用同一套 "tenantID:userID" 编码，否则回填的 key 与查询 key 对不上。
func TestPairKeyMatchesUserKeySuffix(t *testing.T) {
	if got, want := nameDictUserKey(3, 45), nameDictUser+":"+pairKey(3, 45); got != want {
		t.Fatalf("nameDictUserKey(3, 45) = %q, want %q", got, want)
	}
}

// TestParseTenantUserKey 合法键正常拆解；非法键一律拒绝（BatchQueryUserNames 据此静默丢弃，
// 不能把错键当成 tenant 0 / user 0 去查库）。
func TestParseTenantUserKey(t *testing.T) {
	valid := []struct {
		key      string
		tenantID int64
		userID   int64
	}{
		{"3:45", 3, 45},
		{"100:1", 100, 1},
	}
	for _, c := range valid {
		tenantID, userID, ok := parseTenantUserKey(c.key)
		if !ok || tenantID != c.tenantID || userID != c.userID {
			t.Errorf("parseTenantUserKey(%q) = (%d, %d, %v), want (%d, %d, true)",
				c.key, tenantID, userID, ok, c.tenantID, c.userID)
		}
	}

	invalid := []string{
		"",                                // 空串
		"45",                              // 无分隔符
		":45",                             // 缺 tenantID
		"3:",                              // 缺 userID
		"0:45",                            // tenantID 非正
		"3:0",                             // userID 非正
		"-3:45",                           // tenantID 非正
		"3:-45",                           // userID 非正
		"a:45",                            // 非数字
		"3:b",                             // 非数字
		"3:45:6",                          // 多余段（tail 非数字）
		" 3:45",                           // 前导空格
		"2147483648:99999999999999999999", // 溢出
	}
	for _, key := range invalid {
		if tenantID, userID, ok := parseTenantUserKey(key); ok {
			t.Errorf("parseTenantUserKey(%q) 应被拒绝，却返回 (%d, %d, true)", key, tenantID, userID)
		}
	}
}

// TestUniqueInt64s 去重保持首次出现顺序，并丢弃非正值（0 / 负数不是合法主键）。
func TestUniqueInt64s(t *testing.T) {
	cases := []struct {
		name string
		in   []int64
		want []int64
	}{
		{"去重保序", []int64{5, 3, 5, 1, 3}, []int64{5, 3, 1}},
		{"丢弃非正值", []int64{0, -1, 2, 0}, []int64{2}},
		{"全非正", []int64{0, -1}, []int64{}},
		{"空入参", nil, []int64{}},
	}
	for _, c := range cases {
		if got := uniqueInt64s(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: uniqueInt64s(%v) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
}

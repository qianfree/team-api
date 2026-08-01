package dispatchadapter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateRoutingPolicyJSON(t *testing.T) {
	// 空串合法（使用全部内置默认）
	assert.NoError(t, ValidateRoutingPolicyJSON(""))

	// 合法的部分覆盖
	assert.NoError(t, ValidateRoutingPolicyJSON(`{"tierFactors":{"secondary":0.3}}`))
	assert.NoError(t, ValidateRoutingPolicyJSON(`{"breaker":{"failThreshold":5},"retry":{"inPlaceBudget":3}}`))

	// 非法 JSON
	err := ValidateRoutingPolicyJSON(`{"tierFactors":`)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JSON 解析失败")

	// 合法 JSON 但 Schema 越界
	assert.Error(t, ValidateRoutingPolicyJSON(`{"tierFactors":{"primary":0}}`), "primary 因子必须 > 0")
	assert.Error(t, ValidateRoutingPolicyJSON(`{"health":{"alpha":-1}}`))
	assert.Error(t, ValidateRoutingPolicyJSON(`{"retry":{"failoverBudget":-1}}`))
	assert.Error(t, ValidateRoutingPolicyJSON(`{"tierFactors":{"vip":1}}`), "未知层级")
}

func TestBuildTenantPolicy(t *testing.T) {
	// 三层浅合并：默认 → 全局 → 租户，后者覆盖前者
	pol, err := buildTenantPolicy(
		`{"tierFactors":{"secondary":0.3},"retry":{"inPlaceBudget":3}}`,
		`{"retry":{"inPlaceBudget":1}}`,
	)
	assert.NoError(t, err)
	assert.Equal(t, 0.3, pol.TierFactors["secondary"], "全局覆盖保留")
	assert.Equal(t, 1, pol.Retry.InPlaceBudget, "租户覆盖优先于全局")
	assert.Equal(t, 2, pol.Retry.FailoverBudget, "未覆盖字段用内置默认")

	// 全局串非法时忽略全局层，租户层仍生效
	pol, err = buildTenantPolicy(`{invalid`, `{"retry":{"inPlaceBudget":1}}`)
	assert.NoError(t, err)
	assert.Equal(t, 1, pol.Retry.InPlaceBudget)

	// 租户串非法 → 错误
	_, err = buildTenantPolicy("", `{invalid`)
	assert.Error(t, err)

	// 租户覆盖导致 Schema 越界 → 错误
	_, err = buildTenantPolicy("", `{"tierFactors":{"primary":0}}`)
	assert.Error(t, err)
}

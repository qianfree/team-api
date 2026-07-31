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

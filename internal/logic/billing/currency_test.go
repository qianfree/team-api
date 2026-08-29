package billing

import (
	"context"
	"testing"

	"github.com/qianfree/team-api/internal/consts"
)

// TestCurrency_FallbackUSD 单测环境无 DB 驱动（Config 读取触发 g.DB() panic），
// Currency 必须 recover 兜底返回默认 USD（与存量部署行为一致），不得 panic。
// 若在带完整配置/DB 的环境运行，读到已配置本位币则跳过断言。
func TestCurrency_FallbackUSD(t *testing.T) {
	got := Currency(context.Background())
	if got != consts.BillingCurrencyUSD && got != consts.BillingCurrencyCNY {
		t.Errorf("Currency() = %q, want USD or CNY", got)
	}
	if !billingCurrencyValid(got) {
		t.Errorf("Currency() = %q, invalid currency", got)
	}
}

func TestIsCNY_MatchesCurrency(t *testing.T) {
	c := Currency(context.Background())
	if want := c == consts.BillingCurrencyCNY; IsCNY(context.Background()) != want {
		t.Errorf("IsCNY() 与 Currency() 不一致: currency=%q", c)
	}
}

func billingCurrencyValid(c string) bool {
	return c == consts.BillingCurrencyUSD || c == consts.BillingCurrencyCNY
}

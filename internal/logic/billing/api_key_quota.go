package billing

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
)

// CheckApiKeyQuota checks whether an API key has enough remaining quota.
// total_quota <= 0 means unlimited.
func CheckApiKeyQuota(ctx context.Context, apiKeyID int64, preDeductAmount float64) error {
	if apiKeyID <= 0 {
		return nil
	}

	var row *struct {
		TotalQuota float64 `json:"total_quota"`
		UsedQuota  float64 `json:"used_quota"`
	}
	err := dao.ApiKeys.Ctx(ctx).
		Where("id", apiKeyID).
		Fields("COALESCE(total_quota, 0) AS total_quota, COALESCE(used_quota, 0) AS used_quota").
		Scan(&row)
	if err != nil {
		g.Log().Warningf(ctx, "api_key_quota: load failed apiKey=%d: %v, skipping check", apiKeyID, err)
		return nil
	}
	if row == nil || row.TotalQuota <= 0 {
		return nil
	}
	if row.UsedQuota+preDeductAmount > row.TotalQuota {
		return gerror.New("API key quota exceeded")
	}

	return nil
}

// IncrApiKeyQuotaUsed increments an API key's used quota after settlement.
// 无条件累加（允许最后一笔超冲）：额度是控制线而非资金线，钱包预扣已兜底资金安全。
// 若按限额拒绝累加，used_quota 会永远停在限额之下，导致 CheckApiKeyQuota 永远放行、
// 额度被无限绕过；累加后超限由下一次 CheckApiKeyQuota 拦截。
func IncrApiKeyQuotaUsed(ctx context.Context, apiKeyID int64, amount float64) {
	if apiKeyID <= 0 || amount <= 0 {
		return
	}

	result, err := g.DB().Exec(ctx,
		`UPDATE api_keys
		 SET used_quota = COALESCE(used_quota, 0) + $1, updated_at = $2
		 WHERE id = $3`,
		amount, time.Now(), apiKeyID)
	if err != nil {
		g.Log().Errorf(ctx, "api_key_quota: atomic incr failed apiKey=%d amount=%f: %v", apiKeyID, amount, err)
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		g.Log().Errorf(ctx, "api_key_quota: inspect incr result failed apiKey=%d: %v", apiKeyID, err)
		return
	}
	if affected == 0 {
		g.Log().Errorf(ctx, "api_key_quota: settlement increment target key not found apiKey=%d amount=%f", apiKeyID, amount)
	}
}

package tenant

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
)

// generateUniqueTenantCode 生成一个不冲突的默认租户 code。
// 格式：org-<base36>，满足正则 ^[a-z0-9][a-z0-9-]*[a-z0-9]$，长度 3-30。
// 用 crypto/rand 保证不可预测，循环查重最多 5 次避免唯一索引冲突。
func generateUniqueTenantCode(ctx context.Context) (string, error) {
	const maxAttempts = 5
	for i := 0; i < maxAttempts; i++ {
		code, err := randomTenantCode()
		if err != nil {
			return "", err
		}
		count, err := dao.TntTenants.Ctx(ctx).Where("code", code).Count()
		if err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
		g.Log().Warningf(ctx, "生成的租户 code 冲突，重试: %s", code)
	}
	return "", fmt.Errorf("生成唯一租户 code 失败（重试 %d 次）", maxAttempts)
}

// randomTenantCode 生成单个 "org-<base36>" 形式的随机 code。
func randomTenantCode() (string, error) {
	b := make([]byte, 9)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	var chars []byte
	tmp := new(big.Int).SetBytes(b)
	base := big.NewInt(36)
	mod := new(big.Int)
	for tmp.Sign() > 0 && len(chars) < 16 {
		tmp.DivMod(tmp, base, mod)
		chars = append([]byte{alphabet[mod.Int64()]}, chars...)
	}
	if len(chars) == 0 {
		chars = []byte{'0'}
	}
	return "org-" + string(chars), nil
}

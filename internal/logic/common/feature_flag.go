package common

import "context"

// IsFeatureEnabled 检查功能是否启用
// 功能开关已移除，所有功能默认启用
func IsFeatureEnabled(ctx context.Context, tenantID int64, featureKey string) bool {
	return true
}

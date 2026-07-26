package billing

import (
	"fmt"

	"github.com/gogf/gf/v2/util/gconv"
)

// 注意：本文件原有一个 BuildBillingSummary(p *SummaryParams) 实现，与
// snapshot.go 的 GenerateBillingSummary(snapshot *BillingSnapshot) 功能重叠。
// 经查 BuildBillingSummary 仅被 summary_test.go 引用，生产链路（task_billing.go、
// settlement.go）统一走 GenerateBillingSummary，故移除冗余实现，保留共用的
// formatInt / formatCost 给 snapshot.go 复用，避免双份维护漂移。

// formatInt 将整数格式化为字符串（统一千分位等口径的单一入口）。
func formatInt(n int) string {
	return gconv.String(n)
}

// formatCost 格式化金额为 6 位小数字符串，与系统资金展示精度对齐。
func formatCost(c float64) string {
	return fmt.Sprintf("%.6f", c)
}

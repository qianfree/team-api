package billing

import (
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/util/gconv"
)

// SummaryParams 计费快照生成参数
type SummaryParams struct {
	RequestedModel      string
	UpstreamModel       string
	ChannelName         string
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
	InputPrice          float64 // per 1M tokens
	OutputPrice         float64 // per 1M tokens
	CacheReadPrice      float64 // per 1M tokens
	InputCost           float64
	OutputCost          float64
	CacheReadCost       float64
	TotalCost           float64
	DiscountRatio       float64
	ActualCost          float64
	PreDeductAmount     float64
	RefundAmount        float64
	SupplementAmount    float64
	BillingMode         string
	Currency            string
}

// BuildBillingSummary 生成人类可读的计费快照文本
func BuildBillingSummary(p *SummaryParams) string {
	var lines []string

	// 模型行
	modelLine := fmt.Sprintf("模型: %s", p.RequestedModel)
	if p.UpstreamModel != "" && p.UpstreamModel != p.RequestedModel {
		modelLine += fmt.Sprintf(" → %s", p.UpstreamModel)
	}
	if p.ChannelName != "" {
		modelLine += fmt.Sprintf(" (渠道: %s)", p.ChannelName)
	}
	if p.BillingMode != "" && p.BillingMode != "token" {
		modelLine += fmt.Sprintf(" [%s]", p.BillingMode)
	}
	lines = append(lines, modelLine)

	// 输入费用行
	if p.InputTokens > 0 {
		lines = append(lines, fmt.Sprintf("输入: %s tokens × $%.2f/1M = $%s",
			formatInt(p.InputTokens), p.InputPrice, formatCost(p.InputCost)))
	}

	// 输出费用行
	if p.OutputTokens > 0 {
		lines = append(lines, fmt.Sprintf("输出: %s tokens × $%.2f/1M = $%s",
			formatInt(p.OutputTokens), p.OutputPrice, formatCost(p.OutputCost)))
	}

	// 缓存读取行
	if p.CacheReadTokens > 0 {
		lines = append(lines, fmt.Sprintf("缓存读取: %s tokens × $%.2f/1M = $%s",
			formatInt(p.CacheReadTokens), p.CacheReadPrice, formatCost(p.CacheReadCost)))
	}

	// 缓存创建行
	if p.CacheCreationTokens > 0 {
		lines = append(lines, fmt.Sprintf("缓存创建: %s tokens", formatInt(p.CacheCreationTokens)))
	}

	// 合计行
	totalLine := fmt.Sprintf("合计: $%s", formatCost(p.TotalCost))
	if p.DiscountRatio > 0 && p.DiscountRatio < 1.0 {
		totalLine += fmt.Sprintf(" (折扣: %s → 实付: $%s)",
			fmt.Sprintf("%.4f", p.DiscountRatio), formatCost(p.ActualCost))
	}
	lines = append(lines, totalLine)

	// 预扣/结算行
	if p.PreDeductAmount > 0 {
		settleLine := fmt.Sprintf("预扣: $%s", formatCost(p.PreDeductAmount))
		if p.RefundAmount > 0 {
			settleLine += fmt.Sprintf(" → 退回: $%s", formatCost(p.RefundAmount))
		}
		if p.SupplementAmount > 0 {
			settleLine += fmt.Sprintf(" / 补扣: $%s", formatCost(p.SupplementAmount))
		}
		if p.RefundAmount == 0 && p.SupplementAmount == 0 {
			settleLine += " → 无差额"
		}
		lines = append(lines, settleLine)
	}

	return strings.Join(lines, "\n")
}

func formatInt(n int) string {
	return gconv.String(n)
}

func formatCost(c float64) string {
	return fmt.Sprintf("%.6f", c)
}

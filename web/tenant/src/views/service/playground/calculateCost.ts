export interface ModelPricing {
	input_price?: number | string | null
	output_price?: number | string | null
	per_request_price?: number | string | null
	billing_mode?: string | null
}

export interface TokenUsage {
	prompt_tokens?: number
	completion_tokens?: number
	total_tokens?: number
}

/**
 * calculateCost 根据模型定价和 token usage 估算费用
 * @returns 格式化的费用字符串，如 "$0.001234"；无法计算时返回 null
 */
export function calculateCost(
	model: ModelPricing | undefined,
	usage: TokenUsage | undefined,
): string | null {
	if (!model || !usage) return null

	const promptTokens = usage.prompt_tokens ?? 0
	const completionTokens = usage.completion_tokens ?? 0

	// 按次计费
	if (model.billing_mode === 'per_request' && model.per_request_price) {
		const price = Number(model.per_request_price)
		if (price > 0) return `$${price.toFixed(6)}`
		return null
	}

	// Token 计费
	const inputPrice = Number(model.input_price ?? 0)
	const outputPrice = Number(model.output_price ?? 0)

	if (inputPrice <= 0 && outputPrice <= 0) return null

	const cost = (promptTokens * inputPrice + completionTokens * outputPrice) / 1_000_000
	if (cost <= 0) return null

	return `$${cost.toFixed(6)}`
}

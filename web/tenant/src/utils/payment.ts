/**
 * 以 POST 表单方式把支付参数提交到支付网关。
 *
 * 易支付 submit.php 的「页面跳转支付」标准做法：后端只返回 submit.php 的 action
 * 地址 + 签名后的参数，前端构造一个隐藏表单 POST 提交。
 * 相比把 sign/订单参数拼进 GET URL，更安全（不落浏览器历史与服务器日志），
 * 也规避 URL 长度限制和中文/特殊字符的编码边界问题。
 *
 * @param url    表单 action 地址（submit.php）
 * @param params 表单隐藏域参数（含 sign / sign_type）
 */
export function submitPaymentForm(url: string, params: Record<string, string>): void {
	if (!url) return
	const form = document.createElement('form')
	form.action = url
	form.method = 'POST'
	Object.entries(params || {}).forEach(([key, value]) => {
		const input = document.createElement('input')
		input.type = 'hidden'
		input.name = key
		input.value = value
		form.appendChild(input)
	})
	document.body.appendChild(form)
	form.submit()
	document.body.removeChild(form)
}

/**
 * 处理后端返回的支付结果：有 form 参数时走 POST 表单提交，否则普通跳转。
 * 兼容未来返回纯跳转 URL 的支付渠道（如 Stripe Checkout）。
 */
export function dispatchPayment(result: {
	payment_url?: string
	params?: Record<string, string>
}): void {
	const { payment_url, params } = result || {}
	if (!payment_url) return
	if (params && Object.keys(params).length > 0) {
		submitPaymentForm(payment_url, params)
	} else {
		window.location.href = payment_url
	}
}

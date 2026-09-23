<script setup lang="ts">
import { reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'

// 易支付渠道配置卡片：单例配置（每种渠道类型全平台一份），经
// PUT /admin/payment-channels/epay 整体覆盖保存
const props = defineProps<{
	initialConfig: {
		is_enabled: boolean
		pay_address: string
		merchant_id: string
		merchant_key: string
		pay_methods: { name: string; type: string; color: string }[]
	}
}>()

const saving = ref(false)

// 本地编辑副本：保存成功前不影响后端已生效配置
const form = reactive({
	is_enabled: props.initialConfig.is_enabled ?? false,
	pay_address: props.initialConfig.pay_address ?? '',
	merchant_id: props.initialConfig.merchant_id ?? '',
	merchant_key: props.initialConfig.merchant_key ?? '',
})

// 支付方式以 JSON 编辑：各上游网关的 type 取值不一（alipay/wxpay 或站点自定义值），
// 固定下拉无法穷举，直接编辑 JSON 数组最灵活
const payMethodsJSON = ref(formatPayMethods(props.initialConfig.pay_methods))

function formatPayMethods(methods: unknown): string {
	return JSON.stringify(Array.isArray(methods) ? methods : [], null, 2)
}

// 解析并校验支付方式 JSON，非法时弹出对应错误提示并返回 null
function parsePayMethods(): { name: string; type: string; color: string }[] | null {
	let parsed: any
	const text = payMethodsJSON.value.trim()
	try {
		parsed = text === '' ? [] : JSON.parse(text)
	} catch {
		Message.error('支付方式 JSON 格式无效，请检查是否为合法 JSON')
		return null
	}
	if (!Array.isArray(parsed)) {
		Message.error('支付方式必须是 JSON 数组，如 [{"name":"支付宝","type":"alipay","color":"#1677FF"}]')
		return null
	}
	for (let i = 0; i < parsed.length; i++) {
		const m = parsed[i]
		if (!m || typeof m !== 'object' || !String(m.name ?? '').trim() || !String(m.type ?? '').trim()) {
			Message.error(`支付方式第 ${i + 1} 项缺少 name 或 type 字段`)
			return null
		}
	}
	return parsed.map((m: any) => ({
		name: String(m.name).trim(),
		type: String(m.type).trim(),
		color: typeof m.color === 'string' ? m.color : '',
	}))
}

async function save() {
	if (form.is_enabled) {
		if (!form.pay_address.trim() || !form.merchant_id.trim() || !form.merchant_key.trim()) {
			Message.error('启用易支付需完整填写网关地址、商户 ID 和商户密钥')
			return
		}
	}
	const methods = parsePayMethods()
	if (!methods) return
	saving.value = true
	try {
		await request.put('/admin/payment-channels/epay', {
			config: JSON.stringify({ ...form, pay_methods: methods }),
		})
		Message.success('易支付配置已保存')
		// 回填规范化后的 JSON，让界面与实际提交内容保持一致
		payMethodsJSON.value = formatPayMethods(methods)
	} catch {
		// 错误提示由拦截器统一弹出（如启用时回调地址未配置的后端校验）
	} finally {
		saving.value = false
	}
}
</script>

<template>
	<div class="channel-card">
		<div class="channel-head">
			<div>
				<div class="channel-name">易支付</div>
				<div class="channel-desc">聚合支付网关（支付宝/微信等），MD5 签名协议，直接对接网关无需第三方 SDK</div>
			</div>
			<div class="channel-switch">
				<span class="switch-text">{{ form.is_enabled ? '已启用' : '未启用' }}</span>
				<ASwitch v-model="form.is_enabled" />
			</div>
		</div>

		<div class="channel-body">
			<div class="form-grid">
				<AFormItem label="网关地址" help="易支付网关根地址，系统在其后拼接 /submit.php 发起支付" class="field-full">
					<AInput v-model="form.pay_address" placeholder="https://pay.example.com" allow-clear />
				</AFormItem>
				<AFormItem label="商户 ID" help="易支付商户 ID（pid）">
					<AInput v-model="form.merchant_id" placeholder="例如 1001" allow-clear />
				</AFormItem>
				<AFormItem label="商户密钥" help="易支付商户密钥（KEY），用于 MD5 签名与回调验签">
					<AInputPassword v-model="form.merchant_key" placeholder="商户密钥" allow-clear />
				</AFormItem>
				<AFormItem
					label="支付方式"
					help="JSON 数组：name=租户充值页显示名称；type=协议支付类型，按上游网关实际支持填写（alipay/wxpay 或自定义值）；color=徽章颜色（可选）。数组顺序即展示顺序"
					class="field-full"
				>
					<ATextarea
						v-model="payMethodsJSON"
						:auto-size="{ minRows: 4, maxRows: 12 }"
						class="pay-methods-json"
						placeholder='[
  { "name": "支付宝", "type": "alipay", "color": "#1677FF" },
  { "name": "微信支付", "type": "wxpay", "color": "#00C250" }
]'
					/>
				</AFormItem>
			</div>

			<div class="channel-footer">
				<AButton type="primary" :loading="saving" @click="save">保存配置</AButton>
			</div>
		</div>
	</div>
</template>

<style scoped>
.channel-card {
	border: 1px solid var(--color-border-2);
	border-radius: 4px;
	background: var(--color-bg-2);
}

.channel-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 12px;
	padding: 14px 20px;
	border-bottom: 1px solid var(--color-fill-2);
}

.channel-name {
	font-size: 15px;
	font-weight: 600;
	color: var(--color-text-1);
}

.channel-desc {
	font-size: 12px;
	color: var(--color-text-3);
	margin-top: 2px;
}

.channel-switch {
	display: flex;
	align-items: center;
	gap: 8px;
	flex-shrink: 0;
}

.switch-text {
	font-size: 12px;
	color: var(--color-text-3);
}

.channel-body {
	padding: 20px;
}

.form-grid {
	display: grid;
	grid-template-columns: repeat(2, 1fr);
	gap: 16px 28px;
}

.field-full {
	grid-column: 1 / -1;
}

/* JSON 编辑区用等宽字体，多行结构更易读 */
.pay-methods-json :deep(textarea) {
	font-family: ui-monospace, SFMono-Regular, Consolas, 'Courier New', monospace;
	font-size: 13px;
}

.channel-footer {
	display: flex;
	justify-content: flex-end;
	margin-top: 16px;
	padding-top: 14px;
	border-top: 1px solid var(--color-fill-2);
}

/* 移动端：表单字段单列堆叠，头部说明与开关换行，避免内容被挤压 */
@media (max-width: 768px) {
	.channel-head {
		flex-direction: column;
		align-items: flex-start;
	}
	.form-grid {
		grid-template-columns: 1fr;
		gap: 12px;
	}
}
</style>

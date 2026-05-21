<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Message } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const activeTab = ref('channels')

// ========== 支付渠道配置（单例模式） ==========

const channelsLoading = ref(false)
const channels = ref<Record<string, any>>({})

// Epay 表单
const epayForm = reactive({
	is_enabled: false,
	pay_address: '',
	merchant_id: '',
	merchant_key: '',
	pay_methods: '',
})
const epaySaving = ref(false)

const channelNames: Record<string, string> = {
	epay: 'Epay（易支付）',
}

async function fetchChannels() {
	channelsLoading.value = true
	try {
		const res = await request.get('/admin/payment-channels')
		const list = res.data?.data?.list || []
		const map: Record<string, any> = {}
		for (const ch of list) {
			map[ch.channel] = ch.config || {}
		}
		channels.value = map

		// Epay
		const epay = map.epay || {}
		epayForm.is_enabled = !!epay.is_enabled
		epayForm.pay_address = epay.pay_address || ''
		epayForm.merchant_id = epay.merchant_id || ''
		epayForm.merchant_key = epay.merchant_key || ''
		epayForm.pay_methods = epay.pay_methods
			? typeof epay.pay_methods === 'string'
				? epay.pay_methods
				: JSON.stringify(epay.pay_methods)
			: ''

	} catch {
		// interceptor handles error
	} finally {
		channelsLoading.value = false
	}
}

async function saveChannel(channel: string, config: Record<string, any>, loading: { value: boolean }) {
	loading.value = true
	try {
		await request.put(`/admin/payment-channels/${channel}`, {
			config: JSON.stringify(config),
		})
		Message.success(`${channelNames[channel]} 配置已保存`)
	} catch {
		// interceptor handles error
	} finally {
		loading.value = false
	}
}

function handleEpaySave() {
	let payMethods = epayForm.pay_methods
	try { payMethods = JSON.parse(payMethods) } catch { /* keep as string */ }
	saveChannel('epay', {
		is_enabled: epayForm.is_enabled,
		pay_address: epayForm.pay_address,
		merchant_id: epayForm.merchant_id,
		merchant_key: epayForm.merchant_key,
		pay_methods: payMethods || undefined,
	}, epaySaving)
}

// ========== 全局支付设置 ==========

const settingsLoading = ref(false)
const settingsSaving = ref(false)

const settingsForm = reactive({
	amount_options: '10,20,50,100,200,500',
	amount_discount: '{}',
	min_topup: 1,
	currency: 'CNY',
	callback_base_url: '',
})

const currencyOptions = [
	{ value: 'CNY', label: 'CNY - 人民币' },
	{ value: 'USD', label: 'USD - 美元' },
]

async function fetchSettings() {
	settingsLoading.value = true
	try {
		const res = await request.get('/admin/payment-settings')
		const data = res.data?.data
		if (!data) return
		if (data.amount_options) {
			settingsForm.amount_options = Array.isArray(data.amount_options)
				? data.amount_options.join(',')
				: String(data.amount_options)
		}
		if (data.amount_discount) {
			settingsForm.amount_discount = typeof data.amount_discount === 'string'
				? data.amount_discount
				: JSON.stringify(data.amount_discount)
		}
		if (data.min_topup !== undefined) settingsForm.min_topup = Number(data.min_topup)
		if (data.currency) settingsForm.currency = data.currency
		if (data.callback_base_url !== undefined) settingsForm.callback_base_url = data.callback_base_url
	} catch {
		// interceptor handles error
	} finally {
		settingsLoading.value = false
	}
}

async function handleSettingsSave() {
	settingsSaving.value = true
	try {
		let amountOptions: number[] = []
		try {
			amountOptions = settingsForm.amount_options
				.split(',')
				.map((s: string) => Number(s.trim()))
				.filter((n: number) => !isNaN(n) && n > 0)
		} catch {
			Message.error('金额选项格式错误')
			return
		}
		let amountDiscount: Record<string, number> = {}
		try {
			amountDiscount = JSON.parse(settingsForm.amount_discount || '{}')
		} catch {
			Message.error('折扣映射格式错误')
			return
		}
		await request.put('/admin/payment-settings', {
			amount_options: amountOptions,
			amount_discount: amountDiscount,
			min_topup: settingsForm.min_topup,
			currency: settingsForm.currency,
			callback_base_url: settingsForm.callback_base_url,
		})
		Message.success('保存成功')
	} catch {
		// interceptor handles error
	} finally {
		settingsSaving.value = false
	}
}

onMounted(() => {
	fetchChannels()
	fetchSettings()
})
</script>

<template>
	<div>
		<PageHeader title="支付设置" description="管理支付渠道和全局支付配置" />

		<ATabs v-model:active-key="activeTab" type="rounded" class="mb-4">
			<ATabPane key="channels" title="支付渠道" />
			<ATabPane key="settings" title="全局设置" />
		</ATabs>

		<!-- 支付渠道配置 -->
		<div v-if="activeTab === 'channels'" class="space-y-4">
			<!-- Epay -->
			<ACard :bordered="false" :loading="channelsLoading">
				<template #title>
					<div class="flex items-center gap-2">
						<span>Epay（易支付）</span>
						<ATag :color="epayForm.is_enabled ? 'green' : undefined" size="small">
							{{ epayForm.is_enabled ? '已启用' : '未启用' }}
						</ATag>
					</div>
				</template>
				<AForm :model="epayForm" :auto-label-width="true">
					<AFormItem label="启用">
						<ASwitch v-model="epayForm.is_enabled" />
					</AFormItem>
					<AFormItem label="支付网关地址">
						<AInput v-model="epayForm.pay_address" placeholder="https://pay.example.com" />
					</AFormItem>
					<AFormItem label="商户ID">
						<AInput v-model="epayForm.merchant_id" placeholder="1001" />
					</AFormItem>
					<AFormItem label="商户密钥">
						<AInput v-model="epayForm.merchant_key" placeholder="商户密钥" />
					</AFormItem>
					<AFormItem
						label="支付方式"
						extra='JSON 数组。如 [{"name":"支付宝","type":"alipay","color":"#1677ff"},{"name":"微信支付","type":"wxpay","color":"#07c160"}]'
					>
						<AInput
							v-model="epayForm.pay_methods"
							type="textarea"
							:auto-size="{ minRows: 2, maxRows: 4 }"
							placeholder='[{"name":"支付宝","type":"alipay","color":"#1677ff"}]'
						/>
					</AFormItem>
				</AForm>
				<template #actions>
					<AButton type="primary" :loading="epaySaving" @click="handleEpaySave">保存 Epay 配置</AButton>
				</template>
			</ACard>
		</div>

		<!-- 全局支付设置 -->
		<div v-if="activeTab === 'settings'">
			<ACard :bordered="false" :loading="settingsLoading" title="充值与金额">
				<AForm :model="settingsForm" :auto-label-width="true">
					<AFormItem label="金额选项" extra="预设充值金额，用逗号分隔">
						<AInput v-model="settingsForm.amount_options" placeholder="10,20,50,100,200,500" />
					</AFormItem>
					<AFormItem
						label="折扣映射"
						extra='JSON 格式，键为金额，值为折扣率。如 {"100":0.9} 表示充100享9折'
					>
						<AInput
							v-model="settingsForm.amount_discount"
							type="textarea"
							:auto-size="{ minRows: 2, maxRows: 4 }"
							placeholder='{}'
						/>
					</AFormItem>
					<AFormItem label="最低充值">
						<AInputNumber v-model="settingsForm.min_topup" :min="0.01" :step="1" :precision="2" />
					</AFormItem>
					<AFormItem label="默认货币">
						<ASelect v-model="settingsForm.currency" :options="currencyOptions" />
					</AFormItem>
				</AForm>
			</ACard>

			<ACard :bordered="false" class="mt-4" title="回调配置">
				<AForm :model="settingsForm" :auto-label-width="true">
					<AFormItem
						label="回调基础 URL"
						extra="支付网关回调地址的前缀，留空则使用请求时的 Host"
					>
						<AInput v-model="settingsForm.callback_base_url" placeholder="https://api.example.com" />
					</AFormItem>
				</AForm>
			</ACard>

			<div class="settings-footer">
				<AButton type="primary" :loading="settingsSaving" @click="handleSettingsSave">
					保存设置
				</AButton>
			</div>
		</div>
	</div>
</template>

<style scoped>
.settings-footer {
	display: flex;
	justify-content: flex-end;
	margin-top: 16px;
}
</style>

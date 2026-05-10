<script setup lang="ts">
import { ref, reactive, computed, onMounted, h } from 'vue'
import { Message, Modal, Tag, Button, Space } from '@arco-design/web-vue'
import type { TableColumnData, FormInstance } from '@arco-design/web-vue'
import { IconPlus } from '@arco-design/web-vue/es/icon'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

// === Tabs ===
const activeTab = ref('channels')

// ========== 支付渠道 ==========

const loading = ref(false)
const channels = ref<any[]>([])

const channelTypeMap: Record<string, string> = {
	epay: 'Epay',
	stripe: 'Stripe',
	mock: 'Mock',
}

const channelTypeOptions = [
	{ value: 'epay', label: 'Epay（易支付）' },
	{ value: 'stripe', label: 'Stripe' },
	{ value: 'mock', label: 'Mock（测试）' },
]

const columns: TableColumnData[] = [
	{ title: 'ID', dataIndex: 'id', width: 60 },
	{ title: '名称', dataIndex: 'name', width: 160 },
	{
		title: '类型',
		dataIndex: 'channel',
		width: 120,
		render({ record }) {
			return h(Tag, { color: 'arcoblue' }, () => channelTypeMap[record.channel] || record.channel)
		},
	},
	{ title: '子支付方式', dataIndex: 'payment_type', width: 120 },
	{
		title: '排序',
		dataIndex: 'sort_order',
		width: 80,
	},
	{
		title: '状态',
		dataIndex: 'is_enabled',
		width: 100,
		render({ record }) {
			return h(Tag, { color: record.is_enabled ? 'green' : undefined, size: 'small' }, () =>
				record.is_enabled ? '已启用' : '已禁用',
			)
		},
	},
	{ title: '创建时间', dataIndex: 'created_at', width: 170 },
	{
		title: '操作',
		dataIndex: 'actions',
		width: 220,
		fixed: 'right',
		render({ record }) {
			return h(Space, { size: 4 }, () => [
				h(
					Button,
					{ type: 'text', size: 'mini', onClick: () => toggleChannel(record) },
					() => (record.is_enabled ? '禁用' : '启用'),
				),
				h(
					Button,
					{ type: 'text', size: 'mini', onClick: () => openEdit(record) },
					() => '编辑',
				),
				h(
					Button,
					{
						type: 'text',
						size: 'mini',
						status: 'danger',
						onClick: () => handleDelete(record),
					},
					() => '删除',
				),
			])
		},
	},
]

async function fetchChannels() {
	loading.value = true
	try {
		const res = await request.get('/admin/payment-channels')
		const raw = res.data?.data
		channels.value = Array.isArray(raw) ? raw : (raw?.data || [])
	} catch {
		/* interceptor handles error toast */
	} finally {
		loading.value = false
	}
}

async function toggleChannel(ch: any) {
	try {
		const res = await request.put(`/admin/payment-channels/${ch.id}/toggle`)
		const newState = res.data?.data?.is_enabled
		ch.is_enabled = newState !== undefined ? newState : !ch.is_enabled
		Message.success(ch.is_enabled ? '已启用' : '已禁用')
	} catch {
		/* interceptor handles error toast */
	}
}

function handleDelete(ch: any) {
	Modal.warning({
		title: '确认删除',
		content: `确定要删除支付渠道「${ch.name}」吗？此操作不可恢复。`,
		hideCancel: false,
		onOk: async () => {
			try {
				await request.delete(`/admin/payment-channels/${ch.id}`)
				Message.success('已删除')
				fetchChannels()
			} catch {
				/* interceptor handles error toast */
			}
		},
	})
}

// === 渠道编辑弹窗 ===

const showChannelModal = ref(false)
const modalTitle = ref('添加支付渠道')
const editingId = ref<number | null>(null)
const channelFormRef = ref<FormInstance | null>(null)
const channelSaving = ref(false)

const channelForm = reactive({
	channel: 'epay',
	name: '',
	payment_type: '',
	sort_order: 0,
	// Epay 配置
	epay_pay_address: '',
	epay_merchant_id: '',
	epay_merchant_key: '',
	epay_pay_methods: '',
	// Stripe 配置
	stripe_api_secret: '',
	stripe_webhook_secret: '',
	stripe_price_id: '',
	stripe_unit_price: 0,
	stripe_min_topup: 1,
	stripe_test_mode: false,
	// Mock 配置
	mock_auto_fulfill: true,
})

const currentChannelType = computed(() => channelForm.channel)

function resetChannelForm() {
	channelForm.channel = 'epay'
	channelForm.name = ''
	channelForm.payment_type = ''
	channelForm.sort_order = 0
	channelForm.epay_pay_address = ''
	channelForm.epay_merchant_id = ''
	channelForm.epay_merchant_key = ''
	channelForm.epay_pay_methods = ''
	channelForm.stripe_api_secret = ''
	channelForm.stripe_webhook_secret = ''
	channelForm.stripe_price_id = ''
	channelForm.stripe_unit_price = 0
	channelForm.stripe_min_topup = 1
	channelForm.stripe_test_mode = false
	channelForm.mock_auto_fulfill = true
	editingId.value = null
	modalTitle.value = '添加支付渠道'
}

function openCreate() {
	resetChannelForm()
	showChannelModal.value = true
}

function openEdit(ch: any) {
	resetChannelForm()
	editingId.value = ch.id
	modalTitle.value = '编辑支付渠道'

	channelForm.channel = ch.channel || 'epay'
	channelForm.name = ch.name || ''
	channelForm.payment_type = ch.payment_type || ''
	channelForm.sort_order = ch.sort_order || 0

	// 解析 config JSONB
	let config: any = {}
	if (ch.config) {
		try {
			config = typeof ch.config === 'string' ? JSON.parse(ch.config) : ch.config
		} catch {
			config = {}
		}
	}

	if (channelForm.channel === 'epay') {
		channelForm.epay_pay_address = config.pay_address || ''
		channelForm.epay_merchant_id = config.merchant_id || ''
		channelForm.epay_merchant_key = config.merchant_key || ''
		channelForm.epay_pay_methods = config.pay_methods
			? typeof config.pay_methods === 'string'
				? config.pay_methods
				: JSON.stringify(config.pay_methods)
			: ''
	} else if (channelForm.channel === 'stripe') {
		channelForm.stripe_api_secret = config.api_secret || ''
		channelForm.stripe_webhook_secret = config.webhook_secret || ''
		channelForm.stripe_price_id = config.price_id || ''
		channelForm.stripe_unit_price = config.unit_price || 0
		channelForm.stripe_min_topup = config.min_topup || 1
		channelForm.stripe_test_mode = config.test_mode || false
	} else if (channelForm.channel === 'mock') {
		channelForm.mock_auto_fulfill = config.auto_fulfill !== false
	}

	showChannelModal.value = true
}

function buildConfigJSON(): string {
	const type = channelForm.channel
	if (type === 'epay') {
		let payMethods = channelForm.epay_pay_methods
		try {
			payMethods = JSON.parse(payMethods)
		} catch {
			// keep as string
		}
		return JSON.stringify({
			pay_address: channelForm.epay_pay_address,
			merchant_id: channelForm.epay_merchant_id,
			merchant_key: channelForm.epay_merchant_key,
			pay_methods: payMethods || undefined,
		})
	} else if (type === 'stripe') {
		return JSON.stringify({
			api_secret: channelForm.stripe_api_secret,
			webhook_secret: channelForm.stripe_webhook_secret,
			price_id: channelForm.stripe_price_id,
			unit_price: channelForm.stripe_unit_price,
			min_topup: channelForm.stripe_min_topup,
			test_mode: channelForm.stripe_test_mode,
		})
	} else {
		return JSON.stringify({
			auto_fulfill: channelForm.mock_auto_fulfill,
		})
	}
}

async function handleChannelSave() {
	try {
		const errors = await channelFormRef.value?.validate()
		if (errors) return
	} catch {
		return
	}

	channelSaving.value = true
	try {
		const payload: Record<string, any> = {
			channel: channelForm.channel,
			name: channelForm.name,
			payment_type: channelForm.payment_type,
			sort_order: channelForm.sort_order,
			config: buildConfigJSON(),
		}

		if (editingId.value) {
			await request.put(`/admin/payment-channels/${editingId.value}`, payload)
			Message.success('更新成功')
		} else {
			await request.post('/admin/payment-channels', payload)
			Message.success('创建成功')
		}

		showChannelModal.value = false
		fetchChannels()
	} catch {
		/* interceptor handles error toast */
	} finally {
		channelSaving.value = false
	}
}

// ========== 全局支付设置 ==========

const settingsLoading = ref(false)
const settingsSaving = ref(false)
const settingsFormRef = ref<FormInstance | null>(null)

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
			const opts = data.amount_options
			settingsForm.amount_options = Array.isArray(opts) ? opts.join(',') : String(opts)
		}
		if (data.amount_discount) {
			const disc = data.amount_discount
			settingsForm.amount_discount = typeof disc === 'string' ? disc : JSON.stringify(disc)
		}
		if (data.min_topup !== undefined) settingsForm.min_topup = Number(data.min_topup)
		if (data.currency) settingsForm.currency = data.currency
		if (data.callback_base_url !== undefined) settingsForm.callback_base_url = data.callback_base_url
	} catch {
		/* interceptor handles error toast */
	} finally {
		settingsLoading.value = false
	}
}

async function handleSettingsSave() {
	settingsSaving.value = true
	try {
		// 解析金额选项
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

		// 解析折扣映射
		let amountDiscount: Record<string, number> = {}
		try {
			amountDiscount = JSON.parse(settingsForm.amount_discount || '{}')
		} catch {
			Message.error('折扣映射格式错误，请使用 JSON 格式')
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
		/* interceptor handles error toast */
	} finally {
		settingsSaving.value = false
	}
}

// === Init ===

onMounted(() => {
	fetchChannels()
	fetchSettings()
})
</script>

<template>
	<div>
		<PageHeader title="支付设置" description="管理支付渠道和全局支付配置" />

		<ATabs v-model:active-key="activeTab" type="rounded" class="mb-4">
			<ATab key="channels" title="支付渠道" />
			<ATab key="settings" title="全局设置" />
		</ATabs>

		<!-- 支付渠道管理 -->
		<div v-if="activeTab === 'channels'">
			<div class="mb-4 flex justify-end">
				<AButton type="primary" @click="openCreate">
					<template #icon><IconPlus /></template>
					添加渠道
				</AButton>
			</div>

			<ACard :bordered="false">
				<ATable
					:columns="columns"
					:data="channels"
					:loading="loading"
					:pagination="false"
					row-key="id"
					stripe
				/>
			</ACard>
		</div>

		<!-- 全局支付设置 -->
		<div v-if="activeTab === 'settings'">
			<ACard :bordered="false" :loading="settingsLoading" title="充值与金额">
				<AForm ref="settingsFormRef" :model="settingsForm" :auto-label-width="true">
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

		<!-- 渠道编辑弹窗 -->
		<AModal
			v-model:visible="showChannelModal"
			:title="modalTitle"
			:width="560"
			:mask-closable="false"
			@cancel="showChannelModal = false"
			@ok="handleChannelSave"
			:ok-loading="channelSaving"
		>
			<AForm ref="channelFormRef" :model="channelForm" :auto-label-width="true" layout="vertical">
				<AFormItem
					field="name"
					label="渠道名称"
					:rules="[{ required: true, message: '请输入渠道名称' }]"
				>
					<AInput v-model="channelForm.name" placeholder="如：Epay 主商户" />
				</AFormItem>

				<AFormItem
					v-if="!editingId"
					field="channel"
					label="渠道类型"
					:rules="[{ required: true, message: '请选择渠道类型' }]"
				>
					<ASelect v-model="channelForm.channel" :options="channelTypeOptions" :disabled="!!editingId" />
				</AFormItem>
				<AFormItem v-else label="渠道类型">
					<AInput :model-value="channelTypeMap[channelForm.channel] || channelForm.channel" disabled />
				</AFormItem>

				<AFormItem field="payment_type" label="子支付方式" extra="如 alipay、wxpay，留空表示支持所有">
					<AInput v-model="channelForm.payment_type" placeholder="alipay" />
				</AFormItem>

				<AFormItem field="sort_order" label="排序权重">
					<AInputNumber v-model="channelForm.sort_order" :min="0" />
				</AFormItem>

				<!-- Epay 配置 -->
				<template v-if="currentChannelType === 'epay'">
					<ADivider>Epay 配置</ADivider>
					<AFormItem
						field="epay_pay_address"
						label="支付网关地址"
						:rules="[{ required: true, message: '请输入支付网关地址' }]"
					>
						<AInput v-model="channelForm.epay_pay_address" placeholder="https://pay.example.com" />
					</AFormItem>
					<AFormItem
						field="epay_merchant_id"
						label="商户ID"
						:rules="[{ required: true, message: '请输入商户ID' }]"
					>
						<AInput v-model="channelForm.epay_merchant_id" placeholder="1001" />
					</AFormItem>
					<AFormItem
						field="epay_merchant_key"
						label="商户密钥"
						:rules="[{ required: true, message: '请输入商户密钥' }]"
					>
						<AInput v-model="channelForm.epay_merchant_key" placeholder="商户密钥" />
					</AFormItem>
					<AFormItem
						field="epay_pay_methods"
						label="支付方式"
						extra='JSON 数组或留空。如 [{"name":"支付宝","type":"alipay","color":"#1677ff"}]'
					>
						<AInput
							v-model="channelForm.epay_pay_methods"
							type="textarea"
							:auto-size="{ minRows: 2, maxRows: 4 }"
							placeholder='[{"name":"支付宝","type":"alipay","color":"#1677ff"}]'
						/>
					</AFormItem>
				</template>

				<!-- Stripe 配置 -->
				<template v-if="currentChannelType === 'stripe'">
					<ADivider>Stripe 配置</ADivider>
					<AFormItem
						field="stripe_api_secret"
						label="API Secret Key"
						:rules="[{ required: true, message: '请输入 API Secret' }]"
					>
						<AInput v-model="channelForm.stripe_api_secret" placeholder="sk_live_..." />
					</AFormItem>
					<AFormItem
						field="stripe_webhook_secret"
						label="Webhook Signing Secret"
						:rules="[{ required: true, message: '请输入 Webhook Secret' }]"
					>
						<AInput v-model="channelForm.stripe_webhook_secret" placeholder="whsec_..." />
					</AFormItem>
					<AFormItem field="stripe_price_id" label="Price ID" extra="Stripe 中创建的 Price ID，留空则使用自定义单价">
						<AInput v-model="channelForm.stripe_price_id" placeholder="price_..." />
					</AFormItem>
					<AFormItem field="stripe_unit_price" label="自定义单价（分）" extra="未设置 Price ID 时使用此价格，单位为最小货币单位">
						<AInputNumber v-model="channelForm.stripe_unit_price" :min="0" :step="100" />
					</AFormItem>
					<AFormItem field="stripe_min_topup" label="最低充值金额">
						<AInputNumber v-model="channelForm.stripe_min_topup" :min="0.5" :step="1" :precision="2" />
					</AFormItem>
					<AFormItem field="stripe_test_mode" label="测试模式">
						<ASwitch v-model="channelForm.stripe_test_mode" />
					</AFormItem>
				</template>

				<!-- Mock 配置 -->
				<template v-if="currentChannelType === 'mock'">
					<ADivider>Mock 配置</ADivider>
					<AFormItem field="mock_auto_fulfill" label="自动履约">
						<ASwitch v-model="channelForm.mock_auto_fulfill" />
						<template #extra>开启后 Mock 支付会自动完成订单履约（加余额/激活套餐）</template>
					</AFormItem>
				</template>
			</AForm>
		</AModal>
	</div>
</template>

<style scoped>
.settings-footer {
	display: flex;
	justify-content: flex-end;
	margin-top: 16px;
}
</style>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NDropdown, NTag } from 'naive-ui'
import { renderBadge, formatDate, formatTokens, formatMs, tableScrollX } from '@/utils/renderUtils'
import { useRoute, useRouter } from 'vue-router'
import BaseModal from '@/components/common/BaseModal.vue'
import ResponsiveDataTable from '@/components/common/ResponsiveDataTable.vue'
import ApiKeyEditModal from '@/components/common/ApiKeyEditModal.vue'
import type { ApiKeyData } from '@/components/common/ApiKeyEditModal.vue'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'
import { toast } from '@/utils/toast'
import { useConfirm } from '@/composables/useConfirm'
import { formatBilling } from '@/composables/useCurrency'

const { confirm } = useConfirm()

const route = useRoute()
const router = useRouter()
const projectId = computed(() => Number(route.params.id))

// Project info
interface Project {
	id: number
	name: string
	description: string
	status: string
	budget: string | null
	created_by: number
	created_at: string
	updated_at: string
	active_keys: number
	total_keys: number
	month_cost: number
	month_requests: number
}
const project = ref<Partial<Project>>({})
const loading = ref(false)

// Tabs
const activeTab = ref<'overview' | 'keys' | 'usage'>('overview')

// Edit form
const editForm = reactive({
	name: '',
	description: '',
	budget: '',
})
const editLoading = ref(false)

// API Keys
interface ApiKey {
	id: number
	name: string
	key_prefix: string
	scope: string
	model_count: number
	status: string
	expires_at: string | null
	rate_limit_qps: number | null
	rate_limit_concurrency: number | null
	ip_whitelist: string[] | null
	total_quota: number | null
	used_quota: number | null
	created_at: string
}
const keys = ref<ApiKey[]>([])
const keysLoading = ref(false)
const keysPage = ref(1)
const keysPageSize = ref(20)
const keysTotal = ref(0)

// 密钥有效性筛选：默认仅展示可用密钥（与个人 API 密钥页同口径）
const validityFilter = ref('valid')
const validityOptions = [
	{ label: '仅可用', key: 'valid' },
	{ label: '所有', key: 'all' },
	{ label: '仅失效', key: 'invalid' },
]
const currentValidityLabel = computed(
	() => validityOptions.find((o) => o.key === validityFilter.value)?.label ?? '仅可用',
)

function handleValiditySelect(key: string | number) {
	validityFilter.value = String(key)
	keysPage.value = 1
	fetchKeys()
}

// Key modals (shared component)
const showCreateModal = ref(false)
const showEditModal = ref(false)
const editingKey = ref<ApiKeyData | null>(null)

// Model scope modal
const showScopeModal = ref(false)
const scopeModalTitle = ref('')
const scopeModalModels = ref<string[]>([])
const scopeModalLoading = ref(false)

async function openScopeModal(keyId: number, keyName: string) {
	scopeModalTitle.value = keyName + ' — 可用模型'
	scopeModalModels.value = []
	scopeModalLoading.value = true
	showScopeModal.value = true
	try {
		const res: any = await request.get(`/tenant/api-keys/${keyId}/model-scopes`)
		scopeModalModels.value = res.data?.data?.model_names || []
	} catch {
		scopeModalModels.value = []
	} finally {
		scopeModalLoading.value = false
	}
}

onMounted(() => {
	fetchProject()
})

// Usage stats
const usageStats = ref<any>(null)
const usageLoading = ref(false)

// Usage logs
const usageLogs = ref<any[]>([])
const usageLogsLoading = ref(false)
const usageLogsPage = ref(1)
const usageLogsPageSize = ref(20)
const usageLogsTotal = ref(0)

const statusBadgeClasses: Record<string, string> = {
	active: 'badge-success',
	archived: 'badge-gray',
	budget_exhausted: 'badge-danger',
}
const statusLabels: Record<string, string> = {
	active: '活跃',
	archived: '已归档',
	budget_exhausted: '预算耗尽',
}
const keyStatusBadgeClass: Record<string, string> = {
	active: 'badge-success',
	disabled: 'badge-gray',
	revoked: 'badge-danger',
	expired: 'badge-warning',
}
const keyStatusLabel: Record<string, string> = {
	active: '活跃',
	disabled: '已禁用',
	revoked: '已吊销',
	expired: '已过期',
}

// 已过期的密钥 status 仍为 active，展示时按 expires_at 归一为"已过期"
function displayStatus(key: ApiKey): string {
	if (key.status === 'active' && key.expires_at && new Date(key.expires_at).getTime() < Date.now()) {
		return 'expired'
	}
	return key.status
}
const relayModeLabel: Record<string, string> = {
	'chat': '对话',
	'embedding': '嵌入',
	'image': '图像',
	'audio-speech': '语音合成',
	'audio-transcription': '语音识别',
	'rerank': '重排',
}

const budgetUsage = computed(() => {
	const budget = project.value.budget ? Number(project.value.budget) : 0
	if (!budget || budget <= 0) return 0
	const cost = project.value.month_cost || 0
	return Math.min(Math.round((cost / budget) * 10000) / 100, 100)
})

const budgetColor = computed(() => {
	const pct = budgetUsage.value
	if (pct >= 90) return 'text-red-600'
	if (pct >= 70) return 'text-amber-600'
	return 'text-emerald-600'
})

async function fetchProject() {
	loading.value = true
	try {
		const res: any = await request.get(`/tenant/projects/${projectId.value}`)
		const raw = res.data?.data
		const data = raw?.data || raw
		if (data) {
			project.value = data
			editForm.name = data.name || ''
			editForm.description = data.description || ''
			editForm.budget = data.budget ? String(data.budget) : ''
		}
	} catch {
		router.push('/tenant/projects')
	} finally {
		loading.value = false
	}
}

async function handleSave() {
	if (!editForm.name.trim()) return
	editLoading.value = true
	try {
		const data: any = { name: editForm.name, description: editForm.description }
		if (editForm.budget && Number(editForm.budget) > 0) {
			data.budget = Number(editForm.budget)
		} else {
			data.budget = 0
		}
		await request.put(`/tenant/projects/${projectId.value}`, data)
		toast.success('保存成功')
		fetchProject()
	} catch {
	} finally {
		editLoading.value = false
	}
}

// === Keys ===
async function fetchKeys() {
	keysLoading.value = true
	try {
		const res: any = await request.get(`/tenant/projects/${projectId.value}/api-keys`, {
			params: { page: keysPage.value, page_size: keysPageSize.value, validity: validityFilter.value },
		})
		const raw = res.data?.data
		keys.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
		keysTotal.value = raw?.total || 0
	} catch {
		keys.value = []
	} finally {
		keysLoading.value = false
	}
}

function openEditModal(key: ApiKey) {
	editingKey.value = {
		id: key.id,
		name: key.name,
		expires_at: key.expires_at,
		rate_limit_qps: key.rate_limit_qps,
		rate_limit_concurrency: key.rate_limit_concurrency,
		ip_whitelist: key.ip_whitelist || [],
		total_quota: key.total_quota,
		used_quota: key.used_quota,
	}
	showEditModal.value = true
}

function formatKeyLimit(value: number | null | undefined): string {
	if (value === null || value === undefined) return '默认'
	if (value <= 0) return '不限'
	return String(value)
}

// 复制 API Key 明文（与个人 API 密钥页同款交互）
const copyingKeyId = ref<number | null>(null)

async function copyKey(keyId: number) {
	if (copyingKeyId.value !== null) return

	copyingKeyId.value = keyId
	try {
		const res: any = await request.get(`/tenant/api-keys/${keyId}/value`)
		const plainKey = res.data?.data?.key
		if (typeof plainKey !== 'string' || plainKey.length === 0) {
			toast.error('未获取到 API Key')
			return
		}

		try {
			await navigator.clipboard.writeText(plainKey)
			toast.success('API Key 已复制到剪贴板')
		} catch {
			toast.error('复制失败，请检查浏览器剪贴板权限')
		}
	} catch {
		// 请求错误由统一拦截器提示。
	} finally {
		copyingKeyId.value = null
	}
}

// Key 额度展示（bil 层，本位币直显）
function formatKeyQuota(key: ApiKey): string {
	const used = key.used_quota || 0
	if (!key.total_quota || key.total_quota <= 0) return `${formatBilling(used, 2)} / 不限`
	return `${formatBilling(used, 2)} / ${formatBilling(key.total_quota, 2)}`
}

async function deleteKey(keyId: number) {
	if (!await confirm({ message: '确定禁用该密钥？', confirmText: '确认禁用', danger: true })) return
	try {
		await request.delete(`/tenant/projects/${projectId.value}/api-keys/${keyId}`)
		fetchKeys()
	} catch {
	}
}

// === Usage ===
async function fetchUsageStats() {
	usageLoading.value = true
	try {
		const res: any = await request.get(`/tenant/projects/${projectId.value}/usage-stats`)
		const raw = res.data?.data
		usageStats.value = raw?.data || raw
	} catch {
		usageStats.value = null
	} finally {
		usageLoading.value = false
	}
}

async function fetchUsageLogs() {
	usageLogsLoading.value = true
	try {
		const res: any = await request.get(`/tenant/projects/${projectId.value}/usage-logs`, {
			params: { page: usageLogsPage.value, page_size: usageLogsPageSize.value },
		})
		const raw = res.data?.data
		usageLogs.value = Array.isArray(raw) ? raw : (raw?.list || [])
		usageLogsTotal.value = raw?.total || 0
	} catch {
		usageLogs.value = []
	} finally {
		usageLogsLoading.value = false
	}
}

// 金额格式化统一走本位币（formatBilling 内部读取响应式 displayCurrency，配置变化自动重渲染）
function formatCost(n: number): string {
	return formatBilling(n, 4)
}

function switchTab(tab: 'overview' | 'keys' | 'usage') {
	activeTab.value = tab
	if (tab === 'keys') fetchKeys()
	if (tab === 'usage') {
		fetchUsageStats()
		fetchUsageLogs()
	}
}

// 密钥分页：pageSize 变化回第 1 页并刷新
function handleKeysPageSizeChange() {
	keysPage.value = 1
	fetchKeys()
}

// 用量日志分页：pageSize 变化回第 1 页并刷新
function handleUsageLogsPageSizeChange() {
	usageLogsPage.value = 1
	fetchUsageLogs()
}

// API 密钥表列定义
const keysColumns = computed<DataTableColumns<ApiKey>>(() => [
	{
		title: '名称',
		key: 'name',
		width: 140,
		render: (row) => h('span', { class: 'font-medium text-gray-900' }, row.name),
	},
	{
		title: 'Key 前缀',
		key: 'key_prefix',
		width: 170,
		render: (row) =>
			h('div', { class: 'inline-flex items-center gap-1.5' }, [
				h('span', { class: 'code' }, `${row.key_prefix}...`),
				h(
					'button',
					{
						type: 'button',
						class:
							'inline-flex h-7 w-7 flex-none cursor-pointer items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-gray-100 hover:text-primary-600 focus:outline-none focus:ring-2 focus:ring-primary-500/40 disabled:cursor-not-allowed disabled:opacity-50',
						disabled: copyingKeyId.value !== null,
						title: copyingKeyId.value === row.id ? '正在复制' : '复制 API Key',
						onClick: () => copyKey(row.id),
					},
					copyingKeyId.value === row.id
						? h('span', { class: 'spinner h-3.5 w-3.5 border-primary-500' })
						: h(Icon, { name: 'copy', size: 'xs' })
				),
			]),
	},
	{
		title: '权限',
		key: 'scope',
		width: 160,
		render: (row) =>
			row.model_count > 0
				? h(
						NButton,
						{ text: true, type: 'primary', size: 'small', onClick: () => openScopeModal(row.id, row.name) },
						{ default: () => `${row.model_count} 个模型` }
				  )
				: h('span', { class: 'badge badge-gray' }, '不限模型'),
	},
	{
		title: '限制',
		key: 'limit',
		width: 200,
		render: (row) =>
			h('div', { class: 'space-y-1 text-xs text-gray-500' }, [
				h('div', { class: 'flex items-center gap-3 whitespace-nowrap' }, [
					h('span', {}, `QPS：${formatKeyLimit(row.rate_limit_qps)}`),
					h('span', {}, `并发：${formatKeyLimit(row.rate_limit_concurrency)}`),
				]),
				h('div', {}, `额度：${formatKeyQuota(row)}`),
				// 仅配置了 IP 白名单时才显示该行
				...(row.ip_whitelist?.length ? [h('div', {}, `IP：${row.ip_whitelist.length} 条`)] : []),
			]),
	},
	{
		title: '状态',
		key: 'status',
		width: 90,
		render: (row) => renderBadge(displayStatus(row), keyStatusLabel, keyStatusBadgeClass),
	},
	{
		title: '过期时间',
		key: 'expires_at',
		width: 160,
		render: (row) =>
			h(
				'span',
				{ class: 'text-xs text-gray-500' },
				row.expires_at ? String(row.expires_at).replace('T', ' ').substring(0, 16) : '永不过期'
			),
	},
	{
		title: '创建时间',
		key: 'created_at',
		width: 160,
		render: (row) => h('span', { class: 'text-xs text-gray-500' }, formatDate(row.created_at)),
	},
	{
		title: '操作',
		key: 'actions',
		width: 120,
		align: 'right',
		render: (row) => {
			if (row.status === 'active') {
				return h('div', { class: 'flex items-center justify-end gap-1' }, [
					h(
						NButton,
						{ text: true, size: 'small', onClick: () => openEditModal(row) },
						{ default: () => [h(Icon, { name: 'edit', size: 'xs' }), ' 编辑'] }
					),
					h(
						NButton,
						{ text: true, size: 'small', type: 'error', onClick: () => deleteKey(row.id) },
						{ default: () => [h(Icon, { name: 'trash', size: 'xs' }), ' 禁用'] }
					),
				])
			}
			return h('span', { class: 'text-xs text-gray-400' }, keyStatusLabel[row.status] || '已禁用')
		},
	},
])

// 用量日志表列定义
const usageLogsColumns = computed<DataTableColumns<any>>(() => [
	{
		title: '模型',
		key: 'model_name',
		width: 160,
		render: (row) => h('span', { class: 'code text-xs' }, row.model_name),
	},
	{
		title: '类型',
		key: 'relay_mode',
		width: 120,
		render: (row) => h('span', { class: 'text-xs text-gray-500' }, relayModeLabel[row.relay_mode] || row.relay_mode),
	},
	{
		title: '输入',
		key: 'input_tokens',
		width: 120,
		render: (row) => h('span', { class: 'text-xs text-gray-500' }, formatTokens(row.input_tokens || 0)),
	},
	{
		title: '输出',
		key: 'output_tokens',
		width: 120,
		render: (row) => h('span', { class: 'text-xs text-gray-500' }, formatTokens(row.output_tokens || 0)),
	},
	{
		title: '费用',
		key: 'total_cost',
		width: 120,
		render: (row) => h('span', { class: 'text-xs font-medium' }, formatCost(row.total_cost || 0)),
	},
	{
		title: '延迟',
		key: 'latency_ms',
		width: 110,
		render: (row) => h('span', { class: 'text-xs text-gray-500' }, formatMs(row.latency_ms)),
	},
	{
		title: '状态',
		key: 'status',
		width: 100,
		render: (row) =>
			h(
				NTag,
				{ size: 'small', type: row.status === 'success' ? 'success' : 'error' },
				{ default: () => (row.status === 'success' ? '成功' : '失败') }
			),
	},
	{
		title: '时间',
		key: 'created_at',
		width: 170,
		render: (row) => h('span', { class: 'text-xs text-gray-400' }, formatDate(row.created_at)),
	},
])
</script>

<template>
	<div class="space-y-6">
		<!-- Breadcrumb -->
		<div class="flex items-center gap-2 text-sm text-gray-500">
			<button class="hover:text-primary-600 transition-colors" @click="router.push('/tenant/projects')">
				项目管理
			</button>
			<Icon name="chevronRight" size="xs" />
			<span class="text-gray-900 font-medium">{{ project.name || '加载中...' }}</span>
		</div>

		<div v-if="loading" class="flex justify-center py-12">
			<div class="spinner h-6 w-6 text-primary-500"></div>
		</div>

		<template v-else>
			<!-- Project Header Card -->
			<div class="card">
				<div class="card-body">
					<div class="flex flex-col sm:flex-row sm:items-start justify-between gap-4">
						<div class="flex-1 min-w-0">
							<div class="flex items-center gap-3 mb-1">
								<h1 class="text-xl font-bold text-gray-900 truncate">{{ project.name }}</h1>
								<span class="badge flex-shrink-0" :class="statusBadgeClasses[project.status || ''] || 'badge-gray'">
									{{ statusLabels[project.status || ''] || project.status }}
								</span>
							</div>
							<p class="text-sm text-gray-500">{{ project.description || '暂无描述' }}</p>
						</div>
						<div class="flex items-center gap-2 flex-shrink-0">
							<button v-if="project.status === 'active'" class="btn btn-secondary btn-sm" @click="showCreateModal = true">
								<Icon name="plus" size="xs" />
								创建密钥
							</button>
						</div>
					</div>

					<!-- Stats Row -->
					<div class="mt-5 grid grid-cols-2 sm:grid-cols-4 gap-4">
						<div class="stat-card-mini">
							<span class="stat-card-mini-label">活跃密钥</span>
							<span class="stat-card-mini-value">{{ project.active_keys || 0 }} <span class="text-gray-400 text-xs font-normal">/ {{ project.total_keys || 0 }}</span></span>
						</div>
						<div class="stat-card-mini">
							<span class="stat-card-mini-label">本月请求</span>
							<span class="stat-card-mini-value">{{ (project.month_requests || 0).toLocaleString() }}</span>
						</div>
						<div class="stat-card-mini">
							<span class="stat-card-mini-label">本月消费</span>
							<span class="stat-card-mini-value">{{ formatCost(project.month_cost || 0) }}</span>
						</div>
						<div class="stat-card-mini">
							<span class="stat-card-mini-label">预算使用</span>
							<div class="flex items-center gap-2">
								<span class="stat-card-mini-value" :class="budgetColor">
									{{ project.budget && Number(project.budget) > 0 ? budgetUsage + '%' : '不限' }}
								</span>
								<div v-if="project.budget && Number(project.budget) > 0" class="flex-1 h-1.5 bg-gray-100 rounded-full overflow-hidden max-w-[80px]">
									<div class="h-full rounded-full transition-all duration-500"
										:class="budgetUsage >= 90 ? 'bg-red-500' : budgetUsage >= 70 ? 'bg-amber-500' : 'bg-emerald-500'"
										:style="{ width: budgetUsage + '%' }">
									</div>
								</div>
							</div>
						</div>
					</div>
				</div>
			</div>

			<!-- Tabs -->
			<div class="tabs">
				<button class="tab" :class="{ 'tab-active': activeTab === 'overview' }" @click="switchTab('overview')">
					基本信息
				</button>
				<button class="tab" :class="{ 'tab-active': activeTab === 'keys' }" @click="switchTab('keys')">
					API 密钥
				</button>
				<button class="tab" :class="{ 'tab-active': activeTab === 'usage' }" @click="switchTab('usage')">
					用量统计
				</button>
			</div>

			<!-- Overview Tab -->
			<div v-if="activeTab === 'overview'" class="card">
				<div class="card-header">
					<h2 class="text-base font-semibold text-gray-900">项目设置</h2>
				</div>
				<div class="card-body space-y-4">
					<div>
						<label class="input-label">项目名称 <span class="text-red-500">*</span></label>
						<input v-model="editForm.name" type="text" class="input" placeholder="输入项目名称" />
					</div>
					<div>
						<label class="input-label">描述</label>
						<textarea v-model="editForm.description" class="input" rows="3" placeholder="项目描述（选填）"></textarea>
					</div>
					<div>
						<label class="input-label">预算上限</label>
						<input v-model="editForm.budget" type="number" step="0.01" min="0" class="input" placeholder="0 = 不限制" />
						<p class="input-hint">设为 0 表示不限制。达到预算上限后，项目下所有 Key 将停止服务。</p>
					</div>
					<div class="flex justify-end">
						<button class="btn btn-primary" :disabled="editLoading || !editForm.name.trim()" @click="handleSave">
							{{ editLoading ? '保存中...' : '保存修改' }}
						</button>
					</div>
				</div>
			</div>

			<!-- Keys Tab -->
			<div v-if="activeTab === 'keys'" class="space-y-4">
				<div class="flex justify-end">
					<!-- Validity filter -->
					<NDropdown trigger="click" :options="validityOptions" @select="handleValiditySelect">
						<button type="button" class="btn btn-secondary btn-sm">
							{{ currentValidityLabel }}
							<Icon name="chevronDown" size="xs" class="text-gray-400" />
						</button>
					</NDropdown>
				</div>
				<div>
					<ResponsiveDataTable
						remote
						v-model:page="keysPage"
						v-model:page-size="keysPageSize"
						:item-count="keysTotal"
						:page-sizes="[10, 20, 50, 100]"
						show-size-picker
						:loading="keysLoading"
						:columns="keysColumns"
						:scroll-x="tableScrollX(keysColumns)"
						:data="keys"
						:row-key="(row: ApiKey) => row.id"
						card-title-key="name"
						card-badge-key="status"
						card-subtitle-key="created_at"
						:card-fields="[{ key: 'key_prefix', full: true }, 'scope', { key: 'limit', full: true }, 'expires_at']"
						card-actions-key="actions"
						@update:page="fetchKeys"
						@update:page-size="handleKeysPageSizeChange"
					>
						<template #empty>
							<div class="empty-state">
								<Icon name="key" size="xl" class="empty-state-icon" />
								<p class="empty-state-title">
									{{ validityFilter === 'valid' ? '暂无可用密钥' : validityFilter === 'invalid' ? '暂无失效密钥' : '暂无项目密钥' }}
								</p>
								<p class="empty-state-description">
									{{ validityFilter === 'valid' ? '所有密钥均已失效或尚未创建' : validityFilter === 'invalid' ? '所有密钥均正常可用' : '创建密钥以为此项目提供 AI 能力' }}
								</p>
							</div>
						</template>
					</ResponsiveDataTable>
				</div>
			</div>

			<!-- Usage Tab -->
			<div v-if="activeTab === 'usage'" class="space-y-6">
				<!-- Usage Summary Cards -->
				<div v-if="usageLoading" class="flex justify-center py-8">
					<div class="spinner h-6 w-6 text-primary-500"></div>
				</div>
				<template v-else-if="usageStats">
					<div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
						<div class="card p-4">
							<p class="text-xs text-gray-500 mb-1">总消费</p>
							<p class="text-lg font-bold text-gray-900">{{ formatCost(usageStats.total_cost || 0) }}</p>
						</div>
						<div class="card p-4">
							<p class="text-xs text-gray-500 mb-1">总请求数</p>
							<p class="text-lg font-bold text-gray-900">{{ (usageStats.total_requests || 0).toLocaleString() }}</p>
						</div>
						<div class="card p-4">
							<p class="text-xs text-gray-500 mb-1">输入 Token</p>
							<p class="text-lg font-bold text-gray-900">{{ formatTokens(usageStats.total_input_tokens || 0) }}</p>
						</div>
						<div class="card p-4">
							<p class="text-xs text-gray-500 mb-1">输出 Token</p>
							<p class="text-lg font-bold text-gray-900">{{ formatTokens(usageStats.total_output_tokens || 0) }}</p>
						</div>
					</div>

					<!-- Model Distribution -->
					<div v-if="usageStats.models && usageStats.models.length > 0" class="card">
						<div class="card-header">
							<h3 class="text-base font-semibold text-gray-900">模型用量分布</h3>
						</div>
						<div class="card-body">
							<div class="space-y-3">
								<div v-for="m in usageStats.models" :key="m.model_name" class="flex items-center gap-3">
									<span class="code text-xs min-w-[140px] truncate" :title="m.model_name">{{ m.model_name }}</span>
									<div class="flex-1 h-2 bg-gray-100 rounded-full overflow-hidden">
										<div class="h-full bg-primary-500 rounded-full transition-all duration-500"
											:style="{ width: ((m.request_count / usageStats.total_requests) * 100) + '%' }">
										</div>
									</div>
									<span class="text-xs text-gray-500 w-16 text-right">{{ m.request_count }} 次</span>
									<span class="text-xs font-medium text-gray-700 w-20 text-right">{{ formatCost(m.total_cost || 0) }}</span>
								</div>
							</div>
						</div>
					</div>

					<!-- Usage Logs -->
					<div class="space-y-3">
						<div class="card-header">
							<h3 class="text-base font-semibold text-gray-900">用量日志</h3>
						</div>
						<ResponsiveDataTable
							remote
							v-model:page="usageLogsPage"
							v-model:page-size="usageLogsPageSize"
							:item-count="usageLogsTotal"
							:page-sizes="[10, 20, 50, 100]"
							show-size-picker
							:loading="usageLogsLoading"
							:columns="usageLogsColumns"
							:scroll-x="tableScrollX(usageLogsColumns)"
							:data="usageLogs"
							:row-key="(row: any) => row.id"
							card-title-key="model_name"
							card-badge-key="status"
							card-subtitle-key="created_at"
							:card-fields="['relay_mode', 'input_tokens', 'output_tokens', 'total_cost', 'latency_ms']"
							@update:page="fetchUsageLogs"
							@update:page-size="handleUsageLogsPageSizeChange"
						>
							<template #empty>
								<div class="empty-state">
									<Icon name="chart" size="xl" class="empty-state-icon" />
									<p class="empty-state-title">暂无用量记录</p>
									<p class="empty-state-description">项目密钥调用后将在此显示用量数据</p>
								</div>
							</template>
						</ResponsiveDataTable>
					</div>
				</template>
			</div>
		</template>

		<!-- Create Key Modal -->
		<ApiKeyEditModal
			v-model:show="showCreateModal"
			mode="create"
			:project-id="projectId"
			@saved="fetchKeys"
		/>

		<!-- Edit Key Modal -->
		<ApiKeyEditModal
			v-model:show="showEditModal"
			mode="edit"
			:api-key="editingKey"
			:project-id="projectId"
			@saved="fetchKeys"
		/>

		<!-- Model Scope Modal -->
		<BaseModal
			:show="showScopeModal"
			:title="scopeModalTitle"
			width="narrow"
			@close="showScopeModal = false"
		>
			<div v-if="scopeModalLoading" class="flex justify-center py-8">
				<div class="spinner h-6 w-6 text-primary-500"></div>
			</div>
			<div v-else class="max-h-80 overflow-y-auto">
				<div v-for="name in scopeModalModels" :key="name" class="px-3 py-2 border-b border-gray-100 last:border-b-0">
					<p class="text-sm font-mono text-gray-700">{{ name }}</p>
				</div>
				<div v-if="scopeModalModels.length === 0" class="py-8 text-center text-sm text-gray-400">无模型</div>
			</div>
			<template #footer>
				<div class="text-xs text-gray-500">
					共 {{ scopeModalModels.length }} 个模型
				</div>
			</template>
		</BaseModal>
	</div>
</template>

<style scoped>
.stat-card-mini {
	display: flex;
	flex-direction: column;
	gap: 2px;
	padding: 8px 0;
}
.stat-card-mini-label {
	font-size: 12px;
	color: #9ca3af;
}
.stat-card-mini-value {
	font-size: 16px;
	font-weight: 600;
	color: #111827;
}
</style>

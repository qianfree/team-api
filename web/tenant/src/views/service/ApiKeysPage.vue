<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NTag } from 'naive-ui'
import BaseModal from '@/components/common/BaseModal.vue'
import ApiKeyEditModal from '@/components/common/ApiKeyEditModal.vue'
import type { ApiKeyData } from '@/components/common/ApiKeyEditModal.vue'
import Icon from '@/components/common/Icon.vue'
import { renderBadge } from '@/utils/renderUtils'
import request from '@/utils/request'
import { toast } from '@/utils/toast'
import { useExport } from '@/composables/useExport'
import { useConfirm } from '@/composables/useConfirm'

const { confirm } = useConfirm()

interface ApiKey {
	id: number
	name: string
	key_prefix: string
	scope: string
	model_count: number
	status: string
	key_type: string
	expires_at: string | null
	rate_limit_qps: number | null
	rate_limit_concurrency: number | null
	ip_whitelist: string[] | null
	total_quota: number | null
	used_quota: number | null
	created_at: string
	updated_at: string
}

const keys = ref<ApiKey[]>([])
const loading = ref(false)
const copyingKeyId = ref<number | null>(null)
const baseUrl = `${window.location.origin}/v1`
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const showExportDropdown = ref(false)
const { exporting, exportFile } = useExport({
	url: '/tenant/api-keys/export',
	getFilters: () => ({
		key_type: 'personal',
	}),
})

// Create modal
const showCreateModal = ref(false)

// Edit modal
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

const statusBadgeClass: Record<string, string> = {
	active: 'badge-success',
	disabled: 'badge-gray',
	revoked: 'badge-danger',
}

const statusLabel: Record<string, string> = {
	active: '活跃',
	disabled: '已禁用',
	revoked: '已吊销',
}

async function fetchKeys() {
	loading.value = true
	try {
		const res: any = await request.get('/tenant/api-keys', {
			params: { page: page.value, page_size: pageSize.value, key_type: 'personal' },
		})
		const raw = res.data?.data
		keys.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
		total.value = raw?.total || 0
	} catch {
		keys.value = []
	} finally {
		loading.value = false
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

function formatLimit(value: number | null | undefined, suffix = ''): string {
	if (value === null || value === undefined) return '默认'
	if (value <= 0) return '不限'
	return `${value}${suffix}`
}

function formatQuota(key: ApiKey): string {
	const used = key.used_quota || 0
	if (!key.total_quota || key.total_quota <= 0) return `$${used.toFixed(2)} / 不限`
	return `$${used.toFixed(2)} / $${key.total_quota.toFixed(2)}`
}

async function disableKey(keyId: number) {
	if (!await confirm({ message: '确定禁用该 API Key？禁用后将无法使用。', confirmText: '确认禁用', danger: true })) return
	try {
		await request.delete(`/tenant/api-keys/${keyId}`)
		const key = keys.value.find((k) => k.id === keyId)
		if (key) key.status = 'disabled'
	} catch {
	}
}

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

async function copyBaseUrl() {
	try {
		await navigator.clipboard.writeText(baseUrl)
		toast.success('Base URL 已复制到剪贴板')
	} catch {
		toast.error('复制失败，请检查浏览器剪贴板权限')
	}
}

function formatDate(d: string | null): string {
	if (!d) return '永不过期'
	return d.replace('T', ' ').substring(0, 16)
}

// NDataTable 列定义
const columns = computed<DataTableColumns<ApiKey>>(() => [
	{ title: '名称', key: 'name', width: 130, render: (row) => h('span', { class: 'font-medium text-gray-900' }, row.name) },
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
		key: 'model_count',
		width: 110,
		render: (row) =>
			row.model_count > 0
				? h(
						NTag,
						{ type: 'info', size: 'small', style: 'cursor:pointer', onClick: () => openScopeModal(row.id, row.name) },
						{ default: () => `${row.model_count} 个模型` }
					)
				: renderBadge('unlimited', { unlimited: '不限模型' }, { unlimited: 'badge-gray' }),
	},
	{
		title: '限制',
		key: 'limits',
		width: 200,
		render: (row) =>
			h('div', { class: 'space-y-1 text-xs text-gray-500' }, [
				h('div', {}, `QPS：${formatLimit(row.rate_limit_qps)}`),
				h('div', {}, `并发：${formatLimit(row.rate_limit_concurrency)}`),
				h('div', {}, `额度：${formatQuota(row)}`),
				h('div', {}, `IP：${row.ip_whitelist?.length ? row.ip_whitelist.length + ' 条' : '不限'}`),
			]),
	},
	{ title: '状态', key: 'status', width: 90, render: (row) => renderBadge(row.status, statusLabel, statusBadgeClass) },
	{ title: '过期时间', key: 'expires_at', width: 150, render: (row) => h('span', { class: 'text-gray-500 text-xs' }, formatDate(row.expires_at)) },
	{
		title: '创建时间',
		key: 'created_at',
		width: 150,
		render: (row) => h('span', { class: 'text-gray-500 text-xs' }, (row.created_at || '').replace('T', ' ').substring(0, 16)),
	},
	{
		title: '操作',
		key: 'actions',
		align: 'right',
		width: 110,
		render: (row) =>
			h('div', { class: 'flex items-center justify-end gap-1' }, [
				row.status === 'active'
					? h(
							NButton,
							{ text: true, size: 'small', onClick: () => openEditModal(row) },
							{ icon: () => h(Icon, { name: 'edit', size: 'xs' }), default: () => '编辑' }
						)
					: null,
				row.status === 'active'
					? h(
							NButton,
							{ text: true, size: 'small', type: 'error', onClick: () => disableKey(row.id) },
							{ icon: () => h(Icon, { name: 'trash', size: 'xs' }), default: () => '禁用' }
						)
					: null,
				row.status === 'disabled' ? h('span', { class: 'text-xs text-gray-400' }, '已禁用') : null,
			]),
	},
])

function handlePageSizeChange() {
	page.value = 1
	fetchKeys()
}

onMounted(() => {
	fetchKeys()
})
</script>

<template>
	<div class="viewport-table-page space-y-6">
		<!-- Page Header -->
		<div class="page-header flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
			<div class="min-w-0">
				<h1 class="page-title">API 密钥</h1>
				<p class="page-description flex flex-wrap items-center gap-x-1.5 gap-y-2">
					<span>管理您的个人 API 访问密钥。调用 OpenAI 兼容接口时请使用</span>
					<span class="font-medium text-gray-600">Base URL</span>
					<code class="code break-all text-xs sm:text-sm">{{ baseUrl }}</code>
					<button
						type="button"
						class="inline-flex h-7 w-7 flex-none cursor-pointer items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-gray-100 hover:text-primary-600 focus:outline-none focus:ring-2 focus:ring-primary-500/40"
						aria-label="复制 Base URL"
						title="复制 Base URL"
						@click="copyBaseUrl"
					>
						<Icon name="copy" size="xs" />
					</button>
				</p>
			</div>
			<div class="flex flex-wrap items-center gap-2 lg:flex-none">
				<!-- Export dropdown -->
				<div class="relative inline-block">
					<button class="btn btn-secondary" :disabled="exporting" @click="showExportDropdown = !showExportDropdown">
						<svg v-if="!exporting" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3"/></svg>
						<svg v-else class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
						导出
					</button>
					<div v-if="showExportDropdown" class="absolute right-0 mt-2 w-36 bg-white rounded-xl border shadow-lg py-1 z-50">
						<div class="px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 cursor-pointer" @click="exportFile('csv'); showExportDropdown = false">导出 CSV</div>
						<div class="px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 cursor-pointer" @click="exportFile('xlsx'); showExportDropdown = false">导出 Excel</div>
					</div>
				</div>
				<button class="btn btn-primary" @click="showCreateModal = true">
					<Icon name="plus" size="sm" />
					创建密钥
				</button>
			</div>
		</div>

		<!-- Keys Table -->
		<div class="viewport-table-panel card">
			<n-data-table
				remote
				v-model:page="page"
				v-model:page-size="pageSize"
				:item-count="total"
				:page-sizes="[10, 20, 50, 100]"
				show-size-picker
				:loading="loading"
				:columns="columns"
				:data="keys"
				:row-key="(row: ApiKey) => row.id"
				@update:page="fetchKeys"
				@update:page-size="handlePageSizeChange"
			>
				<template #empty>
					<div class="empty-state">
						<Icon name="key" size="xl" class="empty-state-icon" />
						<p class="empty-state-title">暂无个人密钥</p>
						<p class="empty-state-description">创建第一个密钥以开始使用 AI 模型</p>
					</div>
				</template>
			</n-data-table>
		</div>

		<!-- Create Modal -->
		<ApiKeyEditModal
			v-model:show="showCreateModal"
			mode="create"
			@saved="fetchKeys"
		/>

		<!-- Edit Modal -->
		<ApiKeyEditModal
			v-model:show="showEditModal"
			mode="edit"
			:api-key="editingKey"
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

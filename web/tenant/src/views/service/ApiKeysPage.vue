<script setup lang="ts">
import { ref, onMounted } from 'vue'
import BaseModal from '@/components/common/BaseModal.vue'
import ApiKeyEditModal from '@/components/common/ApiKeyEditModal.vue'
import type { ApiKeyData } from '@/components/common/ApiKeyEditModal.vue'
import Icon from '@/components/common/Icon.vue'
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
	total_quota: number | null
	used_quota: number | null
	created_at: string
	updated_at: string
}

const keys = ref<ApiKey[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = 20
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
			params: { page: page.value, page_size: pageSize, key_type: 'personal' },
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
		total_quota: key.total_quota,
		used_quota: key.used_quota,
	}
	showEditModal.value = true
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

function formatDate(d: string | null): string {
	if (!d) return '永不过期'
	return d.replace('T', ' ').substring(0, 16)
}

onMounted(() => {
	fetchKeys()
})
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="page-header flex items-center justify-between">
			<div>
				<h1 class="page-title">API 密钥</h1>
				<p class="page-description">管理您的个人 API 访问密钥</p>
			</div>
			<div class="flex items-center gap-2">
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
		<div class="card">
			<div v-if="loading" class="p-8 flex justify-center">
				<div class="spinner h-6 w-6 border-primary-500"></div>
			</div>

			<div v-else-if="keys.length > 0" class="table-container">
				<table class="table">
					<thead>
						<tr>
							<th>名称</th>
							<th>Key 前缀</th>
							<th>权限</th>
							<th>状态</th>
							<th>过期时间</th>
							<th>创建时间</th>
							<th class="text-right">操作</th>
						</tr>
					</thead>
					<tbody>
						<tr v-for="key in keys" :key="key.id">
							<td class="font-medium text-gray-900">{{ key.name }}</td>
							<td>
								<span class="code">{{ key.key_prefix }}...</span>
							</td>
							<td>
								<template v-if="key.model_count > 0">
									<button class="badge badge-primary cursor-pointer hover:bg-primary-100 transition-colors" @click="openScopeModal(key.id, key.name)">
										{{ key.model_count }} 个模型
									</button>
								</template>
								<span v-else class="badge badge-gray">不限模型</span>
							</td>
							<td>
								<span class="badge" :class="statusBadgeClass[key.status] || 'badge-gray'">
									{{ statusLabel[key.status] || key.status }}
								</span>
							</td>
							<td class="text-gray-500 text-xs">{{ formatDate(key.expires_at) }}</td>
							<td class="text-gray-500 text-xs">{{ (key.created_at || '').replace('T', ' ').substring(0, 16) }}</td>
							<td class="text-right">
								<div class="flex items-center justify-end gap-1">
									<button
										v-if="key.status === 'active'"
										@click="openEditModal(key)"
										class="btn btn-ghost btn-sm"
									>
										<Icon name="edit" size="xs" />
										编辑
									</button>
									<button
										v-if="key.status === 'active'"
										@click="disableKey(key.id)"
										class="btn btn-ghost btn-sm text-red-600 hover:bg-red-50"
									>
										<Icon name="trash" size="xs" />
										禁用
									</button>
									<span v-if="key.status === 'disabled'" class="text-xs text-gray-400">已禁用</span>
								</div>
							</td>
						</tr>
					</tbody>
				</table>
			</div>

			<!-- Empty state -->
			<div v-else class="empty-state">
				<Icon name="key" size="xl" class="empty-state-icon" />
				<p class="empty-state-title">暂无个人密钥</p>
				<p class="empty-state-description">创建第一个密钥以开始使用 AI 模型</p>
			</div>

			<!-- Pagination -->
			<div v-if="total > pageSize" class="card-footer flex justify-end">
				<div class="flex items-center gap-2">
					<button class="btn btn-ghost btn-sm" :disabled="page <= 1" @click="page--; fetchKeys()">上一页</button>
					<span class="text-sm text-gray-500">{{ page }} / {{ Math.ceil(total / pageSize) }}</span>
					<button class="btn btn-ghost btn-sm" :disabled="page * pageSize >= total" @click="page++; fetchKeys()">下一页</button>
				</div>
			</div>
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

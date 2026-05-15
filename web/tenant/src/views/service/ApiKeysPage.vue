<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import BaseModal from '@/components/common/BaseModal.vue'
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
	expires_at: string | null
	rate_limit_qps: number | null
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

const showCreateModal = ref(false)
const createForm = reactive({
	name: '',
	expires_in_days: 0,
})
const createLoading = ref(false)
const createdKey = ref('')

// Model selection
interface ModelItem {
	id: number
	model_id: string
	model_name: string
	category: string
}
const allModels = ref<ModelItem[]>([])
const selectedModelNames = ref<string[]>([])
const modelSearch = ref('')

const categoryLabel: Record<string, string> = {
	chat: '对话',
	embedding: '嵌入',
	image: '图像',
	audio: '语音',
	rerank: '重排',
}
const categoryBadgeClass: Record<string, string> = {
	chat: 'badge-primary',
	embedding: 'badge-purple',
	image: 'badge-warning',
	audio: 'badge-success',
	rerank: 'badge-gray',
}

const filteredModels = computed(() => {
	if (!modelSearch.value) return allModels.value
	const q = modelSearch.value.toLowerCase()
	return allModels.value.filter(
		m => m.model_id.toLowerCase().includes(q) || m.model_name.toLowerCase().includes(q)
	)
})

const groupedFilteredModels = computed(() => {
	const groups: Record<string, ModelItem[]> = {}
	for (const m of filteredModels.value) {
		const cat = m.category || 'other'
		if (!groups[cat]) groups[cat] = []
		groups[cat].push(m)
	}
	return groups
})

function toggleModelName(modelId: string) {
	const idx = selectedModelNames.value.indexOf(modelId)
	if (idx >= 0) {
		selectedModelNames.value.splice(idx, 1)
	} else {
		selectedModelNames.value.push(modelId)
	}
}

function selectAllModels() {
	selectedModelNames.value = allModels.value.map(m => m.model_id)
}

function clearAllModels() {
	selectedModelNames.value = []
}

async function fetchAllModels() {
	try {
		const res: any = await request.get('/tenant/models')
		const raw = res.data?.data
		allModels.value = Array.isArray(raw) ? raw : (raw?.list || [])
	} catch {
		allModels.value = []
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

function openCreateModal() {
	createForm.name = ''
	createForm.expires_in_days = 0
	selectedModelNames.value = []
	modelSearch.value = ''
	createdKey.value = ''
	showCreateModal.value = true
}

async function handleCreate() {
	if (!createForm.name.trim()) return
	createLoading.value = true
	try {
		const body: any = {
			name: createForm.name,
			scope: 'full',
			model_names: selectedModelNames.value,
		}
		if (createForm.expires_in_days > 0) {
			body.expires_in_days = createForm.expires_in_days
		}
		const res: any = await request.post('/tenant/api-keys', body)
		createdKey.value = res.data?.data?.key || ''
		if (createdKey.value) {
			fetchKeys()
		}
	} catch {
	} finally {
		createLoading.value = false
	}
}

function copyKey() {
	if (!createdKey.value) return
	navigator.clipboard.writeText(createdKey.value).then(() => {
		toast.success('密钥已复制到剪贴板')
	})
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
	fetchAllModels()
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
				<button class="btn btn-primary" @click="openCreateModal">
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
								<span class="badge badge-gray">{{ key.model_count > 0 ? key.model_count + ' 个模型' : '不限模型' }}</span>
							</td>
							<td>
								<span class="badge" :class="statusBadgeClass[key.status] || 'badge-gray'">
									{{ statusLabel[key.status] || key.status }}
								</span>
							</td>
							<td class="text-gray-500 text-xs">{{ formatDate(key.expires_at) }}</td>
							<td class="text-gray-500 text-xs">{{ (key.created_at || '').replace('T', ' ').substring(0, 16) }}</td>
							<td class="text-right">
								<button
									v-if="key.status === 'active'"
									@click="disableKey(key.id)"
									class="btn btn-ghost btn-sm text-red-600 hover:bg-red-50"
								>
									<Icon name="trash" size="xs" />
									禁用
								</button>
								<span v-else class="text-xs text-gray-400">已禁用</span>
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
		<BaseModal
			:show="showCreateModal"
			:title="createdKey ? '密钥创建成功' : '创建 API 密钥'"
			width="wide"
			@close="showCreateModal = false"
		>
			<div v-if="!createdKey" class="space-y-4">
				<div>
					<label class="input-label">名称</label>
					<input
						v-model="createForm.name"
						type="text"
						placeholder="例如：生产环境密钥"
						class="input"
					/>
				</div>

				<div>
					<label class="input-label">可用模型</label>
					<p class="text-xs text-gray-500 mb-2">留空表示不限制，密钥可使用所有租户可用模型。</p>
					<div class="flex items-center gap-3 mb-2">
						<div class="relative flex-1">
							<Icon name="search" size="sm" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
							<input v-model="modelSearch" type="text" class="input pl-9" placeholder="搜索模型..." />
						</div>
						<button class="btn btn-ghost btn-sm" @click="selectAllModels">全选</button>
						<button class="btn btn-ghost btn-sm" @click="clearAllModels">清空</button>
					</div>
					<div class="max-h-60 overflow-y-auto border border-gray-200 rounded-xl divide-y divide-gray-100">
						<template v-for="(items, cat) in groupedFilteredModels" :key="cat">
							<div class="px-4 py-2 bg-gray-50 text-xs font-semibold text-gray-500 uppercase tracking-wider sticky top-0">
								{{ categoryLabel[cat as string] || cat }}
								<span class="text-gray-300">({{ items.length }})</span>
							</div>
							<label
								v-for="m in items"
								:key="m.id"
								class="flex items-center gap-3 px-4 py-2 hover:bg-gray-50 cursor-pointer"
							>
								<input
									type="checkbox"
									:checked="selectedModelNames.includes(m.model_id)"
									@change="toggleModelName(m.model_id)"
									class="h-4 w-4 rounded border-gray-300 text-primary-500 focus:ring-primary-500/30"
								/>
								<div class="min-w-0 flex-1">
									<p class="text-sm font-medium text-gray-900 truncate">{{ m.model_name || m.model_id }}</p>
									<p class="text-xs text-gray-400 font-mono truncate">{{ m.model_id }}</p>
								</div>
								<span class="badge shrink-0" :class="categoryBadgeClass[m.category] || 'badge-gray'">
									{{ categoryLabel[m.category] || m.category }}
								</span>
							</label>
						</template>
						<div v-if="filteredModels.length === 0" class="px-4 py-6 text-center text-sm text-gray-400">
							没有匹配的模型
						</div>
					</div>
					<p class="text-xs text-gray-500 mt-1">
						已选择 <span class="font-medium text-gray-700">{{ selectedModelNames.length }}</span> 个模型
						<template v-if="selectedModelNames.length === 0">（不限制）</template>
					</p>
				</div>

				<div>
					<label class="input-label">有效天数（0 = 永不过期）</label>
					<input
						v-model.number="createForm.expires_in_days"
						type="number"
						placeholder="0"
						min="0"
						max="365"
						class="input"
					/>
				</div>
			</div>

			<div v-else class="space-y-4">
				<div class="flex items-center gap-3 mb-2">
					<div class="h-10 w-10 rounded-full bg-emerald-100 flex items-center justify-center flex-shrink-0">
						<Icon name="checkCircle" size="md" class="text-emerald-600" />
					</div>
					<div>
						<p class="text-sm text-gray-500">请立即复制密钥，关闭后将无法再次查看</p>
					</div>
				</div>

				<div class="p-3 bg-gray-900 rounded-xl">
					<p class="text-sm font-mono text-emerald-400 break-all select-all">{{ createdKey }}</p>
				</div>

				<button @click="copyKey" class="btn btn-primary w-full">
					<Icon name="copy" size="sm" />
					复制密钥
				</button>
			</div>

			<template #footer>
				<button v-if="!createdKey" @click="showCreateModal = false" class="btn btn-secondary">取消</button>
				<button
					v-if="!createdKey"
					@click="handleCreate"
					:disabled="createLoading || !createForm.name.trim()"
					class="btn btn-primary"
				>
					{{ createLoading ? '创建中...' : '创建' }}
				</button>
				<button v-else @click="showCreateModal = false" class="btn btn-secondary w-full">完成</button>
			</template>
		</BaseModal>
	</div>
</template>

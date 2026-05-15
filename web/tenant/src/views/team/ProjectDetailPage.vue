<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BaseModal from '@/components/common/BaseModal.vue'
import ApiKeyEditModal from '@/components/common/ApiKeyEditModal.vue'
import type { ApiKeyData } from '@/components/common/ApiKeyEditModal.vue'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'
import { toast } from '@/utils/toast'
import { useConfirm } from '@/composables/useConfirm'

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
	total_quota: number | null
	used_quota: number | null
	created_at: string
}
const keys = ref<ApiKey[]>([])
const keysLoading = ref(false)
const keysPage = ref(1)
const keysPageSize = 20
const keysTotal = ref(0)

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
const usageLogsPageSize = 20
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
}
const keyStatusLabel: Record<string, string> = {
	active: '活跃',
	disabled: '已禁用',
	revoked: '已吊销',
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
			params: { page: keysPage.value, page_size: keysPageSize },
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
		total_quota: key.total_quota,
		used_quota: key.used_quota,
	}
	showEditModal.value = true
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
			params: { page: usageLogsPage.value, page_size: usageLogsPageSize },
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

function formatTokens(n: number): string {
	if (!n) return '0'
	if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
	if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
	return String(n)
}

function formatCost(n: number): string {
	if (!n) return '$0.00'
	return '$' + n.toFixed(4)
}

function formatDate(d: string | null): string {
	if (!d) return '永不过期'
	return d.replace('T', ' ').substring(0, 16)
}

function switchTab(tab: 'overview' | 'keys' | 'usage') {
	activeTab.value = tab
	if (tab === 'keys') fetchKeys()
	if (tab === 'usage') {
		fetchUsageStats()
		fetchUsageLogs()
	}
}
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
				<div class="card">
					<div v-if="keysLoading" class="p-8 flex justify-center">
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
									<td><span class="code">{{ key.key_prefix }}...</span></td>
									<td>
										<template v-if="key.model_count > 0">
											<button class="badge badge-primary cursor-pointer hover:bg-primary-100 transition-colors" @click="openScopeModal(key.id, key.name)">
												{{ key.model_count }} 个模型
											</button>
										</template>
										<span v-else class="badge badge-gray">不限模型</span>
									</td>
									<td>
										<span class="badge" :class="keyStatusBadgeClass[key.status] || 'badge-gray'">
											{{ keyStatusLabel[key.status] || key.status }}
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
												@click="deleteKey(key.id)"
												class="btn btn-ghost btn-sm text-red-600 hover:bg-red-50"
											>
												<Icon name="trash" size="xs" />
												禁用
											</button>
											<span v-if="key.status === 'disabled'" class="text-xs text-gray-400">{{ keyStatusLabel[key.status] || '已禁用' }}</span>
										</div>
									</td>
								</tr>
							</tbody>
						</table>
					</div>

					<div v-else class="empty-state">
						<Icon name="key" size="xl" class="empty-state-icon" />
						<p class="empty-state-title">暂无项目密钥</p>
						<p class="empty-state-description">创建密钥以为此项目提供 AI 能力</p>
					</div>

					<div v-if="keysTotal > keysPageSize" class="card-footer flex justify-end">
						<div class="flex items-center gap-2">
							<button class="btn btn-ghost btn-sm" :disabled="keysPage <= 1" @click="keysPage--; fetchKeys()">上一页</button>
							<span class="text-sm text-gray-500">{{ keysPage }} / {{ Math.ceil(keysTotal / keysPageSize) }}</span>
							<button class="btn btn-ghost btn-sm" :disabled="keysPage * keysPageSize >= keysTotal" @click="keysPage++; fetchKeys()">下一页</button>
						</div>
					</div>
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
					<div class="card">
						<div class="card-header">
							<h3 class="text-base font-semibold text-gray-900">用量日志</h3>
						</div>
						<div v-if="usageLogsLoading" class="p-8 flex justify-center">
							<div class="spinner h-6 w-6 border-primary-500"></div>
						</div>
						<div v-else-if="usageLogs.length > 0" class="table-container">
							<table class="table">
								<thead>
									<tr>
										<th>模型</th>
										<th>类型</th>
										<th>输入</th>
										<th>输出</th>
										<th>费用</th>
										<th>延迟</th>
										<th>状态</th>
										<th>时间</th>
									</tr>
								</thead>
								<tbody>
									<tr v-for="log in usageLogs" :key="log.id">
										<td><span class="code text-xs">{{ log.model_name }}</span></td>
										<td class="text-xs text-gray-500">{{ relayModeLabel[log.relay_mode] || log.relay_mode }}</td>
										<td class="text-xs text-gray-500">{{ formatTokens(log.input_tokens || 0) }}</td>
										<td class="text-xs text-gray-500">{{ formatTokens(log.output_tokens || 0) }}</td>
										<td class="text-xs font-medium">{{ formatCost(log.total_cost || 0) }}</td>
										<td class="text-xs text-gray-500">{{ log.latency_ms ? log.latency_ms + 'ms' : '-' }}</td>
										<td>
											<span class="badge" :class="log.status === 'success' ? 'badge-success' : 'badge-danger'">
												{{ log.status === 'success' ? '成功' : '失败' }}
											</span>
										</td>
										<td class="text-xs text-gray-400">{{ (log.created_at || '').replace('T', ' ').substring(0, 16) }}</td>
									</tr>
								</tbody>
							</table>
						</div>
						<div v-else class="empty-state">
							<Icon name="chart" size="xl" class="empty-state-icon" />
							<p class="empty-state-title">暂无用量记录</p>
							<p class="empty-state-description">项目密钥调用后将在此显示用量数据</p>
						</div>
						<div v-if="usageLogsTotal > usageLogsPageSize" class="card-footer flex justify-end">
							<div class="flex items-center gap-2">
								<button class="btn btn-ghost btn-sm" :disabled="usageLogsPage <= 1" @click="usageLogsPage--; fetchUsageLogs()">上一页</button>
								<span class="text-sm text-gray-500">{{ usageLogsPage }} / {{ Math.ceil(usageLogsTotal / usageLogsPageSize) }}</span>
								<button class="btn btn-ghost btn-sm" :disabled="usageLogsPage * usageLogsPageSize >= usageLogsTotal" @click="usageLogsPage++; fetchUsageLogs()">下一页</button>
							</div>
						</div>
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

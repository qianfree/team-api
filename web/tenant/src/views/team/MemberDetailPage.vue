<script setup lang="ts">
import { formatNumber } from '@/utils/renderUtils'
import { ref, computed, onMounted } from 'vue'
import { NInput, NCheckbox } from 'naive-ui'
import { useRouter, useRoute } from 'vue-router'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import BaseModal from '@/components/common/BaseModal.vue'
import TeamLockedBanner from '@/components/common/TeamLockedBanner.vue'
import Icon from '@/components/common/Icon.vue'
import BaseSelect from '../../components/common/BaseSelect.vue'
import request from '@/utils/request'
import { toast } from '@/utils/toast'
import { displayCurrency, formatBilling } from '@/composables/useCurrency'

const router = useRouter()
const route = useRoute()
const authStore = useTenantAuthStore()
const teamEnabled = computed(() => !!authStore.tenant?.team_enabled)

const memberId = computed(() => Number(route.params.id))

interface MemberInfo {
	id: number
	username: string
	email: string
	display_name: string
	role: string
	status: string
	locked_until: string
	created_at: string
	updated_at: string
}

interface MemberUsage {
	today_requests: number
	month_requests: number
	month_input_tokens: number
	month_output_tokens: number
	month_total_cost: number
}

interface QuotaInfo {
	quota_type: string
	quota_limit: number
	quota_used: number
	period: string
	next_reset_at: string
}

interface ModelItem {
	id: number
	model_id: string
	model_name: string
	category: string
}

const member = ref<MemberInfo | null>(null)
const usage = ref<MemberUsage | null>(null)
const apiKeyCount = ref(0)
const loading = ref(true)

// Quota management
const quota = ref<QuotaInfo | null>(null)
const showQuotaModal = ref(false)
const quotaSaving = ref(false)
const quotaForm = ref({ quota_type: 'none', quota_limit: 0, period: 'month' })

// Model scope management
const allModels = ref<ModelItem[]>([])
const memberModelIds = ref<number[]>([])
const showModelModal = ref(false)
const modelSaving = ref(false)
const selectedModelIds = ref<number[]>([])
const modelSearch = ref('')

// Role management
const roleLoading = ref(false)

// Reset password
const showResetModal = ref(false)
const resetLoading = ref(false)
const resetPassword = ref('')

// Remove member
const showRemoveModal = ref(false)
const removeLoading = ref(false)

const roleBadgeClass: Record<string, string> = {
	owner: 'badge-primary',
	admin: 'badge-warning',
	member: 'badge-gray',
}

const roleLabel: Record<string, string> = {
	owner: '所有者',
	admin: '管理员',
	member: '成员',
}

const statusBadgeClass: Record<string, string> = {
	active: 'badge-success',
	invited: 'badge-primary',
	disabled: 'badge-gray',
}

const statusLabel: Record<string, string> = {
	active: '已激活',
	invited: '已邀请',
	disabled: '已禁用',
}

const statusDotClass: Record<string, string> = {
	active: 'bg-emerald-500',
	invited: 'bg-primary-500',
	disabled: 'bg-gray-400',
}

const isOwner = computed(() => member.value?.role === 'owner')
const canManage = computed(() => !isOwner.value)
const isLocked = computed(() => {
	if (!member.value?.locked_until) return false
	return new Date(member.value.locked_until).getTime() > Date.now()
})

function formatDate(d: string | null | undefined): string {
	if (!d) return '--'
	return d.replace('T', ' ').substring(0, 16)
}

const statCards = computed(() => {
	const u = usage.value
	return [
		{
			label: '今日请求数',
			icon: 'play',
			color: '#06b6d4',
			soft: '#cffafe',
			value: u ? formatNumber(u.today_requests) : '--',
			detail: u ? '次' : '',
		},
		{
			label: '本月请求数',
			icon: 'chart',
			color: '#10b981',
			soft: '#d1fae5',
			value: u ? formatNumber(u.month_requests) : '--',
			detail: u ? '次' : '',
		},
		{
			label: '本月 Token 用量',
			icon: 'clipboard',
			color: '#f59e0b',
			soft: '#fef3c7',
			value: u ? formatNumber(u.month_input_tokens + u.month_output_tokens) : '--',
			detail: u ? `输入 ${formatNumber(u.month_input_tokens)} · 输出 ${formatNumber(u.month_output_tokens)}` : '',
		},
		{
			label: '本月消费',
			icon: 'creditCard',
			color: '#8b5cf6',
			soft: '#ede9fe',
			value: u ? formatBilling(u.month_total_cost, 2) : '--',
			detail: u ? displayCurrency.value : '',
		},
		{
			label: 'API Key 数量',
			icon: 'key',
			color: '#0ea5e9',
			soft: '#e0f2fe',
			value: String(apiKeyCount.value),
			detail: '个',
		},
	]
})

async function fetchMemberDetail() {
	loading.value = true
	try {
		const res: any = await request.get(`/tenant/members/${memberId.value}`)
		member.value = res.data?.data || null
	} catch {
		member.value = null
	} finally {
		loading.value = false
	}
}

async function handleUnlock() {
	try {
		await request.put(`/tenant/members/${memberId.value}/unlock`)
		toast.success('成员已解锁')
		fetchMemberDetail()
	} catch {
		// 错误由 request 拦截器统一提示
	}
}

async function fetchUsage() {
	try {
		const res: any = await request.get(`/tenant/members/${memberId.value}/usage`)
		usage.value = res.data?.data || null
	} catch {
		usage.value = null
	}
}

async function fetchApiKeyCount() {
	try {
		const res: any = await request.get(`/tenant/members/${memberId.value}/api-keys`, {
			params: { page: 1, page_size: 1 },
		})
		const raw = res.data?.data
		apiKeyCount.value = raw?.total || 0
	} catch {
		apiKeyCount.value = 0
	}
}

async function changeRole(newRole: 'admin' | 'member') {
	if (!member.value) return
	roleLoading.value = true
	try {
		await request.put(`/tenant/members/${member.value.id}/role`, { role: newRole })
		if (member.value) member.value.role = newRole
		toast.success('角色已更新')
	} catch {
	} finally {
		roleLoading.value = false
	}
}

async function handleResetPassword() {
	if (!resetPassword.value) return
	resetLoading.value = true
	try {
		await request.put(`/tenant/members/${memberId.value}/reset-password`, {
			password: resetPassword.value,
		})
		toast.success('密码重置成功')
		showResetModal.value = false
		resetPassword.value = ''
	} catch {
	} finally {
		resetLoading.value = false
	}
}

async function handleRemove() {
	removeLoading.value = true
	try {
		await request.delete(`/tenant/members/${memberId.value}`)
		toast.success('成员已移除')
		router.push('/tenant/members')
	} catch {
	} finally {
		removeLoading.value = false
	}
}

// === Quota ===

const quotaPercent = computed(() => {
	if (!quota.value || quota.value.quota_type === 'none' || quota.value.quota_limit <= 0) return 0
	return Math.min((quota.value.quota_used / quota.value.quota_limit) * 100, 100)
})

const quotaBarColor = computed(() => {
	const p = quotaPercent.value
	if (p >= 100) return 'bg-red-500'
	if (p >= 80) return 'bg-amber-500'
	return 'bg-primary-500'
})

const quotaTypeLabel: Record<string, string> = {
	none: '不限制',
	total: '总额度',
	periodic: '周期性',
}

const periodLabel: Record<string, string> = {
	day: '按天',
	week: '按周',
	month: '按月',
}

async function fetchQuota() {
	try {
		const res: any = await request.get(`/tenant/members/${memberId.value}/quota`)
		quota.value = res.data?.data || null
	} catch {
		quota.value = null
	}
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

async function fetchMemberModels() {
	try {
		const res: any = await request.get(`/tenant/members/${memberId.value}/model-scopes`)
		memberModelIds.value = res.data?.data?.model_ids || []
	} catch {
		memberModelIds.value = []
	}
}

function openQuotaModal() {
	quotaForm.value = {
		quota_type: quota.value?.quota_type || 'none',
		quota_limit: quota.value?.quota_limit || 0,
		period: quota.value?.period || 'month',
	}
	showQuotaModal.value = true
}

async function handleSaveQuota() {
	quotaSaving.value = true
	try {
		await request.put(`/tenant/members/${memberId.value}/quota`, quotaForm.value)
		toast.success('额度设置已保存')
		showQuotaModal.value = false
		fetchQuota()
	} catch {
	} finally {
		quotaSaving.value = false
	}
}

// === Model scope ===

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

const memberModelNames = computed(() => {
	if (memberModelIds.value.length === 0) return []
	return allModels.value.filter(m => memberModelIds.value.includes(m.id))
})

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

function openModelModal() {
	selectedModelIds.value = [...memberModelIds.value]
	modelSearch.value = ''
	showModelModal.value = true
}

function toggleModel(id: number) {
	const idx = selectedModelIds.value.indexOf(id)
	if (idx >= 0) {
		selectedModelIds.value.splice(idx, 1)
	} else {
		selectedModelIds.value.push(id)
	}
}

function selectAllModels() {
	selectedModelIds.value = allModels.value.map(m => m.id)
}

function clearAllModels() {
	selectedModelIds.value = []
}

async function handleSaveModels() {
	modelSaving.value = true
	try {
		await request.put(`/tenant/members/${memberId.value}/model-scopes`, {
			model_ids: selectedModelIds.value,
		})
		toast.success('模型范围已保存')
		showModelModal.value = false
		memberModelIds.value = [...selectedModelIds.value]
	} catch {
	} finally {
		modelSaving.value = false
	}
}

function goBack() {
	router.push('/tenant/members')
}

onMounted(() => {
	fetchMemberDetail()
	fetchUsage()
	fetchApiKeyCount()
	fetchQuota()
	fetchAllModels()
	fetchMemberModels()
})
</script>

<template>
	<TeamLockedBanner v-if="!teamEnabled" />
	<div v-else class="space-y-8">
		<!-- Back navigation -->
		<button
			@click="goBack"
			class="btn btn-ghost btn-sm text-gray-500 hover:text-gray-700 -ml-2"
		>
			<Icon name="chevronLeft" size="sm" />
			返回成员列表
		</button>

		<!-- Loading state -->
		<div v-if="loading" class="flex justify-center py-12">
			<div class="spinner h-8 w-8 border-primary-500"></div>
		</div>

		<!-- Not found -->
		<div v-else-if="!member" class="empty-state">
			<Icon name="user" size="xl" class="empty-state-icon" />
			<p class="empty-state-title">成员不存在</p>
			<p class="empty-state-description">该成员可能已被移除</p>
		</div>

		<!-- Member Detail -->
		<template v-else>
			<!-- Profile Header -->
			<div class="card relative z-20 p-6">
				<div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
					<div class="flex items-center gap-4">
						<div
							class="h-14 w-14 rounded-2xl flex items-center justify-center text-white text-xl font-bold flex-shrink-0 bg-gradient-to-br from-primary-500 to-primary-600 shadow-glow"
						>
							{{ member.username.charAt(0).toUpperCase() }}
						</div>
						<div>
							<div class="flex items-center gap-3">
								<h1 class="text-2xl font-bold text-gray-900">{{ member.display_name || member.username }}</h1>
								<span class="badge" :class="roleBadgeClass[member.role]">
									{{ roleLabel[member.role] || member.role }}
								</span>
								<span class="badge" :class="statusBadgeClass[member.status] || 'badge-gray'">
									<span class="h-1.5 w-1.5 rounded-full" :class="statusDotClass[member.status] || 'bg-gray-400'"></span>
									{{ statusLabel[member.status] || member.status }}
								</span>
							</div>
							<p class="text-sm text-gray-500 mt-0.5">@{{ authStore.tenant?.code }}</p>
						</div>
					</div>

					<!-- Action buttons (only for non-owner) -->
					<div v-if="canManage" class="flex items-center gap-2 flex-shrink-0">
						<div class="relative">
							<BaseSelect
								:model-value="member.role"
								@change="changeRole"
								:disabled="roleLoading"
								:options="[{value:'admin',label:'管理员'},{value:'member',label:'成员'}]" />
						</div>
						<button
							@click="showResetModal = true"
							class="btn btn-secondary btn-sm"
						>
							<Icon name="key" size="xs" />
							重置密码
						</button>
						<button
							v-if="isLocked"
							@click="handleUnlock"
							class="btn btn-secondary btn-sm"
						>
							<Icon name="refresh" size="xs" />
							解除锁定
						</button>
						<button
							@click="showRemoveModal = true"
							class="btn btn-ghost btn-sm text-red-600 hover:bg-red-50"
						>
							<Icon name="trash" size="xs" />
							移除
						</button>
					</div>
				</div>
			</div>

			<!-- Stats Cards -->
			<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4">
				<article
					v-for="card in statCards"
					:key="card.label"
					class="stat-card member-stat-card"
					:style="{ '--stat-color': card.color, '--stat-soft': card.soft }"
				>
					<div class="member-stat-accent" aria-hidden="true"></div>
					<div class="flex items-center justify-between gap-3">
						<div class="member-stat-icon">
							<Icon :name="card.icon" size="md" />
						</div>
						<p class="member-stat-value">{{ card.value }}</p>
					</div>
					<div class="mt-2.5 flex items-baseline justify-between gap-3">
						<p class="flex-shrink-0 text-sm font-medium text-slate-600">{{ card.label }}</p>
						<p v-if="card.detail" class="member-stat-unit" :title="card.detail">{{ card.detail }}</p>
					</div>
				</article>
			</div>

			<!-- Info Grid -->
			<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
				<!-- Personal Info -->
				<div class="card">
					<div class="card-header">
						<h3 class="text-lg font-semibold text-gray-900">个人信息</h3>
					</div>
					<div class="card-body space-y-4">
						<div class="flex items-center justify-between py-2 border-b border-gray-100">
							<span class="text-sm text-gray-500">用户名</span>
							<span class="text-sm font-medium text-gray-900">{{ member.username }}</span>
						</div>
						<div class="flex items-center justify-between py-2 border-b border-gray-100">
							<span class="text-sm text-gray-500">显示名称</span>
							<span class="text-sm font-medium text-gray-900">{{ member.display_name || '--' }}</span>
						</div>
						<div class="flex items-center justify-between py-2 border-b border-gray-100">
							<span class="text-sm text-gray-500">邮箱</span>
							<span class="text-sm font-medium text-gray-900">{{ member.email || '--' }}</span>
						</div>
						<div class="flex items-center justify-between py-2 border-b border-gray-100">
							<span class="text-sm text-gray-500">角色</span>
							<span class="badge" :class="roleBadgeClass[member.role]">
								{{ roleLabel[member.role] || member.role }}
							</span>
						</div>
						<div class="flex items-center justify-between py-2 border-b border-gray-100">
							<span class="text-sm text-gray-500">状态</span>
							<span class="badge" :class="statusBadgeClass[member.status] || 'badge-gray'">
								<span class="h-1.5 w-1.5 rounded-full" :class="statusDotClass[member.status] || 'bg-gray-400'"></span>
								{{ statusLabel[member.status] || member.status }}
							</span>
						</div>
						<div class="flex items-center justify-between py-2 border-b border-gray-100">
							<span class="text-sm text-gray-500">加入时间</span>
							<span class="text-sm text-gray-700">{{ formatDate(member.created_at) }}</span>
						</div>
						<div class="flex items-center justify-between py-2">
							<span class="text-sm text-gray-500">最后更新</span>
							<span class="text-sm text-gray-700">{{ formatDate(member.updated_at) }}</span>
						</div>
					</div>
				</div>

				<!-- Usage Detail -->
				<div class="card">
					<div class="card-header">
						<h3 class="text-lg font-semibold text-gray-900">用量概览</h3>
					</div>
					<div class="card-body">
						<div v-if="usage" class="space-y-4">
							<div class="flex items-center justify-between py-2 border-b border-gray-100">
								<span class="text-sm text-gray-500">今日请求数</span>
								<span class="text-sm font-medium text-gray-900">{{ formatNumber(usage.today_requests) }}</span>
							</div>
							<div class="flex items-center justify-between py-2 border-b border-gray-100">
								<span class="text-sm text-gray-500">本月请求数</span>
								<span class="text-sm font-medium text-gray-900">{{ formatNumber(usage.month_requests) }}</span>
							</div>
							<div class="flex items-center justify-between py-2 border-b border-gray-100">
								<span class="text-sm text-gray-500">本月输入 Token</span>
								<span class="text-sm font-medium text-gray-900">{{ formatNumber(usage.month_input_tokens) }}</span>
							</div>
							<div class="flex items-center justify-between py-2 border-b border-gray-100">
								<span class="text-sm text-gray-500">本月输出 Token</span>
								<span class="text-sm font-medium text-gray-900">{{ formatNumber(usage.month_output_tokens) }}</span>
							</div>
							<div class="flex items-center justify-between py-2 border-b border-gray-100">
								<span class="text-sm text-gray-500">本月消费</span>
								<span class="text-sm font-semibold text-primary-600">{{ formatBilling(usage.month_total_cost, 4) }}</span>
							</div>
							<div class="flex items-center justify-between py-2">
								<span class="text-sm text-gray-500">API Key 数量</span>
								<span class="text-sm font-medium text-gray-900">{{ apiKeyCount }}</span>
							</div>
						</div>
						<div v-else class="py-8 text-center text-sm text-gray-400">
							暂无用量数据
						</div>
					</div>
				</div>
			</div>
			<!-- Quota & Model Scope Cards -->
			<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
				<!-- Quota Card -->
				<div class="card">
					<div class="card-header flex items-center justify-between">
						<h3 class="text-lg font-semibold text-gray-900">额度限制</h3>
						<button v-if="canManage" @click="openQuotaModal" class="btn btn-ghost btn-sm">
							<Icon name="edit" size="xs" />
							编辑
						</button>
					</div>
					<div class="card-body">
						<div v-if="!quota" class="py-4 text-center text-sm text-gray-400">加载中...</div>
						<div v-else-if="quota.quota_type === 'none'" class="flex items-center gap-3 py-2">
							<div class="h-10 w-10 rounded-xl bg-gray-100 flex items-center justify-center">
								<Icon name="checkCircle" size="md" class="text-emerald-500" />
							</div>
							<div>
								<p class="text-sm font-medium text-gray-900">不限制</p>
								<p class="text-xs text-gray-500">该成员没有额度上限</p>
							</div>
						</div>
						<div v-else class="space-y-4">
							<div class="flex items-center justify-between">
								<span class="badge" :class="quota.quota_type === 'periodic' ? 'badge-primary' : 'badge-warning'">
									{{ quotaTypeLabel[quota.quota_type] }}
									<template v-if="quota.quota_type === 'periodic'"> · {{ periodLabel[quota.period] }}</template>
								</span>
								<span class="text-sm text-gray-500">
									{{ formatBilling(quota.quota_used, 6) }} / {{ formatBilling(quota.quota_limit, 6) }}
								</span>
							</div>
							<div class="h-2 bg-gray-100 rounded-full overflow-hidden">
								<div
									class="h-full rounded-full transition-all duration-300"
									:class="quotaBarColor"
									:style="{ width: quotaPercent + '%' }"
								></div>
							</div>
							<div class="flex items-center justify-between text-xs text-gray-500">
								<span>已使用 {{ quotaPercent.toFixed(1) }}%</span>
								<span v-if="quota.quota_type === 'periodic' && quota.next_reset_at">
									下次重置：{{ quota.next_reset_at }}
								</span>
							</div>
						</div>
					</div>
				</div>

				<!-- Model Scope Card -->
				<div class="card">
					<div class="card-header flex items-center justify-between">
						<h3 class="text-lg font-semibold text-gray-900">可用模型</h3>
						<button v-if="canManage" @click="openModelModal" class="btn btn-ghost btn-sm">
							<Icon name="edit" size="xs" />
							编辑
						</button>
					</div>
					<div class="card-body">
						<div v-if="memberModelIds.length === 0" class="flex items-center gap-3 py-2">
							<div class="h-10 w-10 rounded-xl bg-gray-100 flex items-center justify-center">
								<Icon name="checkCircle" size="md" class="text-emerald-500" />
							</div>
							<div>
								<p class="text-sm font-medium text-gray-900">不限制</p>
								<p class="text-xs text-gray-500">可使用所有租户可用模型</p>
							</div>
						</div>
						<div v-else class="space-y-3">
							<p class="text-xs text-gray-500">已授权 {{ memberModelNames.length }} 个模型</p>
							<div class="flex flex-wrap gap-2">
								<span
									v-for="m in memberModelNames"
									:key="m.id"
									class="badge"
									:class="categoryBadgeClass[m.category] || 'badge-gray'"
								>
									{{ m.model_name || m.model_id }}
								</span>
							</div>
						</div>
					</div>
				</div>
			</div>
		</template>

		<!-- Reset Password Modal -->
		<BaseModal
			:show="showResetModal"
			title="重置密码"
			width="narrow"
			@close="showResetModal = false; resetPassword = ''"
		>
			<div class="space-y-4">
				<p class="text-sm text-gray-500">
					为 <span class="font-medium text-gray-700">{{ member?.display_name || member?.username }}</span> 设置新密码
				</p>
				<div>
					<label class="input-label">新密码 <span class="text-red-500">*</span></label>
					<n-input
						v-model:value="resetPassword"
						type="password"
						show-password-on="click"
						placeholder="至少 8 位，含字母和数字"
						@keyup.enter="handleResetPassword"
					/>
				</div>
			</div>
			<template #footer>
				<div class="flex justify-end gap-3">
					<button class="btn btn-secondary" @click="showResetModal = false; resetPassword = ''">取消</button>
					<button
						class="btn btn-primary"
						:disabled="resetLoading || !resetPassword"
						@click="handleResetPassword"
					>
						{{ resetLoading ? '重置中...' : '确认重置' }}
					</button>
				</div>
			</template>
		</BaseModal>

		<!-- Remove Member Modal -->
		<BaseModal
			:show="showRemoveModal"
			title="移除成员"
			width="narrow"
			@close="showRemoveModal = false"
		>
			<div class="space-y-4">
				<div class="flex items-center gap-3 p-4 bg-red-50 rounded-xl">
					<div class="h-10 w-10 rounded-full bg-red-100 flex items-center justify-center flex-shrink-0">
						<Icon name="exclamationTriangle" size="md" class="text-red-600" />
					</div>
					<div>
						<p class="text-sm font-medium text-red-900">确认移除该成员？</p>
						<p class="text-xs text-red-700 mt-0.5">此操作不可撤销，该成员的所有访问权限将被立即撤销。</p>
					</div>
				</div>
				<p class="text-sm text-gray-500">
					即将移除：<span class="font-medium text-gray-700">{{ member?.display_name || member?.username }}</span>
				</p>
			</div>
			<template #footer>
				<div class="flex justify-end gap-3">
					<button class="btn btn-secondary" @click="showRemoveModal = false">取消</button>
					<button
						class="btn btn-danger"
						:disabled="removeLoading"
						@click="handleRemove"
					>
						{{ removeLoading ? '移除中...' : '确认移除' }}
					</button>
				</div>
			</template>
		</BaseModal>

		<!-- Quota Edit Modal -->
		<BaseModal
			:show="showQuotaModal"
			title="设置额度限制"
			width="normal"
			@close="showQuotaModal = false"
		>
			<div class="space-y-5">
				<!-- Quota Type Selector -->
				<div>
					<label class="input-label">额度类型</label>
					<div class="grid grid-cols-3 gap-3 mt-1.5">
						<button
							v-for="opt in [
								{ value: 'none', label: '不限制', desc: '无额度上限' },
								{ value: 'total', label: '总额度', desc: '固定总额上限' },
								{ value: 'periodic', label: '周期性', desc: '按周期重置' },
							]"
							:key="opt.value"
							@click="quotaForm.quota_type = opt.value"
							class="p-3 rounded-xl border-2 text-left transition-all duration-150"
							:class="quotaForm.quota_type === opt.value
								? 'border-primary-500 bg-primary-50'
								: 'border-gray-200 hover:border-gray-300'"
						>
							<p class="text-sm font-medium" :class="quotaForm.quota_type === opt.value ? 'text-primary-700' : 'text-gray-900'">
								{{ opt.label }}
							</p>
							<p class="text-xs mt-0.5" :class="quotaForm.quota_type === opt.value ? 'text-primary-500' : 'text-gray-500'">
								{{ opt.desc }}
							</p>
						</button>
					</div>
				</div>

				<!-- Quota Limit -->
				<div v-if="quotaForm.quota_type !== 'none'">
					<label class="input-label">额度上限 ({{ displayCurrency }})</label>
					<n-input
						:value="String(quotaForm.quota_limit)"
						type="number"
						step="0.01"
						min="0"
						placeholder="输入额度上限"
						@update:value="(v: string | null) => { quotaForm.quota_limit = Number(v) }"
					/>
					<p class="input-hint">设置为 0 表示不限制使用量</p>
				</div>

				<!-- Period -->
				<div v-if="quotaForm.quota_type === 'periodic'">
					<label class="input-label">重置周期</label>
					<BaseSelect v-model="quotaForm.period" :options="[{value:'day',label:'按天'},{value:'week',label:'按周'},{value:'month',label:'按月'}]" />
					<p class="input-hint">额度在每个周期开始时自动重置为 0</p>
				</div>
			</div>
			<template #footer>
				<div class="flex justify-end gap-3">
					<button class="btn btn-secondary" @click="showQuotaModal = false">取消</button>
					<button
						class="btn btn-primary"
						:disabled="quotaSaving"
						@click="handleSaveQuota"
					>
						{{ quotaSaving ? '保存中...' : '保存' }}
					</button>
				</div>
			</template>
		</BaseModal>

		<!-- Model Scope Edit Modal -->
		<BaseModal
			:show="showModelModal"
			title="设置可用模型"
			width="wide"
			@close="showModelModal = false"
		>
			<div class="space-y-4">
				<p class="text-sm text-gray-500">
					选择该成员可以使用的模型。留空表示不限制，成员可使用所有租户可用模型。
				</p>

				<!-- Search + Actions -->
				<div class="flex items-center gap-3">
					<div class="flex-1">
						<n-input
							v-model:value="modelSearch"
							type="text"
							placeholder="搜索模型..."
						>
							<template #prefix><Icon name="search" size="sm" class="text-gray-400" /></template>
						</n-input>
					</div>
					<button class="btn btn-ghost btn-sm" @click="selectAllModels">全选</button>
					<button class="btn btn-ghost btn-sm" @click="clearAllModels">清空</button>
				</div>

				<!-- Model List -->
				<div class="max-h-80 overflow-y-auto border border-gray-200 rounded-xl divide-y divide-gray-100">
					<template v-for="(items, cat) in groupedFilteredModels" :key="cat">
						<div class="px-4 py-2 bg-gray-50 text-xs font-semibold text-gray-500 uppercase tracking-wider sticky top-0">
							{{ categoryLabel[cat as string] || cat }}
							<span class="text-gray-300">({{ items.length }})</span>
						</div>
						<label
							v-for="m in items"
							:key="m.id"
							class="flex items-center gap-3 px-4 py-2.5 hover:bg-gray-50 cursor-pointer"
						>
							<n-checkbox
								:checked="selectedModelIds.includes(m.id)"
								@update:checked="toggleModel(m.id)"
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
					<div v-if="filteredModels.length === 0" class="px-4 py-8 text-center text-sm text-gray-400">
						没有匹配的模型
					</div>
				</div>

				<p class="text-xs text-gray-500">
					已选择 <span class="font-medium text-gray-700">{{ selectedModelIds.length }}</span> 个模型
					<template v-if="selectedModelIds.length === 0">（不限制）</template>
				</p>
			</div>
			<template #footer>
				<div class="flex justify-end gap-3">
					<button class="btn btn-secondary" @click="showModelModal = false">取消</button>
					<button
						class="btn btn-primary"
						:disabled="modelSaving"
						@click="handleSaveModels"
					>
						{{ modelSaving ? '保存中...' : '保存' }}
					</button>
				</div>
			</template>
		</BaseModal>
	</div>
</template>

<style scoped>
/* ============ 成员统计卡片（对齐仪表盘 metric-card 风格） ============ */
.member-stat-card {
	min-height: 108px;
	display: flex;
	flex-direction: column;
	padding: 1.125rem 1.25rem;
	transition: transform 220ms ease, box-shadow 220ms ease;
}

.member-stat-card:hover {
	transform: translateY(-3px);
	box-shadow: 0 20px 45px rgba(15, 23, 42, 0.12);
}

.member-stat-accent {
	position: absolute;
	top: 0;
	right: 1.25rem;
	left: 1.25rem;
	height: 2px;
	border-radius: 0 0 9999px 9999px;
	background: linear-gradient(90deg, transparent, var(--stat-color), transparent);
	opacity: 0.7;
}

.member-stat-icon {
	display: flex;
	height: 2.5rem;
	width: 2.5rem;
	flex-shrink: 0;
	align-items: center;
	justify-content: center;
	border: 1px solid rgba(255, 255, 255, 0.82);
	border-radius: 0.75rem;
	background: var(--stat-soft);
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.9);
	color: var(--stat-color);
}

.member-stat-value {
	min-width: 0;
	overflow: hidden;
	color: #172033;
	font-size: 1.75rem;
	font-weight: 750;
	font-variant-numeric: tabular-nums;
	line-height: 1;
	text-align: right;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.member-stat-unit {
	min-width: 0;
	overflow: hidden;
	color: #94a3b8;
	font-size: 0.6875rem;
	text-align: right;
	text-overflow: ellipsis;
	white-space: nowrap;
}
</style>

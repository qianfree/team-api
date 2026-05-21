<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import BaseModal from '@/components/common/BaseModal.vue'
import Icon from '@/components/common/Icon.vue'
import BaseSelect from '../../components/common/BaseSelect.vue'
import request from '@/utils/request'
import { toast } from '@/utils/toast'

interface FeedbackItem {
	id: number
	category: string
	title: string
	description: string
	status: string
	priority: string
	admin_reply: string
	resolution: string
	created_at: string
	updated_at: string
}

const feedbacks = ref<FeedbackItem[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = 20
const total = ref(0)

const filterStatus = ref('')
const filterCategory = ref('')

const showCreateModal = ref(false)
const createLoading = ref(false)
const createForm = reactive({
	category: 'suggestion' as string,
	title: '',
	description: '',
})

const showDetailModal = ref(false)
const detailLoading = ref(false)
const detailFeedback = ref<FeedbackItem | null>(null)

const categoryOptions = [
	{ label: 'Bug 报告', value: 'bug_report' },
	{ label: '功能建议', value: 'feature_request' },
	{ label: '改进建议', value: 'suggestion' },
	{ label: '投诉', value: 'complaint' },
]

const statusOptions = [
	{ label: '待处理', value: 'pending' },
	{ label: '已确认', value: 'acknowledged' },
	{ label: '处理中', value: 'in_progress' },
	{ label: '已解决', value: 'resolved' },
	{ label: '已关闭', value: 'closed' },
]

const statusLabel: Record<string, string> = {
	pending: '待处理',
	acknowledged: '已确认',
	in_progress: '处理中',
	resolved: '已解决',
	closed: '已关闭',
}

const statusBadgeClass: Record<string, string> = {
	pending: 'badge-primary',
	acknowledged: 'badge-purple',
	in_progress: 'badge-warning',
	resolved: 'badge-success',
	closed: 'badge-gray',
}

const categoryLabel: Record<string, string> = {
	bug_report: 'Bug 报告',
	feature_request: '功能建议',
	suggestion: '改进建议',
	complaint: '投诉',
}

const categoryIcon: Record<string, string> = {
	bug_report: 'exclamationTriangle',
	feature_request: 'trendingUp',
	suggestion: 'chat',
	complaint: 'exclamationCircle',
}

const totalPages = computed(() => Math.ceil(total.value / pageSize))

const filterStatusOptions = computed(() => [{value:'',label:'全部状态'}, ...statusOptions])
const filterCategoryOptions = computed(() => [{value:'',label:'全部类型'}, ...categoryOptions])

async function fetchFeedbacks() {
	loading.value = true
	try {
		const params: any = { page: page.value, page_size: pageSize }
		if (filterStatus.value) params.status = filterStatus.value
		if (filterCategory.value) params.category = filterCategory.value
		const res: any = await request.get('/tenant/feedbacks', { params })
		const raw = res.data?.data
		feedbacks.value = raw?.list || []
		total.value = raw?.total || 0
	} catch {
		feedbacks.value = []
		total.value = 0
	} finally {
		loading.value = false
	}
}

async function createFeedback() {
	if (!createForm.title.trim()) { toast.warning('请输入标题'); return }
	if (!createForm.description.trim()) { toast.warning('请输入描述'); return }
	createLoading.value = true
	try {
		await request.post('/tenant/feedbacks', {
			category: createForm.category,
			title: createForm.title,
			description: createForm.description,
		})
		toast.success('反馈提交成功')
		showCreateModal.value = false
		createForm.category = 'suggestion'
		createForm.title = ''
		createForm.description = ''
		fetchFeedbacks()
	} catch {
	} finally {
		createLoading.value = false
	}
}

async function openDetail(fb: FeedbackItem) {
	detailFeedback.value = fb
	showDetailModal.value = true
	detailLoading.value = true
	try {
		const res: any = await request.get(`/tenant/feedbacks/${fb.id}`)
		detailFeedback.value = res.data?.data || fb
	} catch {
	} finally {
		detailLoading.value = false
	}
}

function handlePageChange(newPage: number) {
	page.value = newPage
	fetchFeedbacks()
}

function handleFilter() {
	page.value = 1
	fetchFeedbacks()
}

function resetCreateForm() {
	createForm.category = 'suggestion'
	createForm.title = ''
	createForm.description = ''
}

onMounted(() => {
	fetchFeedbacks()
})
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="page-header flex items-center justify-between">
			<div>
				<h1 class="page-title">意见反馈</h1>
				<p class="page-description">提交 Bug 报告、功能建议或改进意见</p>
			</div>
			<button class="btn btn-primary" @click="showCreateModal = true">
				<Icon name="plus" size="sm" />
				提交反馈
			</button>
		</div>

		<!-- Filters -->
		<div class="card p-4">
			<div class="flex flex-wrap items-center gap-3">
				<BaseSelect v-model="filterStatus" :options="filterStatusOptions" @change="handleFilter" />
				<BaseSelect v-model="filterCategory" :options="filterCategoryOptions" @change="handleFilter" />
				<button
					v-if="filterStatus || filterCategory"
					class="btn btn-ghost btn-sm"
					@click="filterStatus = ''; filterCategory = ''; handleFilter()"
				>
					清除筛选
				</button>
			</div>
		</div>

		<!-- Loading -->
		<div v-if="loading" class="card p-8 text-center">
			<div class="spinner mx-auto mb-3"></div>
			<p class="text-sm text-gray-500">加载中...</p>
		</div>

		<!-- Empty -->
		<div v-else-if="feedbacks.length === 0" class="empty-state card">
			<Icon name="chat" size="xl" class="empty-state-icon text-gray-300" />
			<p class="empty-state-title">暂无反馈记录</p>
			<p class="empty-state-description">有什么想法或建议？提交一条反馈告诉我们</p>
			<button class="btn btn-primary mt-4" @click="showCreateModal = true">
				<Icon name="plus" size="sm" />
				提交反馈
			</button>
		</div>

		<!-- Feedback List (Card Layout) -->
		<div v-else class="space-y-3">
			<div
				v-for="fb in feedbacks"
				:key="fb.id"
				class="card card-hover p-5 cursor-pointer transition-all duration-200"
				@click="openDetail(fb)"
			>
				<div class="flex items-start justify-between gap-4">
					<div class="flex items-start gap-3 min-w-0 flex-1">
						<div
							class="h-9 w-9 rounded-xl flex items-center justify-center flex-shrink-0"
							:class="{
								'bg-red-100 text-red-500': fb.category === 'bug_report',
								'bg-primary-100 text-primary-500': fb.category === 'feature_request',
								'bg-amber-100 text-amber-500': fb.category === 'suggestion',
								'bg-purple-100 text-purple-500': fb.category === 'complaint',
							}"
						>
							<Icon :name="categoryIcon[fb.category] || 'chat'" size="md" />
						</div>
						<div class="min-w-0 flex-1">
							<div class="flex items-center gap-2 mb-1">
								<h3 class="text-sm font-semibold text-gray-900 truncate">{{ fb.title }}</h3>
								<span class="badge" :class="statusBadgeClass[fb.status] || 'badge-gray'">
									{{ statusLabel[fb.status] || fb.status }}
								</span>
							</div>
							<p class="text-xs text-gray-500 truncate">{{ fb.description }}</p>
							<div class="flex items-center gap-3 mt-2 text-xs text-gray-400">
								<span class="badge badge-gray text-xs">{{ categoryLabel[fb.category] || fb.category }}</span>
								<span>{{ fb.created_at ? new Date(fb.created_at).toLocaleString() : '' }}</span>
							</div>
						</div>
					</div>
					<Icon name="chevronRight" size="sm" class="text-gray-300 flex-shrink-0 mt-1" />
				</div>
				<!-- Admin reply preview -->
				<div v-if="fb.admin_reply" class="mt-3 ml-12 p-3 bg-primary-50/50 rounded-xl border border-primary-100">
					<div class="flex items-center gap-1.5 mb-1">
						<Icon name="checkCircle" size="xs" class="text-primary-500" />
						<span class="text-xs font-medium text-primary-600">管理员回复</span>
					</div>
					<p class="text-xs text-gray-600 truncate">{{ fb.admin_reply }}</p>
				</div>
			</div>

			<!-- Pagination -->
			<div v-if="totalPages > 1" class="card flex items-center justify-between px-6 py-3">
				<span class="text-xs text-gray-500">共 {{ total }} 条记录</span>
				<div class="flex items-center gap-2">
					<button
						class="btn btn-ghost btn-sm"
						:disabled="page <= 1"
						@click="handlePageChange(page - 1)"
					>
						上一页
					</button>
					<span class="text-sm text-gray-600">{{ page }} / {{ totalPages }}</span>
					<button
						class="btn btn-ghost btn-sm"
						:disabled="page >= totalPages"
						@click="handlePageChange(page + 1)"
					>
						下一页
					</button>
				</div>
			</div>
		</div>

		<!-- Create Feedback Modal -->
		<BaseModal
			:show="showCreateModal"
			title="提交反馈"
			width="wide"
			@close="showCreateModal = false"
		>
			<div class="space-y-4">
				<div>
					<label class="input-label">反馈类型</label>
					<div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
						<button
							v-for="opt in categoryOptions"
							:key="opt.value"
							class="flex flex-col items-center gap-1.5 rounded-xl border-2 px-3 py-3 text-sm transition-all"
							:class="createForm.category === opt.value
								? 'border-primary-500 bg-primary-50 text-primary-600'
								: 'border-gray-200 text-gray-600 hover:border-gray-300'"
							@click="createForm.category = opt.value"
						>
							<Icon :name="categoryIcon[opt.value]" size="md" />
							<span class="text-xs font-medium">{{ opt.label }}</span>
						</button>
					</div>
				</div>
				<div>
					<label class="input-label">标题 <span class="text-red-500">*</span></label>
					<input
						v-model="createForm.title"
						type="text"
						class="input"
						placeholder="一句话概括您的反馈"
						maxlength="200"
					/>
				</div>
				<div>
					<label class="input-label">详细描述 <span class="text-red-500">*</span></label>
					<textarea
						v-model="createForm.description"
						class="input"
						rows="5"
						placeholder="请详细描述您遇到的问题或建议，包括相关的操作步骤、截图等信息"
						maxlength="5000"
					></textarea>
					<p class="input-hint">{{ createForm.description.length }} / 5000</p>
				</div>
			</div>
			<template #footer>
				<div class="flex justify-end gap-3">
					<button class="btn btn-secondary" @click="showCreateModal = false; resetCreateForm()">取消</button>
					<button
						class="btn btn-primary"
						:disabled="createLoading || !createForm.title.trim() || !createForm.description.trim()"
						@click="createFeedback"
					>
						{{ createLoading ? '提交中...' : '提交反馈' }}
					</button>
				</div>
			</template>
		</BaseModal>

		<!-- Detail Modal -->
		<BaseModal
			:show="showDetailModal"
			:title="'反馈 #' + (detailFeedback?.id || '')"
			width="wide"
			@close="showDetailModal = false"
		>
			<div v-if="detailLoading" class="text-center py-8">
				<div class="spinner mx-auto mb-3"></div>
				<p class="text-sm text-gray-500">加载中...</p>
			</div>
			<div v-else-if="detailFeedback" class="space-y-5">
				<!-- Header Info -->
				<div class="flex items-start justify-between gap-4">
					<div class="flex items-start gap-3 min-w-0 flex-1">
						<div
							class="h-10 w-10 rounded-xl flex items-center justify-center flex-shrink-0"
							:class="{
								'bg-red-100 text-red-500': detailFeedback.category === 'bug_report',
								'bg-primary-100 text-primary-500': detailFeedback.category === 'feature_request',
								'bg-amber-100 text-amber-500': detailFeedback.category === 'suggestion',
								'bg-purple-100 text-purple-500': detailFeedback.category === 'complaint',
							}"
						>
							<Icon :name="categoryIcon[detailFeedback.category] || 'chat'" size="lg" />
						</div>
						<div class="min-w-0">
							<h3 class="text-lg font-semibold text-gray-900">{{ detailFeedback.title }}</h3>
							<div class="flex items-center gap-2 mt-1">
								<span class="badge" :class="statusBadgeClass[detailFeedback.status] || 'badge-gray'">
									{{ statusLabel[detailFeedback.status] || detailFeedback.status }}
								</span>
								<span class="badge badge-gray">{{ categoryLabel[detailFeedback.category] || detailFeedback.category }}</span>
							</div>
						</div>
					</div>
				</div>

				<!-- Description -->
				<div class="p-4 bg-gray-50 rounded-xl">
					<h4 class="text-xs font-medium text-gray-500 uppercase tracking-wider mb-2">描述</h4>
					<p class="text-sm text-gray-700 whitespace-pre-wrap leading-relaxed">{{ detailFeedback.description }}</p>
				</div>

				<!-- Timestamps -->
				<div class="flex items-center gap-4 text-xs text-gray-400">
					<span>提交于 {{ detailFeedback.created_at ? new Date(detailFeedback.created_at).toLocaleString() : '-' }}</span>
					<span v-if="detailFeedback.updated_at && detailFeedback.updated_at !== detailFeedback.created_at">
						更新于 {{ new Date(detailFeedback.updated_at).toLocaleString() }}
					</span>
				</div>

				<!-- Admin Reply -->
				<div v-if="detailFeedback.admin_reply" class="p-4 bg-primary-50/50 rounded-xl border border-primary-100">
					<div class="flex items-center gap-2 mb-3">
						<div class="h-7 w-7 rounded-full bg-gradient-to-r from-primary-500 to-primary-600 flex items-center justify-center text-white text-xs font-medium">
							管
						</div>
						<div>
							<span class="text-sm font-medium text-primary-600">管理员回复</span>
						</div>
					</div>
					<p class="text-sm text-gray-700 whitespace-pre-wrap leading-relaxed">{{ detailFeedback.admin_reply }}</p>
				</div>

				<!-- Resolution -->
				<div v-if="detailFeedback.resolution" class="p-4 bg-emerald-50/50 rounded-xl border border-emerald-100">
					<div class="flex items-center gap-2 mb-2">
						<Icon name="checkCircle" size="sm" class="text-emerald-500" />
						<span class="text-sm font-medium text-emerald-600">处理结果</span>
					</div>
					<p class="text-sm text-gray-700 whitespace-pre-wrap">{{ detailFeedback.resolution }}</p>
				</div>
			</div>
		</BaseModal>
	</div>
</template>

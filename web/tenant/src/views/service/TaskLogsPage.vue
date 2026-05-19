<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import Icon from '@/components/common/Icon.vue'
import BaseModal from '@/components/common/BaseModal.vue'
import request from '@/utils/request'

interface TaskItem {
	id: number
	public_task_id: string
	platform: string
	action: string
	status: string
	progress: string
	model_name: string
	fail_reason?: string
	pre_deduct_amount: number
	actual_cost: number
	billing_settled: boolean
	result_url?: string
	username?: string
	submit_time?: string
	finish_time?: string
	created_at: string
}

const loading = ref(false)
const tasks = ref<TaskItem[]>([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const totalPages = computed(() => Math.ceil(total.value / pageSize))

const filterStatus = ref('')
const filterPlatform = ref('')
const filterTaskId = ref('')

const showDetail = ref(false)
const detailLoading = ref(false)
const detailTask = ref<TaskItem | null>(null)

const statusBadge: Record<string, string> = {
	NOT_START: 'bg-gray-100 text-gray-800',
	SUBMITTED: 'bg-blue-100 text-blue-800',
	IN_PROGRESS: 'bg-amber-100 text-amber-800',
	SUCCESS: 'bg-emerald-100 text-emerald-800',
	FAILURE: 'bg-red-100 text-red-800',
	TIMEOUT: 'bg-orange-100 text-orange-800',
}

const statusLabel: Record<string, string> = {
	NOT_START: '未开始',
	SUBMITTED: '已提交',
	IN_PROGRESS: '进行中',
	SUCCESS: '成功',
	FAILURE: '失败',
	TIMEOUT: '超时',
}

const platformLabel: Record<string, string> = {
	sora: 'Sora',
	kling: 'Kling',
	midjourney: 'Midjourney',
	suno: 'Suno',
}

function formatCost(n: number | undefined): string {
	if (!n) return '$0.0000'
	return '$' + n.toFixed(4)
}

function formatTime(s: string | undefined): string {
	if (!s) return '-'
	return s.replace('T', ' ').substring(0, 19)
}

async function fetchTasks() {
	loading.value = true
	try {
		const params: Record<string, any> = {
			page: page.value,
			page_size: pageSize,
		}
		if (filterStatus.value) params.status = filterStatus.value
		if (filterPlatform.value) params.platform = filterPlatform.value
		if (filterTaskId.value) params.public_task_id = filterTaskId.value

		const res: any = await request.get('/tenant/tasks', { params })
		const raw = res.data?.data
		tasks.value = Array.isArray(raw?.list) ? raw.list : []
		total.value = raw?.total || 0
	} catch {
		tasks.value = []
		total.value = 0
	} finally {
		loading.value = false
	}
}

async function openDetail(task: TaskItem) {
	detailTask.value = task
	showDetail.value = true
	detailLoading.value = true
	try {
		const res: any = await request.get(`/tenant/tasks/${task.id}`)
		const raw = res.data?.data
		if (raw?.task) {
			detailTask.value = raw.task
		}
	} catch {
		// keep the list-level data
	} finally {
		detailLoading.value = false
	}
}

function applyFilters() {
	page.value = 1
	fetchTasks()
}

function resetFilters() {
	filterStatus.value = ''
	filterPlatform.value = ''
	filterTaskId.value = ''
	page.value = 1
	fetchTasks()
}

function prevPage() {
	if (page.value > 1) { page.value--; fetchTasks() }
}

function nextPage() {
	if (page.value * pageSize < total.value) { page.value++; fetchTasks() }
}

function isImageResult(url: string | undefined): boolean {
	if (!url) return false
	return /\.(jpg|jpeg|png|gif|webp)(\?|$)/i.test(url)
}

onMounted(() => {
	const route = useRoute()
	if (route.query.public_task_id) {
		filterTaskId.value = String(route.query.public_task_id)
	}
	fetchTasks()
})
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="page-header">
			<div>
				<h1 class="page-title">任务日志</h1>
				<p class="page-description">查看异步生成任务（视频/图片/音乐）的执行记录</p>
			</div>
		</div>

		<!-- Filters -->
		<div class="card">
			<div class="card-body">
				<div class="flex flex-wrap items-center gap-4">
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">任务 ID</label>
						<input v-model="filterTaskId" class="input" placeholder="搜索任务 ID" style="width:200px" @keydown.enter="applyFilters" />
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">状态</label>
						<select v-model="filterStatus" class="input bg-white" style="width:120px">
							<option value="">全部</option>
							<option value="NOT_START">未开始</option>
							<option value="SUBMITTED">已提交</option>
							<option value="IN_PROGRESS">进行中</option>
							<option value="SUCCESS">成功</option>
							<option value="FAILURE">失败</option>
						</select>
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">平台</label>
						<select v-model="filterPlatform" class="input bg-white" style="width:120px">
							<option value="">全部</option>
							<option value="sora">Sora</option>
							<option value="kling">Kling</option>
							<option value="midjourney">Midjourney</option>
							<option value="suno">Suno</option>
						</select>
					</div>
					<div class="ml-auto flex items-center gap-2">
						<button class="btn btn-primary btn-sm" @click="applyFilters">搜索</button>
						<button class="btn btn-secondary btn-sm" @click="resetFilters">重置</button>
					</div>
				</div>
			</div>
		</div>

		<!-- Table -->
		<div class="card p-0 overflow-hidden">
			<div v-if="loading" class="p-8 text-center">
				<div class="spinner mx-auto mb-3"></div>
				<p class="text-sm text-gray-500">加载中...</p>
			</div>

			<div v-else-if="tasks.length === 0" class="empty-state">
				<Icon name="clipboard" size="xl" class="empty-state-icon text-gray-300" />
				<p class="empty-state-title">暂无任务记录</p>
				<p class="empty-state-description">异步生成任务的执行记录将显示在这里</p>
			</div>

			<div v-else>
				<div class="table-container">
					<table class="table">
						<thead>
							<tr>
								<th>任务 ID</th>
								<th>平台</th>
								<th>状态</th>
								<th>模型</th>
								<th>费用</th>
								<th>提交时间</th>
								<th>完成时间</th>
								<th></th>
							</tr>
						</thead>
						<tbody>
							<tr v-for="task in tasks" :key="task.id">
								<td>
									<span class="font-mono text-xs text-gray-600">{{ task.public_task_id }}</span>
								</td>
								<td>
									<span class="text-sm font-medium text-gray-700">{{ platformLabel[task.platform] || task.platform }}</span>
								</td>
								<td>
									<span class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium" :class="statusBadge[task.status] || 'bg-gray-100 text-gray-800'">
										{{ statusLabel[task.status] || task.status }}
									</span>
								</td>
								<td>
									<span class="text-sm text-gray-700">{{ task.model_name || '-' }}</span>
								</td>
								<td>
									<span v-if="task.billing_settled && task.actual_cost > 0" class="text-sm font-medium text-emerald-600">{{ formatCost(task.actual_cost) }}</span>
									<span v-else-if="task.pre_deduct_amount > 0" class="text-sm text-gray-500">{{ formatCost(task.pre_deduct_amount) }} <span class="text-xs text-gray-400">(预扣)</span></span>
									<span v-else class="text-sm text-gray-400">-</span>
								</td>
								<td>
									<span class="text-xs text-gray-500 whitespace-nowrap">{{ formatTime(task.submit_time) }}</span>
								</td>
								<td>
									<span class="text-xs text-gray-500 whitespace-nowrap">{{ formatTime(task.finish_time) }}</span>
								</td>
								<td>
									<button class="btn btn-ghost btn-sm p-1.5" title="查看详情" @click="openDetail(task)">
										<Icon name="eye" size="sm" class="text-gray-400 hover:text-primary-500" />
									</button>
								</td>
							</tr>
						</tbody>
					</table>
				</div>

				<!-- Pagination -->
				<div v-if="totalPages > 1" class="flex items-center justify-between px-6 py-3 border-t border-gray-100">
					<span class="text-xs text-gray-500">共 {{ total }} 条记录</span>
					<div class="flex items-center gap-2">
						<button class="btn btn-ghost btn-sm" :disabled="page <= 1" @click="prevPage">上一页</button>
						<span class="text-sm text-gray-600">{{ page }} / {{ totalPages }}</span>
						<button class="btn btn-ghost btn-sm" :disabled="page >= totalPages" @click="nextPage">下一页</button>
					</div>
				</div>
			</div>
		</div>

		<!-- Detail Modal -->
		<BaseModal :show="showDetail" title="任务详情" width="extra-wide" @close="showDetail = false">
			<div v-if="detailLoading" class="p-8 text-center">
				<div class="spinner mx-auto mb-3"></div>
				<p class="text-sm text-gray-500">加载中...</p>
			</div>

			<div v-else-if="detailTask" class="space-y-5">
				<!-- Basic Info -->
				<div>
					<h4 class="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
						<Icon name="document" size="sm" class="text-primary-500" />
						基本信息
					</h4>
					<div class="grid grid-cols-2 gap-x-6 gap-y-2.5 text-sm">
						<div class="flex justify-between">
							<span class="text-gray-500">任务 ID</span>
							<span class="font-mono text-xs">{{ detailTask.public_task_id }}</span>
						</div>
						<div class="flex justify-between">
							<span class="text-gray-500">状态</span>
							<span class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium" :class="statusBadge[detailTask.status] || 'bg-gray-100 text-gray-800'">
								{{ statusLabel[detailTask.status] || detailTask.status }}
							</span>
						</div>
						<div class="flex justify-between">
							<span class="text-gray-500">平台</span>
							<span class="text-sm">{{ platformLabel[detailTask.platform] || detailTask.platform }}</span>
						</div>
						<div class="flex justify-between">
							<span class="text-gray-500">动作</span>
							<span class="text-sm">{{ detailTask.action || '-' }}</span>
						</div>
						<div class="flex justify-between">
							<span class="text-gray-500">模型</span>
							<span class="text-sm font-mono">{{ detailTask.model_name }}</span>
						</div>
						<div class="flex justify-between">
							<span class="text-gray-500">进度</span>
							<span class="text-sm">{{ detailTask.progress || '-' }}</span>
						</div>
						<div v-if="detailTask.username" class="flex justify-between">
							<span class="text-gray-500">用户</span>
							<span class="text-sm">{{ detailTask.username }}</span>
						</div>
						<div class="flex justify-between">
							<span class="text-gray-500">提交时间</span>
							<span class="text-xs">{{ formatTime(detailTask.submit_time) }}</span>
						</div>
						<div class="flex justify-between">
							<span class="text-gray-500">完成时间</span>
							<span class="text-xs">{{ formatTime(detailTask.finish_time) }}</span>
						</div>
						<div class="flex justify-between">
							<span class="text-gray-500">创建时间</span>
							<span class="text-xs">{{ formatTime(detailTask.created_at) }}</span>
						</div>
					</div>
				</div>

				<!-- Cost -->
				<div>
					<h4 class="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
						<Icon name="creditCard" size="sm" class="text-primary-500" />
						费用信息
					</h4>
					<div class="space-y-2 text-sm">
						<div class="flex items-center justify-between">
							<span class="text-gray-500">预扣金额</span>
							<span>{{ formatCost(detailTask.pre_deduct_amount) }}</span>
						</div>
						<div class="flex items-center justify-between border-t border-gray-200 pt-2 font-semibold">
							<span class="text-gray-700">实际费用</span>
							<span v-if="detailTask.billing_settled" class="text-emerald-600">{{ formatCost(detailTask.actual_cost) }}</span>
							<span v-else class="text-gray-400">未结算</span>
						</div>
					</div>
				</div>

				<!-- Result -->
				<div v-if="detailTask.result_url">
					<h4 class="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
						<Icon name="image" size="sm" class="text-primary-500" />
						生成结果
					</h4>
					<div class="bg-gray-50 rounded-xl p-4">
						<img
							v-if="isImageResult(detailTask.result_url)"
							:src="detailTask.result_url"
							alt="任务结果"
							class="max-w-full rounded-lg"
						/>
						<a v-else :href="detailTask.result_url" target="_blank" class="text-primary-600 hover:text-primary-700 text-sm break-all">
							{{ detailTask.result_url }}
						</a>
					</div>
				</div>

				<!-- Fail Reason -->
				<div v-if="detailTask.fail_reason">
					<h4 class="text-sm font-semibold text-red-600 mb-3 flex items-center gap-2">
						<Icon name="xCircle" size="sm" />
						失败原因
					</h4>
					<div class="rounded-lg bg-red-50 border border-red-100 px-4 py-3 text-sm text-red-700">
						{{ detailTask.fail_reason }}
					</div>
				</div>
			</div>

			<template #footer>
				<button class="btn btn-secondary" @click="showDetail = false">关闭</button>
			</template>
		</BaseModal>
	</div>
</template>

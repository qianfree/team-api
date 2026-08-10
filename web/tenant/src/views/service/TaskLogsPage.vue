<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NTag, NInput } from 'naive-ui'
import { useRoute } from 'vue-router'
import Icon from '@/components/common/Icon.vue'
import BaseModal from '@/components/common/BaseModal.vue'
import BaseSelect from '../../components/common/BaseSelect.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'
import DateTimeRangePicker from '@/components/common/DateTimeRangePicker.vue'

// 日期辅助（native，避免引入 dayjs 依赖）
function pad2(n: number): string {
	return String(n).padStart(2, '0')
}
// 默认查询当天：开始 = 当天 0 点，结束 = 当天 23:59:59
function todayStart(): string {
	const d = new Date()
	return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} 00:00:00`
}
function todayEnd(): string {
	const d = new Date()
	return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} 23:59:59`
}

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
	result_thumb_url?: string
	username?: string
	submit_time?: string
	finish_time?: string
	created_at: string
}

const loading = ref(false)
const tasks = ref<TaskItem[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const filterStatus = ref('')
const filterPlatform = ref('')
const filterTaskId = ref('')
const filterStartDate = ref(todayStart())
const filterEndDate = ref(todayEnd())
const showExportDropdown = ref(false)

const showDetail = ref(false)
const detailLoading = ref(false)
const detailTask = ref<TaskItem | null>(null)

const { exporting, exportFile } = useExport({
	url: '/tenant/tasks/export',
	getFilters: () => ({
		status: filterStatus.value || undefined,
		platform: filterPlatform.value || undefined,
		public_task_id: filterTaskId.value || undefined,
		start_date: filterStartDate.value || undefined,
		end_date: filterEndDate.value || undefined,
	}),
})

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
	volcengine: '火山引擎',
	ali: '阿里',
}

function formatCost(n: number | undefined): string {
	if (!n) return '$0.000000'
	return '$' + n.toFixed(6)
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
			page_size: pageSize.value,
		}
		if (filterStatus.value) params.status = filterStatus.value
		if (filterPlatform.value) params.platform = filterPlatform.value
		if (filterTaskId.value) params.public_task_id = filterTaskId.value
		if (filterStartDate.value) params.start_date = filterStartDate.value
		if (filterEndDate.value) params.end_date = filterEndDate.value

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
	filterStartDate.value = todayStart()
	filterEndDate.value = todayEnd()
	page.value = 1
	fetchTasks()
}

function isImageResult(url: string | undefined): boolean {
	if (!url) return false
	return /\.(jpg|jpeg|png|gif|webp)(\?|$)/i.test(url)
}

// 任务状态 → NTag type 映射（statusBadge 为 bg-* 类，无法映射到 renderBadge，改为直接渲染 NTag）
const taskStatusType: Record<string, 'default' | 'success' | 'info' | 'warning' | 'error' | 'primary'> = {
	NOT_START: 'default',
	SUBMITTED: 'info',
	IN_PROGRESS: 'warning',
	SUCCESS: 'success',
	FAILURE: 'error',
	TIMEOUT: 'warning',
}

// NDataTable 列定义
const columns = computed<DataTableColumns<TaskItem>>(() => [
	{
		title: '任务 ID',
		key: 'public_task_id',
		render: (row) => h('span', { class: 'font-mono text-xs text-gray-600' }, row.public_task_id),
	},
	{
		title: '平台',
		key: 'platform',
		render: (row) => h('span', { class: 'text-sm font-medium text-gray-700' }, platformLabel[row.platform] || row.platform),
	},
	{
		title: '状态',
		key: 'status',
		render: (row) =>
			h(NTag, { type: taskStatusType[row.status] || 'default', size: 'small' }, { default: () => statusLabel[row.status] || row.status }),
	},
	{
		title: '模型',
		key: 'model_name',
		render: (row) => h('span', { class: 'text-sm text-gray-700' }, row.model_name || '-'),
	},
	{
		title: '费用',
		key: 'cost',
		render: (row) =>
			row.billing_settled && row.actual_cost > 0
				? h('span', { class: 'text-sm font-medium text-emerald-600' }, formatCost(row.actual_cost))
				: row.pre_deduct_amount > 0
				? h('span', { class: 'text-sm text-gray-500' }, formatCost(row.pre_deduct_amount) + ' (预扣)')
				: h('span', { class: 'text-sm text-gray-400' }, '-'),
	},
	{
		title: '提交时间',
		key: 'submit_time',
		render: (row) => h('span', { class: 'text-xs text-gray-500' }, formatTime(row.submit_time)),
	},
	{
		title: '完成时间',
		key: 'finish_time',
		render: (row) => h('span', { class: 'text-xs text-gray-500' }, formatTime(row.finish_time)),
	},
	{
		title: '操作',
		key: 'actions',
		align: 'right',
		render: (row) =>
			h(NButton, { size: 'small', onClick: () => openDetail(row) }, { icon: () => h(Icon, { name: 'eye', size: 'sm' }) }),
	},
])

// pageSize 变化回第 1 页并刷新
function handlePageSizeChange() {
	page.value = 1
	fetchTasks()
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
	<div class="viewport-table-page space-y-6">
		<!-- Filters -->
		<div class="relative z-20 overflow-visible card">
			<div class="card-body !p-4">
				<form class="flex flex-wrap items-center gap-x-3 gap-y-3" @submit.prevent="applyFilters">
					<DateTimeRangePicker v-model:start="filterStartDate" v-model:end="filterEndDate" />
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">任务 ID</label>
						<n-input v-model:value="filterTaskId" placeholder="搜索任务 ID" style="width:200px" @keydown.enter="applyFilters" />
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">状态</label>
						<BaseSelect v-model="filterStatus" :options="[{value:'',label:'全部'},{value:'NOT_START',label:'未开始'},{value:'SUBMITTED',label:'已提交'},{value:'IN_PROGRESS',label:'进行中'},{value:'SUCCESS',label:'成功'},{value:'FAILURE',label:'失败'}]" container-class="w-[120px]" />
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">平台</label>
						<BaseSelect v-model="filterPlatform" :options="[{value:'',label:'全部'},{value:'sora',label:'Sora'},{value:'kling',label:'Kling'},{value:'midjourney',label:'Midjourney'},{value:'suno',label:'Suno'},{value:'volcengine',label:'火山引擎'},{value:'ali',label:'阿里'}]" container-class="w-[120px]" />
					</div>
					<div class="ml-auto flex items-center gap-2">
						<button type="submit" class="btn btn-primary btn-sm">
							<Icon name="search" size="sm" />
							搜索
						</button>
						<button type="button" class="btn btn-secondary btn-sm" @click="resetFilters">重置</button>
						<span class="mx-1 h-6 w-px bg-gray-200" aria-hidden="true"></span>
						<div class="relative">
							<button type="button" class="btn btn-secondary btn-sm" :disabled="exporting || loading" @click="showExportDropdown = !showExportDropdown">
								<Icon v-if="exporting" name="refresh" size="sm" class="animate-spin" />
								<Icon v-else name="download" size="sm" />
								导出
								<Icon name="chevronDown" size="xs" />
							</button>
							<div v-if="showExportDropdown" class="absolute right-0 z-50 mt-2 w-36 overflow-hidden rounded-lg border border-gray-200 bg-white py-1 shadow-lg">
								<button type="button" class="block w-full px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-50" @click="exportFile('csv'); showExportDropdown = false">导出 CSV</button>
								<button type="button" class="block w-full px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-50" @click="exportFile('xlsx'); showExportDropdown = false">导出 Excel</button>
							</div>
						</div>
					</div>
				</form>
			</div>
		</div>

		<!-- Table -->
		<div class="viewport-table-panel relative z-0 card p-0 overflow-hidden">
			<n-data-table
				remote
				v-model:page="page"
				v-model:page-size="pageSize"
				:item-count="total"
				:page-sizes="[10, 20, 50, 100]"
				show-size-picker
				:loading="loading"
				:columns="columns"
				:data="tasks"
				:row-key="(row: TaskItem) => row.id"
				@update:page="fetchTasks"
				@update:page-size="handlePageSizeChange"
			>
				<template #empty>
					<div class="empty-state">
						<Icon name="clipboard" size="xl" class="empty-state-icon text-gray-300" />
						<p class="empty-state-title">暂无任务记录</p>
						<p class="empty-state-description">异步生成任务的执行记录将显示在这里</p>
					</div>
				</template>
			</n-data-table>
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
						<!-- 图片：内联展示缩略图（省流量），点击在新标签打开原图 -->
						<a
							v-if="isImageResult(detailTask.result_url)"
							:href="detailTask.result_url"
							target="_blank"
							rel="noopener"
							title="点击查看原图"
						>
							<img
								:src="detailTask.result_thumb_url || detailTask.result_url"
								alt="任务结果"
								class="max-w-full rounded-lg cursor-zoom-in"
							/>
						</a>
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

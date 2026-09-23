<script setup lang="ts">
import { ref, onMounted, computed, watch, onUnmounted, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NTag, NInput, NDropdown } from 'naive-ui'
import { useRoute } from 'vue-router'
import Icon from '@/components/common/Icon.vue'
import BaseModal from '@/components/common/BaseModal.vue'
import ResponsiveDataTable from '@/components/common/ResponsiveDataTable.vue'
import BaseSelect from '../../components/common/BaseSelect.vue'
import request from '@/utils/request'
import { tableScrollX } from '@/utils/renderUtils'
import { useExport } from '@/composables/useExport'
import { formatBilling } from '@/composables/useCurrency'
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
// 导出格式下拉选项（NDropdown 默认 teleport 到 body，避免被 .card 的 backdrop-filter
// stacking context 困住而被下方表格面板遮挡）
const exportDropdownOptions = [
	{ label: '导出 CSV', key: 'csv' },
	{ label: '导出 Excel', key: 'xlsx' },
]

function handleExport(format: string | number) {
	exportFile(format as 'csv' | 'xlsx')
}

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

// 状态语义色：详情抬头区的状态点与进度条共用（列表徽章另走 taskStatusType 的 NTag）
const statusColor: Record<string, string> = {
	NOT_START: '#9ca3af',
	SUBMITTED: '#3b82f6',
	IN_PROGRESS: '#f59e0b',
	SUCCESS: '#10b981',
	FAILURE: '#ef4444',
	TIMEOUT: '#f97316',
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
	minimax: 'MiniMax',
}

// 平台 → 任务类型，用于详情抬头区的类型标签（覆盖范围与筛选项一致）
const platformKind: Record<string, string> = {
	sora: '视频',
	kling: '视频',
	volcengine: '视频',
	ali: '视频',
	minimax: '视频',
	midjourney: '图片',
	suno: '音乐',
}

// 金额格式化统一走本位币（formatBilling 内部读取响应式 displayCurrency，配置变化自动重渲染）
function formatCost(n: number | undefined): string {
	return formatBilling(n, 6)
}

function formatTime(s: string | undefined): string {
	if (!s) return '-'
	return s.replace('T', ' ').substring(0, 19)
}

// 后端时间为 Asia/Shanghai 墙钟（YYYY-MM-DD HH:mm:ss），此处只用来算时间差，不做时区换算；
// 把 - 换成 / 以兼容 Safari 的 Date 解析
function parseTs(s: string | undefined): number | null {
	if (!s) return null
	const t = new Date(s.replace('T', ' ').substring(0, 19).replace(/-/g, '/')).getTime()
	return Number.isNaN(t) ? null : t
}

function diffMs(start?: string, end?: string): number | null {
	const a = parseTs(start)
	const b = parseTs(end)
	if (a === null || b === null) return null
	return b >= a ? b - a : null
}

function formatDuration(ms: number): string {
	const s = Math.floor(ms / 1000)
	if (s < 60) return `${s} 秒`
	const m = Math.floor(s / 60)
	if (m < 60) return `${m} 分 ${s % 60} 秒`
	const h = Math.floor(m / 60)
	if (h < 24) return `${h} 小时 ${m % 60} 分`
	return `${Math.floor(h / 24)} 天 ${h % 24} 小时`
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
			// 详情接口不返回 username（只有列表接口 join 得到），整体替换会把它抹掉，故显式回落
			detailTask.value = { ...task, ...raw.task, username: raw.task.username || task.username }
		}
	} catch {
		// keep the list-level data
	} finally {
		detailLoading.value = false
	}
}

// === 详情弹窗展示 ===

const detailStatusColor = computed(() => statusColor[detailTask.value?.status || ''] || statusColor.NOT_START)

// 进度百分比：解析后端 "50%" 字符串；成功一律补满，保证终态视觉一致
const progressPercent = computed(() => {
	if (detailTask.value?.status === 'SUCCESS') return 100
	const n = parseInt(String(detailTask.value?.progress || '').replace('%', ''), 10)
	return Number.isNaN(n) ? 0 : Math.min(100, Math.max(0, n))
})

// 结果资源类型：图片走缩略图、视频/音频内联播放，其余才回退成链接
const resultKind = computed<'image' | 'video' | 'audio' | 'file'>(() => {
	const url = detailTask.value?.result_url || ''
	if (/\.(jpg|jpeg|png|gif|webp|bmp|svg)([?#]|$)/i.test(url)) return 'image'
	if (/\.(mp4|mov|webm|m4v|mkv)([?#]|$)/i.test(url)) return 'video'
	if (/\.(mp3|wav|m4a|aac|ogg|flac)([?#]|$)/i.test(url)) return 'audio'
	return 'file'
})

const resultKindLabel = computed(() => ({ image: '图片', video: '视频', audio: '音频', file: '文件' }[resultKind.value]))

// Icon 组件按名取 path，取不到会渲染空图标并留下空白占位，故需按类型给出真实存在的图标名
const resultIconName = computed(
	() => ({ image: 'photo', video: 'film', audio: 'musicalNote', file: 'link' }[resultKind.value]),
)

// 结果文件名：取 URL 路径末段（去掉查询串），供下载按钮落盘用
const resultFileName = computed(() => {
	const path = (detailTask.value?.result_url || '').split('?')[0]
	const base = path.substring(path.lastIndexOf('/') + 1)
	try {
		return base ? decodeURIComponent(base) : ''
	} catch {
		return base
	}
})

// 进行中任务抬头区的「已运行」按秒刷新
const nowTs = ref(Date.now())
let heroTicker: ReturnType<typeof setInterval> | undefined

function startHeroTicker() {
	stopHeroTicker()
	heroTicker = setInterval(() => { nowTs.value = Date.now() }, 1000)
}

function stopHeroTicker() {
	if (heroTicker) {
		clearInterval(heroTicker)
		heroTicker = undefined
	}
}

watch(
	[showDetail, () => detailTask.value?.status],
	([visible, status]) => {
		const running = visible && ['NOT_START', 'SUBMITTED', 'IN_PROGRESS'].includes(String(status || ''))
		if (running) startHeroTicker()
		else stopHeroTicker()
	},
	{ immediate: true },
)

onUnmounted(stopHeroTicker)

// 抬头区右侧文案：终态给总耗时与完成时刻，进行中给已运行时长
const detailTimingText = computed(() => {
	const t = detailTask.value
	if (!t) return ''
	if (t.status === 'SUCCESS' || t.status === 'FAILURE' || t.status === 'TIMEOUT') {
		const total = diffMs(t.created_at, t.finish_time)
		if (total === null) return ''
		return `总耗时 ${formatDuration(total)}${t.finish_time ? ' · ' + formatTime(t.finish_time) + ' 完成' : ''}`
	}
	const since = parseTs(t.submit_time) ?? parseTs(t.created_at)
	if (since === null) return ''
	return `已运行 ${formatDuration(Math.max(0, nowTs.value - since))}`
})

// 执行耗时 = 提交上游 → 完成，无完成时间（进行中）时显示「-」
const detailDurationText = computed(() => {
	const ms = diffMs(detailTask.value?.submit_time, detailTask.value?.finish_time)
	return ms === null ? '-' : formatDuration(ms)
})

// 复制反馈：按 key 区分触发源，短暂显示「已复制」后复位（可编辑文本行内提示，不额外引 toast）
const copiedKey = ref('')
async function copyText(text: string, key = 'default') {
	if (!text) return
	try {
		await navigator.clipboard.writeText(text)
		copiedKey.value = key
		setTimeout(() => {
			if (copiedKey.value === key) copiedKey.value = ''
		}, 1500)
	} catch (e) {
		console.error(e)
	}
}

// 下载结果：跨域直链的 download 属性会被浏览器忽略，故先取 blob 落盘；
// 对象存储未开 CORS 时 fetch 会失败，降级为新标签打开（与「打开原文件」一致）
async function downloadResult() {
	const url = detailTask.value?.result_url
	if (!url) return
	try {
		const res = await fetch(url)
		if (!res.ok) throw new Error(String(res.status))
		const blob = await res.blob()
		const href = URL.createObjectURL(blob)
		const a = document.createElement('a')
		a.href = href
		a.download = resultFileName.value || 'task-result'
		document.body.appendChild(a)
		a.click()
		a.remove()
		URL.revokeObjectURL(href)
	} catch (e) {
		console.error('[task-result] 下载失败，改为新标签打开', e)
		window.open(url, '_blank', 'noopener')
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

// 任务状态 → NTag type 映射（statusColor 为语义色，无法映射到 renderBadge，改为直接渲染 NTag）
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
		width: 200,
		render: (row) => h('span', { class: 'font-mono text-xs text-gray-600' }, row.public_task_id),
	},
	{
		title: '平台',
		key: 'platform',
		width: 100,
		render: (row) => h('span', { class: 'text-sm font-medium text-gray-700' }, platformLabel[row.platform] || row.platform),
	},
	{
		title: '状态',
		key: 'status',
		width: 60,
		render: (row) =>
			h(NTag, { type: taskStatusType[row.status] || 'default', size: 'small' }, { default: () => statusLabel[row.status] || row.status }),
	},
	{
		title: '模型',
		key: 'model_name',
		width: 160,
		render: (row) => h('span', { class: 'text-sm text-gray-700' }, row.model_name || '-'),
	},
	{
		title: '费用',
		key: 'cost',
		width: 110,
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
		width: 150,
		render: (row) => h('span', { class: 'text-xs text-gray-500' }, formatTime(row.submit_time)),
	},
	{
		title: '完成时间',
		key: 'finish_time',
		width: 150,
		render: (row) => h('span', { class: 'text-xs text-gray-500' }, formatTime(row.finish_time)),
	},
	{
		title: '操作',
		key: 'actions',
		width: 120,
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
		<div class="card">
			<div class="card-body !p-4">
				<form class="flex flex-wrap items-center gap-x-3 gap-y-3" @submit.prevent="applyFilters">
					<DateTimeRangePicker
						v-model:start="filterStartDate"
						v-model:end="filterEndDate"
						@change="applyFilters"
					/>
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
						<BaseSelect v-model="filterPlatform" :options="[{value:'',label:'全部'},{value:'sora',label:'Sora'},{value:'kling',label:'Kling'},{value:'midjourney',label:'Midjourney'},{value:'suno',label:'Suno'},{value:'volcengine',label:'火山引擎'},{value:'ali',label:'阿里'},{value:'minimax',label:'MiniMax'}]" container-class="w-[120px]" />
					</div>
					<div class="ml-auto flex items-center gap-2">
						<button type="submit" class="btn btn-primary btn-sm">
							<Icon name="search" size="sm" />
							搜索
						</button>
						<button type="button" class="btn btn-secondary btn-sm" @click="resetFilters">重置</button>
						<span class="mx-1 h-6 w-px bg-gray-200" aria-hidden="true"></span>
						<n-dropdown trigger="click" :options="exportDropdownOptions" @select="handleExport">
							<button type="button" class="btn btn-secondary btn-sm" :disabled="exporting || loading">
								<Icon v-if="exporting" name="refresh" size="sm" class="animate-spin" />
								<Icon v-else name="download" size="sm" />
								导出
								<Icon name="chevronDown" size="xs" />
							</button>
						</n-dropdown>
					</div>
				</form>
			</div>
		</div>

		<!-- Table -->
		<div class="viewport-table-panel relative z-0 p-0 overflow-hidden">
			<ResponsiveDataTable
				remote
				fill-height
				v-model:page="page"
				v-model:page-size="pageSize"
				:item-count="total"
				:page-sizes="[10, 20, 50, 100]"
				show-size-picker
				:loading="loading"
				:columns="columns"
				:scroll-x="tableScrollX(columns)"
				:data="tasks"
				:row-key="(row: TaskItem) => row.id"
				card-title-key="public_task_id"
				card-badge-key="status"
				card-subtitle-key="submit_time"
				:card-fields="['platform', 'model_name', 'cost', 'finish_time']"
				card-actions-key="actions"
				:row-click="openDetail"
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
			</ResponsiveDataTable>
		</div>

		<!-- Detail Modal -->
		<BaseModal :show="showDetail" title="任务详情" width="extra-wide" @close="showDetail = false">
			<div v-if="detailLoading" class="p-8 text-center">
				<div class="spinner mx-auto mb-3"></div>
				<p class="text-sm text-gray-500">加载中...</p>
			</div>

			<div v-else-if="detailTask" class="space-y-5">
				<!-- 状态抬头区：打开弹窗第一眼即可判断结果与耗时 -->
				<div class="rounded-xl bg-gray-50 px-4 py-3">
					<div class="flex items-center justify-between gap-3">
						<div class="flex min-w-0 items-center gap-2">
							<span class="h-2 w-2 shrink-0 rounded-full" :style="{ background: detailStatusColor }"></span>
							<span class="shrink-0 text-base font-semibold text-gray-800">{{ statusLabel[detailTask.status] || detailTask.status }}</span>
							<span class="truncate text-xs text-gray-500">{{ detailTask.model_name || '-' }} · {{ detailTask.action || '-' }}</span>
						</div>
						<div class="flex shrink-0 items-center gap-1.5">
							<span v-if="platformKind[detailTask.platform]" class="rounded-md bg-white px-2 py-0.5 text-xs font-medium text-gray-600">
								{{ platformKind[detailTask.platform] }}
							</span>
							<span class="rounded-md bg-white px-2 py-0.5 text-xs font-medium text-primary-700">
								{{ platformLabel[detailTask.platform] || detailTask.platform }}
							</span>
						</div>
					</div>
					<div class="mt-2.5 h-1.5 overflow-hidden rounded-full bg-gray-200">
						<div
							class="h-full rounded-full transition-all duration-500"
							:style="{ width: progressPercent + '%', background: detailStatusColor }"
						></div>
					</div>
					<div class="mt-1.5 flex items-center justify-between gap-3 text-xs text-gray-500">
						<span>进度 {{ detailTask.progress || '0%' }}</span>
						<span v-if="detailTimingText">{{ detailTimingText }}</span>
					</div>
				</div>

				<!-- 失败原因：紧跟抬头区，避免埋在字段末尾被忽略 -->
				<div v-if="detailTask.fail_reason" class="flex items-start gap-2 rounded-xl border border-red-100 bg-red-50 px-4 py-3">
					<Icon name="xCircle" size="sm" class="mt-0.5 shrink-0 text-red-500" />
					<div class="min-w-0 text-sm text-red-700">
						<p class="mb-1 font-semibold">任务失败</p>
						<p class="break-all leading-relaxed">{{ detailTask.fail_reason }}</p>
					</div>
				</div>

				<!-- Basic Info -->
				<div>
					<h4 class="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
						<Icon name="document" size="sm" class="text-primary-500" />
						基本信息
					</h4>

					<!-- 任务 ID 独占一行：等宽 + 单行省略，长 ID 不再折行挤乱右侧字段的对齐 -->
					<div class="flex items-center gap-2 text-sm">
						<span class="w-[60px] shrink-0 text-gray-500">任务 ID</span>
						<span class="min-w-0 flex-1 truncate font-mono text-xs text-gray-700" :title="detailTask.public_task_id">
							{{ detailTask.public_task_id }}
						</span>
						<button
							class="shrink-0 text-gray-400 transition-colors hover:text-primary-500"
							title="复制任务 ID"
							@click="copyText(detailTask.public_task_id, 'task')"
						>
							<Icon :name="copiedKey === 'task' ? 'check' : 'copy'" size="xs" />
						</button>
					</div>

					<!-- label 固定宽 + 值左对齐：两端对齐留下的中间空洞会让 label 与值难以对照 -->
					<div class="mt-2.5 grid grid-cols-2 gap-x-6 gap-y-2.5 text-sm">
						<div class="flex items-center gap-2">
							<span class="w-[60px] shrink-0 text-gray-500">平台</span>
							<span class="min-w-0 truncate text-gray-700">{{ platformLabel[detailTask.platform] || detailTask.platform || '-' }}</span>
						</div>
						<div class="flex items-center gap-2">
							<span class="w-[60px] shrink-0 text-gray-500">动作</span>
							<span class="min-w-0 truncate text-gray-700">{{ detailTask.action || '-' }}</span>
						</div>
						<div class="flex items-center gap-2">
							<span class="w-[60px] shrink-0 text-gray-500">模型</span>
							<span class="min-w-0 truncate font-mono text-xs text-gray-700">{{ detailTask.model_name || '-' }}</span>
						</div>
						<div class="flex items-center gap-2">
							<span class="w-[60px] shrink-0 text-gray-500">成员</span>
							<span class="min-w-0 truncate text-gray-700">{{ detailTask.username || '-' }}</span>
						</div>
						<div class="flex items-center gap-2">
							<span class="w-[60px] shrink-0 text-gray-500">提交时间</span>
							<span class="min-w-0 truncate text-xs text-gray-700">{{ formatTime(detailTask.submit_time) }}</span>
						</div>
						<div class="flex items-center gap-2">
							<span class="w-[60px] shrink-0 text-gray-500">完成时间</span>
							<span class="min-w-0 truncate text-xs text-gray-700">{{ formatTime(detailTask.finish_time) }}</span>
						</div>
						<div class="flex items-center gap-2">
							<span class="w-[60px] shrink-0 text-gray-500">创建时间</span>
							<span class="min-w-0 truncate text-xs text-gray-700">{{ formatTime(detailTask.created_at) }}</span>
						</div>
						<div class="flex items-center gap-2">
							<span class="w-[60px] shrink-0 text-gray-500">执行耗时</span>
							<span class="min-w-0 truncate text-xs text-gray-700">{{ detailDurationText }}</span>
						</div>
					</div>
				</div>

				<!-- Cost -->
				<div>
					<h4 class="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
						<Icon name="creditCard" size="sm" class="text-primary-500" />
						费用信息
					</h4>
					<div class="grid grid-cols-2 gap-3">
						<div class="rounded-xl bg-gray-50 px-4 py-3">
							<p class="text-xs text-gray-500">预扣金额</p>
							<p class="mt-1 text-base font-semibold text-gray-800">{{ formatCost(detailTask.pre_deduct_amount) }}</p>
						</div>
						<div class="rounded-xl bg-gray-50 px-4 py-3">
							<div class="flex items-center gap-1.5">
								<p class="text-xs text-gray-500">实际费用</p>
								<span v-if="detailTask.billing_settled" class="rounded bg-emerald-50 px-1.5 py-0.5 text-[10px] font-medium leading-tight text-emerald-600">已结算</span>
							</div>
							<p v-if="detailTask.billing_settled" class="mt-1 text-base font-semibold text-emerald-600">
								{{ formatCost(detailTask.actual_cost) }}
							</p>
							<p v-else class="mt-1 text-base font-semibold text-gray-400">未结算</p>
						</div>
					</div>
				</div>

				<!-- Result -->
				<div v-if="detailTask.result_url">
					<div class="mb-3 flex items-center justify-between gap-3">
						<h4 class="text-sm font-semibold text-gray-700 flex items-center gap-2">
							<Icon :name="resultIconName" size="sm" class="text-primary-500" />
							生成结果
							<span class="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium leading-tight text-gray-500">{{ resultKindLabel }}</span>
						</h4>
						<div class="flex shrink-0 items-center gap-3 text-xs">
							<a :href="detailTask.result_url" target="_blank" rel="noopener" class="text-primary-600 hover:text-primary-700">打开原文件</a>
							<button class="text-primary-600 hover:text-primary-700" @click="downloadResult">下载</button>
							<button class="text-primary-600 hover:text-primary-700" @click="copyText(detailTask.result_url || '', 'result')">
								{{ copiedKey === 'result' ? '已复制' : '复制链接' }}
							</button>
						</div>
					</div>
					<div class="bg-gray-50 rounded-xl p-3">
						<!-- 图片：内联展示缩略图（省流量），点击在新标签打开原图 -->
						<a
							v-if="resultKind === 'image'"
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
						<!-- 视频/音频：内联播放，客户不必再另开签名链接 -->
						<video
							v-else-if="resultKind === 'video'"
							:src="detailTask.result_url"
							class="w-full max-h-[420px] rounded-lg bg-black"
							controls
							preload="metadata"
						></video>
						<audio
							v-else-if="resultKind === 'audio'"
							:src="detailTask.result_url"
							class="w-full"
							controls
							preload="metadata"
						></audio>
						<a v-else :href="detailTask.result_url" target="_blank" rel="noopener" class="text-primary-600 hover:text-primary-700 text-sm break-all">
							{{ detailTask.result_url }}
						</a>
					</div>
				</div>
			</div>

		</BaseModal>
	</div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import {
	Message, Tag, Tooltip, Space, Button,
} from '@arco-design/web-vue'
import { IconSave, IconFile, IconBarChart, IconCamera, IconExclamationCircle } from '@arco-design/web-vue/es/icon'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import TableStats from '@/components/TableStats.vue'
import request from '@/utils/request'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import { useExport } from '@/composables/useExport'
import { useDateRange } from '@/composables/useDateRange'
import { useIsMobile } from '@/composables/useIsMobile'
import { displayCurrency, formatBilling } from '@/composables/useCurrency'

const { defaultEnd, defaultTodayRange, quickDateRanges } = useDateRange()
const isMobileView = useIsMobile()

const loading = ref(false)
const summaryLoading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({
	current: 1,
	pageSize: 20,
	total: 0,
	showPageSize: true,
	pageSizeOptions: [10, 20, 50, 100],
})
const summary = ref<{
	total_cost: number
	total_output_tokens: number
	cache_read_ratio: number
} | null>(null)

// 用于判断是否需要重新加载统计数据的筛选条件快照
let lastSummaryFilters = ''

const filterId = ref<number | undefined>(undefined)
const filterTenantId = ref<number | undefined>(undefined)
const filterUserId = ref<number | undefined>(undefined)
const filterModel = ref<string | undefined>(undefined)
const filterApiKeyId = ref<number | undefined>(undefined)
const filterRequestType = ref<string | undefined>(undefined)
const filterChannelId = ref<number | undefined>(undefined)
const filterStatus = ref<string | undefined>(undefined)
const filterDateRange = ref<string[]>(defaultTodayRange())

const tenantOptions = ref<{ label: string; value: number }[]>([])
const modelOptions = ref<{ label: string; value: string }[]>([])

const statusOptions = [
	{ label: '成功', value: 'success' },
	{ label: '失败', value: 'error' },
	{ label: '超时', value: 'timeout' },
	{ label: '已取消', value: 'cancelled' },
]

const requestTypeOptions = [
	{ label: '同步', value: '1' },
	{ label: '流式', value: '2' },
	{ label: '异步', value: '3' },
	{ label: 'WebSocket', value: '4' },
]

let tenantSearchTimer: ReturnType<typeof setTimeout> | null = null
let modelSearchTimer: ReturnType<typeof setTimeout> | null = null

async function fetchTenantOptions(keyword = '') {
	try {
		const res: any = await request.get('/admin/tenants/select', {
			params: { page: 1, page_size: 50, keyword }
		})
		const list = res.data?.data?.list || []
		tenantOptions.value = list.map((t: any) => ({
			label: `${t.name}（${t.code}）`,
			value: t.id,
		}))
	} catch {
		tenantOptions.value = []
	}
}

function handleTenantSearch(value: string) {
	if (tenantSearchTimer) clearTimeout(tenantSearchTimer)
	tenantSearchTimer = setTimeout(() => fetchTenantOptions(value), 300)
}

async function fetchModelOptions(search = '') {
	try {
		const res: any = await request.get('/admin/models', {
			params: { page: 1, page_size: 50, status: 'active', search }
		})
		const list = res.data?.data?.list || []
		modelOptions.value = list.map((m: any) => ({
			label: m.model_name || m.model_id,
			value: m.model_id,
		}))
	} catch {
		modelOptions.value = []
	}
}

function handleModelSearch(value: string) {
	if (modelSearchTimer) clearTimeout(modelSearchTimer)
	modelSearchTimer = setTimeout(() => fetchModelOptions(value), 300)
}

const statusTagColor: Record<string, string> = {
	success: 'green',
	failed: 'red',
	error: 'red',
	interrupted: 'orangered',
	timeout: 'orangered',
	cancelled: 'gray',
}

const statusLabel: Record<string, string> = {
	success: '成功',
	failed: '失败',
	error: '失败',
	interrupted: '中断',
	timeout: '超时',
	cancelled: '已取消',
}

const requestTypeLabel: Record<string, string> = {
	'1': '同步',
	'2': '流式',
	'3': '异步',
	'4': 'WebSocket',
}

const requestTypeColor: Record<string, string> = {
	'1': 'gray',
	'2': 'blue',
	'3': 'orange',
	'4': 'purple',
}

const billingModeLabel: Record<string, string> = {
	token: '按量',
	per_request: '按次',
	tiered: '阶梯',
}

const billingModeColor: Record<string, string> = {
	token: 'gray',
	per_request: 'blue',
	tiered: 'arcoblue',
}

const billingSourceLabel: Record<string, string> = {
	base: '基础定价',
	tenant_custom: '租户独立价',
	tenant: '租户定价',
	custom: '自定义',
	plan: '套餐价',
}

const detailVisible = ref(false)
// 计费快照默认折叠，需要时展开查看定价/倍率等审计细节
const snapshotCollapsed = ref(true)
const detailLog = ref<any>(null)
const router = useRouter()

// 费用格式化：bil 层本位币直显，符号跟随 displayCurrency
function formatCost(n: number): string {
	if (n == null || isNaN(n)) return formatBilling(0, 6)
	return formatBilling(n, 6)
}

function formatMs(n: number): string {
	if (n == null || n <= 0) return '-'
	return n < 1000 ? `${n}ms` : `${(n / 1000).toFixed(2)}s`
}

// 耗时分级着色：按三档阈值返回 绿/蓝/橙，超出最大档返回红色（输入毫秒）
function durationColor(ms: number, thresholds: [number, number, number]): string {
	if (ms == null || ms <= 0) return 'inherit'
	if (ms < thresholds[0]) return '#00b42a' // 绿色：优
	if (ms < thresholds[1]) return '#165dff' // 蓝色：良
	if (ms < thresholds[2]) return '#ff7d00' // 橙色：中
	return '#f53f3f' // 红色：差
}

// 延迟分级：<30s 绿 / <60s 蓝 / <180s 橙 / ≥180s 红
function latencyColor(ms: number): string {
	return durationColor(ms, [30000, 60000, 180000])
}

// TTFT 分级：<3s 绿 / <10s 蓝 / <30s 橙 / ≥30s 红
function ttftColor(ms: number): string {
	return durationColor(ms, [3000, 10000, 30000])
}

function formatTime(s: string): string {
	if (!s) return '-'
	return s.replace('T', ' ').substring(0, 19)
}

// 格式化金额（bil 层本位币，保留2位小数）
function formatMoney(amount: number): string {
	if (amount == null || isNaN(amount)) return formatBilling(0, 2)
	return formatBilling(amount, 2)
}

// 格式化数字（添加千分位）
function formatNumber(num: number): string {
	if (num == null || isNaN(num)) return '0'
	return num.toLocaleString()
}

function hasUpstreamModel(log: any): boolean {
	return log.upstream_model && log.upstream_model !== log.model_name && log.upstream_model !== ''
}

function totalTokens(log: any): number {
	return (log.input_tokens || 0) + (log.output_tokens || 0) +
		(log.cache_creation_tokens || 0) + (log.cache_read_tokens || 0) +
		(log.cache_creation_5m_tokens || 0) + (log.cache_creation_1h_tokens || 0) +
		(log.reasoning_tokens || 0) + (log.audio_input_tokens || 0) +
		(log.audio_output_tokens || 0) + (log.image_output_tokens || 0)
}

function parseSnapshot(log: any): any {
	if (!log?.billing_snapshot) return null
	try {
		return typeof log.billing_snapshot === 'string' ? JSON.parse(log.billing_snapshot) : log.billing_snapshot
	} catch {
		return null
	}
}

// 计费快照解析结果：computed 只解析一次，避免模板里几十处调用反复 JSON.parse
const snapshot = computed(() => parseSnapshot(detailLog.value))

// 快照 token_costs 的 key → 中文标签
const tokenCostLabels: Record<string, string> = {
	input: '输入',
	output: '输出',
	cache_read: '缓存读取',
	cache_creation: '缓存创建',
	cache_creation_5m: '缓存创建(5分钟)',
	cache_creation_1h: '缓存创建(1小时)',
}

// 消费明细小票行：divider=true 渲染分隔线，strong=true 渲染小计行，total=true 渲染合计行
interface ReceiptRow {
	label?: string
	value?: string
	sub?: string
	valueClass?: string
	strong?: boolean
	total?: boolean
	divider?: boolean
}

function tokenRow(label: string, tokens: number | null | undefined): ReceiptRow {
	return { label, value: (tokens || 0).toLocaleString() }
}

// 小票行数据驱动生成，替代手写重复模板
const receiptRows = computed<ReceiptRow[]>(() => {
	const d = detailLog.value
	if (!d) return []
	const rows: ReceiptRow[] = [
		tokenRow('输入 Token', d.input_tokens),
		tokenRow('输出 Token', d.output_tokens),
	]
	if ((d.cache_read_tokens || 0) > 0) rows.push(tokenRow('缓存读取', d.cache_read_tokens))
	if ((d.cache_creation_tokens || 0) > 0) rows.push(tokenRow('缓存创建', d.cache_creation_tokens))
	if ((d.cache_creation_5m_tokens || 0) > 0) rows.push(tokenRow('缓存创建(5分钟)', d.cache_creation_5m_tokens))
	if ((d.cache_creation_1h_tokens || 0) > 0) rows.push(tokenRow('缓存创建(1小时)', d.cache_creation_1h_tokens))
	if ((d.reasoning_tokens || 0) > 0) rows.push(tokenRow('推理 Token', d.reasoning_tokens))
	if ((d.audio_input_tokens || 0) > 0) rows.push(tokenRow('音频输入', d.audio_input_tokens))
	if ((d.audio_output_tokens || 0) > 0) rows.push(tokenRow('音频输出', d.audio_output_tokens))
	if ((d.image_output_tokens || 0) > 0) rows.push(tokenRow('图像输出', d.image_output_tokens))
	if ((d.image_count || 0) > 0) rows.push({ label: '生成图片', value: `${d.image_count} 张`, sub: d.image_size ? `(${d.image_size})` : '' })
	// Token 合计行
	rows.push({ label: 'Token 合计', value: totalTokens(d).toLocaleString(), strong: true })
	rows.push({ divider: true })
	rows.push({ label: '输入费用', value: formatCost(d.input_cost || 0) })
	rows.push({ label: '输出费用', value: formatCost(d.output_cost || 0) })
	if ((d.cache_creation_cost || 0) > 0) rows.push({ label: '缓存创建费用', value: formatCost(d.cache_creation_cost) })
	if ((d.cache_read_cost || 0) > 0) rows.push({ label: '缓存读取费用', value: formatCost(d.cache_read_cost) })
	const rm = d.rate_multiplier
	rows.push({
		label: '费率倍率',
		value: rm && rm !== 1 ? `${rm.toFixed(4)}x` : '-',
		valueClass: rm && rm !== 1 ? (rm < 1 ? 'text-success' : 'text-warning') : undefined,
	})
	rows.push({ label: '基础费用', value: formatCost(d.total_cost || 0) })
	rows.push({ label: '定价来源', value: billingSourceLabel[d.billing_source] || d.billing_source || '-' })
	if ((d.pre_deduct_amount || 0) > 0 || (d.refund_amount || 0) > 0 || (d.supplement_amount || 0) > 0) {
		rows.push({ divider: true })
		rows.push({ label: '预扣金额', value: formatCost(d.pre_deduct_amount) })
		if ((d.refund_amount || 0) > 0) rows.push({ label: '退回金额', value: formatCost(d.refund_amount), valueClass: 'text-success' })
		if ((d.supplement_amount || 0) > 0) rows.push({ label: '补扣金额', value: formatCost(d.supplement_amount), valueClass: 'text-warning' })
	}
	rows.push({ divider: true })
	rows.push({ label: '实际费用', value: formatCost(d.actual_cost || 0), total: true })
	return rows
})

function copyText(text: string) {
	navigator.clipboard.writeText(text).then(() => {
		Message.success('已复制')
	}).catch(() => {})
}

function viewAuditLog(requestId: string, taskId?: string) {
	const query: Record<string, string> = {}
	if (taskId) query.task_id = taskId
	else query.request_id = requestId
	router.push({ name: 'AdminRequestAuditLogs', query })
}

function openDetail(record: any) {
	detailLog.value = record
	// 每次打开重置为折叠，避免上一条记录的快照展开状态带入
	snapshotCollapsed.value = true
	detailVisible.value = true
}

function tooltipRow(label: string, value: string, valueClass = 'dark-tooltip-value') {
	return h('div', { class: 'dark-tooltip-row' }, [
		h('span', { class: 'dark-tooltip-label' }, label),
		h('span', { class: valueClass }, value),
	])
}

const columns: TableColumnData[] = [
	{
		title: '时间', dataIndex: 'created_at', width: 160,
		render({ record }) {
			return h('span', { style: 'white-space: nowrap' }, formatTime(record.created_at))
		},
	},
	{
		title: '租户', dataIndex: 'tenant_name', width: 120, ellipsis: true,
		render({ record }) { return record.tenant_name || record.tenant_id },
	},
	{
		title: '用户/项目', dataIndex: 'username', minWidth: 120, ellipsis: true,
		render({ record }) {
			if (record.project_name) return h('span', { style: 'color: #165dff' }, record.project_name)
			return record.username || '-'
		},
	},
	{
		title: 'API Key', dataIndex: 'api_key_name', minWidth: 120, ellipsis: true,
		render({ record }) { return record.api_key_name || record.api_key_id || '-' },
	},
	{
		title: '模型', dataIndex: 'model_name', width: 200,
		render({ record }) {
			if (hasUpstreamModel(record)) {
				return h('div', [
					h('div', { style: 'font-weight: 500' }, record.model_name),
					h('div', { class: 'upstream-model' }, `↳ ${record.upstream_model}`),
				])
			}
			return h('span', { style: 'font-weight: 500' }, record.model_name)
		},
	},
	{ title: '渠道', dataIndex: 'channel_name', minWidth: 160, ellipsis: true },
	{
		title: '类型', dataIndex: 'request_type', width: 130,
		render({ record }) {
			const tags = [
				h(Tag, { color: requestTypeColor[record.request_type], size: 'small' }, () => requestTypeLabel[record.request_type] || '-'),
			]
			if (record.billing_mode) {
				tags.push(h(Tag, { color: billingModeColor[record.billing_mode], size: 'small' }, () => billingModeLabel[record.billing_mode]))
			}
			return h(Space, { size: 4 }, () => tags)
		},
	},
	{
		title: 'Token', dataIndex: 'tokens', width: 180,
		render({ record }) {
			const tooltipContent = [
				h('div', { class: 'dark-tooltip-title' }, 'Token 详情'),
				tooltipRow('输入 Token', (record.input_tokens || 0).toLocaleString()),
				tooltipRow('输出 Token', (record.output_tokens || 0).toLocaleString()),
				tooltipRow('缓存创建', (record.cache_creation_tokens || 0).toLocaleString()),
				tooltipRow('缓存读取', (record.cache_read_tokens || 0).toLocaleString()),
				tooltipRow('缓存创建(5分钟)', (record.cache_creation_5m_tokens || 0).toLocaleString()),
				tooltipRow('缓存创建(1小时)', (record.cache_creation_1h_tokens || 0).toLocaleString()),
				record.reasoning_tokens > 0 ? tooltipRow('推理 Token', record.reasoning_tokens.toLocaleString()) : null,
				record.audio_input_tokens > 0 ? tooltipRow('音频输入', record.audio_input_tokens.toLocaleString()) : null,
				record.audio_output_tokens > 0 ? tooltipRow('音频输出', record.audio_output_tokens.toLocaleString()) : null,
				record.image_output_tokens > 0 ? tooltipRow('图像输出', record.image_output_tokens.toLocaleString()) : null,
				h('div', { class: 'dark-tooltip-divider' }),
				h('div', { class: 'dark-tooltip-row' }, [
					h('span', { class: 'dark-tooltip-label' }, '合计'),
					h('span', { class: 'dark-tooltip-total' }, totalTokens(record).toLocaleString()),
				]),
			].filter(Boolean)

			return h(Tooltip, {
				backgroundColor: '#1e293b',
				popupStyle: { padding: 0, borderRadius: '8px' },
				position: 'right',
			}, {
				default: () => h('div', { class: 'token-cell' }, [
					h('div', { class: 'token-row' }, [
						h('span', { class: 'token-item', style: 'color: #18a058', 'aria-label': `输入 Token ${(record.input_tokens || 0).toLocaleString()}` }, [
							h('span', { class: 'token-symbol', 'aria-hidden': 'true' }, '↑'),
							h('span', { class: 'token-value' }, (record.input_tokens || 0).toLocaleString()),
						]),
						h('span', { class: 'token-item', style: 'color: #722ed1', 'aria-label': `输出 Token ${(record.output_tokens || 0).toLocaleString()}` }, [
							h('span', { class: 'token-symbol', 'aria-hidden': 'true' }, '↓'),
							h('span', { class: 'token-value' }, (record.output_tokens || 0).toLocaleString()),
						]),
            h('span', { class: 'token-item', style: 'color: #0fc6c2', 'aria-label': `缓存读取 ${(record.cache_read_tokens || 0).toLocaleString()}` }, [
              h('span', { class: 'token-symbol', 'aria-hidden': 'true' }, '⚡'),
              h('span', { class: 'token-value' }, (record.cache_read_tokens || 0).toLocaleString()),
            ]),
						h('span', { class: 'token-item', style: 'color: #ff7d00', 'aria-label': `缓存写入 ${(record.cache_creation_tokens || 0).toLocaleString()}` }, [
							h(IconSave, { size: 13, 'aria-hidden': 'true' }),
							h('span', { class: 'token-value' }, (record.cache_creation_tokens || 0).toLocaleString()),
						]),

					]),
				]),
				content: () => h('div', { class: 'dark-tooltip' }, tooltipContent),
			})
		},
	},
	{
		title: '费用', dataIndex: 'cost', width: 100,
		render({ record }) {
			const tooltipContent = [
				h('div', { class: 'dark-tooltip-title' }, '费用明细'),
				tooltipRow('输入费用', formatCost(record.input_cost || 0)),
				tooltipRow('输出费用', formatCost(record.output_cost || 0)),
				record.cache_creation_cost > 0 ? tooltipRow('缓存创建费用', formatCost(record.cache_creation_cost)) : null,
				record.cache_read_cost > 0 ? tooltipRow('缓存读取费用', formatCost(record.cache_read_cost)) : null,
				record.rate_multiplier && record.rate_multiplier !== 1
					? h('div', { class: 'dark-tooltip-row' }, [
						h('span', { class: 'dark-tooltip-label' }, '费率倍率'),
						h('span', { class: 'dark-tooltip-highlight' }, `${record.rate_multiplier.toFixed(4)}x`),
					])
					: null,
				h('div', { class: 'dark-tooltip-divider' }),
				tooltipRow('基础费用', formatCost(record.total_cost || 0)),
				h('div', { class: 'dark-tooltip-row' }, [
					h('span', { class: 'dark-tooltip-label' }, '实际费用'),
					h('span', { class: 'dark-tooltip-success' }, formatCost(record.actual_cost || 0)),
				]),
			].filter(Boolean)

			return h(Tooltip, {
				backgroundColor: '#1e293b',
				popupStyle: { padding: 0, borderRadius: '8px' },
				position: 'right',
			}, {
				default: () => h('div', { class: 'cost-cell' }, [
					h('span', { class: 'cost-value' }, formatCost(record.actual_cost || record.total_cost)),
				]),
				content: () => h('div', { class: 'dark-tooltip' }, tooltipContent),
			})
		},
	},
	{
		title: '延迟', dataIndex: 'latency_ms', width: 100,
		render({ record }) {
			const items: any[] = [
				h('div', { class: 'time-text', style: { color: latencyColor(record.latency_ms) } }, formatMs(record.latency_ms)),
			]
			if (record.first_token_ms > 0) {
				items.push(h('div', { class: 'sub-text', style: { fontSize: '11px', color: ttftColor(record.first_token_ms) } }, `TTFT ${formatMs(record.first_token_ms)}`))
			}
			return h('div', { style: 'line-height: 1.4' }, items)
		},
	},
	{
		title: '状态', dataIndex: 'status', width: 130,
		render({ record }) {
			const items: any[] = [
				h(Tag, { color: statusTagColor[record.status], size: 'small' }, () => statusLabel[record.status] || record.status),
			]
			if (record.retry_index > 0) {
				items.push(h('span', { class: 'retry-badge' }, `R${record.retry_index}`))
			}
			return h(Space, { size: 4 }, () => items)
		},
	},
	{
		title: '操作', dataIndex: 'actions', width: 80, fixed: 'right',
		render({ record }) {
			return h(Button, { type: 'text', size: 'mini', onClick: () => openDetail(record) }, () => '详情')
		},
	},
]

async function fetchData() {
	loading.value = true
	try {
		const params: Record<string, any> = {
			page: pagination.current,
			page_size: pagination.pageSize,
		}
		if (filterId.value) params.id = filterId.value
		if (filterTenantId.value) params.tenant_id = filterTenantId.value
		if (filterUserId.value) params.user_id = filterUserId.value
		if (filterModel.value) params.model = filterModel.value
		if (filterApiKeyId.value) params.api_key_id = filterApiKeyId.value
		if (filterRequestType.value) params.request_type = filterRequestType.value
		if (filterChannelId.value) params.channel_id = filterChannelId.value
		if (filterStatus.value) params.status = filterStatus.value
		if (filterDateRange.value && filterDateRange.value.length === 2) {
			params.start_date = filterDateRange.value[0]
			// 截止时间为默认「现在」时不传 end_date（后端按「到现在」实时处理），仅手动选择后才显式下发
			if (filterDateRange.value[1] && filterDateRange.value[1] !== defaultEnd) {
				params.end_date = filterDateRange.value[1]
			}
		}

		const res: any = await request.get('/admin/usage-logs', { params })
		const raw = res.data?.data
		data.value = raw?.list || []
		pagination.total = raw?.total || 0
	} catch {
		data.value = []
		pagination.total = 0
	} finally {
		loading.value = false
	}
}

// 获取统计数据（独立接口异步加载）
async function fetchSummary() {
	// 构建筛选条件参数（不包含分页）
	const params: Record<string, any> = {}
	if (filterId.value) params.id = filterId.value
	if (filterTenantId.value) params.tenant_id = filterTenantId.value
	if (filterUserId.value) params.user_id = filterUserId.value
	if (filterModel.value) params.model = filterModel.value
	if (filterApiKeyId.value) params.api_key_id = filterApiKeyId.value
	if (filterRequestType.value) params.request_type = filterRequestType.value
	if (filterChannelId.value) params.channel_id = filterChannelId.value
	if (filterStatus.value) params.status = filterStatus.value
	if (filterDateRange.value && filterDateRange.value.length === 2) {
		params.start_date = filterDateRange.value[0]
		if (filterDateRange.value[1] && filterDateRange.value[1] !== defaultEnd) {
			params.end_date = filterDateRange.value[1]
		}
	}

	// 生成筛选条件的唯一标识
	const currentFilters = JSON.stringify(params)

	// 如果筛选条件没有变化，不重新加载统计数据
	if (currentFilters === lastSummaryFilters && summary.value !== null) {
		return
	}

	summaryLoading.value = true
	try {
		const res: any = await request.get('/admin/usage-logs/summary', { params })
		summary.value = res.data?.data || null
		lastSummaryFilters = currentFilters
	} catch {
		summary.value = null
	} finally {
		summaryLoading.value = false
	}
}

function handleFilter() {
	pagination.current = 1
	fetchData()
	fetchSummary() // 筛选条件变化时重新加载统计数据
}

function handleReset() {
	filterId.value = undefined
	filterTenantId.value = undefined
	filterUserId.value = undefined
	filterModel.value = undefined
	filterApiKeyId.value = undefined
	filterRequestType.value = undefined
	filterChannelId.value = undefined
	filterStatus.value = undefined
	filterDateRange.value = defaultTodayRange()
	pagination.current = 1
	fetchData()
	fetchSummary() // 重置后重新加载统计数据
}

// 刷新：清空所有筛选条件，仅按当天起始时间查询最新记录（截止留空 = 到现在）
function handleRefresh() {
	handleReset()
}

onMounted(() => {
	fetchTenantOptions()
	fetchModelOptions()
	fetchData()
	fetchSummary() // 初始加载时获取统计数据
})

const { exporting, exportFile } = useExport({
	url: '/admin/usage-logs/export',
	getFilters: () => ({
		id: filterId.value,
		tenant_id: filterTenantId.value,
		user_id: filterUserId.value,
		model: filterModel.value,
		api_key_id: filterApiKeyId.value,
		request_type: filterRequestType.value,
		channel_id: filterChannelId.value,
		status: filterStatus.value,
		start_date: filterDateRange.value?.[0],
		// 与列表查询一致：默认截止「现在」不传截止时间，按「到现在」实时导出
		end_date: filterDateRange.value?.[1] && filterDateRange.value[1] !== defaultEnd ? filterDateRange.value[1] : undefined,
	}),
})
</script>

<template>
	<div class="page-table">
		<PageHeader title="用量日志" description="查看所有租户的 API 调用记录和消费明细">
			<template #actions>
				<ADropdown trigger="hover">
					<AButton :loading="exporting">导出</AButton>
					<template #content>
						<ADoption @click="exportFile('csv')">导出 CSV</ADoption>
						<ADoption @click="exportFile('xlsx')">导出 Excel</ADoption>
					</template>
				</ADropdown>
				<a-button size="small" @click="handleReset">重置筛选</a-button>
				<a-button size="small" @click="handleRefresh">刷新</a-button>
			</template>
		</PageHeader>

		<a-card :bordered="false" class="mb-4">
			<a-space wrap>
				<a-range-picker
					v-model="filterDateRange"
					show-time
					:shortcuts="quickDateRanges"
					shortcuts-position="bottom"
					style="width: 340px"
					@change="handleFilter"
				/>
				<a-input-number
					v-model="filterId"
					placeholder="记录ID"
					:min="1"
					allow-clear
					style="width: 120px"
					@change="handleFilter"
					@clear="handleFilter"
				/>
				<a-select
						v-model="filterTenantId"
						:options="tenantOptions"
						placeholder="租户ID"
						allow-search
						allow-clear
						:filter-option="false"
						style="width: 200px"
						@search="handleTenantSearch"
						@change="handleFilter"
						@clear="handleFilter"
					/>
				<a-input-number
					v-model="filterUserId"
					placeholder="用户ID"
					:min="1"
					allow-clear
					style="width: 120px"
					@change="handleFilter"
					@clear="handleFilter"
				/>
				<a-select
						v-model="filterModel"
						:options="modelOptions"
						placeholder="搜索模型"
						allow-search
						allow-clear
						:filter-option="false"
						style="width: 200px"
						@search="handleModelSearch"
						@change="handleFilter"
						@clear="handleFilter"
					/>
				<a-input-number
					v-model="filterApiKeyId"
					placeholder="API Key ID"
					:min="1"
					allow-clear
					style="width: 120px"
					@change="handleFilter"
					@clear="handleFilter"
				/>
				<a-select
					v-model="filterRequestType"
					:options="requestTypeOptions"
					placeholder="请求类型"
					allow-clear
					style="width: 120px"
					@change="handleFilter"
				/>
				<a-input-number
					v-model="filterChannelId"
					placeholder="渠道ID"
					:min="1"
					allow-clear
					style="width: 120px"
					@change="handleFilter"
					@clear="handleFilter"
				/>
				<a-select
					v-model="filterStatus"
					:options="statusOptions"
					placeholder="状态"
					allow-clear
					style="width: 120px"
					@change="handleFilter"
				/>
				<a-button type="primary" @click="handleFilter">搜索</a-button>
			</a-space>
		</a-card>

		<a-card :bordered="false">
			<ResponsiveTable
				:columns="columns"
				:data="data"
				:loading="loading"
				:scroll="{ x: 1400 }"
				:stripe="true"
				size="small"
				row-key="id"
				card-title-key="model_name"
				card-subtitle-key="tenant_name"
				card-badge-key="status"
				:card-fields="[{ key: 'username' }, { key: 'channel_name' }, { key: 'tokens', full: true }, { key: 'request_type' }, { key: 'cost' }, { key: 'latency_ms' }, { key: 'created_at' }]"
			/>

			<div class="table-footer">
				<div class="table-footer-left">
					<!-- 统计汇总 -->
					<a-space v-if="summary" :size="16" wrap>
						<div class="summary-item summary-cost">
							<span class="summary-label">总费用：</span>
							<span class="summary-value">{{ formatMoney(summary.total_cost) }}</span>
						</div>

						<a-divider direction="vertical" style="height: 16px; margin: 0" />

						<div class="summary-item summary-tokens">
							<span class="summary-label">总输出Token：</span>
							<span class="summary-value">{{ formatNumber(summary.total_output_tokens) }}</span>
						</div>

						<a-divider direction="vertical" style="height: 16px; margin: 0" />

						<div class="summary-item summary-cache">
							<span class="summary-label">缓存读取占比：</span>
							<span class="summary-value">{{ (summary.cache_read_ratio || 0).toFixed(2) }}%</span>
						</div>
					</a-space>
				</div>

				<div class="table-footer-right">
					<TableStats :total="pagination.total" />
					<a-pagination
						v-model:current="pagination.current"
						v-model:page-size="pagination.pageSize"
						:total="pagination.total"
						:page-size-options="pagination.pageSizeOptions"
						:simple="isMobileView"
						show-page-size
						@change="fetchData"
						@page-size-change="(size: number) => { pagination.pageSize = size; pagination.current = 1; fetchData() }"
					/>
				</div>
			</div>
		</a-card>

		<a-modal
			v-model:visible="detailVisible"
			title="用量详情"
			:width="isMobileView ? '92%' : 720"
			:footer="false"
			:body-style="{ maxHeight: '65vh', overflowY: 'auto' }"
			unmount-on-close
		>
			<template v-if="detailLog">
				<!-- 关键结果摘要：模型 / 状态 / 费用 / 耗时一眼可读 -->
				<div class="detail-summary">
					<div class="detail-summary-main">
						<span class="summary-model">{{ detailLog.model_name }}<span v-if="hasUpstreamModel(detailLog)" class="summary-upstream"> ↳ {{ detailLog.upstream_model }}</span></span>
						<a-tag :color="statusTagColor[detailLog.status]" size="small">{{ statusLabel[detailLog.status] || detailLog.status }}</a-tag>
						<span v-if="detailLog.retry_index > 0" class="retry-badge">重试 {{ detailLog.retry_index }} 次</span>
						<span class="summary-cost">{{ formatCost(detailLog.actual_cost || 0) }}</span>
					</div>
					<div class="detail-summary-sub">
						<span :style="{ color: latencyColor(detailLog.latency_ms) }">总延迟 {{ formatMs(detailLog.latency_ms) }}</span>
						<template v-if="detailLog.first_token_ms > 0">
							<span class="summary-dot">·</span>
							<span :style="{ color: ttftColor(detailLog.first_token_ms) }">首 Token {{ formatMs(detailLog.first_token_ms) }}</span>
						</template>
						<span class="summary-dot">·</span>
						<span>{{ formatTime(detailLog.created_at) }}</span>
						<a-tag v-if="detailLog.stream_end_reason" :color="detailLog.stream_end_reason === 'done' ? 'green' : 'gray'" size="small">{{ detailLog.stream_end_reason }}</a-tag>
					</div>
				</div>

				<div v-if="detailLog.error_message" class="detail-section">
					<div class="detail-section-title error-title">
						<span class="detail-icon detail-icon-error"><IconExclamationCircle /></span>
						错误信息
					</div>
					<div class="error-message">{{ detailLog.error_message }}</div>
				</div>

				<div class="detail-section">
					<div class="detail-section-title">
						<span class="detail-icon"><IconFile /></span>
						基本信息
					</div>
					<div class="detail-grid">
						<div class="detail-item detail-item-full">
							<span class="detail-label">请求 ID</span>
							<span class="detail-value mono-text">
								{{ detailLog.request_id }}
								<a-link class="copy-btn" @click="copyText(detailLog.request_id)">复制</a-link>
								<a-link class="copy-btn" @click="viewAuditLog(detailLog.request_id, detailLog.task_id || undefined)">查看审计日志</a-link>
							</span>
						</div>
						<div v-if="detailLog.task_id" class="detail-item detail-item-full">
							<span class="detail-label">关联任务</span>
							<span class="detail-value mono-text">
								{{ detailLog.task_id }}
								<a-link class="copy-btn" @click="$router.push({ path: '/admin/task-logs', query: { public_task_id: detailLog.task_id } })">查看任务</a-link>
							</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">渠道</span>
							<span class="detail-value">
								{{ detailLog.channel_name || '-' }}
								<span v-if="detailLog.channel_type" class="sub-text">({{ detailLog.channel_type }})</span>
							</span>
						</div>
						<div v-if="detailLog.relay_mode" class="detail-item">
							<span class="detail-label">代理模式</span>
							<span class="detail-value mono-text">{{ detailLog.relay_mode }}</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">请求类型</span>
							<span class="detail-value">
								<a-tag :color="requestTypeColor[detailLog.request_type]" size="small">
									{{ requestTypeLabel[detailLog.request_type] || '-' }}
								</a-tag>
							</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">计费模式</span>
							<span class="detail-value">
								<a-tag :color="billingModeColor[detailLog.billing_mode]" size="small">
									{{ billingModeLabel[detailLog.billing_mode] || '-' }}
								</a-tag>
							</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">API Key</span>
							<span class="detail-value mono-text">
								{{ detailLog.api_key_name || detailLog.api_key_id || '-' }}<span v-if="detailLog.api_key_name && detailLog.api_key_id" class="sub-text">({{ detailLog.api_key_id }})</span>
							</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">客户端 IP</span>
							<span class="detail-value mono-text">{{ detailLog.client_ip || '-' }}</span>
						</div>
						<div v-if="detailLog.inbound_endpoint" class="detail-item">
							<span class="detail-label">请求端点</span>
							<span class="detail-value mono-text">{{ detailLog.inbound_endpoint }}</span>
						</div>
						<div v-if="detailLog.service_tier" class="detail-item">
							<span class="detail-label">Service Tier</span>
							<span class="detail-value">{{ detailLog.service_tier }}</span>
						</div>
						<div v-if="detailLog.reasoning_effort" class="detail-item">
							<span class="detail-label">Reasoning Effort</span>
							<span class="detail-value">{{ detailLog.reasoning_effort }}</span>
						</div>
						<div v-if="detailLog.user_agent" class="detail-item detail-item-full">
							<span class="detail-label">User-Agent</span>
							<span class="detail-value sub-text detail-ellipsis" :title="detailLog.user_agent">{{ detailLog.user_agent }}</span>
						</div>
					</div>
				</div>

				<div class="detail-section">
					<div class="detail-section-title">
						<span class="detail-icon"><IconBarChart /></span>
						消费明细
					</div>
					<div class="receipt">
						<div class="receipt-head">
							<span class="receipt-brand">用量 · 费用 · 结算</span>
							<span class="receipt-meta">{{ displayCurrency }}</span>
						</div>
						<!-- 小票行由 receiptRows 数据驱动生成 -->
						<template v-for="(row, index) in receiptRows" :key="index">
							<div v-if="row.divider" class="receipt-divider"></div>
							<div v-else class="receipt-row" :class="[row.strong ? 'receipt-strong' : '', row.total ? 'receipt-total' : '']">
								<span class="receipt-label">{{ row.label }}</span>
								<span class="receipt-value" :class="row.valueClass">
									{{ row.value }}<span v-if="row.sub" class="receipt-sub">{{ row.sub }}</span>
								</span>
							</div>
						</template>
					</div>
				</div>

				<div v-if="detailLog.billing_summary || snapshot" class="detail-section">
					<div class="detail-section-title">
						<span class="detail-icon"><IconCamera /></span>
						计费快照
						<a-link class="snapshot-toggle" @click="snapshotCollapsed = !snapshotCollapsed">{{ snapshotCollapsed ? '展开' : '收起' }}</a-link>
					</div>

					<div v-if="snapshot && !snapshotCollapsed">
						<div class="snapshot-grid">
							<div class="snapshot-block">
								<div class="snapshot-block-title">定价信息</div>
								<div class="snapshot-block-body">
									<div class="snapshot-row">
										<span class="snapshot-label">基础输入价</span>
										<span class="snapshot-value">{{ formatBilling(snapshot.pricing.base_input_price || 0, 6) }}/1M</span>
									</div>
									<div class="snapshot-row">
										<span class="snapshot-label">基础输出价</span>
										<span class="snapshot-value">{{ formatBilling(snapshot.pricing.base_output_price || 0, 6) }}/1M</span>
									</div>
									<div v-if="snapshot.pricing.effective_input_price !== snapshot.pricing.base_input_price" class="snapshot-row">
										<span class="snapshot-label">实际输入价</span>
										<span class="snapshot-value text-success">{{ formatBilling(snapshot.pricing.effective_input_price || 0, 6) }}/1M</span>
									</div>
									<div v-if="snapshot.pricing.effective_output_price !== snapshot.pricing.base_output_price" class="snapshot-row">
										<span class="snapshot-label">实际输出价</span>
										<span class="snapshot-value text-success">{{ formatBilling(snapshot.pricing.effective_output_price || 0, 6) }}/1M</span>
									</div>
									<div v-if="snapshot.cache_prices && snapshot.cache_prices.cache_creation_price > 0" class="snapshot-row">
										<span class="snapshot-label">缓存创建单价</span>
										<span class="snapshot-value">{{ formatBilling(snapshot.cache_prices.cache_creation_price || 0, 6) }}/1M</span>
									</div>
									<div v-if="snapshot.cache_prices && snapshot.cache_prices.cache_read_price > 0" class="snapshot-row">
										<span class="snapshot-label">缓存读取单价</span>
										<span class="snapshot-value">{{ formatBilling(snapshot.cache_prices.cache_read_price || 0, 6) }}/1M</span>
									</div>
								</div>
							</div>

							<div class="snapshot-block">
								<div class="snapshot-block-title">倍率信息</div>
								<div class="snapshot-block-body">
									<div v-if="snapshot.multipliers.model_multiplier && snapshot.multipliers.model_multiplier !== 1" class="snapshot-row">
										<span class="snapshot-label">模型倍率</span>
										<span class="snapshot-value">{{ (snapshot.multipliers.model_multiplier).toFixed(4) }}x</span>
									</div>
									<div class="snapshot-row">
										<span class="snapshot-label">租户倍率</span>
										<span class="snapshot-value" :class="(snapshot.multipliers.tenant_multiplier || 1) < 1 ? 'text-success' : ''">
											{{ (snapshot.multipliers.tenant_multiplier || 1).toFixed(4) }}x
										</span>
									</div>
									<div v-if="snapshot.multipliers.discount_ratio && snapshot.multipliers.discount_ratio !== 1" class="snapshot-row">
										<span class="snapshot-label">折扣比例</span>
										<span class="snapshot-value text-success">{{ (snapshot.multipliers.discount_ratio).toFixed(4) }}x</span>
									</div>
								</div>
							</div>

							<div v-if="snapshot.token_costs" class="snapshot-block snapshot-block-full">
								<div class="snapshot-block-title">Token 费用计算</div>
								<div class="snapshot-block-body">
									<template v-for="(tc, key) in snapshot.token_costs" :key="key">
										<div v-if="(tc.tokens || 0) > 0" class="snapshot-row">
											<span class="snapshot-label">{{ tokenCostLabels[key] || key }}</span>
											<span class="snapshot-value">
												{{ (tc.tokens || 0).toLocaleString() }} tokens &times; {{ formatBilling(tc.unit_price || 0, 6) }}/1M = <strong>{{ formatBilling(tc.cost || 0, 6) }}</strong>
											</span>
										</div>
									</template>
								</div>
							</div>

						</div>
					</div>

					<div v-if="detailLog.billing_summary && !snapshotCollapsed" style="margin-top: 12px;">
						<div class="snapshot-text-title">计算过程</div>
						<pre class="billing-snapshot">{{ detailLog.billing_summary }}</pre>
					</div>
				</div>

			</template>
		</a-modal>
	</div>
</template>

<style>
/* Token 单元格 */
.token-cell {
	cursor: pointer;
	border-radius: 4px;
	padding: 2px 4px;
	margin: -2px -4px;
	transition: background 0.15s;
}
.token-cell:hover {
	background: rgba(22, 93, 255, 0.06);
}
.token-row {
	display: grid;
	grid-template-columns: repeat(2, minmax(0, 1fr));
	align-items: center;
	column-gap: 12px;
	row-gap: 0;
	line-height: 1.4;
}
.token-item {
	min-width: 0;
	width: 100%;
	font-size: 12px;
	font-weight: 600;
	white-space: nowrap;
	display: inline-flex;
	align-items: center;
	gap: 4px;
}
.token-item > svg,
.token-symbol {
	flex-shrink: 0;
  width: 16px;
  text-align: center;
}
.token-value {
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	font-variant-numeric: tabular-nums;
}
.token-in { color: #18a058; }
.token-out { color: #722ed1; }
.token-cache-read { color: #0fc6c2; }
.token-cache-create { color: #ff7d00; }
.token-cache-5m { color: #f77234; }
.token-cache-1h { color: #eb2f96; }
.token-reasoning { color: #7b61ff; }
.token-audio { color: #3491fa; }
.token-image { color: #f53f3f; }

/* 彩色圆点指示器 */
.token-dot {
	display: inline-block;
	width: 6px;
	height: 6px;
	border-radius: 50%;
	flex-shrink: 0;
}
.token-dot-in { background: #18a058; }
.token-dot-out { background: #722ed1; }
.token-dot-cache-read { background: #0fc6c2; }
.token-dot-cache-create { background: #ff7d00; }
.token-dot-cache-5m { background: #f77234; }
.token-dot-cache-1h { background: #eb2f96; }
.token-dot-reasoning { background: #7b61ff; }
.token-dot-audio { background: #3491fa; }
.token-dot-image { background: #f53f3f; }

/* 费用单元格 */
.cost-cell {
	cursor: pointer;
	border-radius: 4px;
	padding: 2px 4px;
	margin: -2px -4px;
	transition: background 0.15s;
}
.cost-cell:hover {
	background: rgba(22, 93, 255, 0.06);
}
.cost-value {
	font-weight: 600;
	font-size: 13px;
}
.dark-tooltip {
	min-width: 180px;
	font-size: 12px;
	background: #1e293b;
	color: #f1f5f9;
	padding: 10px 14px;
	border-radius: 8px;
	line-height: 1.8;
}
.dark-tooltip-title {
	font-weight: 600;
	color: #94a3b8;
	margin-bottom: 4px;
	font-size: 11px;
	text-transform: uppercase;
	letter-spacing: 0.5px;
}
.dark-tooltip-row {
	display: flex;
	justify-content: space-between;
	align-items: center;
	gap: 20px;
	padding: 1px 0;
}
.dark-tooltip-label {
	color: #94a3b8;
}
.dark-tooltip-value {
	font-weight: 500;
	color: #f1f5f9;
}
.dark-tooltip-divider {
	height: 1px;
	background: #334155;
	margin: 6px 0;
}
.dark-tooltip-total {
	color: #38bdf8;
	font-weight: 700;
}
.dark-tooltip-highlight {
	color: #38bdf8;
	font-weight: 600;
}
.dark-tooltip-success {
	color: #34d399;
	font-weight: 700;
}

/* 模型上游 */
.upstream-model {
	font-size: 11px;
	color: #86909c;
}

/* 重试徽章 */
.retry-badge {
	display: inline-flex;
	align-items: center;
	padding: 1px 5px;
	font-size: 10px;
	font-weight: 600;
	line-height: 1.5;
	border-radius: 4px;
	background: #fff7e6;
	color: #d48806;
}

/* 时间 */
.time-text {
	font-size: 12px;
	color: #4e5969;
	white-space: nowrap;
}

</style>

<style scoped>
/* 详情弹窗 */
.detail-section {
	margin-top: 16px;
}
.detail-section-title {
	font-size: 13px;
	font-weight: 600;
	color: #1d2129;
	margin-bottom: 8px;
	padding-left: 8px;
	border-left: 3px solid #165dff;
	display: flex;
	align-items: center;
	gap: 6px;
}
.snapshot-toggle {
	margin-left: auto;
	font-size: 12px;
}
.detail-icon {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	color: #165dff;
	font-size: 14px;
}
.detail-icon-error {
	color: #f53f3f;
}
.detail-grid {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 6px 24px;
}
.detail-item {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 4px 0;
	font-size: 13px;
}
.detail-item-full {
	grid-column: 1 / -1;
}
.detail-label {
	color: #86909c;
	flex-shrink: 0;
	display: flex;
	align-items: center;
	gap: 6px;
}
.detail-value {
	font-weight: 500;
	color: #1d2129;
	text-align: right;
	word-break: break-all;
}
/* 消费明细小票风格 */
.receipt {
	margin-top: 8px;
	padding: 12px 14px;
	background: #fafafa;
	border: 1px dashed #d9dce0;
	border-radius: 8px;
	font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
	font-size: 12.5px;
}
.receipt-head {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding-bottom: 8px;
	border-bottom: 1px dashed #d9dce0;
	margin-bottom: 4px;
}
.receipt-brand {
	font-weight: 600;
	color: #1d2129;
	letter-spacing: 0.5px;
}
.receipt-meta {
	color: #86909c;
	font-size: 11px;
}
.receipt-row {
	display: flex;
	justify-content: space-between;
	align-items: baseline;
	gap: 16px;
	padding: 3.5px 0;
	line-height: 1.5;
}
.receipt-label {
	color: #86909c;
	flex-shrink: 0;
}
.receipt-value {
	font-weight: 600;
	color: #1d2129;
	text-align: right;
	white-space: nowrap;
	font-variant-numeric: tabular-nums;
}
.receipt-divider {
	margin: 6px 0;
	border-top: 1px dashed #d9dce0;
}
.receipt-total {
	margin-top: 2px;
	font-size: 14px;
}
.receipt-total .receipt-label {
	color: #1d2129;
	font-weight: 600;
}
.receipt-total .receipt-value {
	font-size: 15px;
}
/* 小票小计行（Token 合计） */
.receipt-strong .receipt-label {
	color: #1d2129;
	font-weight: 600;
}
/* 小票值的灰色补充说明（如图片尺寸） */
.receipt-sub {
	margin-left: 4px;
	color: #86909c;
	font-weight: 400;
	font-size: 11px;
}
/* 关键结果摘要条 */
.detail-summary {
	display: flex;
	flex-direction: column;
	gap: 6px;
	padding: 12px 14px;
	background: #f7f8fa;
	border-radius: 8px;
}
.detail-summary-main {
	display: flex;
	align-items: center;
	gap: 8px;
	min-width: 0;
}
.summary-model {
	font-size: 14px;
	font-weight: 600;
	color: #1d2129;
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
}
.summary-upstream {
	font-size: 12px;
	font-weight: 400;
	color: #86909c;
}
.summary-cost {
	margin-left: auto;
	flex-shrink: 0;
	font-size: 16px;
	font-weight: 700;
	color: #1d2129;
	font-variant-numeric: tabular-nums;
}
.detail-summary-sub {
	display: flex;
	align-items: center;
	flex-wrap: wrap;
	gap: 4px 8px;
	font-size: 12px;
	color: #4e5969;
}
.summary-dot {
	color: #c9cdd4;
}
.detail-summary-main .retry-badge {
	flex-shrink: 0;
}
/* 超长值单行省略（配合 title 提示） */
.detail-ellipsis {
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}
.mono-text {
	font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
	font-size: 12px;
}
.sub-text {
	color: #86909c;
	font-size: 12px;
}
.copy-btn {
	margin-left: 6px;
	font-size: 12px;
}

/* 计费快照结构化展示 */
.snapshot-grid {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 10px;
}
.snapshot-block {
	background: #f7f8fa;
	border-radius: 6px;
	padding: 10px 12px;
}
.snapshot-block-full {
	grid-column: 1 / -1;
}
.snapshot-block-title {
	font-size: 12px;
	font-weight: 600;
	color: #1d2129;
	margin-bottom: 6px;
}
.snapshot-block-body {
	font-size: 12px;
}
.snapshot-row {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 2px 0;
	gap: 12px;
}
.snapshot-label {
	color: #86909c;
	white-space: nowrap;
}
.snapshot-value {
	font-weight: 500;
	color: #1d2129;
	text-align: right;
}
.snapshot-text-title {
	font-size: 12px;
	font-weight: 600;
	color: #86909c;
	margin-bottom: 4px;
}
.billing-snapshot {
	background: #1e293b;
	color: #c9d1d9;
	padding: 12px;
	border-radius: 6px;
	font-size: 12px;
	font-family: 'SFMono-Regular', Consolas, monospace;
	white-space: pre-wrap;
	word-break: break-all;
	margin: 0;
	line-height: 1.6;
}
.detail-section-title.error-title {
	border-left-color: #f53f3f;
	color: #f53f3f;
}
.error-message {
	background: #fff2f0;
	border: 1px solid #ffccc7;
	color: #cb2634;
	padding: 12px;
	border-radius: 6px;
	font-size: 12px;
	font-family: 'SFMono-Regular', Consolas, monospace;
	word-break: break-all;
}
.text-success { color: #00b42a !important; }
.text-warning { color: #ff7d00 !important; }

/* 统计汇总样式 */
.summary-item {
	display: inline-flex;
	align-items: center;
	font-size: 13px;
}

.summary-label {
	color: var(--color-text-2);
	margin-right: 6px;
}

.summary-value {
	font-weight: 600;
	font-size: 14px;
}

/* 不同指标使用不同颜色 */
.summary-cost .summary-value {
	color: rgb(var(--primary-6)); /* 蓝色 - 总费用 */
}

.summary-tokens .summary-value {
	color: rgb(var(--success-6)); /* 绿色 - Token数 */
}

.summary-cache .summary-value {
	color: rgb(var(--warning-6)); /* 橙色 - 缓存占比 */
}

.table-footer {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 16px;
	margin-top: 16px;
	padding-top: 16px;
	border-top: 1px solid var(--color-border-light, #e5e6eb);
}

.table-footer-left {
	flex: 1;
	min-width: 0;
}

.table-footer-right {
	display: flex;
	align-items: center;
	gap: 12px;
	flex-shrink: 0;
}

/* 移动端：统计栏和分页栏分两行显示 */
@media (max-width: 768px) {
	.table-footer {
		flex-direction: column;
		align-items: stretch;
		gap: 12px;
	}

	.table-footer-right {
		flex-direction: column;
		align-items: stretch;
		gap: 8px;
	}

	.summary-item {
		font-size: 12px;
	}

	.summary-item .summary-value {
		font-size: 13px;
	}
}

/* 统计栏移入底部后，去掉全局样式的下边距，与分页栏垂直居中 */
.table-footer :deep(.table-stats) {
	margin-bottom: 0;
}
</style>

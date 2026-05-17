<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import { Tag } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const data = ref<any[]>([])
const total = ref(0)
const pagination = reactive({ current: 1, pageSize: 20 })

const filterMode = ref('')
const filterBlocked = ref('')
const filterTenantId = ref<number | undefined>(undefined)
const filterDateRange = ref<string[]>([])
const filterKeyword = ref('')

// Tenant selector
const tenantOptions = ref<{ value: number; label: string }[]>([])
const tenantLoading = ref(false)
const tenantKeyword = ref('')
const tenantPage = ref(1)
const tenantHasMore = ref(false)
let tenantSearchTimer: ReturnType<typeof setTimeout> | null = null

async function fetchTenantOptions(reset = true) {
	tenantLoading.value = true
	try {
		const params: any = { page: tenantPage.value, page_size: 20 }
		if (tenantKeyword.value) params.keyword = tenantKeyword.value
		const res = await request.get('/admin/tenants/select', { params })
		const d = res.data?.data
		const list: { id: number; name: string; code: string }[] = d?.list || []
		const items = list.map(t => ({ value: t.id, label: `${t.name} (${t.code})` }))
		if (reset) {
			tenantOptions.value = items
		} else {
			tenantOptions.value = [...tenantOptions.value, ...items]
		}
		tenantHasMore.value = tenantOptions.value.length < (d?.total || 0)
	} catch {
		// ignore
	} finally {
		tenantLoading.value = false
	}
}

function handleTenantSearch(val: string) {
	if (tenantSearchTimer) clearTimeout(tenantSearchTimer)
	tenantSearchTimer = setTimeout(() => {
		tenantKeyword.value = val
		tenantPage.value = 1
		fetchTenantOptions(true)
	}, 300)
}

function handleTenantScrollReachEdge(e: Event) {
	if (!tenantHasMore.value || tenantLoading.value) return
	const el = e.target as HTMLElement
	if (el.scrollHeight - el.scrollTop - el.clientHeight < 30) {
		tenantPage.value++
		fetchTenantOptions(false)
	}
}

function openTenantDropdown() {
	if (tenantOptions.value.length === 0) {
		tenantKeyword.value = ''
		tenantPage.value = 1
		fetchTenantOptions(true)
	}
}

const modeColorMap: Record<string, string> = {
	log: 'arcoblue',
	replace: 'orange',
	block: 'red',
}
const modeLabelMap: Record<string, string> = {
	log: '仅记录',
	replace: '替换',
	block: '拦截',
}

const columns: TableColumnData[] = [
	{ title: 'ID', dataIndex: 'id', width: 60 },
	{
		title: '过滤模式',
		dataIndex: 'filter_mode',
		width: 90,
		render: ({ record }: any) => {
			const mode = record.filter_mode
			return h(Tag, { color: modeColorMap[mode] || 'gray' }, () => modeLabelMap[mode] || mode)
		},
	},
	{
		title: '是否拦截',
		dataIndex: 'blocked',
		width: 90,
		render: ({ record }: any) => {
			return record.blocked
				? h(Tag, { color: 'red' }, () => '已拦截')
				: h(Tag, { color: 'green' }, () => '通过')
		},
	},
	{
		title: '租户',
		dataIndex: 'tenant_name',
		width: 120,
		ellipsis: true,
		tooltip: true,
	},
	{
		title: '用户',
		dataIndex: 'user_name',
		width: 110,
		ellipsis: true,
		tooltip: true,
	},
	{
		title: '项目',
		dataIndex: 'project_name',
		width: 110,
		ellipsis: true,
		tooltip: true,
	},
	{
		title: 'API Key',
		dataIndex: 'api_key_name',
		width: 120,
		ellipsis: true,
		tooltip: true,
	},
	{ title: '请求方法', dataIndex: 'method', width: 80 },
	{
		title: '请求路径',
		dataIndex: 'path',
		width: 200,
		ellipsis: true,
		tooltip: true,
	},
	{
		title: '命中敏感词',
		dataIndex: 'matched_words',
		width: 250,
		render: ({ record }: any) => {
			let words: string[] = []
			if (typeof record.matched_words === 'string') {
				try { words = JSON.parse(record.matched_words) } catch { words = [record.matched_words] }
			} else if (Array.isArray(record.matched_words)) {
				words = record.matched_words
			}
			return h('div', { style: 'display:flex;flex-wrap:wrap;gap:4px' },
				words.map((w: string) => h(Tag, { color: 'red', size: 'small' }, () => w))
			)
		},
	},
	{ title: '客户端IP', dataIndex: 'client_ip', width: 130 },
	{
		title: '时间',
		dataIndex: 'created_at',
		width: 160,
		render: ({ record }: any) => {
			if (!record.created_at) return '-'
			return new Date(record.created_at).toLocaleString('zh-CN')
		},
	},
]

async function fetchData() {
	loading.value = true
	try {
		const params: any = {
			page: pagination.current,
			page_size: pagination.pageSize,
		}
		if (filterMode.value) params.mode = filterMode.value
		if (filterBlocked.value) params.blocked = filterBlocked.value
		if (filterTenantId.value) params.tenant_id = filterTenantId.value
		if (filterKeyword.value) params.keyword = filterKeyword.value
		if (filterDateRange.value?.length === 2) {
			params.start_date = filterDateRange.value[0]
			params.end_date = filterDateRange.value[1]
		}
		const res: any = await request.get('/admin/audit/content-filter-logs', { params })
		const d = res.data?.data || res.data
		data.value = d?.list || []
		total.value = d?.total || 0
	} catch {
		data.value = []
		total.value = 0
	} finally {
		loading.value = false
	}
}

function handlePageChange(page: number) {
	pagination.current = page
	fetchData()
}

function handlePageSizeChange(pageSize: number) {
	pagination.pageSize = pageSize
	pagination.current = 1
	fetchData()
}

function handleSearch() {
	pagination.current = 1
	fetchData()
}

function handleReset() {
	filterMode.value = ''
	filterBlocked.value = ''
	filterTenantId.value = undefined
	filterDateRange.value = []
	filterKeyword.value = ''
	pagination.current = 1
	fetchData()
}

onMounted(() => {
	fetchData()
})
</script>

<template>
	<div>
		<PageHeader title="过滤拦截日志" description="查看内容过滤拦截记录，分析敏感词触发趋势" />

		<a-card :bordered="false" class="mb-4">
			<a-space wrap>
				<a-select v-model="filterMode" placeholder="过滤模式" allow-clear style="width: 130px">
					<a-option value="log">仅记录</a-option>
					<a-option value="replace">替换</a-option>
					<a-option value="block">拦截</a-option>
				</a-select>
				<a-select v-model="filterBlocked" placeholder="拦截状态" allow-clear style="width: 130px">
					<a-option value="true">已拦截</a-option>
					<a-option value="false">未拦截</a-option>
				</a-select>
				<a-select
					v-model="filterTenantId"
					:options="tenantOptions"
					:loading="tenantLoading"
					placeholder="选择租户"
					allow-search
					allow-clear
					:filter-option="false"
					style="width: 200px"
					@search="handleTenantSearch"
					@dropdown-scroll-reach-edge="handleTenantScrollReachEdge"
					@popup-visible-change="(visible: boolean) => { if (visible) openTenantDropdown() }"
				/>
					<a-range-picker v-model="filterDateRange" style="width: 260px" />
				<a-input v-model="filterKeyword" placeholder="搜索敏感词/路径" allow-clear style="width: 200px" @keydown.enter="handleSearch" />
				<a-button type="primary" @click="handleSearch">搜索</a-button>
				<a-button @click="handleReset">重置</a-button>
			</a-space>
		</a-card>

		<a-card :bordered="false">
			<a-table
				:data="data"
				:columns="columns"
				:loading="loading"
				:pagination="{
					current: pagination.current,
					pageSize: pagination.pageSize,
					total: total,
					showTotal: true,
					showPageSize: true,
				}"
				@page-change="handlePageChange"
				@page-size-change="handlePageSizeChange"
				row-key="id"
			/>
		</a-card>
	</div>
</template>

<style scoped>
.mb-4 {
	margin-bottom: 16px;
}
</style>

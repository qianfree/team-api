<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NInput } from 'naive-ui'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'
import { renderBadge, tableScrollX } from '@/utils/renderUtils'
import BaseSelect from '../../components/common/BaseSelect.vue'

const list = ref<any[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const filterUsername = ref('')
const filterIpAddress = ref('')
const filterSuccess = ref('')
const filterMethod = ref('')
const filterStartDate = ref('')
const filterEndDate = ref('')

onMounted(() => {
	fetchData()
})

async function fetchData() {
	loading.value = true
	try {
		const params: Record<string, any> = {
			page: page.value,
			page_size: pageSize.value,
		}
		if (filterUsername.value) params.username = filterUsername.value
		if (filterIpAddress.value) params.ip_address = filterIpAddress.value
		if (filterSuccess.value !== '') params.success = filterSuccess.value === 'true'
		if (filterMethod.value) params.login_method = filterMethod.value
		if (filterStartDate.value) params.start_time = filterStartDate.value
		if (filterEndDate.value) params.end_time = filterEndDate.value

		const res = await request.get('/tenant/security/login-history', { params })
		const raw = res.data?.data
		list.value = raw?.list || []
		total.value = raw?.total || 0
	} catch {
		list.value = []
	} finally {
		loading.value = false
	}
}

function applyFilters() {
	page.value = 1
	fetchData()
}

function resetFilters() {
	filterUsername.value = ''
	filterIpAddress.value = ''
	filterSuccess.value = ''
	filterMethod.value = ''
	filterStartDate.value = ''
	filterEndDate.value = ''
	page.value = 1
	fetchData()
}

const methodBadge: Record<string, string> = {
	password: 'badge-primary',
	totp: 'badge-success',
	sso: 'badge-purple',
}

const methodLabel: Record<string, string> = {
	password: '密码',
	totp: '双因素',
	sso: '单点登录',
}

// NDataTable 列定义
const columns = computed<DataTableColumns<any>>(() => [
	{
		title: '时间',
		key: 'created_at',
		width: 170,
		render: (row) => h('span', { class: 'whitespace-nowrap' }, row.created_at),
	},
	{
		title: '用户',
		key: 'user',
		width: 140,
		render: (row) =>
			h('div', {}, [
				row.display_name || row.username || '-',
				row.display_name && row.username ? h('div', { class: 'text-xs text-gray-400' }, row.username) : null,
			]),
	},
	{
		title: '登录方式',
		key: 'login_method',
		width: 120,
		render: (row) => renderBadge(row.login_method, methodLabel, methodBadge),
	},
	{
		title: 'IP 地址',
		key: 'ip_address',
		width: 150,
		render: (row) => h('span', { class: 'font-mono text-sm' }, row.ip_address),
	},
	{
		title: '设备',
		key: 'user_agent',
		width: 240,
		render: (row) =>
			h('span', { class: 'max-w-[200px] truncate block text-gray-500', title: row.user_agent }, row.user_agent),
	},
	{
		title: '状态',
		key: 'success',
		width: 100,
		render: (row) =>
			row.success
				? renderBadge('ok', { ok: '成功' }, { ok: 'badge-success' })
				: renderBadge('fail', { fail: '失败' }, { fail: 'badge-danger' }),
	},
	{
		title: '失败原因',
		key: 'fail_reason',
		width: 180,
		render: (row) => h('span', { class: 'text-gray-500' }, row.fail_reason || '-'),
	},
])

function handlePageSizeChange() {
	page.value = 1
	fetchData()
}
</script>

<template>
	<div class="viewport-table-page">
		<div class="page-header">
			<h1 class="page-title">登录历史</h1>
			<p class="page-description">查看组织成员的登录记录，包括成功和失败的登录尝试</p>
		</div>

		<!-- Filters -->
		<div class="card mb-6">
			<div class="card-body">
				<div class="flex flex-wrap items-end justify-between gap-4">
					<div class="flex flex-1 flex-wrap items-end gap-4">
						<div class="w-full sm:w-auto sm:min-w-[120px]">
							<label class="input-label">用户名</label>
							<n-input
								v-model:value="filterUsername"
								type="text"
								placeholder="搜索用户名"
								@keyup.enter="applyFilters"
							/>
						</div>
						<div class="w-full sm:w-auto sm:min-w-[140px]">
							<label class="input-label">IP 地址</label>
							<n-input
								v-model:value="filterIpAddress"
								type="text"
								placeholder="搜索 IP"
								@keyup.enter="applyFilters"
							/>
						</div>
						<div class="w-full sm:w-auto sm:min-w-[100px]">
							<label class="input-label">状态</label>
							<BaseSelect v-model="filterSuccess" :options="[{value:'',label:'全部'},{value:'true',label:'成功'},{value:'false',label:'失败'}]" />
						</div>
						<div class="w-full sm:w-auto sm:min-w-[100px]">
							<label class="input-label">登录方式</label>
							<BaseSelect v-model="filterMethod" :options="[{value:'',label:'全部'},{value:'password',label:'密码'},{value:'totp',label:'双因素'},{value:'sso',label:'单点登录'}]" />
						</div>
						<div class="w-full sm:w-auto sm:min-w-[140px]">
							<label class="input-label">开始日期</label>
							<input v-model="filterStartDate" type="date" class="input" />
						</div>
						<div class="w-full sm:w-auto sm:min-w-[140px]">
							<label class="input-label">结束日期</label>
							<input v-model="filterEndDate" type="date" class="input" />
						</div>
					</div>
					<div class="flex items-center gap-2">
						<button class="btn btn-secondary" @click="applyFilters">
							<Icon name="search" size="sm" />
							搜索
						</button>
						<button class="btn btn-ghost" @click="resetFilters">重置</button>
					</div>
				</div>
			</div>
		</div>

		<!-- Table -->
		<div class="viewport-table-panel">
			<n-data-table
				remote
				v-model:page="page"
				v-model:page-size="pageSize"
				:item-count="total"
				:page-sizes="[10, 20, 50, 100]"
				show-size-picker
				:loading="loading"
				:columns="columns"
				:scroll-x="tableScrollX(columns)"
				:data="list"
				:row-key="(row: any) => row.id"
				@update:page="fetchData"
				@update:page-size="handlePageSizeChange"
			>
				<template #empty>
					<div class="empty-state">
						<Icon name="shield" size="xl" class="empty-state-icon" />
						<p class="empty-state-title">暂无登录记录</p>
						<p class="empty-state-description">当前没有任何登录历史记录</p>
					</div>
				</template>
			</n-data-table>
		</div>
	</div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'
import BaseSelect from '../../components/common/BaseSelect.vue'

const list = ref<any[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = 20
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
			page_size: pageSize,
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
</script>

<template>
	<div>
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
							<input
								v-model="filterUsername"
								type="text"
								placeholder="搜索用户名"
								class="input"
								@keyup.enter="applyFilters"
							/>
						</div>
						<div class="w-full sm:w-auto sm:min-w-[140px]">
							<label class="input-label">IP 地址</label>
							<input
								v-model="filterIpAddress"
								type="text"
								placeholder="搜索 IP"
								class="input"
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
		<div class="card">
			<div v-if="loading" class="p-8 flex justify-center">
				<div class="spinner h-6 w-6 border-primary-500"></div>
			</div>

			<div v-else-if="list.length > 0">
				<div class="table-container">
					<table class="table">
						<thead>
							<tr>
								<th>时间</th>
								<th>用户</th>
								<th>登录方式</th>
								<th>IP 地址</th>
								<th>设备</th>
								<th>状态</th>
								<th>失败原因</th>
							</tr>
						</thead>
						<tbody>
							<tr v-for="item in list" :key="item.id">
								<td class="whitespace-nowrap">{{ item.created_at }}</td>
								<td>
									<div>{{ item.display_name || item.username || '-' }}</div>
									<div v-if="item.display_name && item.username" class="text-xs text-gray-400">{{ item.username }}</div>
								</td>
								<td>
									<span class="badge" :class="methodBadge[item.login_method] || 'badge-gray'">
										{{ methodLabel[item.login_method] || item.login_method }}
									</span>
								</td>
								<td class="font-mono text-sm">{{ item.ip_address }}</td>
								<td class="max-w-[200px] truncate text-gray-500" :title="item.user_agent">{{ item.user_agent }}</td>
								<td>
									<span v-if="item.success" class="badge badge-success">成功</span>
									<span v-else class="badge badge-danger">失败</span>
								</td>
								<td class="text-gray-500">{{ item.fail_reason || '-' }}</td>
							</tr>
						</tbody>
					</table>
				</div>

				<!-- Pagination -->
				<div v-if="total > pageSize" class="card-footer flex justify-between items-center">
					<span class="text-sm text-gray-500">共 {{ total }} 条记录</span>
					<div class="flex items-center gap-2">
						<button
							class="btn btn-ghost btn-sm"
							:disabled="page <= 1"
							@click="page--; fetchData()"
						>
							上一页
						</button>
						<span class="text-sm text-gray-500">
							{{ page }} / {{ Math.ceil(total / pageSize) }}
						</span>
						<button
							class="btn btn-ghost btn-sm"
							:disabled="page * pageSize >= total"
							@click="page++; fetchData()"
						>
							下一页
						</button>
					</div>
				</div>
			</div>

			<!-- Empty state -->
			<div v-else class="empty-state">
				<Icon name="shield" size="xl" class="empty-state-icon" />
				<p class="empty-state-title">暂无登录记录</p>
				<p class="empty-state-description">当前没有任何登录历史记录</p>
			</div>
		</div>
	</div>
</template>

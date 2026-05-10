<script setup lang="ts">
import { ref, onMounted } from 'vue'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'

const code = ref('')
const loading = ref(false)
const result = ref<any>(null)
const error = ref('')
const recentRedemptions = ref<any[]>([])
const historyLoading = ref(false)

const typeLabels: Record<string, string> = { quota: '额度', plan: '套餐', duration: '时长' }
const typeBadgeClasses: Record<string, string> = { quota: 'badge-success', plan: 'badge-primary', duration: 'badge-warning' }

async function handleRedeem() {
	if (!code.value.trim()) {
		error.value = '请输入兑换码'
		return
	}
	loading.value = true
	error.value = ''
	result.value = null
	try {
		const res: any = await request.post('/tenant/redemptions/redeem', { code: code.value.trim() })
		result.value = res.data?.data
		code.value = ''
		await fetchHistory()
	} catch {
		// interceptor handles error toast
	} finally {
		loading.value = false
	}
}

async function fetchHistory() {
	historyLoading.value = true
	try {
		const res: any = await request.get('/tenant/redemptions/usages', {
			params: { page: 1, page_size: 10 }
		})
		recentRedemptions.value = res.data?.data?.list || []
	} catch {
		// ignore
	} finally {
		historyLoading.value = false
	}
}

onMounted(fetchHistory)
</script>

<template>
	<div>
		<div class="page-header">
			<h1 class="page-title">兑换码</h1>
			<p class="page-description">输入兑换码领取额度、套餐时长等福利</p>
		</div>

		<div class="card mb-6 p-6">
			<h2 class="text-lg font-semibold text-gray-900 mb-1">输入兑换码</h2>
			<p class="text-sm text-gray-500 mb-4">兑换码由平台管理员生成，可通过活动、合作等渠道获取</p>

			<div class="flex gap-3">
				<div class="flex-1">
					<input
						v-model="code"
						type="text"
						class="input font-mono"
						placeholder="请输入兑换码"
						maxlength="32"
						@keyup.enter="handleRedeem"
					/>
				</div>
				<button
					class="btn btn-primary"
					:disabled="loading || !code.trim()"
					@click="handleRedeem"
				>
					<Icon v-if="loading" name="refresh" size="sm" class="animate-spin" />
					<Icon v-else name="check" size="sm" />
					{{ loading ? '兑换中...' : '立即兑换' }}
				</button>
			</div>

			<!-- Error -->
			<div v-if="error" class="mt-3 flex items-center gap-2 text-sm text-red-600 bg-red-50 rounded-lg px-3 py-2">
				<Icon name="xCircle" size="sm" />
				{{ error }}
			</div>

			<!-- Success -->
			<div v-if="result" class="mt-3 flex items-center gap-2 text-sm text-emerald-700 bg-emerald-50 rounded-lg px-3 py-2">
				<Icon name="checkCircle" size="sm" />
				兑换成功！
				<span v-if="result.type === 'quota'" class="font-medium">
					获得 {{ result.credited?.toLocaleString() }} 额度
				</span>
				<span v-else-if="result.type === 'plan'" class="font-medium">
					获得 {{ result.months }} 个月套餐
				</span>
				<span v-else-if="result.type === 'duration'" class="font-medium">
					账户有效期延长 {{ result.extended_days }} 天
				</span>
			</div>
		</div>

		<!-- Recent redemption history -->
		<div class="card">
			<div class="card-header">
				<h3 class="text-base font-semibold text-gray-900">兑换记录</h3>
			</div>

			<!-- Loading -->
			<div v-if="historyLoading" class="flex items-center justify-center py-12">
				<div class="spinner"></div>
				<span class="ml-2 text-sm text-gray-400">加载中...</span>
			</div>

			<!-- Empty -->
			<div v-else-if="recentRedemptions.length === 0" class="flex flex-col items-center justify-center px-4 py-12 text-center">
				<div class="mb-4 text-gray-300">
					<Icon name="document" size="xl" />
				</div>
				<p class="text-sm text-gray-500">暂无兑换记录</p>
			</div>

			<!-- Table -->
			<div v-else class="table-container">
				<table class="table">
					<thead>
						<tr>
							<th>兑换码</th>
							<th>兑换类型</th>
							<th>面值</th>
							<th>时间</th>
						</tr>
					</thead>
					<tbody>
						<tr v-for="item in recentRedemptions" :key="item.id">
							<td class="font-mono text-xs">{{ item.code || '-' }}</td>
							<td>
								<span class="badge" :class="typeBadgeClasses[item.type] || 'badge-gray'">
									{{ typeLabels[item.type] || item.type }}
								</span>
							</td>
							<td class="font-mono">
								<template v-if="item.type === 'quota'">
									+{{ Number(item.value).toFixed(2) }}
								</template>
								<template v-else>
									-
								</template>
							</td>
							<td class="text-gray-400 text-xs">{{ item.created_at?.substring(0, 16) }}</td>
						</tr>
					</tbody>
				</table>
			</div>
		</div>
	</div>
</template>

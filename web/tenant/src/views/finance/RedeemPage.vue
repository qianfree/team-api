<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NInput } from 'naive-ui'
import Icon from '@/components/common/Icon.vue'
import ResponsiveDataTable from '@/components/common/ResponsiveDataTable.vue'
import { renderBadge, tableScrollX } from '@/utils/renderUtils'
import request from '@/utils/request'
import { formatBilling } from '@/composables/useCurrency'

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

// NDataTable 列定义（无分页，固定最近 10 条）
const columns = computed<DataTableColumns<any>>(() => [
	{
		title: '兑换码',
		key: 'code',
		width: 180,
		render: (row) => h('span', { class: 'font-mono text-xs' }, row.code || '-'),
	},
	{
		title: '兑换类型',
		key: 'type',
		width: 110,
		render: (row) => renderBadge(row.type, typeLabels, typeBadgeClasses),
	},
	{
		title: '面值',
		key: 'value',
		width: 130,
		// 兑换码面值属 bil 层（本位币），兑换入账恒为正数
		render: (row) =>
			row.type === 'quota'
				? h('span', { class: 'font-mono' }, formatBilling(row.value, 6, true))
				: h('span', { class: 'font-mono' }, '-'),
	},
	{
		title: '时间',
		key: 'created_at',
		width: 170,
		render: (row) => h('span', { class: 'text-gray-400 text-xs' }, row.created_at?.substring(0, 16)),
	},
])

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
					<n-input
						v-model:value="code"
						type="text"
						class="font-mono"
						placeholder="请输入兑换码"
						:maxlength="32"
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

			<ResponsiveDataTable
				:show-pagination="false"
				:loading="historyLoading"
				:columns="columns"
				:scroll-x="tableScrollX(columns)"
				:data="recentRedemptions"
				:row-key="(row: any) => row.id"
				card-title-key="code"
				card-badge-key="type"
				card-subtitle-key="created_at"
				:card-fields="['value']"
			>
				<template #empty>
					<div class="flex flex-col items-center justify-center px-4 py-12 text-center">
						<div class="mb-4 text-gray-300">
							<Icon name="document" size="xl" />
						</div>
						<p class="text-sm text-gray-500">暂无兑换记录</p>
					</div>
				</template>
			</ResponsiveDataTable>
		</div>
	</div>
</template>

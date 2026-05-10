<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import request from '@/utils/request'
import Icon from '@/components/common/Icon.vue'

interface ModelItem { model_id: string; model_name: string; category: string }
interface ComparisonItem {
	model_name: string
	requests: number
	success_rate: number
	avg_latency_ms: number
	p95_latency_ms: number
	total_cost: number
	avg_cost_per_request: number
	input_tokens: number
	output_tokens: number
	score: number
	is_recommended: boolean
}
interface TrendDetail { model_name: string; requests: number; cost: number; latency_ms: number }
interface TrendDay { date: string; details: TrendDetail[] }

const loading = ref(false)
const models = ref<ModelItem[]>([])
const selectedModels = ref<string[]>([])
const days = ref(7)
const dayOptions = [7, 30, 90]

const items = ref<ComparisonItem[]>([])
const trends = ref<TrendDay[]>([])
const summary = reactive({ total_requests: 0, total_cost: 0, recommended: '', reason: '' })

onMounted(loadModels)

async function loadModels() {
	try {
		const res = await request.get('/tenant/models')
		if (res.data?.code === 0) {
			models.value = (res.data.data.list || []).filter((m: ModelItem) => m.category === 'chat')
		}
	} catch (e) { console.error(e) }
}

const canCompare = computed(() => selectedModels.value.length >= 2 && selectedModels.value.length <= 4)

function toggleModel(name: string) {
	const idx = selectedModels.value.indexOf(name)
	if (idx >= 0) {
		selectedModels.value.splice(idx, 1)
	} else if (selectedModels.value.length < 4) {
		selectedModels.value.push(name)
	}
}

async function compare() {
	if (!canCompare.value) return
	loading.value = true
	try {
		const params = new URLSearchParams()
		params.set('models', selectedModels.value.join(','))
		params.set('days', String(days.value))
		const res = await request.get(`/tenant/model-comparison?${params}`)
		if (res.data?.code === 0) {
			const data = res.data.data
			items.value = data.items || []
			trends.value = data.trends || []
			summary.total_requests = data.summary.total_requests
			summary.total_cost = data.summary.total_cost
			summary.recommended = data.summary.recommended
			summary.reason = data.summary.reason
		}
	} catch (e) { console.error(e) } finally {
		loading.value = false
	}
}

// Find best value in a column for highlighting
function isBest(item: ComparisonItem, field: keyof ComparisonItem): boolean {
	if (items.value.length < 2) return false
	const values = items.value.filter(i => i.requests > 0).map(i => Number(i[field]))
	if (values.length === 0) return false
	const val = Number(item[field])
	if (field === 'success_rate' || field === 'score') return val === Math.max(...values)
	return val === Math.min(...values)
}

function formatCost(v: number): string {
	if (v < 0.01 && v > 0) return v.toFixed(6)
	return v.toFixed(4)
}

function formatPct(v: number): string {
	return v.toFixed(1) + '%'
}

// Simple trend chart via ASCII-like bars
const chartDays = computed(() => {
	if (trends.value.length === 0) return []
	return trends.value.slice(-14)
})

function getTrendCost(day: TrendDay, modelName: string): number {
	const d = day.details.find(x => x.model_name === modelName)
	return d?.cost || 0
}

function getTrendLatency(day: TrendDay, modelName: string): number {
	const d = day.details.find(x => x.model_name === modelName)
	return d?.latency_ms || 0
}

const maxTrendCost = computed(() => {
	let max = 0
	for (const day of chartDays.value) {
		for (const d of day.details) {
			if (d.cost > max) max = d.cost
		}
	}
	return max || 1
})

const modelColors = ['#14b8a6', '#f59e0b', '#8b5cf6', '#ef4444']
function modelColor(idx: number): string {
	return modelColors[idx % modelColors.length]
}
</script>

<template>
	<div>
		<div class="page-header">
			<h1 class="page-title">模型对比</h1>
			<p class="page-description">对比不同模型的费用、延迟和成功率，选择最适合您的模型</p>
		</div>

		<!-- Model Selection -->
		<div class="card mb-6">
			<div class="card-header">
				<h3 class="text-sm font-semibold text-gray-900">选择模型</h3>
				<span class="text-xs text-gray-400">已选 {{ selectedModels.length }} / 4</span>
			</div>
			<div class="card-body">
				<div class="flex flex-wrap gap-2 mb-4">
					<button
						v-for="m in models"
						:key="m.model_id"
						@click="toggleModel(m.model_id)"
						class="badge border transition-all cursor-pointer text-sm px-3 py-1.5"
						:class="selectedModels.includes(m.model_id)
							? 'bg-primary-50 text-primary-700 border-primary-300 shadow-sm'
							: 'bg-white text-gray-600 border-gray-200 hover:border-gray-300'"
					>
						{{ m.model_name || m.model_id }}
					</button>
				</div>
				<div class="flex items-center gap-4">
					<div class="flex items-center gap-2">
						<span class="text-sm text-gray-500">时间范围</span>
						<div class="tabs !inline-flex">
							<button
								v-for="d in dayOptions"
								:key="d"
								@click="days = d"
								class="tab !px-3 !py-1 text-xs"
								:class="days === d ? 'tab-active' : ''"
							>
								{{ d }}天
							</button>
						</div>
					</div>
					<button
						class="btn btn-primary"
						:disabled="!canCompare || loading"
						@click="compare"
					>
						<Icon v-if="loading" name="refresh" size="sm" class="animate-spin" />
						<Icon v-else name="chart" size="sm" />
						开始对比
					</button>
				</div>
			</div>
		</div>

		<!-- Results -->
		<template v-if="items.length > 0">
			<!-- Summary -->
			<div class="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
				<div class="stat-card">
					<div class="stat-icon stat-icon-primary">
						<Icon name="chart" size="lg" />
					</div>
					<div>
						<div class="stat-value">{{ summary.total_requests.toLocaleString() }}</div>
						<div class="stat-label">总请求数</div>
					</div>
				</div>
				<div class="stat-card">
					<div class="stat-icon stat-icon-warning">
						<Icon name="creditCard" size="lg" />
					</div>
					<div>
						<div class="stat-value">${{ formatCost(summary.total_cost) }}</div>
						<div class="stat-label">总费用</div>
					</div>
				</div>
				<div class="stat-card">
					<div class="stat-icon stat-icon-success">
						<Icon name="checkCircle" size="lg" />
					</div>
					<div>
						<div class="stat-value">{{ summary.recommended || '-' }}</div>
						<div class="stat-label">{{ summary.reason || '推荐模型' }}</div>
					</div>
				</div>
			</div>

			<!-- Comparison Table -->
			<div class="card mb-6">
				<div class="card-header">
					<h3 class="text-sm font-semibold text-gray-900">详细对比</h3>
				</div>
				<div class="table-container">
					<table class="table">
						<thead>
							<tr>
								<th>模型</th>
								<th>请求数</th>
								<th>成功率</th>
								<th>平均延迟</th>
								<th>P95 延迟</th>
								<th>总费用</th>
								<th>平均费用/次</th>
								<th>Token</th>
								<th>评分</th>
							</tr>
						</thead>
						<tbody>
							<tr v-for="item in items" :key="item.model_name">
								<td>
									<div class="flex items-center gap-2">
										<span class="font-medium text-gray-900 text-sm">{{ item.model_name }}</span>
										<span v-if="item.is_recommended" class="badge badge-success text-xs">推荐</span>
									</div>
								</td>
								<td :class="{ 'text-emerald-600 font-medium': isBest(item, 'requests') && item.requests > 0 }">
									{{ item.requests.toLocaleString() }}
								</td>
								<td :class="{ 'text-emerald-600 font-medium': isBest(item, 'success_rate') && item.requests > 0 }">
									{{ item.requests > 0 ? formatPct(item.success_rate) : '-' }}
								</td>
								<td :class="{ 'text-emerald-600 font-medium': isBest(item, 'avg_latency_ms') && item.requests > 0 }">
									{{ item.requests > 0 ? Math.round(item.avg_latency_ms) + 'ms' : '-' }}
								</td>
								<td :class="{ 'text-emerald-600 font-medium': isBest(item, 'p95_latency_ms') && item.requests > 0 }">
									{{ item.requests > 0 ? Math.round(item.p95_latency_ms) + 'ms' : '-' }}
								</td>
								<td :class="{ 'text-emerald-600 font-medium': isBest(item, 'total_cost') && item.requests > 0 }">
									{{ item.requests > 0 ? '$' + formatCost(item.total_cost) : '-' }}
								</td>
								<td :class="{ 'text-emerald-600 font-medium': isBest(item, 'avg_cost_per_request') && item.requests > 0 }">
									{{ item.requests > 0 ? '$' + formatCost(item.avg_cost_per_request) : '-' }}
								</td>
								<td class="text-xs text-gray-500">
									{{ item.requests > 0 ? (item.input_tokens + item.output_tokens).toLocaleString() : '-' }}
								</td>
								<td>
									<span v-if="item.requests > 0" class="badge" :class="item.score >= 70 ? 'badge-success' : item.score >= 40 ? 'badge-warning' : 'badge-gray'">
										{{ item.score.toFixed(0) }}
									</span>
									<span v-else class="text-gray-300">-</span>
								</td>
							</tr>
						</tbody>
					</table>
				</div>
			</div>

			<!-- Trend Chart (CSS bars) -->
			<div v-if="chartDays.length > 0" class="card">
				<div class="card-header">
					<h3 class="text-sm font-semibold text-gray-900">费用趋势</h3>
					<span class="text-xs text-gray-400">最近 {{ chartDays.length }} 天</span>
				</div>
				<div class="card-body">
					<div class="flex items-end gap-1 h-40">
						<div v-for="(day, di) in chartDays" :key="day.date" class="flex-1 flex flex-col items-center gap-1">
							<div class="flex gap-0.5 items-end h-32">
								<template v-for="(model, mi) in selectedModels" :key="model">
									<div
										class="w-2 rounded-t transition-all"
										:style="{
											height: Math.max(2, (getTrendCost(day, model) / maxTrendCost) * 120) + 'px',
											backgroundColor: modelColor(mi),
										}"
										:title="`${model}: $${getTrendCost(day, model).toFixed(4)}`"
									></div>
								</template>
							</div>
							<span class="text-xs text-gray-400 truncate w-full text-center">{{ day.date.slice(5) }}</span>
						</div>
					</div>
					<!-- Legend -->
					<div class="flex gap-4 mt-3 justify-center">
						<div v-for="(model, mi) in selectedModels" :key="model" class="flex items-center gap-1.5">
							<div class="w-3 h-3 rounded-sm" :style="{ backgroundColor: modelColor(mi) }"></div>
							<span class="text-xs text-gray-600">{{ model }}</span>
						</div>
					</div>
				</div>
			</div>
		</template>

		<!-- Empty State -->
		<div v-else-if="!loading" class="card">
			<div class="empty-state">
				<div class="empty-state-icon">
					<Icon name="chart" size="xl" />
				</div>
				<h3 class="empty-state-title">选择模型开始对比</h3>
				<p class="empty-state-description">选择 2-4 个模型，设定时间范围，点击「开始对比」查看详细数据</p>
			</div>
		</div>
	</div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Doughnut } from 'vue-chartjs'
import {
	Chart as ChartJS,
	ArcElement,
	Tooltip,
	Legend,
	type ChartOptions,
	type ChartData
} from 'chart.js'

ChartJS.register(ArcElement, Tooltip, Legend)

const props = withDefaults(defineProps<{
	data?: Array<{
		model_name: string
		requests: number
		input_tokens: number
		output_tokens: number
		total_cost: number
	}>
	loading?: boolean
}>(), {
	data: () => []
})

const COLORS = [
	'#14b8a6', '#3b82f6', '#8b5cf6', '#f59e0b', '#ef4444',
	'#ec4899', '#06b6d4', '#84cc16', '#f97316', '#6366f1',
	'#10b981', '#e11d48', '#0ea5e9', '#a855f7', '#eab308',
	'#d946ef', '#22d3ee', '#4ade80', '#fb923c', '#818cf8'
]

const formatNumber = (n: number): string => {
	if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
	if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
	return String(n)
}

const chartData = computed<ChartData<'doughnut'>>(() => ({
	labels: props.data.map(d => d.model_name),
	datasets: [{
		data: props.data.map(d => d.total_cost),
		backgroundColor: props.data.map((_, i) => COLORS[i % COLORS.length]),
		borderWidth: 2,
		borderColor: '#ffffff',
		hoverOffset: 6
	}]
}))

const chartOptions = computed<ChartOptions<'doughnut'>>(() => ({
	responsive: true,
	maintainAspectRatio: true,
	cutout: '65%',
	plugins: {
		legend: { display: false },
		tooltip: {
			backgroundColor: 'rgba(15, 23, 42, 0.9)',
			padding: 10,
			cornerRadius: 8,
			callbacks: {
				label: (ctx) => {
					const total = ctx.dataset.data.reduce((s, v) => s + (v as number), 0)
					const pct = total > 0 ? ((ctx.parsed / total) * 100).toFixed(1) : '0'
					return ` $${(ctx.parsed as number).toFixed(4)} (${pct}%)`
				}
			}
		}
	}
}))

const totalCost = computed(() => props.data.reduce((s, d) => s + d.total_cost, 0))

const modelList = computed(() =>
	props.data.map((d, i) => ({
		...d,
		total_tokens: d.input_tokens + d.output_tokens,
		color: COLORS[i % COLORS.length],
		percent: totalCost.value > 0 ? ((d.total_cost / totalCost.value) * 100).toFixed(1) : '0'
	}))
)
</script>

<template>
	<div class="card p-6">
		<div class="flex items-center justify-between mb-4">
			<h3 class="text-base font-semibold text-gray-900">模型使用分布</h3>
			<span class="text-xs text-gray-400">按费用排序</span>
		</div>
		<div v-if="loading" class="h-64 flex items-center justify-center">
			<div class="spinner h-8 w-8 text-primary-500"></div>
		</div>
		<div v-else-if="data.length === 0" class="h-64 flex flex-col items-center justify-center text-gray-400">
			<svg class="h-12 w-12 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" />
			</svg>
			<p class="text-sm">暂无模型数据</p>
		</div>
		<template v-else>
			<div class="flex flex-col lg:flex-row gap-6">
				<!-- Doughnut Chart -->
				<div class="flex-shrink-0 w-full lg:w-48 flex items-center justify-center">
					<div class="w-40 h-40">
						<Doughnut :data="chartData" :options="chartOptions" />
					</div>
				</div>
				<!-- Model Table -->
				<div class="flex-1 min-w-0 overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="border-b border-gray-100">
								<th class="pb-2 text-left font-medium text-gray-500 text-xs">模型</th>
								<th class="pb-2 text-right font-medium text-gray-500 text-xs">请求</th>
								<th class="pb-2 text-right font-medium text-gray-500 text-xs">Token</th>
								<th class="pb-2 text-right font-medium text-gray-500 text-xs">费用</th>
							</tr>
						</thead>
						<tbody>
							<tr v-for="item in modelList" :key="item.model_name" class="border-b border-gray-50 last:border-0">
								<td class="py-2 pr-3">
									<div class="flex items-center gap-2">
										<span class="w-2.5 h-2.5 rounded-full flex-shrink-0" :style="{ backgroundColor: item.color }"></span>
										<span class="text-gray-700 truncate max-w-[120px]" :title="item.model_name">{{ item.model_name }}</span>
									</div>
								</td>
								<td class="py-2 text-right text-gray-600 tabular-nums">{{ formatNumber(item.requests) }}</td>
								<td class="py-2 text-right text-gray-600 tabular-nums">{{ formatNumber(item.total_tokens) }}</td>
								<td class="py-2 text-right">
									<span class="text-emerald-600 font-medium tabular-nums">${{ item.total_cost.toFixed(4) }}</span>
								</td>
							</tr>
						</tbody>
					</table>
				</div>
			</div>
		</template>
	</div>
</template>

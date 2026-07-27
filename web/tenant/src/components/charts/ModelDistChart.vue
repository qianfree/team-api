<script setup lang="ts">
import { computed } from 'vue'
import { Doughnut } from 'vue-chartjs'
import {
	ArcElement,
	Chart as ChartJS,
	Legend,
	Tooltip,
	type ChartData,
	type ChartOptions,
} from 'chart.js'
import Icon from '@/components/common/Icon.vue'

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
	data: () => [],
})

const colors = ['#7568f8', '#42a4f5', '#2cc5c1', '#ffb552', '#f487a8', '#9f74ee', '#67d49d', '#ff826f']

function formatCost(value: number): string {
	if (value === 0) return '$0.00'
	if (value >= 1) return `$${value.toFixed(2)}`
	return `$${value.toFixed(4)}`
}

const totalCost = computed(() => props.data.reduce((sum, item) => sum + Number(item.total_cost || 0), 0))

const chartData = computed<ChartData<'doughnut'>>(() => ({
	labels: props.data.map((item) => item.model_name),
	datasets: [{
		data: props.data.map((item) => item.total_cost),
		backgroundColor: props.data.map((_, index) => colors[index % colors.length]),
		borderWidth: 4,
		borderColor: 'rgba(255, 255, 255, 0.92)',
		hoverBorderColor: '#ffffff',
		hoverOffset: 7,
		borderRadius: 4,
	}],
}))

const chartOptions = computed<ChartOptions<'doughnut'>>(() => ({
	responsive: true,
	maintainAspectRatio: true,
	cutout: '70%',
	animation: { duration: 650, easing: 'easeOutQuart' },
	plugins: {
		legend: { display: false },
		tooltip: {
			backgroundColor: 'rgba(30, 36, 61, 0.92)',
			padding: 12,
			cornerRadius: 12,
			callbacks: {
				label: (context) => {
					const value = Number(context.parsed) || 0
					const percent = totalCost.value > 0 ? ((value / totalCost.value) * 100).toFixed(1) : '0'
					return ` ${formatCost(value)} · ${percent}%`
				},
			},
		},
	},
}))

const modelList = computed(() =>
	props.data.slice(0, 5).map((item, index) => ({
		...item,
		color: colors[index % colors.length],
		percent: totalCost.value > 0 ? ((item.total_cost / totalCost.value) * 100).toFixed(1) : '0',
	})),
)
</script>

<template>
	<div class="card p-5 sm:p-6">
		<div class="mb-4 flex items-start justify-between">
			<div>
				<h2 class="text-base font-semibold text-slate-900">模型费用占比</h2>
				<p class="mt-0.5 text-xs text-slate-400">按模型统计调用成本</p>
			</div>
			<Icon name="more" size="md" class="text-slate-400" />
		</div>

		<div v-if="loading" class="flex h-[300px] items-center justify-center">
			<div class="spinner h-8 w-8 text-primary-500"></div>
		</div>
		<div v-else-if="data.length === 0" class="flex h-[300px] flex-col items-center justify-center text-slate-400">
			<div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-violet-50 text-violet-400">
				<Icon name="chart" size="lg" />
			</div>
			<p class="mt-3 text-sm font-medium text-slate-500">暂无模型数据</p>
			<p class="mt-1 text-xs">模型产生费用后即可查看占比</p>
		</div>
		<div v-else class="flex min-h-[300px] flex-col items-center gap-5 sm:flex-row 2xl:flex-col">
			<div class="relative h-52 w-52 flex-shrink-0">
				<Doughnut :data="chartData" :options="chartOptions" />
				<div class="pointer-events-none absolute inset-0 flex flex-col items-center justify-center text-center">
					<span class="text-xs text-slate-400">总费用</span>
					<strong class="mt-1 max-w-28 truncate text-xl font-bold tracking-tight text-slate-800">{{ formatCost(totalCost) }}</strong>
				</div>
			</div>
			<div class="w-full min-w-0 flex-1 space-y-3">
				<div v-for="item in modelList" :key="item.model_name" class="flex items-center gap-3">
					<span class="h-2.5 w-2.5 flex-shrink-0 rounded-full" :style="{ backgroundColor: item.color, boxShadow: `0 0 0 4px ${item.color}18` }"></span>
					<div class="min-w-0 flex-1">
						<p class="truncate text-xs font-medium text-slate-600" :title="item.model_name">{{ item.model_name }}</p>
						<p class="mt-0.5 text-[10px] text-slate-400">{{ formatCost(item.total_cost) }}</p>
					</div>
					<span class="text-xs font-semibold tabular-nums text-slate-700">{{ item.percent }}%</span>
				</div>
			</div>
		</div>
	</div>
</template>

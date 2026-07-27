<script setup lang="ts">
import { computed } from 'vue'
import { Line } from 'vue-chartjs'
import {
	CategoryScale,
	Chart as ChartJS,
	Filler,
	Legend,
	LinearScale,
	LineElement,
	PointElement,
	Title,
	Tooltip,
	type ChartData,
	type ChartOptions,
} from 'chart.js'
import Icon from '@/components/common/Icon.vue'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler)

const props = withDefaults(defineProps<{
	data?: Array<{
		date: string
		input_tokens: number
		output_tokens: number
		total_cost: number
		requests: number
	}>
	loading?: boolean
	days?: number
}>(), {
	data: () => [],
	days: 7,
})

const emit = defineEmits<{
	changeDays: [days: number]
}>()

function formatCompact(value: number): string {
	if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(1)}B`
	if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`
	if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`
	return String(value)
}

function formatDateLabel(date: string): string {
	const parts = date.split('-')
	return parts.length === 3 ? `${parts[1]}-${parts[2]}` : date
}

const chartData = computed<ChartData<'line'>>(() => ({
	labels: props.data.map((item) => formatDateLabel(item.date)),
	datasets: [
		{
			label: 'Token 用量',
			data: props.data.map((item) => item.input_tokens + item.output_tokens),
			borderColor: '#7568f8',
			backgroundColor: 'rgba(117, 104, 248, 0.1)',
			fill: true,
			tension: 0.42,
			pointRadius: 2.5,
			pointHoverRadius: 5,
			pointBackgroundColor: '#ffffff',
			pointBorderColor: '#7568f8',
			pointBorderWidth: 2,
			borderWidth: 2.25,
			yAxisID: 'y',
		},
		{
			label: '请求数',
			data: props.data.map((item) => item.requests),
			borderColor: '#399cf4',
			backgroundColor: 'rgba(57, 156, 244, 0.04)',
			fill: false,
			tension: 0.42,
			pointRadius: 2.5,
			pointHoverRadius: 5,
			pointBackgroundColor: '#ffffff',
			pointBorderColor: '#399cf4',
			pointBorderWidth: 2,
			borderWidth: 2.25,
			yAxisID: 'y1',
		},
	],
}))

const chartOptions = computed<ChartOptions<'line'>>(() => ({
	responsive: true,
	maintainAspectRatio: false,
	interaction: { mode: 'index', intersect: false },
	animation: { duration: 650, easing: 'easeOutQuart' },
	plugins: {
		legend: {
			position: 'top',
			align: 'start',
			labels: {
				usePointStyle: true,
				pointStyle: 'circle',
				boxWidth: 7,
				boxHeight: 7,
				padding: 18,
				color: '#718096',
				font: { size: 11, weight: 500 },
			},
		},
		tooltip: {
			backgroundColor: 'rgba(30, 36, 61, 0.92)',
			titleColor: '#ffffff',
			bodyColor: '#e2e8f0',
			padding: 12,
			cornerRadius: 12,
			displayColors: true,
			callbacks: {
				label: (context) => {
					const value = context.parsed.y ?? 0
					return `${context.dataset.label}: ${formatCompact(value)}`
				},
			},
		},
	},
	scales: {
		x: {
			border: { display: false },
			grid: { display: false },
			ticks: { color: '#9aa7bd', font: { size: 10 }, maxRotation: 0 },
		},
		y: {
			position: 'left',
			border: { display: false },
			grid: { color: 'rgba(148, 163, 184, 0.14)', lineWidth: 1 },
			ticks: {
				color: '#9aa7bd',
				font: { size: 10 },
				padding: 8,
				callback: (value) => formatCompact(Number(value) || 0),
			},
			beginAtZero: true,
		},
		y1: {
			position: 'right',
			border: { display: false },
			grid: { display: false },
			ticks: {
				color: '#9aa7bd',
				font: { size: 10 },
				padding: 8,
				callback: (value) => formatCompact(Number(value) || 0),
			},
			beginAtZero: true,
		},
	},
}))
</script>

<template>
	<div class="card p-5 sm:p-6">
		<div class="mb-2 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
			<div>
				<h2 class="text-base font-semibold text-slate-900">使用趋势</h2>
				<p class="mt-0.5 text-xs text-slate-400">Token 消耗与请求量变化</p>
			</div>
			<div class="inline-flex w-fit items-center gap-1 rounded-xl border border-white/90 bg-white/60 p-1 shadow-sm">
				<button
					v-for="option in [7, 30, 90]"
					:key="option"
					class="rounded-lg px-3 py-1.5 text-xs font-medium transition-all"
					:class="days === option ? 'bg-white text-primary-600 shadow-sm' : 'text-slate-400 hover:text-slate-600'"
					@click="emit('changeDays', option)"
				>
					{{ option }} 天
				</button>
			</div>
		</div>

		<div v-if="loading" class="flex h-[300px] items-center justify-center">
			<div class="spinner h-8 w-8 text-primary-500"></div>
		</div>
		<div v-else-if="data.length === 0" class="flex h-[300px] flex-col items-center justify-center text-slate-400">
			<div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary-50 text-primary-400">
				<Icon name="chart" size="lg" />
			</div>
			<p class="mt-3 text-sm font-medium text-slate-500">暂无趋势数据</p>
			<p class="mt-1 text-xs">完成 API 调用后即可查看趋势</p>
		</div>
		<div v-else class="h-[300px]">
			<Line :data="chartData" :options="chartOptions" />
		</div>
	</div>
</template>

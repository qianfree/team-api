<script setup lang="ts">
import { computed } from 'vue'
import { Line } from 'vue-chartjs'
import {
	Chart as ChartJS,
	CategoryScale,
	LinearScale,
	PointElement,
	LineElement,
	Title,
	Tooltip,
	Legend,
	Filler,
	type ChartOptions,
	type ChartData
} from 'chart.js'

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
}>(), {
	data: () => []
})

const formatTokenLabel = (value: number): string => {
	if (value >= 1_000_000_000) return (value / 1_000_000_000).toFixed(1) + 'B'
	if (value >= 1_000_000) return (value / 1_000_000).toFixed(1) + 'M'
	if (value >= 1_000) return (value / 1_000).toFixed(1) + 'K'
	return String(value)
}

const formatDateLabel = (dateStr: string): string => {
	if (!dateStr) return ''
	const parts = dateStr.split('-')
	if (parts.length === 3) return `${parts[1]}/${parts[2]}`
	return dateStr
}

const chartData = computed<ChartData<'line'>>(() => ({
	labels: props.data.map(d => formatDateLabel(d.date)),
	datasets: [
		{
			label: '输入 Token',
			data: props.data.map(d => d.input_tokens),
			borderColor: '#3b82f6',
			backgroundColor: 'rgba(59, 130, 246, 0.08)',
			fill: true,
			tension: 0.4,
			pointRadius: 2,
			pointHoverRadius: 5,
			borderWidth: 2,
			yAxisID: 'y'
		},
		{
			label: '输出 Token',
			data: props.data.map(d => d.output_tokens),
			borderColor: '#10b981',
			backgroundColor: 'rgba(16, 185, 129, 0.08)',
			fill: true,
			tension: 0.4,
			pointRadius: 2,
			pointHoverRadius: 5,
			borderWidth: 2,
			yAxisID: 'y'
		},
		{
			label: '费用 ($)',
			data: props.data.map(d => d.total_cost),
			borderColor: '#f59e0b',
			backgroundColor: 'transparent',
			fill: false,
			tension: 0.4,
			pointRadius: 2,
			pointHoverRadius: 5,
			borderWidth: 2,
			borderDash: [5, 5],
			yAxisID: 'y1'
		}
	]
}))

const chartOptions = computed<ChartOptions<'line'>>(() => ({
	responsive: true,
	maintainAspectRatio: false,
	interaction: {
		mode: 'index',
		intersect: false
	},
	plugins: {
		legend: {
			position: 'top',
			align: 'end',
			labels: {
				usePointStyle: true,
				pointStyle: 'circle',
				boxWidth: 6,
				boxHeight: 6,
				padding: 16,
				font: { size: 12 }
			}
		},
		tooltip: {
			backgroundColor: 'rgba(15, 23, 42, 0.9)',
			titleFont: { size: 12 },
			bodyFont: { size: 12 },
			padding: 12,
			cornerRadius: 8,
			callbacks: {
				label: (ctx) => {
					const label = ctx.dataset.label || ''
					const value = ctx.parsed.y ?? 0
					if (label.includes('费用')) return `${label}: $${value.toFixed(4)}`
					return `${label}: ${formatTokenLabel(value)}`
				}
			}
		}
	},
	scales: {
		x: {
			grid: { display: false },
			ticks: {
				font: { size: 11 },
				color: '#9ca3af',
				maxRotation: 0
			}
		},
		y: {
			position: 'left',
			grid: { color: 'rgba(0,0,0,0.04)' },
			ticks: {
				font: { size: 11 },
				color: '#9ca3af',
				callback: (value) => formatTokenLabel((value as number) ?? 0)
			},
			beginAtZero: true
		},
		y1: {
			position: 'right',
			grid: { display: false },
			ticks: {
				font: { size: 11 },
				color: '#f59e0b',
				callback: (value) => '$' + ((value as number) ?? 0).toFixed(2)
			},
			beginAtZero: true
		}
	}
}))
</script>

<template>
	<div class="card p-6">
		<div class="flex items-center justify-between mb-4">
			<h3 class="text-base font-semibold text-gray-900">Token 使用趋势</h3>
		</div>
		<div v-if="loading" class="h-64 flex items-center justify-center">
			<div class="spinner h-8 w-8 text-primary-500"></div>
		</div>
		<div v-else-if="data.length === 0" class="h-64 flex flex-col items-center justify-center text-gray-400">
			<svg class="h-12 w-12 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" />
			</svg>
			<p class="text-sm">暂无趋势数据</p>
		</div>
		<div v-else class="h-64">
			<Line :data="chartData" :options="chartOptions" />
		</div>
	</div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
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
import { formatBilling } from '@/composables/useCurrency'
import { formatNumber } from '@/utils/renderUtils'

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
	title?: string
	subtitle?: string
}>(), {
	data: () => [],
	days: 7,
	title: '消费趋势',
	subtitle: '按天聚合，一次查看一个度量',
})

const emit = defineEmits<{
	changeDays: [days: number]
}>()

type MetricKey = 'cost' | 'tokens' | 'requests'

// 单轴 + 度量切换：原实现把「Token 用量」挂左轴、「请求数」挂右轴，
// 两条不同量纲的线共用一张图，交叉点与相对高低都不代表任何含义。
// 改为一次只画一个度量，纵轴含义始终唯一。
const METRICS: Record<MetricKey, {
	label: string
	color: string
	soft: string
	value: (item: { input_tokens: number; output_tokens: number; total_cost: number; requests: number }) => number
	format: (value: number) => string
}> = {
	cost: {
		label: '消费',
		color: '#14b8a6',
		soft: 'rgba(20, 184, 166, 0.12)',
		value: (item) => Number(item.total_cost) || 0,
		// 金额一律走本位币格式化，精度规则与 ModelDistChart 保持一致
		format: (value) => (value === 0 || value >= 1 ? formatBilling(value, 2) : formatBilling(value, 4)),
	},
	tokens: {
		label: 'Token',
		color: '#8b5cf6',
		soft: 'rgba(139, 92, 246, 0.12)',
		value: (item) => (Number(item.input_tokens) || 0) + (Number(item.output_tokens) || 0),
		format: (value) => formatNumber(value),
	},
	requests: {
		label: '请求数',
		color: '#3b82f6',
		soft: 'rgba(59, 130, 246, 0.12)',
		value: (item) => Number(item.requests) || 0,
		format: (value) => formatNumber(value),
	},
}

const METRIC_ORDER: MetricKey[] = ['cost', 'tokens', 'requests']

const metric = ref<MetricKey>('cost')
const activeMetric = computed(() => METRICS[metric.value])

function formatDateLabel(date: string): string {
	const parts = date.split('-')
	return parts.length === 3 ? `${parts[1]}-${parts[2]}` : date
}

const chartData = computed<ChartData<'line'>>(() => {
	const cfg = activeMetric.value
	return {
		labels: props.data.map((item) => formatDateLabel(item.date)),
		datasets: [
			{
				label: cfg.label,
				data: props.data.map((item) => cfg.value(item)),
				borderColor: cfg.color,
				backgroundColor: cfg.soft,
				fill: true,
				tension: 0.42,
				pointRadius: 2.5,
				pointHoverRadius: 5,
				pointBackgroundColor: '#ffffff',
				pointBorderColor: cfg.color,
				pointBorderWidth: 2,
				borderWidth: 2.25,
			},
		],
	}
})

// 必须保持 computed：tooltip 与刻度回调要在 displayCurrency 变化时重新求值
const chartOptions = computed<ChartOptions<'line'>>(() => {
	const cfg = activeMetric.value
	return {
		responsive: true,
		maintainAspectRatio: false,
		interaction: { mode: 'index', intersect: false },
		animation: { duration: 650, easing: 'easeOutQuart' },
		plugins: {
			// 只有一条线时不需要图例——上方的度量切换器本身就是图例
			legend: { display: false },
			tooltip: {
				backgroundColor: 'rgba(30, 36, 61, 0.92)',
				titleColor: '#ffffff',
				bodyColor: '#e2e8f0',
				padding: 12,
				cornerRadius: 12,
				displayColors: true,
				callbacks: {
					label: (context) => `${cfg.label}: ${cfg.format(context.parsed.y ?? 0)}`,
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
					callback: (value) => cfg.format(Number(value) || 0),
				},
				beginAtZero: true,
			},
		},
	}
})
</script>

<template>
	<div class="card card-prominent p-5 sm:p-6">
		<div class="mb-3 flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
			<div>
				<h2 class="text-base font-semibold text-slate-900">{{ title }}</h2>
				<p class="mt-0.5 text-xs text-slate-400">{{ subtitle }}</p>
			</div>

			<div class="flex flex-wrap items-center gap-2">
				<div class="inline-flex w-fit items-center gap-1 rounded-xl bg-slate-100 p-1" role="group" aria-label="切换度量">
					<button
						v-for="key in METRIC_ORDER"
						:key="key"
						type="button"
						class="rounded-lg px-3 py-1.5 text-xs font-medium transition-all"
						:class="metric === key ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-400 hover:text-slate-600'"
						:aria-pressed="metric === key"
						@click="metric = key"
					>
						{{ METRICS[key].label }}
					</button>
				</div>

				<div class="inline-flex w-fit items-center gap-1 rounded-xl border border-white/90 bg-white/60 p-1 shadow-sm" role="group" aria-label="切换时间范围">
					<button
						v-for="option in [7, 30, 90]"
						:key="option"
						type="button"
						class="rounded-lg px-3 py-1.5 text-xs font-medium transition-all"
						:class="days === option ? 'bg-white text-primary-600 shadow-sm' : 'text-slate-400 hover:text-slate-600'"
						:aria-pressed="days === option"
						@click="emit('changeDays', option)"
					>
						{{ option }} 天
					</button>
				</div>
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

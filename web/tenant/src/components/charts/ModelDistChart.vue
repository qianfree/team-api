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
import { formatBilling } from '@/composables/useCurrency'
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

// 分类色序：相邻切片必须能分辨。原顺序前两位 #14b8a6 / #06b6d4 在常视觉下色差
// ΔE 仅 7.1（阈值 15）、色盲模式 6.8，占比接近的两个模型几乎无法区分。
// 现顺序经校验：明度带 / 彩度 / 色盲分辨 / 常视觉分辨全部通过。
// 绿色 #10b981 已移出——它是语义状态色（成功），不兼任数据系列。
const colors = ['#14b8a6', '#8b5cf6', '#f59e0b', '#3b82f6', '#ec4899', '#06b6d4', '#ef4444', '#6366f1']

// 金额格式化统一走本位币；chartOptions 为 computed，tooltip 回调随 displayCurrency 变化重渲染
function formatCost(value: number): string {
	if (value === 0 || value >= 1) return formatBilling(value, 2)
	return formatBilling(value, 4)
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

// 列表在卡内滚动，不再截断——原先只显示前 5 条，其余模型完全看不到
const modelList = computed(() =>
	props.data.map((item, index) => ({
		...item,
		color: colors[index % colors.length],
		percent: totalCost.value > 0 ? ((item.total_cost / totalCost.value) * 100).toFixed(1) : '0',
	})),
)
</script>

<template>
	<div class="card card-prominent p-5 sm:p-6">
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
		<!-- 左环右列：卡片高度由环图决定，列表超出部分在卡内滚动，不再撑长整张卡 -->
		<div v-else class="flex flex-col items-center gap-4 sm:flex-row sm:items-start sm:gap-5">
			<div class="relative h-44 w-44 flex-shrink-0 sm:h-40 sm:w-40 xl:h-44 xl:w-44">
				<Doughnut :data="chartData" :options="chartOptions" />
				<div class="pointer-events-none absolute inset-0 flex flex-col items-center justify-center text-center">
					<span class="text-[11px] text-slate-400">总费用</span>
					<strong class="mt-0.5 max-w-24 truncate text-lg font-bold tracking-tight text-slate-800">{{ formatCost(totalCost) }}</strong>
				</div>
			</div>

			<div class="model-scroll w-full min-w-0 flex-1">
				<div class="space-y-2.5 pr-1">
					<div v-for="item in modelList" :key="item.model_name" class="flex items-center gap-2.5">
						<span
							class="h-2.5 w-2.5 flex-shrink-0 rounded-full"
							:style="{ backgroundColor: item.color, boxShadow: `0 0 0 4px ${item.color}18` }"
						></span>
						<div class="min-w-0 flex-1">
							<p class="truncate text-xs font-medium text-slate-600" :title="item.model_name">{{ item.model_name }}</p>
							<p class="mt-0.5 text-[10px] tabular-nums text-slate-400">{{ formatCost(item.total_cost) }}</p>
						</div>
						<span class="flex-shrink-0 text-xs font-semibold tabular-nums text-slate-700">{{ item.percent }}%</span>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped>
/* 列表最大高度与左侧环图对齐：卡片高度由环图决定，多出的模型在卡内滚动。
   滚动条沿用 main.css 的全局策略——默认透明，悬停/聚焦时才显形。 */
.model-scroll {
	max-height: 11rem;
	overflow-y: auto;
	overscroll-behavior: contain;
}

@media (min-width: 640px) {
	.model-scroll {
		max-height: 10rem;
	}
}

@media (min-width: 1280px) {
	.model-scroll {
		max-height: 11rem;
	}
}
</style>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/common/Icon.vue'
import VendorLogo from '@/components/common/VendorLogo.vue'
import type { MarketplaceModel } from '@/api/marketplace'
import { formatPrice, formatTokens, getCategoryMeta, getCapabilityList } from '../marketplaceMeta'

const props = defineProps<{
	model: MarketplaceModel
	index: number
}>()

const emit = defineEmits<{
	open: [model: MarketplaceModel]
	copy: [modelId: string]
}>()

const meta = computed(() => getCategoryMeta(props.model.category))
const capabilities = computed(() => getCapabilityList(props.model, 3))

// 无简介时用类别说明兜底：十几张卡片重复同一句占位文案纯属噪音，
// 类别说明既稳住两行高度，又顺带交代了这类模型能做什么
const description = computed(() => props.model.description?.trim() || meta.value.description)

// 价格文案按计费模式派生，避免模板里堆叠条件
const priceView = computed(() => {
	const model = props.model
	if (model.billing_mode === 'per_request') {
		return { caption: '按次计费', main: formatPrice(model.per_request_price), secondary: '', unit: '/ 次' }
	}
	return {
		caption: model.billing_mode === 'tiered' ? '输入 / 输出（首档起）' : '输入 / 输出',
		main: formatPrice(model.input_price),
		secondary: formatPrice(model.output_price),
		unit: '/ 1M tokens',
	}
})

// 类别徽章沿用详情抽屉的配色规则：类别色文字 + 同色 10% 底
const categoryBadgeStyle = computed(() => ({
	color: meta.value.to,
	backgroundColor: `rgba(${meta.value.glow}, 0.1)`,
}))

// 类别渐变注入 CSS 变量（卡片顶部指示线）；入场延迟按序号递增形成 stagger（封顶 11 张）
const cardStyle = computed(() => ({
	'--cat-from': meta.value.from,
	'--cat-to': meta.value.to,
	animationDelay: `${Math.min(props.index, 11) * 45}ms`,
}))
</script>

<template>
	<article
		class="card-in card group relative flex cursor-pointer flex-col overflow-hidden p-4 transition-all duration-300 hover:-translate-y-1 hover:border-primary-200/70 hover:shadow-card-hover"
		:style="cardStyle"
		@click="emit('open', model)"
	>
		<!-- 顶部类别指示线：hover 时从左展开 -->
		<span
			aria-hidden="true"
			class="absolute inset-x-0 top-0 h-[2px] origin-left scale-x-0 bg-gradient-to-r from-[var(--cat-from)] to-[var(--cat-to)] transition-transform duration-300 group-hover:scale-x-100"
		></span>

		<!-- 头部：厂商芯片 + 名称 / 模型 ID，右侧类别与折扣徽章 -->
		<div class="flex items-start gap-3">
			<VendorLogo :vendor="model.vendor" size="md" class="mt-px" />
			<div class="min-w-0 flex-1">
				<h3 class="truncate text-[15px] font-semibold leading-6 text-gray-900">{{ model.model_name || model.model_id }}</h3>
				<button
					type="button"
					:aria-label="`复制模型 ID ${model.model_id}`"
					title="复制模型 ID"
					class="group/copy mt-0.5 flex max-w-full items-center gap-1 text-left"
					@click.stop="emit('copy', model.model_id)"
				>
					<code class="truncate font-mono text-[11px] text-gray-400 transition-colors group-hover/copy:text-primary-600">{{ model.model_id }}</code>
					<Icon name="copy" size="xs" class="shrink-0 text-gray-300 transition-colors group-hover/copy:text-primary-600" />
				</button>
			</div>
			<div class="flex shrink-0 flex-col items-end gap-1">
				<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" :style="categoryBadgeStyle">{{ meta.label }}</span>
				<span v-if="model.discount_label" class="badge badge-warning !px-2 !py-0 !text-[11px]">{{ model.discount_label }}</span>
			</div>
		</div>

		<!-- 简介：无描述时回退为类别说明，保证卡片高度稳定 -->
		<p class="mt-3 line-clamp-2 min-h-[2.75rem] text-[13px] leading-[1.6] text-gray-500">{{ description }}</p>

		<!-- 规格：上下文 / 最大输出，紧凑单行 -->
		<div class="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-gray-400">
			<span class="inline-flex items-center gap-1">
				<Icon name="document" size="xs" class="text-gray-300" />
				<span class="text-gray-500">{{ formatTokens(model.max_context_tokens) }}</span> 上下文
			</span>
			<span class="text-gray-200">·</span>
			<span class="inline-flex items-center gap-1">
				<Icon name="expand" size="xs" class="text-gray-300" />
				<span class="text-gray-500">{{ formatTokens(model.max_output_tokens) }}</span> 输出
			</span>
		</div>

		<!-- 能力：最多 3 个 -->
		<div v-if="capabilities.length" class="mt-2.5 flex min-h-[1.375rem] flex-wrap gap-1.5">
			<span
				v-for="item in capabilities"
				:key="item"
				class="inline-flex items-center rounded-md border border-gray-200/80 bg-gray-50/80 px-1.5 py-0.5 text-[11px] font-medium text-gray-500"
			>
				{{ item }}
			</span>
		</div>

		<!-- 底栏：价格 + 详情入口（mt-auto 保证同行卡片价格底线对齐） -->
		<div class="mt-auto flex items-end justify-between gap-3 border-t border-gray-100 pt-3.5">
			<div class="min-w-0">
				<span class="text-[11px] text-gray-400">{{ priceView.caption }}</span>
				<div class="mt-0.5 flex items-baseline gap-2">
					<strong class="text-base font-bold tabular-nums text-gray-900">{{ priceView.main }}</strong>
					<strong v-if="priceView.secondary" class="text-[13px] font-semibold tabular-nums text-gray-500">{{ priceView.secondary }}</strong>
					<span class="whitespace-nowrap text-[11px] text-gray-400">{{ priceView.unit }}</span>
				</div>
			</div>
			<button
				type="button"
				:aria-label="`查看 ${model.model_name || model.model_id} 详情`"
				class="inline-flex shrink-0 items-center gap-1 rounded-lg px-2.5 py-1.5 text-[13px] font-medium text-primary-600 transition-all duration-200 group-hover:bg-primary-500 group-hover:text-white group-hover:shadow-glow"
				@click.stop="emit('open', model)"
			>
				详情
				<Icon name="arrowRight" size="sm" class="transition-transform duration-200 group-hover:translate-x-0.5" />
			</button>
		</div>
	</article>
</template>

<style scoped>
/* 卡片入场：backwards 填充保证延迟期间隐藏、结束后不覆盖 hover 位移 */
.card-in {
	animation: card-in 0.45s cubic-bezier(0.22, 1, 0.36, 1) backwards;
}

@keyframes card-in {
	from {
		opacity: 0;
		transform: translateY(12px);
	}
	to {
		opacity: 1;
		transform: translateY(0);
	}
}

@media (prefers-reduced-motion: reduce) {
	.card-in {
		animation: none;
	}
}
</style>

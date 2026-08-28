<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/common/Icon.vue'
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
const capabilities = computed(() => getCapabilityList(props.model, 4))

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

// 类别渐变注入 CSS 变量；入场延迟按序号递增形成 stagger（封顶 11 张）
const cardStyle = computed(() => ({
	'--cat-from': meta.value.from,
	'--cat-to': meta.value.to,
	'--cat-glow': meta.value.glow,
	animationDelay: `${Math.min(props.index, 11) * 45}ms`,
}))
</script>

<template>
	<article
		class="card-in group relative flex cursor-pointer flex-col overflow-hidden rounded-[20px] border border-white/80 bg-white/85 shadow-card backdrop-blur-xl transition-all duration-300 hover:-translate-y-1.5 hover:border-primary-200/70 hover:shadow-card-hover"
		:style="cardStyle"
		@click="emit('open', model)"
	>
		<!-- 顶部类别渐变线：hover 时从左展开 -->
		<span
			aria-hidden="true"
			class="absolute inset-x-0 top-0 z-10 h-[3px] origin-left scale-x-0 bg-gradient-to-r from-[var(--cat-from)] to-[var(--cat-to)] transition-transform duration-300 group-hover:scale-x-100"
		></span>
		<!-- 右上角类别光晕：hover 淡入 -->
		<span
			aria-hidden="true"
			class="pointer-events-none absolute -right-12 -top-12 h-32 w-32 rounded-full opacity-0 blur-2xl transition-opacity duration-300 group-hover:opacity-25"
			style="background: var(--cat-to)"
		></span>

		<div class="relative flex-1 pb-4">
			<!-- 头部：类别磁贴 + 名称 / 模型 ID -->
			<div class="flex items-start gap-4 p-5 pb-0">
				<div
					class="grid h-14 w-14 shrink-0 place-items-center rounded-2xl text-white transition-transform duration-300 group-hover:-rotate-3 group-hover:scale-105"
					:style="{ background: 'linear-gradient(135deg, var(--cat-from), var(--cat-to))', boxShadow: '0 8px 24px rgba(var(--cat-glow), 0.35)' }"
				>
					<Icon :name="meta.icon" size="lg" />
				</div>
				<div class="min-w-0 flex-1">
					<div class="flex items-start gap-2">
						<h3 class="line-clamp-1 text-base font-semibold text-gray-900">{{ model.model_name || model.model_id }}</h3>
						<span v-if="model.discount_label" class="shrink-0 rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-semibold text-amber-700">
							{{ model.discount_label }}
						</span>
					</div>
					<button
						type="button"
						:aria-label="`复制模型 ID ${model.model_id}`"
						title="复制模型 ID"
						class="group/copy mt-1 flex max-w-full items-center gap-1.5 text-left"
						@click.stop="emit('copy', model.model_id)"
					>
						<code class="truncate font-mono text-xs text-gray-400 transition-colors group-hover/copy:text-primary-600">{{ model.model_id }}</code>
						<Icon name="copy" size="xs" class="shrink-0 text-gray-300 transition-colors group-hover/copy:text-primary-600" />
					</button>
				</div>
			</div>

			<!-- 简介 -->
			<p class="line-clamp-2 min-h-[2.5rem] px-5 pt-3 text-sm leading-relaxed text-gray-500">
				{{ model.description || '暂无模型描述，点击查看技术规格与计费信息。' }}
			</p>

			<!-- 规格：上下文 / 最大输出 -->
			<div class="flex gap-2 px-5 pt-3">
				<div class="flex-1 rounded-xl bg-gray-50/90 px-3 py-2">
					<span class="text-[11px] text-gray-400">上下文</span>
					<strong class="block text-sm font-semibold tabular-nums text-gray-800">{{ formatTokens(model.max_context_tokens) }}</strong>
				</div>
				<div class="flex-1 rounded-xl bg-gray-50/90 px-3 py-2">
					<span class="text-[11px] text-gray-400">最大输出</span>
					<strong class="block text-sm font-semibold tabular-nums text-gray-800">{{ formatTokens(model.max_output_tokens) }}</strong>
				</div>
			</div>

			<!-- 能力芯片：最多 3 个 -->
			<div v-if="capabilities.length" class="flex flex-wrap gap-1.5 px-5 pt-3">
				<span
					v-for="item in capabilities"
					:key="item"
					class="inline-flex items-center gap-1 rounded-full bg-primary-50 px-2 py-0.5 text-[11px] font-medium text-primary-700"
				>
					<Icon name="check" size="xs" />{{ item }}
				</span>
			</div>
		</div>

		<!-- 底栏：价格 + 详情入口 -->
		<div class="flex items-end justify-between gap-3 border-t border-gray-100/80 px-5 pb-4 pt-4">
			<div class="min-w-0">
				<span class="text-[11px] text-gray-400">{{ priceView.caption }}</span>
				<div class="flex items-baseline gap-1">
					<strong class="text-lg font-bold tabular-nums text-gray-900">{{ priceView.main }}</strong>
					<strong v-if="priceView.secondary" class="text-sm font-semibold tabular-nums text-gray-600">{{ priceView.secondary }}</strong>
					<span class="whitespace-nowrap text-[11px] text-gray-400">{{ priceView.unit }}</span>
				</div>
			</div>
			<button
				type="button"
				class="inline-flex shrink-0 items-center gap-1.5 rounded-xl px-3.5 py-2 text-sm font-medium text-primary-600 transition-all duration-200 hover:bg-primary-50 active:scale-[0.98] group-hover:bg-primary-500 group-hover:text-white group-hover:shadow-glow"
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
	animation: card-in 0.5s cubic-bezier(0.22, 1, 0.36, 1) backwards;
}

@keyframes card-in {
	from {
		opacity: 0;
		transform: translateY(14px);
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

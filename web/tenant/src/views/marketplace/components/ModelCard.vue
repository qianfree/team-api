<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/common/Icon.vue'
import VendorLogo from '@/components/common/VendorLogo.vue'
import type { MarketplaceModel } from '@/api/marketplace'
import { getCapabilityList, getCategoryMeta, getCardView, getCardBadge } from '../marketplaceMeta'

const props = defineProps<{
	model: MarketplaceModel
}>()

const emit = defineEmits<{
	open: [model: MarketplaceModel]
	copy: [modelId: string]
}>()

const view = computed(() => getCardView(props.model))
const badge = computed(() => getCardBadge(props.model))
const meta = computed(() => getCategoryMeta(props.model.category))

// 简介兜底：无描述时用类别说明占位，稳住两行高度
const description = computed(() => props.model.description?.trim() || meta.value.description)

// 能力压缩到 2 个 + 溢出计数（完整列表在详情抽屉）
const capabilities = computed(() => getCapabilityList(props.model, 2))
const capabilityOverflow = computed(() => Math.max(0, getCapabilityList(props.model, Number.MAX_SAFE_INTEGER).length - 2))

const displayName = computed(() => props.model.model_name || props.model.model_id)

function onKeydown(event: KeyboardEvent): void {
	if (event.key === 'Enter' || event.key === ' ') {
		event.preventDefault()
		emit('open', props.model)
	}
}
</script>

<template>
	<article
		class="mp-card group"
		role="button"
		tabindex="0"
		:aria-label="`查看 ${displayName} 详情`"
		@click="emit('open', model)"
		@keydown="onKeydown"
	>
		<!-- 身份行：厂商 logo + 名称 + 单状态徽章 + mono ID -->
		<div class="flex items-start gap-2.5">
			<VendorLogo :vendor="model.vendor" size="md" class="mt-px shrink-0" />
			<div class="min-w-0 flex-1">
				<div class="flex items-start justify-between gap-2">
					<h3 class="mp-name">{{ displayName }}</h3>
					<span v-if="badge" class="mp-badge" :class="`is-${badge.type}`">{{ badge.text }}</span>
				</div>
				<button
					type="button"
					class="mp-id group/id"
					:aria-label="`复制模型 ID ${model.model_id}`"
					title="复制模型 ID"
					@click.stop="emit('copy', model.model_id)"
				>
					<code>{{ model.model_id }}</code>
					<Icon name="copy" size="xs" class="shrink-0" />
				</button>
			</div>
		</div>

		<!-- 价格锚：全卡唯一大号元素（输入亮、输出暗；按次/按秒单锚） -->
		<div class="mt-3 flex items-end justify-between gap-3">
			<div class="flex min-w-0 flex-wrap items-end gap-x-5 gap-y-1">
				<div v-for="slot in view.price.slots" :key="slot.label" class="min-w-0">
					<div class="flex items-baseline">
						<span class="mp-price" :class="{ 'is-secondary': !slot.primary }">{{ slot.value }}</span>
						<span v-if="view.price.fromNote && slot.primary" class="mp-from">起</span>
					</div>
					<span class="mp-unit">{{ slot.label }} {{ slot.unit }}</span>
				</div>
			</div>
			<span v-if="view.price.hasTimePrices" class="mp-time-flag" title="配置了分时段价格，详见详情">分时价</span>
		</div>

		<!-- 简介：clamp 2 行 + 固定 min-height 保证网格底对齐 -->
		<p class="mp-desc">{{ description }}</p>

		<!-- 规格 + 能力：合并为紧凑单行区 -->
		<div class="mp-meta-row">
			<template v-for="(seg, i) in view.specSegments" :key="i">
				<span v-if="i > 0" class="mp-dot" aria-hidden="true">·</span>
				<span v-if="seg.value" class="mp-spec"><span class="mp-spec-label">{{ seg.label }}</span><span class="mp-spec-value">{{ seg.value }}</span></span>
				<span v-else class="mp-spec-value">{{ seg.label }}</span>
			</template>
			<template v-if="capabilities.length">
				<span v-if="view.specSegments.length" class="mp-dot" aria-hidden="true">·</span>
				<span v-for="cap in capabilities" :key="cap" class="mp-cap">{{ cap }}</span>
				<span v-if="capabilityOverflow > 0" class="mp-cap is-more">+{{ capabilityOverflow }}</span>
			</template>
		</div>
	</article>
</template>

<style scoped>
/* 卡片：无阴影、1px 描边分层；hover 上浮 + teal 描边发光（颜色全部走页面级 --mp-* token） */
.mp-card {
	display: flex;
	flex-direction: column;
	height: 100%;
	padding: 18px;
	background: var(--mp-panel);
	border: 1px solid var(--mp-line);
	border-radius: 12px;
	cursor: pointer;
	transition: transform 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease, background 0.2s ease;
}

.mp-card:hover,
.mp-card:focus-visible {
	transform: translateY(-2px);
	border-color: rgba(var(--mp-brand-rgb), 0.45);
	box-shadow: 0 0 0 1px rgba(var(--mp-brand-rgb), 0.28), 0 8px 32px rgba(var(--mp-brand-rgb), 0.1);
	background: var(--mp-panel-hover);
}

.mp-card:focus-visible {
	outline: 2px solid var(--mp-brand);
	outline-offset: 2px;
}

.mp-name {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 16px;
	font-weight: 600;
	line-height: 1.4;
	color: var(--mp-text);
}

.mp-id {
	display: flex;
	align-items: center;
	gap: 4px;
	max-width: 100%;
	margin-top: 1px;
	padding: 0;
	border: 0;
	background: none;
	cursor: copy;
}

.mp-id code {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-family: var(--mp-font-mono);
	font-size: 12px;
	color: var(--mp-text-mute);
	transition: color 0.15s ease;
}

.mp-id:hover code,
.mp-id:focus-visible code {
	color: var(--mp-brand);
}

.mp-id:focus-visible {
	outline: none;
}

.mp-id:focus-visible code {
	outline: 1px dashed var(--mp-brand);
	outline-offset: 2px;
}

/* 状态徽章：至多一枚（折扣玫红 / NEW 靛蓝 / 推荐琥珀） */
.mp-badge {
	flex-shrink: 0;
	padding: 1px 8px;
	border-radius: 999px;
	font-size: 12px;
	font-weight: 600;
	line-height: 1.5;
	white-space: nowrap;
}

.mp-badge.is-deal {
	color: var(--mp-deal);
	background: rgba(var(--mp-deal-rgb), 0.12);
	border: 1px solid rgba(var(--mp-deal-rgb), 0.35);
}

.mp-badge.is-new {
	color: var(--mp-new);
	background: rgba(var(--mp-new-rgb), 0.12);
	border: 1px solid rgba(var(--mp-new-rgb), 0.35);
}

.mp-badge.is-rec {
	color: var(--mp-rec);
	background: rgba(var(--mp-rec-rgb), 0.1);
	border: 1px solid rgba(var(--mp-rec-rgb), 0.3);
}

/* 价格锚：22px/700 等宽 tabular */
.mp-price {
	font-family: var(--mp-font-mono);
	font-variant-numeric: tabular-nums;
	font-size: 22px;
	font-weight: 700;
	line-height: 1.2;
	color: var(--mp-text);
}

.mp-price.is-secondary {
	color: var(--mp-text-dim);
}

.mp-from {
	margin-left: 2px;
	font-size: 10px;
	color: var(--mp-rec);
}

.mp-unit {
	display: block;
	margin-top: 1px;
	font-size: 12px;
	color: var(--mp-text-mute);
	white-space: nowrap;
}

.mp-time-flag {
	flex-shrink: 0;
	margin-bottom: 16px;
	padding: 1px 6px;
	border-radius: 6px;
	border: 1px solid var(--mp-line);
	font-size: 10px;
	color: var(--mp-text-dim);
	cursor: help;
}

/* 简介：固定两行高度（13px × 1.6 × 2，em 单位随字号自适应） */
.mp-desc {
	display: -webkit-box;
	-webkit-box-orient: vertical;
	-webkit-line-clamp: 2;
	overflow: hidden;
	margin-top: 10px;
	min-height: 2.5em;
	font-size: 13px;
	line-height: 1.6;
	color: var(--mp-text-dim);
}

/* 规格 + 能力行 */
.mp-meta-row {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: 4px 6px;
	margin-top: 10px;
	min-height: 24px;
	font-size: 12px;
}

.mp-dot {
	color: var(--mp-text-mute);
}

.mp-spec {
	display: inline-flex;
	align-items: baseline;
	gap: 3px;
}

.mp-spec-label {
	color: var(--mp-text-mute);
}

.mp-spec-value {
	font-family: var(--mp-font-mono);
	font-variant-numeric: tabular-nums;
	color: var(--mp-text-dim);
}

.mp-cap {
	padding: 1px 6px;
	border-radius: 6px;
	border: 1px solid var(--mp-line);
	background: var(--mp-fill-faint);
	color: var(--mp-text-dim);
	white-space: nowrap;
}

.mp-cap.is-more {
	border-style: dashed;
	color: var(--mp-text-mute);
}

@media (prefers-reduced-motion: reduce) {
	.mp-card {
		transition: none;
	}

	.mp-card:hover,
	.mp-card:focus-visible {
		transform: none;
	}
}
</style>

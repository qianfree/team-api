<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/common/Icon.vue'
import VendorLogo from '@/components/common/VendorLogo.vue'
import type { MarketplaceModel } from '@/api/marketplace'
import { getCardView, getCategoryMeta, getCardBadge, relativeCreatedAt } from '../marketplaceMeta'
import { vendorLabel } from '@/constants/vendor'

const props = defineProps<{
	/** 条带语义：recommended=推荐模型 / newest=新上线 */
	variant: 'recommended' | 'newest'
	items: MarketplaceModel[]
}>()

const emit = defineEmits<{
	open: [model: MarketplaceModel]
	viewAll: []
}>()

const title = computed(() => (props.variant === 'recommended' ? '推荐模型' : '新上线'))
const iconName = computed(() => (props.variant === 'recommended' ? 'star' : 'bolt'))

// 条带卡价格：单行紧凑（主锚 + 次锚），按次/按秒模型自适应单位
function priceLine(model: MarketplaceModel): string {
	const view = getCardView(model)
	const [first, second] = view.price.slots
	const shortUnit = (unit: string) => unit.replace('/1M tokens', '/M').replace('/ ', '/')
	if (!second) return `${first.value}${shortUnit(first.unit)}${view.price.fromNote ? ' 起' : ''}`
	return `${first.value}${shortUnit(first.unit)} · ${second.value}${shortUnit(second.unit)}`
}

function metaLine(model: MarketplaceModel): string {
	const category = getCategoryMeta(model.category)
	const vendor = model.vendor ? vendorLabel(model.vendor) : '未分类'
	return `${vendor} · ${category.label}`
}

// 推荐卡简介兜底：无描述时用类别说明占位（与网格卡一致）
function categoryFallback(model: MarketplaceModel): string {
	return getCategoryMeta(model.category).description
}
</script>

<template>
	<section class="mp-strip" :aria-label="title">
		<header class="mp-strip-head">
			<h2 class="mp-strip-title">
				<Icon :name="iconName" size="sm" />
				{{ title }}
			</h2>
			<button v-if="variant === 'newest'" type="button" class="mp-strip-more" @click="emit('viewAll')">
				查看全部
				<Icon name="arrowRight" size="xs" />
			</button>
		</header>

		<div class="mp-strip-scroll">
			<article
				v-for="model in items"
				:key="model.model_id"
				class="mp-strip-card"
				:class="`is-${variant}`"
				role="button"
				tabindex="0"
				:aria-label="`查看 ${model.model_name || model.model_id} 详情`"
				@click="emit('open', model)"
				@keydown.enter.prevent="emit('open', model)"
			>
				<div class="flex items-start justify-between gap-2">
					<VendorLogo :vendor="model.vendor" size="sm" class="shrink-0" />
					<span v-if="getCardBadge(model)" class="mp-strip-badge" :class="`is-${getCardBadge(model)!.type}`">{{ getCardBadge(model)!.text }}</span>
				</div>
				<h3 class="mp-strip-name">{{ model.model_name || model.model_id }}</h3>
				<p class="mp-strip-meta">{{ metaLine(model) }}</p>
				<!-- 推荐位带一行简介（编辑精选的「为什么推荐」语境）；新上位保持紧凑 -->
				<p v-if="variant === 'recommended'" class="mp-strip-desc">{{ model.description?.trim() || categoryFallback(model) }}</p>
				<div class="mp-strip-bottom">
					<span class="mp-strip-price">{{ priceLine(model) }}</span>
					<span v-if="variant === 'newest' && model.created_at" class="mp-strip-time">{{ relativeCreatedAt(model.created_at) }}</span>
				</div>
			</article>
		</div>
	</section>
</template>

<style scoped>
.mp-strip-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	margin-bottom: 10px;
}

.mp-strip-title {
	display: flex;
	align-items: center;
	gap: 6px;
	margin: 0;
	font-size: 13px;
	font-weight: 600;
	color: var(--mp-text);
}

.mp-strip-more {
	display: inline-flex;
	align-items: center;
	gap: 3px;
	border: 0;
	background: none;
	color: var(--mp-text-dim);
	font-size: 12px;
	cursor: pointer;
	transition: color 0.15s ease;
}

.mp-strip-more:hover {
	color: var(--mp-brand);
}

.mp-strip-scroll {
	display: flex;
	gap: 10px;
	overflow-x: auto;
	scroll-snap-type: x mandatory;
	padding-bottom: 4px;
	/* 桌面隐藏滚动指示（可拖拽/滚轮），移动端自然露出 */
	scrollbar-width: none;
}

.mp-strip-scroll::-webkit-scrollbar {
	display: none;
}

.mp-strip-card {
	display: flex;
	flex-direction: column;
	gap: 3px;
	flex: 0 0 200px;
	scroll-snap-align: start;
	padding: 12px;
	background: var(--mp-panel);
	border: 1px solid var(--mp-line);
	border-radius: 12px;
	cursor: pointer;
	transition: transform 0.2s ease, border-color 0.2s ease, background 0.2s ease;
}

/* 推荐位：编辑精选重形态（更宽 + 简介一行 + teal 描边微强调） */
.mp-strip-card.is-recommended {
	flex-basis: 250px;
	border-color: rgba(var(--mp-brand-rgb), 0.22);
}

.mp-strip-card.is-recommended:hover,
.mp-strip-card.is-recommended:focus-visible {
	border-color: rgba(var(--mp-brand-rgb), 0.55);
}

/* 新上位：时效轻形态，保持紧凑 */
.mp-strip-card.is-newest {
	flex-basis: 196px;
}

.mp-strip-card:hover,
.mp-strip-card:focus-visible {
	transform: translateY(-2px);
	border-color: rgba(var(--mp-brand-rgb), 0.45);
	background: var(--mp-panel-hover);
}

.mp-strip-card:focus-visible {
	outline: 2px solid var(--mp-brand);
	outline-offset: 2px;
}

.mp-strip-badge {
	padding: 0 6px;
	border-radius: 999px;
	font-size: 10px;
	font-weight: 600;
	line-height: 1.6;
}

.mp-strip-badge.is-deal {
	color: var(--mp-deal);
	background: rgba(var(--mp-deal-rgb), 0.12);
}

.mp-strip-badge.is-new {
	color: var(--mp-new);
	background: rgba(var(--mp-new-rgb), 0.12);
}

.mp-strip-badge.is-rec {
	color: var(--mp-rec);
	background: rgba(var(--mp-rec-rgb), 0.1);
}

.mp-strip-name {
	margin: 4px 0 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 13px;
	font-weight: 600;
	color: var(--mp-text);
}

.mp-strip-meta {
	margin: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 11px;
	color: var(--mp-text-mute);
}

.mp-strip-desc {
	margin: 2px 0 0;
	display: -webkit-box;
	-webkit-box-orient: vertical;
	-webkit-line-clamp: 1;
	overflow: hidden;
	font-size: 11px;
	line-height: 1.5;
	color: var(--mp-text-dim);
}

.mp-strip-bottom {
	display: flex;
	align-items: baseline;
	justify-content: space-between;
	gap: 6px;
	margin-top: 6px;
}

.mp-strip-price {
	font-family: var(--mp-font-mono);
	font-variant-numeric: tabular-nums;
	font-size: 12px;
	font-weight: 600;
	color: var(--mp-text);
	white-space: nowrap;
}

.mp-strip-time {
	flex-shrink: 0;
	font-size: 10px;
	color: var(--mp-text-mute);
	white-space: nowrap;
}

@media (prefers-reduced-motion: reduce) {
	.mp-strip-card {
		transition: none;
	}

	.mp-strip-card:hover,
	.mp-strip-card:focus-visible {
		transform: none;
	}
}
</style>

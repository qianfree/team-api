<script setup lang="ts">
import { computed, ref } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import { NDrawer, NDrawerContent } from 'naive-ui'
import Icon from '@/components/common/Icon.vue'
import VendorLogo from '@/components/common/VendorLogo.vue'
import type { MarketplaceModel } from '@/api/marketplace'
import { useIsMobile } from '@/composables/useIsMobile'
import { displayCurrency } from '@/composables/useCurrency'
import {
	billingModeLabel,
	formatPrice,
	formatTokens,
	getCapabilityList,
	getCategoryMeta,
	getCardBadge,
	quickStartSnippets,
	relativeCreatedAt,
	timePriceText,
	timeWindow,
} from '../marketplaceMeta'

const props = defineProps<{
	show: boolean
	model: MarketplaceModel | null
	loading: boolean
	isLoggedIn: boolean
	primaryActionRoute: RouteLocationRaw
}>()

const emit = defineEmits<{
	'update:show': [value: boolean]
	copy: [modelId: string]
	copySnippet: [code: string]
}>()

const { isMobile } = useIsMobile()

const meta = computed(() => getCategoryMeta(props.model?.category))
const badge = computed(() => (props.model ? getCardBadge(props.model) : null))
const displayName = computed(() => props.model?.model_name || props.model?.model_id || '')

// 计费瓦片：按计费模式决定展示哪些价格；0 价缓存瓦片属展示噪音，隐藏
const priceTiles = computed(() => {
	const model = props.model
	if (!model) return []
	if (model.billing_mode === 'per_request') {
		return [{ label: '单次调用', value: formatPrice(model.per_request_price), unit: `${displayCurrency.value} / 次`, primary: true, wide: true }]
	}
	const tiles = [
		{ label: '输入', value: formatPrice(model.input_price), unit: `${displayCurrency.value} / 1M tokens`, primary: true, wide: false },
		{ label: '输出', value: formatPrice(model.output_price), unit: `${displayCurrency.value} / 1M tokens`, primary: false, wide: false },
	]
	if (model.cache_read_price) {
		tiles.push({ label: '缓存读取', value: formatPrice(model.cache_read_price), unit: `${displayCurrency.value} / 1M tokens`, primary: false, wide: false })
	}
	if (model.cache_creation_price) {
		tiles.push({ label: '缓存创建', value: formatPrice(model.cache_creation_price), unit: `${displayCurrency.value} / 1M tokens`, primary: false, wide: false })
	}
	return tiles
})

// 按秒计费矩阵：规格 → 单价（"*" 兜底档标注为默认）
const perSecondRows = computed(() => {
	const prices = props.model?.per_second_prices
	if (!prices || !Object.keys(prices).length) return []
	return Object.keys(prices)
		.sort((a, b) => (a === '*' ? 1 : b === '*' ? -1 : a.localeCompare(b)))
		.map(spec => ({ spec, price: formatPrice(prices[spec]), isFallback: spec === '*' }))
})

const capabilities = computed(() => getCapabilityList(props.model ?? ({} as MarketplaceModel), Number.MAX_SAFE_INTEGER))
const tags = computed(() => props.model?.tags || [])

// 快速接入：当前部署地址 + /v1，model_id 自动填充
const snippetTab = ref<'curl' | 'python'>('curl')
const snippets = computed(() =>
	props.model ? quickStartSnippets(props.model.model_id, `${window.location.origin}/v1`) : [],
)
const activeSnippet = computed(() => snippets.value.find(s => s.key === snippetTab.value)?.code ?? '')

const drawerWidth = computed(() => (isMobile.value ? '100%' : 560))
</script>

<template>
	<n-drawer
		:show="show"
		:width="drawerWidth"
		placement="right"
		:auto-focus="false"
		@update:show="(value: boolean) => emit('update:show', value)"
	>
		<n-drawer-content v-if="model" closable :body-content-style="{ padding: 0 }">
			<template #header>
				<div class="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-2">
					<div class="mp-drawer-tile">
						<Icon :name="meta.icon" size="lg" />
					</div>
					<div class="min-w-0 flex-1">
						<div class="flex items-center gap-2">
							<h2 class="mp-drawer-name">{{ displayName }}</h2>
							<span v-if="badge" class="mp-drawer-badge" :class="`is-${badge.type}`">{{ badge.text }}</span>
						</div>
						<button type="button" class="group mt-0.5 flex max-w-full items-center gap-1.5" title="复制模型 ID" @click="emit('copy', model.model_id)">
							<code class="truncate font-mono text-xs text-[var(--mp-text-dim)]">{{ model.model_id }}</code>
							<Icon name="copy" size="xs" class="shrink-0 text-[var(--mp-text-mute)] transition-colors group-hover:text-[var(--mp-brand)]" />
						</button>
					</div>
					<VendorLogo :vendor="model.vendor" size="sm" show-name class="shrink-0" />
				</div>
			</template>

			<div class="mp-drawer-body">
				<p v-if="loading" class="flex items-center gap-2 text-xs text-[var(--mp-text-mute)]">
					<span class="mp-spinner" />正在加载完整模型信息...
				</p>

				<!-- 模型简介 -->
				<section>
					<h3 class="mp-section-title"><Icon name="document" size="sm" />模型简介</h3>
					<p class="text-sm leading-relaxed text-[var(--mp-text-dim)]">{{ model.description || '暂无模型描述。' }}</p>
					<p v-if="model.created_at" class="mt-2 text-xs text-[var(--mp-text-mute)]">
						上架于 {{ model.created_at }}<template v-if="relativeCreatedAt(model.created_at)"> · {{ relativeCreatedAt(model.created_at) }}</template>
					</p>
				</section>

				<!-- 计费信息 -->
				<section>
					<h3 class="mp-section-title">
						<Icon name="creditCard" size="sm" />计费信息
						<span v-if="model.billing_mode === 'tiered'" class="mp-drawer-note">阶梯计费 · 以下为首档起价</span>
					</h3>
					<div class="mp-price-panel">
						<div class="grid grid-cols-2 gap-2.5">
							<div v-for="tile in priceTiles" :key="tile.label" class="mp-price-tile" :class="{ 'col-span-2': tile.wide }">
								<span class="text-xs text-[var(--mp-text-mute)]">{{ tile.label }}</span>
								<div class="mt-0.5 text-xl font-bold tabular-nums" :class="tile.primary ? 'text-[var(--mp-brand)]' : 'text-[var(--mp-text)]'">
									{{ tile.value }}
								</div>
								<span class="text-[11px] text-[var(--mp-text-mute)]">{{ tile.unit }}</span>
							</div>
						</div>

						<!-- 按秒计费矩阵 -->
						<div v-if="perSecondRows.length" class="mt-3 border-t border-[var(--mp-line)] pt-3">
							<p class="mb-1.5 text-xs font-medium text-[var(--mp-text-dim)]">规格单价（按秒计费）</p>
							<div class="space-y-1">
								<div v-for="row in perSecondRows" :key="row.spec" class="flex items-center justify-between gap-3 text-xs">
									<span class="font-mono text-[var(--mp-text-dim)]">
										{{ row.spec === '*' ? '默认' : row.spec }}
									</span>
									<strong class="font-mono font-semibold tabular-nums text-[var(--mp-text)]">{{ row.price }}{{ displayCurrency }} / 秒</strong>
								</div>
							</div>
						</div>

						<!-- 分时段价目 -->
						<div v-if="model.time_prices?.length" class="mt-3 border-t border-[var(--mp-line)] pt-3">
							<p class="mb-1 text-xs font-medium text-[var(--mp-text-dim)]">
								分时段价格
								<small class="ml-1 font-normal text-[var(--mp-text-mute)]">命中时段按下方价格计费，未命中按默认价</small>
							</p>
							<div v-for="(tp, index) in model.time_prices" :key="index" class="flex items-center justify-between gap-3 py-1.5">
								<span class="min-w-0 text-xs text-[var(--mp-text-dim)]">
									{{ tp.name }}
									<small class="block text-[11px] text-[var(--mp-text-mute)]">{{ timeWindow(tp) }}</small>
								</span>
								<strong class="shrink-0 text-right font-mono text-xs font-semibold tabular-nums text-[var(--mp-text)]">{{ timePriceText(model, tp) }}</strong>
							</div>
						</div>

						<p v-if="model.price_change_note" class="mt-3 flex items-start gap-1.5 text-xs text-[var(--mp-rec)]">
							<Icon name="infoCircle" size="xs" class="mt-0.5 shrink-0" />{{ model.price_change_note }}
						</p>
					</div>
				</section>

				<!-- 快速接入 -->
				<section>
					<h3 class="mp-section-title"><Icon name="terminal" size="sm" />快速接入</h3>
					<div class="mp-snippet">
						<div class="flex items-center justify-between">
							<div class="flex gap-1">
								<button
									v-for="s in snippets"
									:key="s.key"
									type="button"
									class="mp-snippet-tab"
									:class="{ 'is-active': snippetTab === s.key }"
									:aria-pressed="snippetTab === s.key"
									@click="snippetTab = s.key"
								>
									{{ s.label }}
								</button>
							</div>
							<button type="button" class="mp-snippet-copy" @click="emit('copySnippet', activeSnippet)">
								<Icon name="copy" size="xs" />复制
							</button>
						</div>
						<pre class="mp-snippet-code"><code>{{ activeSnippet }}</code></pre>
						<p class="mt-1.5 text-[11px] text-[var(--mp-text-mute)]">在控制台创建 API Key 后替换 $API_KEY 即可调用</p>
					</div>
				</section>

				<!-- 技术规格 -->
				<section>
					<h3 class="mp-section-title"><Icon name="cog" size="sm" />技术规格</h3>
					<div class="mp-spec-table">
						<div class="mp-spec-row">
							<span><Icon name="document" size="sm" />最大上下文</span>
							<strong class="font-mono tabular-nums">{{ formatTokens(model.max_context_tokens) }} tokens</strong>
						</div>
						<div class="mp-spec-row">
							<span><Icon name="expand" size="sm" />最大输出</span>
							<strong class="font-mono tabular-nums">{{ formatTokens(model.max_output_tokens) }} tokens</strong>
						</div>
						<div class="mp-spec-row">
							<span><Icon name="grid" size="sm" />模型类别</span>
							<strong>{{ meta.label }}</strong>
						</div>
						<div class="mp-spec-row">
							<span><Icon name="creditCard" size="sm" />计费模式</span>
							<strong>{{ billingModeLabel(model.billing_mode) }}</strong>
						</div>
					</div>
				</section>

				<!-- 能力支持 -->
				<section>
					<h3 class="mp-section-title"><Icon name="checkCircle" size="sm" />能力支持</h3>
					<div class="flex flex-wrap gap-1.5">
						<span v-for="item in capabilities" :key="item" class="mp-cap-chip">
							<Icon name="checkCircle" size="xs" />{{ item }}
						</span>
						<span v-if="capabilities.length === 0" class="text-xs text-[var(--mp-text-mute)]">标准 API 接入</span>
					</div>
				</section>

				<!-- 模型标签 -->
				<section v-if="tags.length">
					<h3 class="mp-section-title"><Icon name="link" size="sm" />模型标签</h3>
					<div class="flex flex-wrap gap-1.5">
						<span v-for="tag in tags" :key="tag" class="mp-tag-chip">{{ tag }}</span>
					</div>
				</section>
			</div>

			<template #footer>
				<div class="flex w-full flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
					<p class="text-xs text-[var(--mp-text-mute)]">实际可用范围以账户套餐与 API 密钥权限为准</p>
					<router-link :to="primaryActionRoute" class="mp-cta" @click="emit('update:show', false)">
						{{ isLoggedIn ? '前往使用' : '免费开始' }}
						<Icon name="arrowRight" size="sm" />
					</router-link>
				</div>
			</template>
		</n-drawer-content>
	</n-drawer>
</template>

<style scoped>
.mp-drawer-tile {
	display: grid;
	place-items: center;
	width: 48px;
	height: 48px;
	flex-shrink: 0;
	border-radius: 12px;
	background: linear-gradient(135deg, rgba(var(--mp-brand-rgb), 0.18), rgba(var(--mp-accent-rgb), 0.12));
	border: 1px solid rgba(var(--mp-brand-rgb), 0.3);
	color: var(--mp-brand);
}

.mp-drawer-name {
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
	font-size: 17px;
	font-weight: 700;
	color: var(--mp-text);
}

.mp-drawer-badge {
	flex-shrink: 0;
	padding: 1px 7px;
	border-radius: 999px;
	font-size: 11px;
	font-weight: 600;
}

.mp-drawer-badge.is-deal {
	color: var(--mp-deal);
	background: rgba(var(--mp-deal-rgb), 0.12);
	border: 1px solid rgba(var(--mp-deal-rgb), 0.35);
}

.mp-drawer-badge.is-new {
	color: var(--mp-new);
	background: rgba(var(--mp-new-rgb), 0.12);
	border: 1px solid rgba(var(--mp-new-rgb), 0.35);
}

.mp-drawer-badge.is-rec {
	color: var(--mp-rec);
	background: rgba(var(--mp-rec-rgb), 0.1);
	border: 1px solid rgba(var(--mp-rec-rgb), 0.3);
}

.mp-drawer-body {
	display: flex;
	flex-direction: column;
	gap: 22px;
	padding: 20px;
}

.mp-drawer-body > section + section {
	margin-top: 2px;
}

.mp-section-title {
	display: flex;
	align-items: center;
	gap: 6px;
	margin-bottom: 10px;
	font-size: 13px;
	font-weight: 600;
	color: var(--mp-text);
}

.mp-section-title :deep(svg) {
	color: var(--mp-brand);
}

.mp-drawer-note {
	margin-left: auto;
	padding: 1px 7px;
	border-radius: 999px;
	background: rgba(var(--mp-rec-rgb), 0.1);
	color: var(--mp-rec);
	font-size: 11px;
	font-weight: 500;
}

.mp-price-panel {
	padding: 14px;
	border: 1px solid rgba(var(--mp-brand-rgb), 0.18);
	border-radius: 14px;
	background: rgba(var(--mp-brand-rgb), 0.04);
}

.mp-price-tile {
	padding: 10px 12px;
	border: 1px solid var(--mp-line);
	border-radius: 10px;
	background: var(--mp-fill-faint);
}

.mp-snippet {
	border: 1px solid var(--mp-line);
	border-radius: 12px;
	overflow: hidden;
}

.mp-snippet-tab {
	padding: 5px 12px;
	border: 0;
	background: none;
	color: var(--mp-text-mute);
	font-size: 12px;
	font-weight: 500;
	cursor: pointer;
	transition: color 0.15s ease, background 0.15s ease;
}

.mp-snippet-tab.is-active {
	color: var(--mp-brand);
	background: rgba(var(--mp-brand-rgb), 0.08);
}

.mp-snippet-copy {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	margin-right: 8px;
	padding: 3px 8px;
	border: 1px solid var(--mp-line);
	border-radius: 6px;
	background: none;
	color: var(--mp-text-dim);
	font-size: 11px;
	cursor: pointer;
	transition: all 0.15s ease;
}

.mp-snippet-copy:hover {
	border-color: rgba(var(--mp-brand-rgb), 0.4);
	color: var(--mp-brand);
}

.mp-snippet-code {
	margin: 0;
	padding: 12px 14px;
	border-top: 1px solid var(--mp-line);
	background: var(--mp-code-bg);
	font-family: var(--mp-font-mono);
	font-size: 12px;
	line-height: 1.7;
	color: var(--mp-text-dim);
	overflow-x: auto;
}

.mp-snippet-code code {
	font-family: inherit;
}

.mp-spec-table {
	border: 1px solid var(--mp-line);
	border-radius: 12px;
	overflow: hidden;
}

.mp-spec-row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	padding: 10px 14px;
	font-size: 13px;
}

.mp-spec-row + .mp-spec-row {
	border-top: 1px solid var(--mp-line);
}

.mp-spec-row span {
	display: inline-flex;
	align-items: center;
	gap: 8px;
	color: var(--mp-text-mute);
}

.mp-spec-row span :deep(svg) {
	color: var(--mp-text-mute);
}

.mp-spec-row strong {
	color: var(--mp-text);
	font-weight: 600;
}

.mp-cap-chip {
	display: inline-flex;
	align-items: center;
	gap: 5px;
	padding: 3px 10px;
	border-radius: 999px;
	background: rgba(var(--mp-brand-rgb), 0.08);
	border: 1px solid rgba(var(--mp-brand-rgb), 0.2);
	color: var(--mp-brand);
	font-size: 12px;
}

.mp-tag-chip {
	padding: 2px 8px;
	border-radius: 6px;
	background: var(--mp-fill-faint);
	border: 1px solid var(--mp-line);
	color: var(--mp-text-dim);
	font-size: 12px;
}

.mp-cta {
	display: inline-flex;
	align-items: center;
	gap: 6px;
	flex-shrink: 0;
	padding: 7px 14px;
	border-radius: 10px;
	background: linear-gradient(135deg, var(--mp-brand), var(--mp-brand-deep));
	color: var(--mp-on-brand);
	font-size: 13px;
	font-weight: 600;
	transition: filter 0.15s ease;
}

.mp-cta:hover {
	filter: brightness(1.1);
}

.mp-spinner {
	width: 14px;
	height: 14px;
	flex-shrink: 0;
	border: 2px solid var(--mp-brand);
	border-top-color: transparent;
	border-radius: 50%;
	animation: mp-spin 0.8s linear infinite;
}

@keyframes mp-spin {
	to {
		transform: rotate(360deg);
	}
}
</style>

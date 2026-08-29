<script setup lang="ts">
import { computed } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import { NDrawer, NDrawerContent } from 'naive-ui'
import Icon from '@/components/common/Icon.vue'
import type { MarketplaceModel } from '@/api/marketplace'
import { useIsMobile } from '@/composables/useIsMobile'
import { displayCurrency } from '@/composables/useCurrency'
import {
	billingModeLabel,
	formatPrice,
	formatTokens,
	getCapabilityList,
	getCategoryMeta,
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
}>()

const { isMobile } = useIsMobile()

const meta = computed(() => getCategoryMeta(props.model?.category))

// 计费瓦片：按计费模式决定展示哪些价格，有值的缓存价附加在后面
// 价格走 formatPrice（本位币），单位文案跟随 displayCurrency（computed 内读取，配置变化自动重渲染）
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
	// 后端未设置缓存价时返回 0 而非 null，0 价瓦片属于展示噪音，直接隐藏
	if (model.cache_read_price) {
		tiles.push({ label: '缓存读取', value: formatPrice(model.cache_read_price), unit: `${displayCurrency.value} / 1M tokens`, primary: false, wide: false })
	}
	if (model.cache_creation_price) {
		tiles.push({ label: '缓存创建', value: formatPrice(model.cache_creation_price), unit: `${displayCurrency.value} / 1M tokens`, primary: false, wide: false })
	}
	return tiles
})

// 抽屉内展示全部能力，不做数量截断
const capabilities = computed(() => getCapabilityList(props.model ?? ({} as MarketplaceModel), Number.MAX_SAFE_INTEGER))

const tags = computed(() => props.model?.tags || [])

const tileStyle = computed(() => ({
	background: `linear-gradient(135deg, ${meta.value.from}, ${meta.value.to})`,
	boxShadow: `0 8px 24px rgba(${meta.value.glow}, 0.35)`,
}))

const categoryBadgeStyle = computed(() => ({
	color: meta.value.to,
	backgroundColor: `rgba(${meta.value.glow}, 0.1)`,
}))

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
				<div class="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-2">
					<div class="grid h-14 w-14 shrink-0 place-items-center rounded-2xl text-white" :style="tileStyle">
						<Icon :name="meta.icon" size="lg" />
					</div>
					<div class="min-w-0 flex-1">
						<div class="flex items-center gap-2">
							<h2 class="truncate text-lg font-bold text-gray-900">{{ model.model_name || model.model_id }}</h2>
							<span v-if="model.discount_label" class="badge badge-warning shrink-0">{{ model.discount_label }}</span>
						</div>
						<button type="button" class="group mt-0.5 flex max-w-full items-center gap-1.5" title="复制模型 ID" @click="emit('copy', model.model_id)">
							<code class="truncate font-mono text-xs text-gray-500">{{ model.model_id }}</code>
							<Icon name="copy" size="xs" class="shrink-0 text-gray-400 transition-colors group-hover:text-primary-600" />
						</button>
					</div>
					<span class="shrink-0 rounded-full px-2.5 py-0.5 text-xs font-medium" :style="categoryBadgeStyle">{{ meta.label }}</span>
				</div>
			</template>

			<div class="animate-fade-in space-y-6 px-5 py-5 sm:px-6">
				<!-- 详情接口加载中：列表数据先行展示，仅提示补充信息加载 -->
				<p v-if="loading" class="flex items-center gap-2 text-xs text-gray-400">
					<span class="spinner border-primary-500" />正在加载完整模型信息...
				</p>

				<!-- 模型简介 -->
				<section>
					<h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-gray-900">
						<Icon name="document" size="sm" class="text-primary-500" />模型简介
					</h3>
					<p class="text-sm leading-relaxed text-gray-600">{{ model.description || '暂无模型描述。' }}</p>
				</section>

				<!-- 计费信息 -->
				<section>
					<h3 class="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900">
						<Icon name="creditCard" size="sm" class="text-primary-500" />计费信息
						<span v-if="model.billing_mode === 'tiered'" class="rounded-full bg-amber-50 px-2 py-0.5 text-[11px] font-medium text-amber-700">
							阶梯计费 · 以下为首档起价
						</span>
					</h3>
					<div class="rounded-2xl border border-primary-100/80 bg-gradient-to-br from-primary-50/70 via-white to-white p-4 shadow-glass-sm">
						<div class="grid grid-cols-2 gap-3">
							<div
								v-for="tile in priceTiles"
								:key="tile.label"
								class="rounded-xl bg-white/85 p-3 shadow-sm ring-1 ring-gray-100"
								:class="tile.wide ? 'col-span-2' : ''"
							>
								<span class="text-xs text-gray-400">{{ tile.label }}</span>
								<div class="mt-0.5 text-xl font-bold tabular-nums" :class="tile.primary ? 'text-primary-700' : 'text-gray-900'">
									{{ tile.value }}
								</div>
								<span class="text-[11px] text-gray-400">{{ tile.unit }}</span>
							</div>
						</div>

						<!-- 分时段价目 -->
						<div v-if="model.time_prices?.length" class="mt-4 border-t border-primary-100/70 pt-3">
							<p class="mb-1 text-xs font-medium text-gray-500">
								分时段价格
								<small class="ml-1 font-normal text-gray-400">命中时段按下方价格计费，未命中按默认价</small>
							</p>
							<div v-for="(tp, index) in model.time_prices" :key="index" class="flex items-center justify-between gap-3 py-1.5">
								<span class="min-w-0 text-xs text-gray-600">
									{{ tp.name }}
									<small class="block text-[11px] text-gray-400">{{ timeWindow(tp) }}</small>
								</span>
								<strong class="shrink-0 text-right text-xs font-semibold tabular-nums text-gray-800">{{ timePriceText(model, tp) }}</strong>
							</div>
						</div>

						<p v-if="model.price_change_note" class="mt-3 flex items-start gap-1.5 text-xs text-amber-700">
							<Icon name="infoCircle" size="xs" class="mt-0.5 shrink-0" />{{ model.price_change_note }}
						</p>
					</div>
				</section>

				<!-- 技术规格 -->
				<section>
					<h3 class="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900">
						<Icon name="cog" size="sm" class="text-primary-500" />技术规格
					</h3>
					<div class="divide-y divide-gray-100 rounded-2xl border border-gray-100 bg-white/70">
						<div class="flex items-center justify-between px-4 py-3">
							<span class="flex items-center gap-2 text-sm text-gray-500"><Icon name="document" size="sm" class="text-gray-400" />最大上下文</span>
							<strong class="text-sm font-semibold tabular-nums text-gray-900">{{ formatTokens(model.max_context_tokens) }} tokens</strong>
						</div>
						<div class="flex items-center justify-between px-4 py-3">
							<span class="flex items-center gap-2 text-sm text-gray-500"><Icon name="expand" size="sm" class="text-gray-400" />最大输出</span>
							<strong class="text-sm font-semibold tabular-nums text-gray-900">{{ formatTokens(model.max_output_tokens) }} tokens</strong>
						</div>
						<div class="flex items-center justify-between px-4 py-3">
							<span class="flex items-center gap-2 text-sm text-gray-500"><Icon name="grid" size="sm" class="text-gray-400" />模型类别</span>
							<strong class="text-sm font-semibold text-gray-900">{{ meta.label }}</strong>
						</div>
						<div class="flex items-center justify-between px-4 py-3">
							<span class="flex items-center gap-2 text-sm text-gray-500"><Icon name="creditCard" size="sm" class="text-gray-400" />计费模式</span>
							<strong class="text-sm font-semibold text-gray-900">{{ billingModeLabel(model.billing_mode) }}</strong>
						</div>
					</div>
				</section>

				<!-- 能力支持 -->
				<section>
					<h3 class="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900">
						<Icon name="checkCircle" size="sm" class="text-primary-500" />能力支持
					</h3>
					<div class="flex flex-wrap gap-2">
						<span
							v-for="item in capabilities"
							:key="item"
							class="inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700"
						>
							<Icon name="checkCircle" size="xs" />{{ item }}
						</span>
						<span v-if="capabilities.length === 0" class="text-xs text-gray-400">标准 API 接入</span>
					</div>
				</section>

				<!-- 模型标签 -->
				<section v-if="tags.length">
					<h3 class="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-900">
						<Icon name="link" size="sm" class="text-primary-500" />模型标签
					</h3>
					<div class="flex flex-wrap gap-2">
						<span v-for="tag in tags" :key="tag" class="rounded-lg bg-gray-100 px-2 py-1 text-xs text-gray-600">{{ tag }}</span>
					</div>
				</section>
			</div>

			<template #footer>
				<div class="flex w-full flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
					<p class="text-xs text-gray-400">实际可用范围以账户套餐与 API 密钥权限为准</p>
					<router-link :to="primaryActionRoute" class="btn btn-primary btn-sm shrink-0" @click="emit('update:show', false)">
						{{ isLoggedIn ? '前往使用' : '免费开始' }}
						<Icon name="arrowRight" size="sm" />
					</router-link>
				</div>
			</template>
		</n-drawer-content>
	</n-drawer>
</template>

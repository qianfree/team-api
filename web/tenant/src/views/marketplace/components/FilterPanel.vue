<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '@/components/common/Icon.vue'
import VendorLogo from '@/components/common/VendorLogo.vue'
import type { MarketplaceFacets } from '@/api/marketplace'
import { capabilityOptionLabel } from '../marketplaceMeta'
import { categoryMetaList } from '../marketplaceMeta'
import { vendorMetaList } from '@/constants/vendor'

/** 未分类厂商的 facet 键（后端空 vendor 归并键） */
const UNCATEGORIZED = 'uncategorized'

const props = defineProps<{
	/** 析取分面计数；null/空维度 = 无计数模式（只渲染选项不带数字） */
	facets: MarketplaceFacets | null
	category: string | null
	vendor: string | null
	capabilities: string[]
	billingMode: string
}>()

const emit = defineEmits<{
	'update:category': [value: string | null]
	'update:vendor': [value: string | null]
	'update:capabilities': [value: string[]]
	'update:billingMode': [value: string]
	reset: []
}>()

const hasFacetCounts = computed(() => {
	const f = props.facets
	return !!f && Object.keys(f.categories ?? {}).length + Object.keys(f.vendors ?? {}).length > 0
})

const vendorExpanded = ref(false)
/** 厂商默认折叠展示数 */
const VENDOR_COLLAPSED_COUNT = 8

// 品类选项：固定字典 + facet 计数；计数为 0 的禁用（点击必然无结果）
const categoryOptions = computed(() =>
	categoryMetaList.map(cat => ({
		...cat,
		count: hasFacetCounts.value ? (props.facets?.categories?.[cat.value ?? ''] ?? 0) : null,
	})),
)

// 「全部」计数 = 各品类计数之和（facet 未返回 total，从类别维度求和）
const totalFromFacets = computed(() =>
	categoryOptions.value.reduce((sum, cat) => sum + (cat.count ?? 0), 0),
)

function selectCategory(value: string | null): void {
	emit('update:category', props.category === value ? null : value)
}

// 厂商选项：facet 键集（含 uncategorized）；已知厂商按字典序，未收录按原值追加；
// 无 facet（超阈值降级）时至少渲染已选厂商，保证选中项可被取消
const vendorOptions = computed(() => {
	const counts = props.facets?.vendors ?? null
	const keys = counts ? Object.keys(counts) : [props.vendor].filter((v): v is string => !!v)
	const known = vendorMetaList
		.filter(v => keys.includes(v.value))
		.map(v => ({ value: v.value, label: v.label, count: counts?.[v.value] ?? null }))
	const knownSet = new Set(vendorMetaList.map(v => v.value))
	const unknown = keys
		.filter(k => !knownSet.has(k) && k !== UNCATEGORIZED)
		.map(k => ({ value: k, label: k, count: counts?.[k] ?? null }))
	const options = [...known, ...unknown]
	if (keys.includes(UNCATEGORIZED)) {
		options.push({ value: UNCATEGORIZED, label: '未分类', count: counts?.[UNCATEGORIZED] ?? null })
	}
	return options
})

const visibleVendorOptions = computed(() => {
	const all = vendorOptions.value
	if (vendorExpanded.value) return all
	const sliced = all.slice(0, VENDOR_COLLAPSED_COUNT)
	// 已选厂商不在折叠区时强制可见，否则选中后按钮消失、无法取消
	if (props.vendor && !sliced.some(o => o.value === props.vendor)) {
		const selected = all.find(o => o.value === props.vendor)
		if (selected) sliced.push(selected)
	}
	return sliced
})

// 厂商单选：点击即切换，再点已选项取消（回到全部）
function selectVendor(value: string): void {
	emit('update:vendor', props.vendor === value ? null : value)
}

// 能力选项：facet 键按计数降序（无 facet 时不渲染该组——无法枚举选项）
const capabilityOptions = computed(() => {
	const counts = props.facets?.capabilities ?? null
	if (!counts) return []
	return Object.entries(counts)
		.sort((a, b) => b[1] - a[1])
		.map(([key, count]) => ({ value: key, label: capabilityOptionLabel(key), count }))
})

function toggleCapability(value: string): void {
	const next = props.capabilities.includes(value)
		? props.capabilities.filter(v => v !== value)
		: [...props.capabilities, value]
	emit('update:capabilities', next)
}

function setBillingMode(value: string): void {
	emit('update:billingMode', value)
}

const billingOptions = [
	{ value: '', label: '全部' },
	{ value: 'token', label: '按量计费' },
	{ value: 'per_request', label: '按次计费' },
	{ value: 'per_second', label: '按秒计费' },
	{ value: 'tiered', label: '阶梯计费' },
]
</script>

<template>
	<div class="mp-filter">
		<!-- 品类：单选（radio 语义） -->
		<section class="mp-filter-group">
			<h3 class="mp-filter-title">品类</h3>
			<div class="mp-option-list">
				<button
					v-for="cat in categoryOptions"
					:key="cat.key"
					type="button"
					class="mp-option"
					:class="{ 'is-active': category === cat.value, 'is-disabled': hasFacetCounts && cat.value !== null && (cat.count ?? 0) === 0 }"
					:aria-pressed="category === cat.value"
					:disabled="hasFacetCounts && cat.value !== null && (cat.count ?? 0) === 0"
					@click="selectCategory(cat.value)"
				>
					<Icon :name="cat.icon" size="sm" class="shrink-0" />
					<span class="mp-option-label">{{ cat.label }}</span>
					<span v-if="hasFacetCounts" class="mp-option-count">{{ cat.value === null ? totalFromFacets : cat.count }}</span>
				</button>
			</div>
		</section>

		<!-- 厂商：单选（点击即切换，再点取消） -->
		<section v-if="vendorOptions.length" class="mp-filter-group">
			<h3 class="mp-filter-title">厂商</h3>
			<div class="mp-option-list">
				<button
					v-for="v in visibleVendorOptions"
					:key="v.value"
					type="button"
					class="mp-option"
					:class="{ 'is-active': vendor === v.value }"
					:aria-pressed="vendor === v.value"
					@click="selectVendor(v.value)"
				>
					<VendorLogo v-if="v.value !== UNCATEGORIZED" :vendor="v.value" size="xs" class="shrink-0" />
					<span v-else class="mp-option-fallback" aria-hidden="true">?</span>
					<span class="mp-option-label">{{ v.label }}</span>
					<span v-if="hasFacetCounts" class="mp-option-count">{{ v.count }}</span>
				</button>
				<button
					v-if="vendorOptions.length > VENDOR_COLLAPSED_COUNT"
					type="button"
					class="mp-expand"
					@click="vendorExpanded = !vendorExpanded"
				>
					{{ vendorExpanded ? '收起' : `展开全部 ${vendorOptions.length}` }}
					<Icon :name="vendorExpanded ? 'chevronUp' : 'chevronDown'" size="xs" />
				</button>
			</div>
		</section>

		<!-- 能力：多选 checkbox（facet 不可用时整组隐藏） -->
		<section v-if="capabilityOptions.length" class="mp-filter-group">
			<h3 class="mp-filter-title">能力</h3>
			<div class="mp-option-list">
				<button
					v-for="cap in capabilityOptions"
					:key="cap.value"
					type="button"
					class="mp-option is-check"
					:aria-pressed="capabilities.includes(cap.value)"
					@click="toggleCapability(cap.value)"
				>
					<span class="mp-option-label">{{ cap.label }}</span>
					<span class="mp-option-count">{{ cap.count }}</span>
					<span class="mp-check" :class="{ 'is-on': capabilities.includes(cap.value) }" aria-hidden="true">
						<Icon v-if="capabilities.includes(cap.value)" name="check" size="xs" />
					</span>
				</button>
			</div>
		</section>

		<!-- 计费方式：单选 -->
		<section class="mp-filter-group">
			<h3 class="mp-filter-title">计费方式</h3>
			<div class="mp-option-list mp-billing-list">
				<button
					v-for="opt in billingOptions"
					:key="opt.value"
					type="button"
					class="mp-option"
					:class="{ 'is-active': billingMode === opt.value }"
					:aria-pressed="billingMode === opt.value"
					@click="setBillingMode(opt.value)"
				>
					<span class="mp-option-label">{{ opt.label }}</span>
					<span v-if="hasFacetCounts && opt.value" class="mp-option-count">{{ facets?.billing_modes?.[opt.value] ?? 0 }}</span>
				</button>
			</div>
		</section>
	</div>
</template>

<style scoped>
.mp-filter {
	display: flex;
	flex-direction: column;
}

.mp-filter-group + .mp-filter-group {
	margin-top: 4px;
	border-top: 1px solid var(--mp-line);
	padding-top: 12px;
}

.mp-filter-title {
	margin: 0 0 8px;
	font-size: 11px;
	font-weight: 600;
	letter-spacing: 0.08em;
	color: var(--mp-text-mute);
}

.mp-option-list {
	display: flex;
	flex-direction: column;
	gap: 2px;
}

.mp-option {
	display: flex;
	align-items: center;
	gap: 8px;
	width: 100%;
	padding: 6px 8px;
	border: 0;
	border-radius: 8px;
	background: none;
	color: var(--mp-text-dim);
	font-size: 13px;
	text-align: left;
	cursor: pointer;
	transition: background 0.15s ease, color 0.15s ease;
}

.mp-option:hover:not(:disabled):not(.is-active) {
	background: var(--mp-fill-faint);
	color: var(--mp-text);
}

.mp-option:focus-visible {
	outline: 2px solid var(--mp-brand);
	outline-offset: -2px;
}

.mp-option.is-active {
	background: rgba(var(--mp-brand-rgb), 0.1);
	color: var(--mp-brand);
}

.mp-option.is-disabled {
	opacity: 0.35;
	cursor: not-allowed;
}

.mp-option-label {
	flex: 1;
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.mp-option-count {
	flex-shrink: 0;
	font-family: var(--mp-font-mono);
	font-variant-numeric: tabular-nums;
	font-size: 11px;
	color: var(--mp-text-mute);
}

.mp-option.is-active .mp-option-count {
	color: var(--mp-brand);
}

.mp-option-fallback {
	display: grid;
	place-items: center;
	width: 16px;
	height: 16px;
	border-radius: 4px;
	border: 1px dashed var(--mp-line);
	font-size: 10px;
	color: var(--mp-text-mute);
}

/* checkbox 指示框（多选项右侧） */
.mp-check {
	display: grid;
	place-items: center;
	width: 16px;
	height: 16px;
	flex-shrink: 0;
	border: 1px solid var(--mp-line);
	border-radius: 4px;
	color: transparent;
	transition: all 0.15s ease;
}

.mp-check.is-on {
	background: var(--mp-brand);
	border-color: var(--mp-brand);
	color: var(--mp-on-brand);
}

.mp-billing-list {
	display: grid;
	grid-template-columns: 1fr 1fr;
}

.mp-expand {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	margin-top: 4px;
	padding: 4px 8px;
	border: 0;
	background: none;
	color: var(--mp-brand);
	font-size: 12px;
	cursor: pointer;
}

.mp-expand:hover {
	text-decoration: underline;
}
</style>

<!-- 按秒计费矩阵编辑器（分辨率规格 × 每秒单价）。
     平台模型定价（ModelPricingModal 主矩阵/官方参考价矩阵）与租户模型覆盖（ModelsTab）共用。
     行数组由父级持有（与 PricingTimeSegmentsEditor/PricingParamRulesEditor 同模式）；
     未挂载场景（二层弹窗）用前置块导出的纯函数填充/构造载荷 -->
<script lang="ts">
// 行编辑态：spec 规格键（如 480p/720p/1080p/* 兜底），price 每秒单价（本位币）
export interface PerSecondRow {
	spec: string
	price: number
}

// 回显：后端 map → 行数组（返回新数组，调用方自行塞进父级行数组）
export function perSecondRowsFromAPI(prices: Record<string, number> | null | undefined): PerSecondRow[] {
	if (!prices) return []
	return Object.entries(prices).map(([spec, price]) => ({ spec, price: Number(price) || 0 }))
}

// 行数组 → 提交 map；requirePositive=true 要求至少一档正价（计费定价必需，官方参考价不强制）。
// map 为 null 表示校验失败（error 为中文提示文案）
export function perSecondPayloadFrom(rows: PerSecondRow[], requirePositive: boolean): { map: Record<string, number> | null; error?: string } {
	const map: Record<string, number> = {}
	let hasPositive = false
	for (const row of rows) {
		const spec = row.spec.trim()
		if (!spec) continue
		if (map[spec] !== undefined) {
			return { map: null, error: `按秒计费规格「${spec}」重复` }
		}
		if (row.price < 0) {
			return { map: null, error: `按秒计费规格「${spec}」单价不能为负` }
		}
		map[spec] = row.price
		if (row.price > 0) hasPositive = true
	}
	if (requirePositive && !hasPositive) {
		return { map: null, error: '按秒计费至少配置一档正价（建议额外配置 * 兜底价）' }
	}
	return { map }
}

// 时段预览基准价：优先 * 兜底价，否则矩阵最低正价
export function perSecondBasePriceOf(rows: PerSecondRow[]): number {
	const prices = rows.map((r) => Number(r.price) || 0).filter((p) => p > 0)
	if (prices.length === 0) return 0
	const wildcard = rows.find((r) => r.spec.trim() === '*')
	return wildcard && wildcard.price > 0 ? wildcard.price : Math.min(...prices)
}
</script>

<script setup lang="ts">
// 行数组由父级持有（reactive），增删行逻辑内聚在本组件
// （与 PricingTimeSegmentsEditor 的 segments 属性同模式，编辑器就地变更）
import { currencySymbol } from '@/composables/useCurrency'

const props = withDefaults(
	defineProps<{
		rows: PerSecondRow[]
		presets?: string[]
	}>(),
	{
		presets: () => ['480p', '720p', '1080p', '*'],
	},
)

function addRow(spec = '') {
	props.rows.push({ spec, price: 0 })
}

function removeRow(index: number) {
	props.rows.splice(index, 1)
}
</script>

<template>
	<div class="per-second-card">
		<div v-if="rows.length" class="per-second-col-head">
			<span>规格</span>
			<span>每秒单价</span>
			<span class="per-second-op-col"></span>
		</div>
		<div v-for="(row, index) in rows" :key="index" class="per-second-row">
			<AInput
				v-model="row.spec"
				placeholder="如 720p / 1080p / *（兜底）"
				:max-length="32"
				class="per-second-spec"
			/>
			<AInputNumber
				v-model="row.price"
				:min="0"
				:precision="4"
				placeholder="0"
				class="per-second-price"
			>
				<template #suffix>{{ currencySymbol }} / 秒</template>
			</AInputNumber>
			<AButton
				size="mini"
				status="danger"
				title="删除该规格"
				@click="removeRow(index)"
			>✕</AButton>
		</div>
		<div v-if="!rows.length" class="per-second-empty">
			暂无规格，点击下方按钮或预设快捷添加
		</div>
	</div>
	<div class="flex items-center gap-2 mt-2">
		<AButton size="small" type="outline" @click="addRow()">+ 添加规格</AButton>
		<AButton
			v-for="preset in presets"
			:key="preset"
			size="mini"
			@click="addRow(preset)"
		>{{ preset }}</AButton>
	</div>
</template>

<style scoped>
.per-second-card {
	padding: 10px 12px;
	background: var(--color-fill-1);
	border: 1px solid var(--ta-border-light);
	border-radius: 8px;
}

.per-second-col-head {
	display: flex;
	align-items: center;
	gap: 8px;
	margin-bottom: 6px;
	font-size: 12px;
	color: var(--ta-text-tertiary);
}

.per-second-row {
	display: flex;
	align-items: center;
	gap: 8px;
}

.per-second-row + .per-second-row {
	margin-top: 6px;
}

.per-second-spec {
	flex: 1 1 40%;
	min-width: 0;
}

.per-second-price {
	flex: 1;
	min-width: 0;
}

.per-second-op-col {
	flex: 0 0 28px;
}

.per-second-empty {
	font-size: 12px;
	color: var(--ta-text-tertiary);
	text-align: center;
	padding: 6px 0;
}
</style>

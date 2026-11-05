<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import SettingsSection from './SettingsSection.vue'
import { useFormValues } from './useSettings'
import { displayCurrency, cnyToUsd, currencySymbol } from '@/composables/useCurrency'
const values = useFormValues()

// 工具调用价格表（sys_options: tool_call_prices，JSON 对象）：
// key = 工具名 或 工具名:模型前缀*（最长前缀优先），value = 每次调用单价（本位币）。
// 未列出的工具使用内置默认价；显式置 0 = 关闭该工具计费
const KEY_PATTERN = /^[a-z0-9_]+(:[A-Za-z0-9._*\-]+)?$/
const rawJson = computed(() => (values['tool_call_prices'] ?? '{}').toString())

interface PriceRow {
	key: string
	price: number | undefined
}
const rows = ref<PriceRow[]>([])
// serialize 后 rawJson 会回显自身改动，用该标志跳过一次回解析
let suppressEcho = false

watch(rawJson, (v) => {
	if (suppressEcho) {
		suppressEcho = false
		return
	}
	parseRows(v)
}, { immediate: true })

function parseRows(v: string) {
	try {
		const obj = JSON.parse(v || '{}')
		rows.value = Object.entries(obj).map(([k, p]) => ({ key: k, price: Number(p) }))
	} catch {
		// 非法 JSON（如手工编辑残留）：保留当前行，保存时后端校验兜底
	}
}

function serialize() {
	const obj: Record<string, number> = {}
	for (const r of rows.value) {
		const k = r.key.trim()
		if (!k) continue
		obj[k] = Number((r.price ?? 0).toFixed(6))
	}
	suppressEcho = true
	values['tool_call_prices'] = JSON.stringify(obj)
}

function addRow() {
	rows.value.push({ key: '', price: 0 })
}
function removeRow(idx: number) {
	rows.value.splice(idx, 1)
	serialize()
}

const keyInvalid = computed(() =>
	rows.value.some((r) => r.key.trim() !== '' && !KEY_PATTERN.test(r.key.trim()))
)
const priceInvalid = computed(() =>
	rows.value.some((r) => r.price !== undefined && (r.price < 0 || r.price > 100))
)

// 内置默认价参考（与后端 billing/tool_pricing.go 的 USD 种子一致）。
// 计费生效价 = USD 种子按当前本位币折算（本位币=CNY 时用支付汇率倒数），
// 展示跟随全站显示货币（useCurrency），与后端折算口径一致
const seedPrices: Array<{ key: string; usd: number; note: string }> = [
	{ key: 'web_search', usd: 0.01, note: 'Claude / OpenAI 联网搜索（$10/1K）' },
	{ key: 'web_search_preview', usd: 0.01, note: 'gpt-4o / 4.1 系 $0.025/次' },
	{ key: 'google_search', usd: 0.014, note: 'Gemini 3 按查询计费；gemini-2.5 及更早按查询免费' },
	{ key: 'google_search_grounded_prompt', usd: 0.035, note: 'Gemini ≤2.5 按 grounded prompt 计费' },
	{ key: 'file_search', usd: 0.0025, note: 'OpenAI 文件检索（$2.5/1K）' },
	{ key: 'web_fetch', usd: 0.01, note: 'Claude 网页抓取（$10/1K）' },
]

// USD 种子 → 显示货币（=本位币）折算：USD 直显；CNY 按支付汇率倒数（usd / cnyToUsd）
function seedPriceLabel(usd: number): string {
	const v = displayCurrency.value === 'CNY' ? usd / cnyToUsd.value : usd
	// 去尾零：USD 种子最小 0.0025（4 位），CNY 折算后同量级，4 位足够
	const trimmed = v.toFixed(4).replace(/\.?0+$/, '')
	return `${currencySymbol.value}${trimmed}/次`
}
</script>

<template>
	<div class="tab-content">
		<SettingsSection
			title="工具调用价格表"
			desc="联网搜索/云端工具按次附加费：渠道从响应中采集调用次数，按固定单价 × 次数独立计费（租户折扣、时段、参数倍率均不作用于工具费）后计入总费用"
		>
			<div class="tool-price-toolbar">
				<AButton size="small" @click="addRow">+ 添加价格项</AButton>
				<span v-if="keyInvalid" class="tool-price-error">存在非法 key：应为 工具名 或 工具名:模型前缀*（工具名仅小写字母/数字/下划线）</span>
				<span v-else-if="priceInvalid" class="tool-price-error">单价必须在 0 ~ 100 之间</span>
			</div>
			<div v-if="rows.length === 0" class="tool-price-empty">
				暂无自定义价格项——所有工具按内置默认价计费（见下方参考表）；如需关闭某工具计费，添加对应 key 并将单价置 0
			</div>
			<div v-else class="tool-price-rows">
				<div class="tool-price-row tool-price-row--head">
					<span>key（工具名 或 工具名:模型前缀*）</span>
					<span>单价 / 次</span>
					<span></span>
				</div>
				<div v-for="(row, idx) in rows" :key="idx" class="tool-price-row">
					<AInput
						v-model="row.key"
						placeholder="如 web_search 或 web_search:claude-*"
						:status="row.key.trim() !== '' && !KEY_PATTERN.test(row.key.trim()) ? 'error' : undefined"
						@change="serialize"
					/>
					<AInputNumber
						v-model="row.price"
						:min="0"
						:max="100"
						:step="0.001"
						:precision="6"
						placeholder="0.01"
						style="width: 100%"
						@change="serialize"
					/>
					<AButton size="mini" type="text" status="danger" @click="removeRow(idx)">删除</AButton>
				</div>
			</div>
		</SettingsSection>

		<SettingsSection
			title="内置默认价参考"
			desc="厂商 USD 列表价按支付汇率折算为本位币后生效（下表已折算）；同名 key 覆盖后以自定义价（本位币直配，不折算）为准，置 0 关闭"
			size="half"
		>
			<div class="seed-table">
				<div class="seed-row seed-row--head">
					<span>key</span><span>生效价（{{ displayCurrency }}）</span><span>说明</span>
				</div>
				<div v-for="s in seedPrices" :key="s.key" class="seed-row">
					<span class="seed-key">{{ s.key }}</span>
					<span>{{ seedPriceLabel(s.usd) }}</span>
					<span class="seed-note">{{ s.note }}</span>
				</div>
			</div>
		</SettingsSection>
	</div>
</template>

<style scoped>
@import './common.css';

.tool-price-toolbar {
	display: flex;
	align-items: center;
	gap: 12px;
	margin-bottom: 12px;
}
.tool-price-error {
	color: rgb(var(--danger-6));
	font-size: 12px;
}
.tool-price-empty {
	color: var(--color-text-3);
	font-size: 13px;
	padding: 8px 0;
}
.tool-price-rows {
	display: flex;
	flex-direction: column;
	gap: 8px;
	max-width: 720px;
}
.tool-price-row {
	display: grid;
	grid-template-columns: 1fr 180px 64px;
	gap: 8px;
	align-items: center;
}
.tool-price-row--head {
	font-size: 12px;
	color: var(--color-text-3);
	padding: 0 0 2px;
}
.seed-table {
	display: flex;
	flex-direction: column;
	gap: 6px;
	font-size: 13px;
}
.seed-row {
	display: grid;
	grid-template-columns: 240px 110px 1fr;
	gap: 8px;
	align-items: baseline;
}
.seed-row--head {
	font-size: 12px;
	color: var(--color-text-3);
}
.seed-key {
	font-family: var(--font-family-mono, monospace);
}
.seed-note {
	color: var(--color-text-3);
	font-size: 12px;
}
</style>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import Icon from './Icon.vue'

// 快捷范围项：label 为按钮文字，range 返回 [startTs, endTs]（ms 时间戳）
export interface RangeShortcut {
	label: string
	range: () => [number, number]
}

const props = withDefaults(
	defineProps<{
		modelValue: [number, number] | null
		// 快捷范围配置（未传时使用默认：今日/昨天/近3天/近一周）
		shortcuts?: Record<string, () => [number, number]>
		// 字段类型：datetimerange 用 datetime-local 原生输入，daterange 用 date
		type?: 'daterange' | 'datetimerange'
	}>(),
	{
		shortcuts: undefined,
		type: 'datetimerange',
	}
)

const emit = defineEmits<{
	'update:modelValue': [value: [number, number] | null]
}>()

// 以本地时区"天"为粒度的快捷范围
function dayStart(offsetDays: number): number {
	const d = new Date()
	d.setDate(d.getDate() + offsetDays)
	d.setHours(0, 0, 0, 0)
	return d.getTime()
}
function dayEnd(offsetDays: number): number {
	const d = new Date()
	d.setDate(d.getDate() + offsetDays)
	d.setHours(23, 59, 59, 999)
	return d.getTime()
}

const defaultShortcuts: Record<string, () => [number, number]> = {
	今日: () => [dayStart(0), dayEnd(0)],
	昨天: () => [dayStart(-1), dayEnd(-1)],
	近3天: () => [dayStart(-2), dayEnd(0)],
	近一周: () => [dayStart(-6), dayEnd(0)],
}

const quickItems = computed<RangeShortcut[]>(() => {
	const src = props.shortcuts || defaultShortcuts
	return Object.entries(src).map(([label, range]) => ({ label, range }))
})

// 时间戳 ↔ 原生输入框字符串（datetime-local: YYYY-MM-DDTHH:mm / date: YYYY-MM-DD）
function tsToInput(ts: number): string {
	const d = new Date(ts)
	const pad = (n: number) => String(n).padStart(2, '0')
	const date = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
	if (props.type === 'daterange') return date
	return `${date}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}
function inputToTs(v: string): number | null {
	if (!v) return null
	// 纯日期串（YYYY-MM-DD）会被 JS 按 UTC 解析，需按本地时区构造，避免时区偏移
	if (props.type === 'daterange') {
		const parts = v.split('-').map(Number)
		if (parts.length !== 3) return null
		const t = new Date(parts[0], parts[1] - 1, parts[2]).getTime()
		return Number.isNaN(t) ? null : t
	}
	const t = new Date(v).getTime()
	return Number.isNaN(t) ? null : t
}

const showCustom = ref(false)
const customStart = ref('')
const customEnd = ref('')

function isActive(item: RangeShortcut): boolean {
	const v = props.modelValue
	if (!v || v[0] == null || v[1] == null) return false
	const [s, e] = item.range()
	return v[0] === s && v[1] === e
}

// 点击快捷项：直接写值并同步到自定义输入框
function pick(item: RangeShortcut) {
	const [s, e] = item.range()
	customStart.value = tsToInput(s)
	customEnd.value = tsToInput(e)
	emit('update:modelValue', [s, e])
	showCustom.value = false
}

// 自定义输入变化：两端均有效才写回，任一端无效则置空
function onCustomInput() {
	const s = inputToTs(customStart.value)
	const e = inputToTs(customEnd.value)
	if (s != null && e != null) {
		emit('update:modelValue', [s, e])
	} else {
		emit('update:modelValue', null)
	}
}

onMounted(() => {
	// 初始值不是快捷项（自定义范围）时，自动展开自定义输入框并回填
	if (props.modelValue && props.modelValue[0] != null && props.modelValue[1] != null) {
		customStart.value = tsToInput(props.modelValue[0])
		customEnd.value = tsToInput(props.modelValue[1])
		if (!quickItems.value.some(isActive)) {
			showCustom.value = true
		}
	}
})
</script>

<template>
	<div class="space-y-2">
		<!-- 快捷范围按钮 -->
		<div class="flex flex-wrap gap-1.5">
			<button
				v-for="item in quickItems"
				:key="item.label"
				type="button"
				class="rounded-lg border px-2.5 py-1 text-xs font-medium transition-colors"
				:class="
					isActive(item)
						? 'border-primary-500 bg-primary-50 text-primary-700'
						: 'border-gray-200 bg-white text-gray-600 hover:border-primary-300 hover:text-primary-600'
				"
				@click="pick(item)"
			>
				{{ item.label }}
			</button>
			<button
				type="button"
				class="rounded-lg border px-2.5 py-1 text-xs font-medium transition-colors"
				:class="
					showCustom
						? 'border-primary-500 bg-primary-50 text-primary-700'
						: 'border-gray-200 bg-white text-gray-600 hover:border-primary-300 hover:text-primary-600'
				"
				@click="showCustom = !showCustom"
			>
				自定义
				<Icon :name="showCustom ? 'chevronUp' : 'chevronDown'" size="xs" />
			</button>
		</div>

		<!-- 自定义范围：原生输入，移动端弹出系统全屏选择器，无溢出 -->
		<div v-if="showCustom" class="space-y-2">
			<label class="block min-w-0">
				<span class="mb-1 block text-xs text-gray-500">开始</span>
				<input
					v-model="customStart"
					:type="type === 'daterange' ? 'date' : 'datetime-local'"
					class="w-full rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-sm text-gray-900 focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/30"
					@change="onCustomInput"
				/>
			</label>
			<label class="block min-w-0">
				<span class="mb-1 block text-xs text-gray-500">结束</span>
				<input
					v-model="customEnd"
					:type="type === 'daterange' ? 'date' : 'datetime-local'"
					class="w-full rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-sm text-gray-900 focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/30"
					@change="onCustomInput"
				/>
			</label>
		</div>
	</div>
</template>

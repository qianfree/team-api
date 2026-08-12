<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import Icon from './Icon.vue'

// 响应式日期范围选择器（纯日期，不含时间）：
// - 桌面端：使用 Naive NDatePicker（daterange）+ 快捷选择按钮
// - 移动端：使用快捷按钮 + 原生 <input type="date">（调用系统底部弹出选择器）
// 对外暴露 v-model:start / v-model:end，值为后端格式字符串 YYYY-MM-DD
const props = withDefaults(defineProps<{
	start?: string
	end?: string
	mobileBreakpoint?: number  // 移动端断点（px），默认 768
}>(), {
	mobileBreakpoint: 768,
})

const emit = defineEmits<{
	'update:start': [value: string]
	'update:end': [value: string]
	'change': []  // 快捷按钮点击后触发，通知父组件自动搜索
}>()

// 检测是否为移动端
const isMobile = ref(false)
const checkMobile = () => {
	isMobile.value = window.innerWidth < props.mobileBreakpoint
}

onMounted(() => {
	checkMobile()
	window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
	window.removeEventListener('resize', checkMobile)
})

// timestamp(ms) ↔ 后端字符串 YYYY-MM-DD
function tsToStr(ts: number): string {
	const d = new Date(ts)
	const pad = (n: number) => String(n).padStart(2, '0')
	return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}
function strToTs(v: string | undefined): number | null {
	if (!v) return null
	const t = new Date(v + 'T00:00:00').getTime()
	return Number.isNaN(t) ? null : t
}

// 时间戳 ↔ 原生输入框字符串（date: YYYY-MM-DD）
function tsToInput(ts: number): string {
	const d = new Date(ts)
	const pad = (n: number) => String(n).padStart(2, '0')
	return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}
function inputToTs(v: string): number | null {
	if (!v) return null
	const t = new Date(v + 'T00:00:00').getTime()
	return Number.isNaN(t) ? null : t
}

// NDatePicker 显示值：仅当 start/end 均有效时返回 [s, e]
const rangeValue = computed<[number, number] | null>(() => {
	const s = strToTs(props.start)
	const e = strToTs(props.end)
	return s != null && e != null ? [s, e] : null
})

function onRangeChange(v: [number, number] | null) {
	if (v) {
		emit('update:start', tsToStr(v[0]))
		emit('update:end', tsToStr(v[1]))
	} else {
		emit('update:start', '')
		emit('update:end', '')
	}
}

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

// NDatePicker 面板内的快捷选项（点击后自动设置 start/end）
const shortcuts = {
	今日: () => [dayStart(0), dayEnd(0)] as [number, number],
	昨天: () => [dayStart(-1), dayEnd(-1)] as [number, number],
	近7天: () => [dayStart(-6), dayEnd(0)] as [number, number],
	近30天: () => [dayStart(-29), dayEnd(0)] as [number, number],
}

// 移动端：快捷按钮 + 自定义输入
const showCustom = ref(false)
const customStart = ref('')
const customEnd = ref('')

const quickItems = computed(() => Object.entries(shortcuts).map(([label, range]) => ({ label, range })))

function isActive(item: { label: string; range: () => [number, number] }): boolean {
	const v = rangeValue.value
	if (!v) return false
	const [s, e] = item.range()
	return v[0] === s && v[1] === e
}

// 点击快捷项：直接写值并同步到自定义输入框
function pick(item: { label: string; range: () => [number, number] }, event?: Event) {
	event?.preventDefault()
	event?.stopPropagation()
	const [s, e] = item.range()
	customStart.value = tsToInput(s)
	customEnd.value = tsToInput(e)
	emit('update:start', tsToStr(s))
	emit('update:end', tsToStr(e))
	showCustom.value = false
	// 触发 change 事件，通知父组件自动搜索
	emit('change')
}

// 切换自定义输入显示
function toggleCustom(event?: Event) {
	event?.preventDefault()
	event?.stopPropagation()
	showCustom.value = !showCustom.value
}

// 自定义输入变化：两端均有效才写回，任一端无效则置空
function onCustomInput() {
	const s = inputToTs(customStart.value)
	const e = inputToTs(customEnd.value)
	if (s != null && e != null) {
		emit('update:start', tsToStr(s))
		emit('update:end', tsToStr(e))
	} else {
		emit('update:start', '')
		emit('update:end', '')
	}
}

onMounted(() => {
	// 初始值回填到移动端输入框
	if (rangeValue.value) {
		customStart.value = tsToInput(rangeValue.value[0])
		customEnd.value = tsToInput(rangeValue.value[1])
		// 如果不是快捷项，自动展开自定义输入
		if (!quickItems.value.some(isActive)) {
			showCustom.value = true
		}
	}
})
</script>

<template>
	<!-- 桌面端：Naive NDatePicker -->
	<n-date-picker
		v-if="!isMobile"
		type="daterange"
		:value="rangeValue"
		:shortcuts="shortcuts"
		:clearable="true"
		:actions="['clear', 'confirm']"
		style="width: 280px"
		@update:value="onRangeChange"
	/>

	<!-- 移动端：快捷按钮 + 原生输入 -->
	<div v-else class="space-y-2 w-full">
		<!-- 快捷范围按钮 -->
		<div class="flex flex-wrap gap-1.5">
			<button
				v-for="item in quickItems"
				:key="item.label"
				type="button"
				class="rounded-lg border px-2.5 py-1 text-xs font-medium transition-all duration-150 active:scale-95 active:bg-primary-100"
				:class="
					isActive(item)
						? 'border-primary-500 bg-primary-50 text-primary-700'
						: 'border-gray-200 bg-white text-gray-600 hover:border-primary-300 hover:bg-primary-50 hover:text-primary-600'
				"
				@click.prevent.stop="pick(item, $event)"
			>
				{{ item.label }}
			</button>
			<button
				type="button"
				class="flex items-center gap-0.5 rounded-lg border px-2.5 py-1 text-xs font-medium transition-all duration-150 active:scale-95 active:bg-primary-100"
				:class="
					showCustom
						? 'border-primary-500 bg-primary-50 text-primary-700'
						: 'border-gray-200 bg-white text-gray-600 hover:border-primary-300 hover:bg-primary-50 hover:text-primary-600'
				"
				@click.prevent.stop="toggleCustom($event)"
			>
				<span>自定义</span>
				<Icon :name="showCustom ? 'chevronUp' : 'chevronDown'" size="xs" />
			</button>
		</div>

		<!-- 自定义范围：原生输入，移动端弹出系统全屏选择器 -->
		<div v-if="showCustom" class="space-y-2">
			<label class="block min-w-0">
				<span class="mb-1 block text-xs text-gray-500">开始日期</span>
				<input
					v-model="customStart"
					type="date"
					class="w-full rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-sm text-gray-900 focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/30"
					@change="onCustomInput"
				/>
			</label>
			<label class="block min-w-0">
				<span class="mb-1 block text-xs text-gray-500">结束日期</span>
				<input
					v-model="customEnd"
					type="date"
					class="w-full rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-sm text-gray-900 focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/30"
					@change="onCustomInput"
				/>
			</label>
		</div>
	</div>
</template>

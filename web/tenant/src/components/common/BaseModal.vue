<template>
	<n-modal
		:show="show"
		preset="card"
		:title="title"
		:style="{ width }"
		:closable="showClose"
		:mask-closable="closeOnClickOutside"
		:close-on-esc="closeOnEscape"
		:bordered="false"
		@update:show="handleShowChange"
		@after-leave="handleAfterLeave"
	>
		<slot />
		<template v-if="$slots.footer" #footer>
			<slot name="footer" />
		</template>
	</n-modal>
</template>

<script setup lang="ts">
import { nextTick, watch } from 'vue'

const props = withDefaults(defineProps<{
	show: boolean
	title?: string
	width?: 'narrow' | 'normal' | 'wide' | 'extra-wide' | 'full'
	showClose?: boolean
	closeOnClickOutside?: boolean
	closeOnEscape?: boolean
}>(), {
	title: '',
	width: 'normal',
	showClose: true,
	closeOnClickOutside: true,
	closeOnEscape: true
})

const emit = defineEmits<{
	close: []
}>()

// 宽度映射为像素（full 用视口百分比）
const maxWidthMap: Record<string, string> = {
	narrow: '400px',
	normal: '480px',
	wide: '640px',
	'extra-wide': '800px',
	full: '95vw'
}

const width = maxWidthMap[props.width] || '480px'

// NModal 内部触发关闭（点 X / 点遮罩 / Esc）时，统一回调父组件的 close
function handleShowChange(show: boolean) {
	if (!show) {
		emit('close')
	}
}

// 焦点归还：NModal 只负责把焦点圈进弹窗，关闭后焦点会落到 body 上，
// 读屏/键盘用户会"迷路"——记录打开前的焦点元素，关闭后送回去
let lastFocused: HTMLElement | null = null

watch(() => props.show, (show) => {
	if (show) {
		// 必须在 NModal 移动焦点之前捕获触发元素
		lastFocused = document.activeElement as HTMLElement | null
	}
})

function handleAfterLeave() {
	const target = lastFocused
	lastFocused = null
	nextTick(() => {
		if (target && document.contains(target)) {
			target.focus()
		}
	})
}
</script>
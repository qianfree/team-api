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
	>
		<slot />
		<template v-if="$slots.footer" #footer>
			<slot name="footer" />
		</template>
	</n-modal>
</template>

<script setup lang="ts">
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
</script>
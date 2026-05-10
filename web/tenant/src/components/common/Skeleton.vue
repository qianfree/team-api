<template>
	<div
		:class="[
			'skeleton',
			variant === 'circle' ? 'rounded-full' : 'rounded-lg',
		]"
		:style="{ width, height }"
	></div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
	variant?: 'rect' | 'circle' | 'text'
	width?: string
	height?: string
}>(), {
	variant: 'rect',
	width: '100%',
	height: '20px'
})
</script>

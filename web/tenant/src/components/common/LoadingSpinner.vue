<template>
	<div class="flex items-center justify-center" :class="containerClass">
		<div
			:class="[
				'rounded-full border-2 animate-spin',
				sizeMap[size],
				colorMap[color]
			]"
			style="border-top-color: transparent"
		></div>
		<span v-if="text" class="ml-2 text-sm text-gray-500">{{ text }}</span>
	</div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
	size?: 'sm' | 'md' | 'lg' | 'xl'
	color?: 'primary' | 'gray' | 'white'
	text?: string
	fullScreen?: boolean
}>(), {
	size: 'md',
	color: 'primary',
	text: '',
	fullScreen: false
})

const sizeMap: Record<string, string> = {
	sm: 'h-4 w-4',
	md: 'h-5 w-5',
	lg: 'h-8 w-8',
	xl: 'h-12 w-12'
}

const colorMap: Record<string, string> = {
	primary: 'border-primary-500',
	gray: 'border-gray-400',
	white: 'border-white'
}

const containerClass = computed(() =>
	props.fullScreen ? 'fixed inset-0 z-50 bg-white/80 backdrop-blur-sm' : ''
)
</script>

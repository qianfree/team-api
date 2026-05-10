<template>
	<span :class="['badge', variantClass]">
		<Icon v-if="showDot" name="checkCircle" size="xs" class="mr-0.5" />
		{{ label }}
	</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from './Icon.vue'

const props = withDefaults(defineProps<{
	status: 'active' | 'success' | 'warning' | 'danger' | 'info' | 'default'
	label?: string
	showDot?: boolean
}>(), {
	label: '',
	showDot: false
})

const variantMap: Record<string, string> = {
	active: 'badge-primary',
	success: 'badge-success',
	warning: 'badge-warning',
	danger: 'badge-danger',
	info: 'badge-primary',
	default: 'badge-gray'
}

const variantClass = computed(() => variantMap[props.status] || variantMap.default)
</script>

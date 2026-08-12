<template>
	<div :class="containerClass">
		<n-select
			:value="modelValue"
			:options="options"
			:placeholder="placeholder"
			:disabled="disabled"
			:status="error ? 'error' : undefined"
			:size="naiveSize"
			@update:value="handleChange"
		/>
	</div>
</template>

<script setup lang="ts">
export interface SelectOption {
	value: string | number
	label: string
}

const props = withDefaults(defineProps<{
	modelValue: string | number
	options: SelectOption[]
	placeholder?: string
	disabled?: boolean
	error?: boolean
	size?: 'sm' | 'md'
	containerClass?: string
	panelClass?: string
}>(), {
	placeholder: '请选择',
	disabled: false,
	error: false,
	size: 'md',
	containerClass: '',
	panelClass: ''
})

const emit = defineEmits<{
	'update:modelValue': [value: string | number]
	change: [value: string | number]
}>()

// BaseSelect 的 size 语义映射到 Naive 的 size
const naiveSize = props.size === 'sm' ? 'small' : 'medium'

function handleChange(value: string | number | null) {
	const v = (value ?? '') as string | number
	emit('update:modelValue', v)
	emit('change', v)
}
</script>
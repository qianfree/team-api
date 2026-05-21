<template>
	<div ref="containerRef" class="relative" :class="containerClass">
		<button
			type="button"
			ref="triggerRef"
			:disabled="disabled"
			class="select-trigger"
			:class="[
				size === 'sm' ? 'select-trigger-sm' : 'select-trigger-md',
				{ 'select-trigger-error': error, 'select-trigger-disabled': disabled, 'select-trigger-open': isOpen }
			]"
			@click="toggle"
			@keydown="onKeydown"
		>
			<span v-if="selectedLabel" class="truncate">{{ selectedLabel }}</span>
			<span v-else class="text-gray-400">{{ placeholder }}</span>
			<span class="select-arrow" :class="{ 'rotate-180': isOpen }">
				<Icon name="chevronDown" size="sm" />
			</span>
		</button>

		<Transition name="select-fade">
			<div v-if="isOpen" class="select-panel" :class="panelClass">
				<div v-if="options.length === 0" class="select-empty">暂无选项</div>
				<template v-else>
					<div
						v-for="(opt, idx) in options"
						:key="opt.value"
						class="select-option"
						:class="{
							'select-option-active': idx === activeIndex,
							'select-option-selected': opt.value === modelValue
						}"
						@mouseenter="activeIndex = idx"
						@click="selectOption(opt)"
					>
						<span class="truncate">{{ opt.label }}</span>
						<Icon v-if="opt.value === modelValue" name="check" size="sm" class="text-primary-500 flex-shrink-0" />
					</div>
				</template>
			</div>
		</Transition>
	</div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import Icon from './Icon.vue'

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

const containerRef = ref<HTMLElement>()
const triggerRef = ref<HTMLButtonElement>()
const isOpen = ref(false)
const activeIndex = ref(-1)

const selectedLabel = computed(() => {
	const opt = props.options.find(o => o.value === props.modelValue)
	return opt?.label ?? ''
})

function toggle() {
	if (props.disabled) return
	isOpen.value = !isOpen.value
	if (isOpen.value) {
		activeIndex.value = props.options.findIndex(o => o.value === props.modelValue)
	}
}

function close() {
	isOpen.value = false
	activeIndex.value = -1
}

function selectOption(opt: SelectOption) {
	emit('update:modelValue', opt.value)
	emit('change', opt.value)
	close()
	triggerRef.value?.focus()
}

function onKeydown(e: KeyboardEvent) {
	if (!isOpen.value) {
		if (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown') {
			e.preventDefault()
			isOpen.value = true
			activeIndex.value = Math.max(0, props.options.findIndex(o => o.value === props.modelValue))
		}
		return
	}

	switch (e.key) {
		case 'ArrowDown':
			e.preventDefault()
			activeIndex.value = (activeIndex.value + 1) % props.options.length
			break
		case 'ArrowUp':
			e.preventDefault()
			activeIndex.value = activeIndex.value <= 0 ? props.options.length - 1 : activeIndex.value - 1
			break
		case 'Enter':
		case ' ':
			e.preventDefault()
			if (activeIndex.value >= 0 && activeIndex.value < props.options.length) {
				selectOption(props.options[activeIndex.value])
			}
			break
		case 'Escape':
			e.preventDefault()
			close()
			triggerRef.value?.focus()
			break
	}
}

function onClickOutside(e: MouseEvent) {
	if (containerRef.value && !containerRef.value.contains(e.target as Node)) {
		close()
	}
}

watch(isOpen, (val) => {
	if (val) {
		document.addEventListener('mousedown', onClickOutside)
	} else {
		document.removeEventListener('mousedown', onClickOutside)
	}
})

onBeforeUnmount(() => {
	document.removeEventListener('mousedown', onClickOutside)
})
</script>

<style scoped>
@reference "../../styles/main.css";
.select-trigger {
	@apply w-full flex items-center justify-between gap-2 rounded-xl px-4 py-2.5 text-sm;
	@apply bg-white border border-gray-200 text-left;
	@apply text-gray-900;
	@apply transition-all duration-200;
	@apply focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/30;
	@apply cursor-pointer;
}
.select-trigger-sm {
	@apply rounded-lg px-3 py-1.5 text-xs;
}
.select-trigger-md {
	@apply rounded-xl px-4 py-2.5 text-sm;
}
.select-trigger-error {
	@apply border-red-400 focus:border-red-500 focus:ring-red-500/30;
}
.select-trigger-disabled {
	@apply cursor-not-allowed bg-gray-50 text-gray-400;
}
.select-trigger-disabled .select-arrow {
	@apply text-gray-300;
}
.select-trigger-open {
	@apply border-primary-500 ring-2 ring-primary-500/30;
}
.select-arrow {
	@apply flex-shrink-0 text-gray-400 transition-transform duration-200;
}

/* Panel */
.select-panel {
	@apply absolute z-50 mt-1 w-full bg-white rounded-xl border border-gray-200 shadow-lg;
	@apply py-1 max-h-60 overflow-y-auto;
	@apply origin-top animate-scale-in;
}
.select-empty {
	@apply px-4 py-3 text-sm text-gray-400 text-center;
}

/* Option */
.select-option {
	@apply flex items-center justify-between px-4 py-2 text-sm text-gray-700 cursor-pointer;
	@apply transition-colors duration-100;
}
.select-option-active {
	@apply bg-primary-50 text-primary-700;
}
.select-option-selected {
	@apply font-medium;
}

/* Transition */
.select-fade-enter-active {
	transition: opacity 150ms ease-out, transform 150ms ease-out;
}
.select-fade-leave-active {
	transition: opacity 100ms ease-in, transform 100ms ease-in;
}
.select-fade-enter-from,
.select-fade-leave-to {
	opacity: 0;
	transform: scaleY(0.95) translateY(-4px);
	transform-origin: top;
}
</style>

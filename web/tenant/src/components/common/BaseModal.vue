<template>
	<Teleport to="body">
		<Transition name="modal">
			<div v-if="show" class="modal-overlay" @click.self="handleOverlayClick">
				<div class="modal-content" :style="{ maxWidth: maxWidthMap[width] }">
					<!-- Header -->
					<div class="modal-header">
						<h3 class="modal-title">{{ title }}</h3>
						<button
							v-if="showClose"
							class="btn-ghost btn-icon text-gray-400 hover:text-gray-600"
							@click="handleClose"
						>
							<Icon name="x" size="md" />
						</button>
					</div>

					<!-- Body -->
					<div class="modal-body">
						<slot />
					</div>

					<!-- Footer -->
					<div v-if="$slots.footer" class="modal-footer">
						<slot name="footer" />
					</div>
				</div>
			</div>
		</Transition>
	</Teleport>
</template>

<script setup lang="ts">
import { watch } from 'vue'
import Icon from './Icon.vue'

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

const maxWidthMap: Record<string, string> = {
	narrow: '400px',
	normal: '480px',
	wide: '640px',
	'extra-wide': '800px',
	full: '95vw'
}

function handleClose() {
	emit('close')
}

function handleOverlayClick() {
	if (props.closeOnClickOutside) {
		handleClose()
	}
}

function handleEscape(e: KeyboardEvent) {
	if (props.closeOnEscape && e.key === 'Escape' && props.show) {
		handleClose()
	}
}

watch(() => props.show, (val) => {
	if (val) {
		document.body.classList.add('modal-open')
		document.addEventListener('keydown', handleEscape)
	} else {
		document.body.classList.remove('modal-open')
		document.removeEventListener('keydown', handleEscape)
	}
}, { immediate: true })
</script>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'

interface Props {
	modelValue?: string
	siteKey?: string
	theme?: 'light' | 'dark' | 'auto'
	size?: 'normal' | 'compact'
	tabindex?: number
}

const props = withDefaults(defineProps<Props>(), {
	modelValue: '',
	siteKey: '',
	theme: 'auto',
	size: 'normal',
	tabindex: 0,
})

const emit = defineEmits<{
	(e: 'update:modelValue', value: string): void
	(e: 'verified', token: string): void
	(e: 'error', error: any): void
}>()

const containerId = `turnstile-container-${Math.random().toString(36).substr(2, 9)}`
const isLoaded = ref(false)
const isVerifying = ref(false)
const turnstileScriptId = 'turnstile-script'
let widgetId: string | null = null

// Load Cloudflare Turnstile script
function loadTurnstileScript(): Promise<void> {
	return new Promise((resolve, reject) => {
		if (window.turnstile) {
			isLoaded.value = true
			resolve()
			return
		}

		const script = document.createElement('script')
		script.id = turnstileScriptId
		script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js'
		script.async = true
		script.defer = true
		script.onload = () => {
			isLoaded.value = true
			resolve()
		}
		script.onerror = () => {
			reject(new Error('Failed to load Turnstile script'))
		}
		document.head.appendChild(script)
	})
}

// Render Turnstile widget
function renderWidget() {
	if (!window.turnstile || !props.siteKey) return

	const container = document.getElementById(containerId)
	if (!container) return

	// Clear existing widget
	if (widgetId !== null) {
		window.turnstile.remove(widgetId)
	}

	// Render new widget
	widgetId = window.turnstile.render(container, {
		sitekey: props.siteKey,
		theme: props.theme,
		size: props.size,
		tabindex: props.tabindex,
		callback: (token: string) => {
			isVerifying.value = false
			emit('update:modelValue', token)
			emit('verified', token)
		},
		'error-callback': (error: any) => {
			isVerifying.value = false
			emit('error', error)
		},
		'expired-callback': () => {
			widgetId = null
			emit('update:modelValue', '')
		},
	})
}

// Reset widget
function reset() {
	if (widgetId !== null && window.turnstile) {
		window.turnstile.reset(widgetId)
		emit('update:modelValue', '')
	}
}

// Expose reset method
defineExpose({
	reset,
})

onMounted(async () => {
	try {
		await loadTurnstileScript()
		renderWidget()
	} catch (error) {
		console.error('Failed to initialize Turnstile:', error)
		emit('error', error)
	}
})

onUnmounted(() => {
	if (widgetId !== null && window.turnstile) {
		window.turnstile.remove(widgetId)
	}
})

// Watch for siteKey changes
watch(() => props.siteKey, () => {
	if (isLoaded.value) {
		renderWidget()
	}
})

// Watch for modelValue changes from parent
watch(() => props.modelValue, (newValue) => {
	if (!newValue && widgetId !== null && window.turnstile) {
		window.turnstile.reset(widgetId)
	}
})
</script>

<template>
	<div
		:id="containerId"
		class="turnstile-widget"
		:class="{ 'turnstile-loading': isVerifying }"
	/>
</template>

<style scoped>
.turnstile-widget {
	min-height: 65px;
	display: flex;
	align-items: center;
	justify-content: center;
}

.turnstile-loading {
	opacity: 0.6;
}
</style>

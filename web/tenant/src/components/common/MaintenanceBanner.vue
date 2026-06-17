<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { usePublicSettings } from '@/composables/usePublicSettings'
import Icon from '@/components/common/Icon.vue'

const { settings, fetchSettings } = usePublicSettings()
const dismissed = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

onMounted(async () => {
	await fetchSettings()
	timer = setInterval(() => fetchSettings(), 30 * 60 * 1000)
})

onBeforeUnmount(() => {
	if (timer) clearInterval(timer)
})

function dismiss() {
	dismissed.value = true
}
</script>

<template>
	<!-- Maintenance Mode Banner -->
	<div
		v-if="settings.maintenance_mode && !dismissed"
		class="relative bg-amber-500/90 backdrop-blur-sm border-b border-amber-400"
	>
		<div class="flex items-center justify-center gap-2 px-4 py-2.5 text-sm text-white">
			<Icon name="exclamationTriangle" size="sm" class="flex-shrink-0" />
			<span>
				系统维护中
				<span v-if="settings.maintenance_message" class="ml-1">— {{ settings.maintenance_message }}</span>
				<span v-if="settings.maintenance_duration" class="ml-1 text-amber-100">（预计 {{ settings.maintenance_duration }}）</span>
			</span>
		</div>
		<button
			@click="dismiss"
			class="absolute right-3 top-1/2 -translate-y-1/2 flex h-6 w-6 items-center justify-center rounded-lg text-amber-100 hover:text-white hover:bg-amber-600/50 transition-colors"
		>
			<Icon name="x" size="sm" />
		</button>
	</div>
</template>

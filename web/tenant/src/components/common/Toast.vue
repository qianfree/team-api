<script setup lang="ts">
import { toasts, removeToast } from '@/utils/toast'
import Icon from './Icon.vue'

const iconMap: Record<string, { name: string; class: string }> = {
	success: { name: 'checkCircle', class: 'text-emerald-500' },
	error: { name: 'xCircle', class: 'text-red-500' },
	warning: { name: 'exclamationTriangle', class: 'text-amber-500' },
	info: { name: 'infoCircle', class: 'text-primary-500' },
}
</script>

<template>
	<Teleport to="body">
		<div class="fixed top-4 right-4 z-[100] flex flex-col gap-2 pointer-events-none" style="max-width: 400px;">
			<TransitionGroup name="toast">
				<div
					v-for="t in toasts"
					:key="t.id"
					class="pointer-events-auto flex items-start gap-3 rounded-xl border border-gray-200 bg-white px-4 py-3 shadow-lg"
				>
					<Icon :name="iconMap[t.type]?.name || 'infoCircle'" size="md" :class="iconMap[t.type]?.class || 'text-gray-400'" class="mt-0.5 flex-shrink-0" />
					<p class="flex-1 text-sm text-gray-700">{{ t.message }}</p>
					<button class="flex-shrink-0 text-gray-400 hover:text-gray-600 transition-colors" @click="removeToast(t.id)">
						<Icon name="x" size="sm" />
					</button>
				</div>
			</TransitionGroup>
		</div>
	</Teleport>
</template>

<style scoped>
.toast-enter-active {
	transition: all 0.25s ease-out;
}
.toast-leave-active {
	transition: all 0.2s ease-in;
}
.toast-enter-from {
	opacity: 0;
	transform: translateX(20px);
}
.toast-leave-to {
	opacity: 0;
	transform: translateX(20px) scale(0.95);
}
.toast-move {
	transition: transform 0.2s ease;
}
</style>

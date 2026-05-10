<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import Icon from '@/components/common/Icon.vue'

const props = defineProps<{
	announcements: {
		id: number
		title: string
		type: string
		content: string
		is_pinned: number
		created_at: string
	}[]
}>()

const currentIndex = ref(0)
const showDetail = ref(false)
const dismissedIds = ref<Set<number>>(new Set())

let autoTimer: ReturnType<typeof setInterval> | null = null

const visibleItems = computed(() => {
	return props.announcements.filter(a => !dismissedIds.value.has(a.id))
})

const currentItem = computed(() => {
	return visibleItems.value[currentIndex.value] || null
})

const hasMultiple = computed(() => visibleItems.value.length > 1)

const typeConfig: Record<string, { bg: string; border: string; icon: string }> = {
	info: { bg: 'bg-primary-500/90', border: 'border-primary-400', icon: 'infoCircle' },
	warning: { bg: 'bg-amber-500/90', border: 'border-amber-400', icon: 'exclamationTriangle' },
	important: { bg: 'bg-red-500/90', border: 'border-red-400', icon: 'xCircle' },
}

function getTypeConfig(type: string) {
	return typeConfig[type] || typeConfig.info
}

function dismissCurrent() {
	if (!currentItem.value) return
	if (currentItem.value.type === 'important') return
	dismissedIds.value.add(currentItem.value.id)
	if (currentIndex.value >= visibleItems.value.length) {
		currentIndex.value = 0
	}
}

function next() {
	if (visibleItems.value.length <= 1) return
	currentIndex.value = (currentIndex.value + 1) % visibleItems.value.length
}

function prev() {
	if (visibleItems.value.length <= 1) return
	currentIndex.value = (currentIndex.value - 1 + visibleItems.value.length) % visibleItems.value.length
}

function openDetail() {
	if (!currentItem.value) return
	showDetail.value = true
}

function startAutoRotate() {
	stopAutoRotate()
	if (visibleItems.value.length > 1) {
		autoTimer = setInterval(() => {
			next()
		}, 6000)
	}
}

function stopAutoRotate() {
	if (autoTimer) {
		clearInterval(autoTimer)
		autoTimer = null
	}
}

watch(() => props.announcements, () => {
	currentIndex.value = 0
	startAutoRotate()
})

onMounted(() => {
	startAutoRotate()
})

onBeforeUnmount(() => {
	stopAutoRotate()
})
</script>

<template>
	<Transition name="slide-down">
		<div v-if="currentItem" :class="['border-b', getTypeConfig(currentItem.type).bg, getTypeConfig(currentItem.type).border]">
			<div class="flex items-center justify-center gap-2 px-4 py-2.5 text-sm text-white">
				<Icon :name="getTypeConfig(currentItem.type).icon" size="sm" class="flex-shrink-0" />

				<!-- Navigation (prev) -->
				<button v-if="hasMultiple" @click="prev" class="flex-shrink-0 rounded-lg p-0.5 hover:bg-white/20 transition-colors">
					<Icon name="chevronLeft" size="xs" />
				</button>

				<!-- Content -->
				<button class="flex-1 text-center truncate hover:underline cursor-pointer" @click="openDetail">
					<span :class="{ 'font-semibold': currentItem.is_pinned }">{{ currentItem.title }}</span>
					<span v-if="hasMultiple" class="ml-2 text-white/70">{{ currentIndex + 1 }}/{{ visibleItems.length }}</span>
				</button>

				<!-- Navigation (next) -->
				<button v-if="hasMultiple" @click="next" class="flex-shrink-0 rounded-lg p-0.5 hover:bg-white/20 transition-colors">
					<Icon name="chevronRight" size="xs" />
				</button>

				<!-- Dismiss -->
				<button
					v-if="currentItem.type !== 'important'"
					@click="dismissCurrent"
					class="flex-shrink-0 rounded-lg p-0.5 hover:bg-white/20 transition-colors"
				>
					<Icon name="x" size="sm" />
				</button>
			</div>
		</div>
	</Transition>

	<!-- Detail Modal -->
	<Teleport to="body">
		<Transition name="fade">
			<div v-if="showDetail && currentItem" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm" @click.self="showDetail = false">
				<div class="w-full max-w-lg rounded-2xl bg-white shadow-2xl border border-gray-200 animate-scale-in" @click.stop>
					<!-- Header -->
					<div class="flex items-start gap-3 px-6 py-4 border-b border-gray-100">
						<div :class="['flex-shrink-0 mt-0.5 h-8 w-8 rounded-lg flex items-center justify-center', getTypeConfig(currentItem.type).bg]">
							<Icon :name="getTypeConfig(currentItem.type).icon" size="sm" class="text-white" />
						</div>
						<div class="flex-1 min-w-0">
							<h3 class="text-lg font-semibold text-gray-900">{{ currentItem.title }}</h3>
							<p class="text-xs text-gray-500 mt-0.5">{{ currentItem.created_at }}</p>
						</div>
						<button @click="showDetail = false" class="flex-shrink-0 rounded-lg p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors">
							<Icon name="x" size="md" />
						</button>
					</div>
					<!-- Body -->
					<div class="px-6 py-5 max-h-[60vh] overflow-y-auto">
						<div class="text-sm text-gray-700 whitespace-pre-wrap leading-relaxed">{{ currentItem.content }}</div>
					</div>
					<!-- Footer -->
					<div class="px-6 py-3 border-t border-gray-100 flex justify-end">
						<button @click="showDetail = false" class="btn btn-secondary btn-sm">关闭</button>
					</div>
				</div>
			</div>
		</Transition>
	</Teleport>
</template>

<style scoped>
.slide-down-enter-active,
.slide-down-leave-active {
	transition: all 0.3s ease-out;
}
.slide-down-enter-from,
.slide-down-leave-to {
	opacity: 0;
	transform: translateY(-100%);
}

.fade-enter-active { transition: opacity 0.2s ease-out; }
.fade-leave-active { transition: opacity 0.15s ease-in; }
.fade-enter-from, .fade-leave-to { opacity: 0; }

.animate-scale-in {
	animation: scaleIn 0.25s ease-out;
}
@keyframes scaleIn {
	from { transform: scale(0.95); opacity: 0; }
	to { transform: scale(1); opacity: 1; }
}
</style>

<template>
	<div class="relative flex min-h-screen items-center justify-center overflow-hidden p-4">
		<!-- Announcement Banner (teleported to body, fixed top) -->
		<Teleport to="body">
			<div v-if="announcements.length" class="fixed top-0 left-0 right-0 z-50">
				<AnnouncementBanner :announcements="announcements" />
			</div>
		</Teleport>

		<!-- Background -->
		<div class="absolute inset-0 bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100"></div>

		<!-- Decorative Elements -->
		<div class="pointer-events-none absolute inset-0 overflow-hidden">
			<!-- Gradient Orbs -->
			<div class="absolute -right-40 -top-40 h-80 w-80 rounded-full bg-primary-400/20 blur-3xl"></div>
			<div class="absolute -bottom-40 -left-40 h-80 w-80 rounded-full bg-primary-500/15 blur-3xl"></div>
			<div class="absolute left-1/2 top-1/2 h-96 w-96 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary-300/10 blur-3xl"></div>

			<!-- Grid Pattern -->
			<div
				class="absolute inset-0"
				style="background-image: linear-gradient(rgba(20,184,166,0.03) 1px, transparent 1px), linear-gradient(90deg, rgba(20,184,166,0.03) 1px, transparent 1px); background-size: 64px 64px"
			></div>
		</div>

		<!-- Content Container -->
		<div class="relative z-10 w-full max-w-md">
			<!-- Logo/Brand -->
			<div class="mb-8 text-center">
				<div class="mb-4 inline-flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl shadow-glow">
					<Icon name="building" size="xl" class="text-primary-600" />
				</div>
				<h1 class="text-gradient mb-2 text-3xl font-bold">Team API</h1>
				<p class="text-sm text-gray-500">多租户 AI API 网关平台</p>
			</div>

			<!-- Card Container -->
			<div class="card-glass rounded-2xl p-8 shadow-glass">
				<slot />
			</div>

			<!-- Footer Links -->
			<div class="mt-6 text-center text-sm">
				<slot name="footer" />
			</div>
		</div>
	</div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import Icon from '../common/Icon.vue'
import AnnouncementBanner from '../common/AnnouncementBanner.vue'
import request from '@/utils/request'

const announcements = ref<any[]>([])
let timer: ReturnType<typeof setInterval> | null = null

async function fetchAnnouncements() {
	try {
		const res = await request.get('/settings/announcements', { params: { position: 'login' }, _suppressErrorMsg: true } as any)
		announcements.value = res.data?.data?.list || []
	} catch {
		// silently ignore
	}
}

onMounted(() => {
	fetchAnnouncements()
	timer = setInterval(fetchAnnouncements, 60_000)
})

onBeforeUnmount(() => {
	if (timer) clearInterval(timer)
})
</script>

<style scoped>
.text-gradient {
	background: linear-gradient(to right, var(--color-primary-600), var(--color-primary-500));
	-webkit-background-clip: text;
	-webkit-text-fill-color: transparent;
	background-clip: text;
}
</style>

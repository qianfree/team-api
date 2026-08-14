<template>
	<div class="auth-layout relative min-h-[100dvh] overflow-x-hidden">
		<!-- Announcement Banner (teleported to body, fixed top) -->
		<Teleport to="body">
			<div v-if="announcements.length" class="fixed top-0 left-0 right-0 z-50">
				<AnnouncementBanner :announcements="announcements" />
			</div>
		</Teleport>

		<div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

		<div class="relative z-10 flex min-h-[100dvh] flex-col">
			<main class="flex flex-1 items-center justify-center px-4 py-10 sm:py-14">
				<div class="w-full max-w-md">
					<!-- Logo/Brand -->
					<div class="mb-8 text-center">
						<div class="flex items-center justify-center gap-3">
							<div class="auth-brand-mark">
								<img src="/favicon.png" :alt="siteName" class="h-8 w-8 rounded-lg object-contain" />
							</div>
							<h1 class="text-2xl font-bold leading-tight text-slate-800">{{ siteName }}</h1>
						</div>
						<p class="mt-3 text-sm text-slate-500">{{ publicSettings.site_description || '开源自托管的多租户 大模型 API 网关平台' }}</p>
					</div>

					<!-- Card Container -->
					<div class="card card-prominent auth-card p-6 sm:p-8">
						<slot />
					</div>

					<!-- Footer Links -->
					<div class="mt-6 text-center text-sm">
						<slot name="footer" />
					</div>
				</div>
			</main>

			<!-- Copyright -->
			<footer class="auth-footer flex flex-shrink-0 items-center justify-between gap-2 px-6 pb-5 text-xs text-slate-400 sm:px-8">
				<span>&copy; {{ new Date().getFullYear() }} qianfree. Licensed under AGPL-3.0.</span>
				<a href="https://github.com/qianfree/team-api" target="_blank" rel="noopener noreferrer" class="text-slate-400 transition-colors hover:text-slate-600">Powered by Team-API</a>
			</footer>
		</div>
	</div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import AnnouncementBanner from '../common/AnnouncementBanner.vue'
import request from '@/utils/request'
import { usePublicSettings } from '@/composables/usePublicSettings'

const { settings: publicSettings } = usePublicSettings()
const siteName = computed(() => publicSettings.value.site_name || 'Team-API')
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
	timer = setInterval(fetchAnnouncements, 30 * 60 * 1000)
})

onBeforeUnmount(() => {
	if (timer) clearInterval(timer)
})
</script>

<style scoped>
.auth-brand-mark {
	display: flex;
	height: 3rem;
	width: 3rem;
	align-items: center;
	justify-content: center;
	border: 1px solid rgba(255, 255, 255, 0.86);
	border-radius: 1rem;
	background: rgba(255, 255, 255, 0.66);
	box-shadow: var(--shadow-glass-sm), inset 0 1px 0 rgba(255, 255, 255, 0.94);
	backdrop-filter: blur(20px) saturate(1.16);
}

.auth-card {
	box-shadow: var(--shadow-glass), var(--glass-highlight);
}

@media (max-width: 640px) {
	.auth-footer {
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 4px;
		text-align: center;
	}

		.auth-footer a {
			order: -1;
		}
}
</style>

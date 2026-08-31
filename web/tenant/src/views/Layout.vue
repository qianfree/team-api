<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import { useNotificationCount } from '@/composables/useNotificationCount'
import { useAnnouncementRead } from '@/composables/useAnnouncementRead'
import { usePublicSettings } from '@/composables/usePublicSettings'
import { useWatermark } from '@/composables/useWatermark'
import { formatBilling } from '@/composables/useCurrency'
import { toast } from '@/utils/toast'
import Icon from '@/components/common/Icon.vue'
import MaintenanceBanner from '@/components/common/MaintenanceBanner.vue'
import AnnouncementBanner from '@/components/common/AnnouncementBanner.vue'
import RouteErrorBoundary from '@/components/common/RouteErrorBoundary.vue'
import { marked } from 'marked'
import request from '@/utils/request'

const router = useRouter()
const route = useRoute()
const authStore = useTenantAuthStore()

const sidebarCollapsed = ref(false)
const mobileOpen = ref(false)
const userMenuOpen = ref(false)
const announcePanelOpen = ref(false)
const announceDetailItem = ref<any>(null)
const consoleAnnouncements = ref<any[]>([])
const { unreadCount: announceUnreadCount, markAsRead: markAnnouncementRead, markAllRead: markAllAnnouncementsRead, isRead: isAnnouncementRead } = useAnnouncementRead(consoleAnnouncements)
let announcementTimer: ReturnType<typeof setInterval> | null = null
const walletBalance = ref<string>('')
let walletTimer: ReturnType<typeof setInterval> | null = null
const memberQuota = ref<{ used: number; limit: number } | null>(null)
const { unreadCount, startPolling: startNotificationPolling, stopPolling: stopNotificationPolling, setOnNewNotification } = useNotificationCount()
setOnNewNotification((newCount: number) => {
	if (newCount > 0) {
		toast.info(newCount === 1 ? '收到 1 条新通知' : `收到 ${newCount} 条新通知`)
	}
})

const { settings: publicSettings, fetchSettings: fetchPublicSettings } = usePublicSettings()
const demoMessage = ref('')
const { mount: mountWatermark, unmount: unmountWatermark } = useWatermark(demoMessage)

const currentUser = computed(() => ({
	username: authStore.user?.username || '',
	role: authStore.user?.role || '',
}))

const tenantInfo = computed(() => ({
	name: authStore.tenant?.name || '',
	code: authStore.tenant?.code || '',
}))

// 从路由配置中读取菜单项，路由 meta 中的 sort 字段决定是否显示在菜单中
interface NavItem {
	label: string
	path: string
	icon: string
	badge?: string
}

const navItems = computed<NavItem[]>(() => {
	const role = authStore.user?.role || 'member'
	return router.getRoutes()
		.filter(r => r.meta.sort !== undefined)
		.filter(r => r.path !== '/tenant/docs')
		.filter(r => {
			const roles = r.meta.roles as string[] | undefined
			return !roles || roles.includes(role)
		})
		.sort((a, b) => (a.meta.sort as number) - (b.meta.sort as number))
		.map(r => ({
			label: r.meta.title as string,
			path: r.path,
			icon: r.meta.icon as string,
			badge: r.meta.badge as string | undefined,
		}))
})

const canViewWallet = computed(() => authStore.isManager)
const activePath = computed(() => route.path)
const pageTitle = computed(() => {
		const matched = route.matched
		const leaf = matched[matched.length - 1]
		return (leaf?.meta?.title as string) || navItems.value.find((i) => isActive(i.path))?.label || '仪表盘'
	})

function isActive(path: string): boolean {
	return activePath.value === path || activePath.value.startsWith(path + '/')
}

function toggleSidebar() {
	sidebarCollapsed.value = !sidebarCollapsed.value
}

function toggleMobile() {
	mobileOpen.value = !mobileOpen.value
}

function closeMobile() {
	mobileOpen.value = false
}

function toggleUserMenu() {
	userMenuOpen.value = !userMenuOpen.value
}

function closeUserMenu() {
	userMenuOpen.value = false
}

function toggleAnnouncePanel() {
	announcePanelOpen.value = !announcePanelOpen.value
}


function openAnnouncementDetail(item: any) {
	markAnnouncementRead(item.id)
	announceDetailItem.value = item
	announcePanelOpen.value = false
}

function closeAnnouncementDetail() {
	announceDetailItem.value = null
}

function renderMarkdown(text: string): string {
	return marked.parse(text) as string
}

async function handleLogout() {
	stopNotificationPolling()
	if (announcementTimer) {
		clearInterval(announcementTimer)
		announcementTimer = null
	}
	await authStore.logout()
	router.push('/tenant/login')
}

function handleNavClick() {
	if (mobileOpen.value) {
		setTimeout(closeMobile, 150)
	}
}

// Close user menu on outside click
function handleClickOutside(e: MouseEvent) {
	const target = e.target as HTMLElement
	if (!target.closest('[data-user-menu]')) {
		userMenuOpen.value = false
	}
	if (!target.closest('[data-announce-panel]')) {
		announcePanelOpen.value = false
	}
}

async function fetchAnnouncements() {
	try {
		const res = await request.get('/tenant/announcements')
		const list = res.data?.data?.list || []
		consoleAnnouncements.value = list.filter((a: any) => a.display_position === 'console' || a.display_position === 'both')
	} catch {
		// silently ignore
	}
}

async function fetchWalletBalance() {
	try {
		const res = await request.get('/tenant/wallet')
		const w = res.data?.data
		if (w) {
			const bal = w.available_balance ?? w.balance ?? 0
			walletBalance.value = bal >= 100 ? formatBilling(bal, 0) : formatBilling(bal, 2)
		}
	} catch {
		// silently ignore
	}
}

async function fetchMemberQuota() {
	try {
		const res = await request.get('/tenant/personal-dashboard')
		const q = res.data?.data?.quota
		if (q && q.quota_type !== 'none') {
			memberQuota.value = { used: q.quota_used ?? 0, limit: q.quota_limit ?? 0 }
		} else {
			memberQuota.value = null
		}
	} catch {
		// silently ignore
	}
}

onMounted(async () => {
	authStore.loadFromStorage()
	document.addEventListener('click', handleClickOutside)
	fetchAnnouncements()
	announcementTimer = setInterval(fetchAnnouncements, 30 * 60 * 1000)
	startNotificationPolling()
	if (canViewWallet.value) {
		fetchWalletBalance()
		walletTimer = setInterval(fetchWalletBalance, 5 * 60 * 1000)
	} else {
		fetchMemberQuota()
	}

	await fetchPublicSettings()
	if (publicSettings.site_name && route.meta.title) {
		document.title = `${pageTitle.value} — ${publicSettings.site_name}`
	}
	if (publicSettings.demo_mode) {
		demoMessage.value = publicSettings.demo_message || '演示环境，数据不可修改'
		mountWatermark(document.body)
	}
})

onBeforeUnmount(() => {
	document.removeEventListener('click', handleClickOutside)
	stopNotificationPolling()
	if (announcementTimer) {
		clearInterval(announcementTimer)
		announcementTimer = null
	}
	if (walletTimer) {
		clearInterval(walletTimer)
		walletTimer = null
	}
	unmountWatermark()
})
</script>

<template>
	<div class="tenant-layout min-h-screen overflow-x-clip">
	<!-- Maintenance Banner -->
		<MaintenanceBanner />
			<AnnouncementBanner v-if="consoleAnnouncements.length" :announcements="consoleAnnouncements" />

		<!-- Background Decoration -->
		<div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

		<!-- Sidebar -->
		<aside
			class="sidebar"
			:class="[
				sidebarCollapsed ? 'w-[76px]' : 'w-64',
				{ 'sidebar-mobile-hidden': !mobileOpen }
			]"
		>
			<!-- Sidebar Header -->
			<div class="sidebar-header">
				<div class="relative flex h-10 w-10 items-center justify-center overflow-hidden rounded-2xl shadow-glow bg-gradient-to-br from-cyan-400 via-primary-500 to-emerald-500">
					<span class="absolute h-5 w-8 -rotate-12 rounded-[50%] border border-white/60"></span>
					<span class="relative text-white font-bold text-sm">{{ tenantInfo.name.charAt(0) || 'T' }}</span>
				</div>
				<transition name="fade">
					<div v-if="!sidebarCollapsed" class="flex min-w-0 flex-col overflow-hidden">
						<span class="text-lg font-bold text-gray-900 truncate" :title="tenantInfo.name">{{ tenantInfo.name }}</span>
						<span class="text-xs text-gray-400 font-mono truncate">{{ tenantInfo.code }}</span>
					</div>
				</transition>
			</div>

			<!-- Navigation -->
			<nav class="sidebar-nav scrollbar-hide">
				<div class="sidebar-section">
					<router-link
						v-for="item in navItems"
						:key="item.path"
						:to="item.path"
						class="sidebar-link mb-1"
						:class="{ 'sidebar-link-active': isActive(item.path) }"
						:title="sidebarCollapsed ? item.label : undefined"
						@click="handleNavClick"
					>
						<Icon
							:name="item.icon"
							size="md"
							class="h-5 w-5 flex-shrink-0"
						/>
						<transition name="fade">
							<span v-if="!sidebarCollapsed" class="truncate">{{ item.label }}</span>
						</transition>
						<transition name="fade">
							<span
								v-if="!sidebarCollapsed && item.badge"
								class="ml-auto text-base"
								style="filter: drop-shadow(0 0 2px rgba(251, 191, 36, 0.6));"
							>
								{{ item.badge }}
							</span>
						</transition>
					</router-link>
				</div>
			</nav>

			<!-- Sidebar Footer -->
			<div class="mt-auto p-3">
				<button
					@click="toggleSidebar"
					class="sidebar-toggle"
					:class="sidebarCollapsed ? 'justify-center px-0' : 'justify-start gap-3 px-3.5'"
					:title="sidebarCollapsed ? '展开菜单' : '收起菜单'"
				>
					<Icon :name="sidebarCollapsed ? 'chevronDoubleRight' : 'chevronDoubleLeft'" size="md" class="flex-shrink-0" />
					<transition name="fade">
						<span v-if="!sidebarCollapsed">收起菜单</span>
					</transition>
				</button>
			</div>
		</aside>

		<!-- Mobile Overlay -->
		<transition name="fade">
			<div
				v-if="mobileOpen"
				class="fixed inset-0 z-30 bg-black/25 backdrop-blur-[2px] lg:hidden"
				@click="closeMobile"
			></div>
		</transition>

		<!-- Main Content Area -->
		<div
			class="tenant-shell relative min-h-screen transition-all duration-300"
			:class="[sidebarCollapsed ? 'lg:ml-[116px]' : 'lg:ml-[296px]']"
		>
			<!-- Header -->
			<header class="sticky top-0 z-[25] px-3 pt-3 md:px-5 md:pt-5">
				<div class="glass flex h-16 items-center justify-between rounded-2xl border border-white/70 px-3 shadow-glass-sm md:px-5">
					<!-- Left: Mobile Menu + Title -->
					<div class="flex min-w-0 items-center gap-3">
						<button
							@click="toggleMobile"
							class="btn-ghost btn-icon lg:hidden"
						>
							<Icon name="menu" size="md" />
						</button>
						<div class="hidden min-w-0 lg:block">
							<h1 class="truncate text-base font-semibold text-slate-800">{{ pageTitle }}</h1>
						</div>
					</div>

					<!-- Right: Actions -->
					<div class="topbar-actions flex items-center gap-2">
						<router-link
							to="/tenant/docs"
							class="docs-nav-entry"
							:class="{ 'docs-nav-entry-active': isActive('/tenant/docs') }"
							title="API 文档"
						>
							<Icon name="bookOpen" size="md" />
							<span class="hidden lg:inline">API 文档</span>
							<span class="docs-nav-badge hidden xl:inline-flex">DOCS</span>
						</router-link>

						<!-- Announcements -->
						<div class="relative" data-announce-panel>
						<button
							@click="toggleAnnouncePanel"
							class="topbar-icon-button relative flex h-10 w-10 items-center justify-center rounded-full border border-white/80 bg-white/65 text-slate-400 shadow-sm transition-all hover:-translate-y-0.5 hover:bg-white hover:text-primary-600"
								title="平台公告"
							>
								<Icon name="megaphone" size="md" />
								<span
									v-if="announceUnreadCount > 0"
									class="absolute -top-0.5 -right-0.5 h-2.5 w-2.5 rounded-full bg-red-500 ring-2 ring-white"
								></span>
							</button>
							<!-- Pulse animation layer -->
							<div
								v-if="announceUnreadCount > 0"
								class="absolute inset-0 rounded-xl animate-pulse-soft  pointer-events-none"
							></div>

							<!-- Announcement Dropdown Panel -->
							<transition name="fade">
								<div
									v-if="announcePanelOpen"
									class="absolute right-0 mt-2 w-80 bg-white rounded-xl border border-gray-200 shadow-lg overflow-hidden z-50 animate-scale-in"
								>
									<!-- Panel Header -->
									<div class="flex items-center justify-between px-4 py-3 border-b border-gray-100">
										<h3 class="text-sm font-semibold text-gray-900">平台公告</h3>
										<button
											v-if="announceUnreadCount > 0"
											@click="markAllAnnouncementsRead"
											class="text-xs text-primary-600 hover:text-primary-700 font-medium"
										>
											全部已读
										</button>
									</div>
									<!-- Panel Body -->
									<div class="max-h-72 overflow-y-auto">
										<div v-if="consoleAnnouncements.length === 0" class="px-4 py-8 text-center text-sm text-gray-400">
											暂无公告
										</div>
										<div
											v-for="item in consoleAnnouncements"
											:key="item.id"
											@click="openAnnouncementDetail(item)"
											class="px-4 py-3 flex items-start gap-3 cursor-pointer transition-colors hover:bg-gray-50 border-b border-gray-50 last:border-b-0"
										>
											<!-- Unread dot -->
											<div
												v-if="!isAnnouncementRead(item.id)"
												class="flex-shrink-0 mt-1.5 h-2 w-2 rounded-full bg-primary-500"
											></div>
											<div v-else class="flex-shrink-0 mt-1.5 h-2 w-2"></div>
											<!-- Content -->
											<div class="flex-1 min-w-0">
												<p
													class="text-sm truncate"
													:class="isAnnouncementRead(item.id) ? 'text-gray-500' : 'text-gray-900 font-medium'"
												>
													{{ item.title }}
												</p>
												<p class="text-xs text-gray-400 mt-0.5">{{ item.created_at }}</p>
											</div>
										</div>
									</div>
								</div>
							</transition>
						</div>

						<!-- Notifications -->
					<router-link
						to="/tenant/notifications"
						class="topbar-icon-button relative flex h-10 w-10 items-center justify-center rounded-full border border-white/80 bg-white/65 text-slate-400 shadow-sm transition-all hover:-translate-y-0.5 hover:bg-white hover:text-primary-600"
							title="通知中心"
						>
							<Icon name="bell" size="md" />
							<span
								v-if="unreadCount > 0"
								class="absolute -top-0.5 -right-0.5 flex h-4 min-w-[1rem] items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-medium text-white"
							>
								{{ unreadCount > 99 ? '99+' : unreadCount }}
							</span>
						</router-link>

					<!-- Wallet Capsule -->
					<router-link
						v-if="canViewWallet"
						to="/tenant/wallet"
						class="topbar-wallet flex h-10 items-center gap-1.5 rounded-full border border-white/80 bg-white/65 px-3 text-primary-600 shadow-sm transition-all hover:-translate-y-0.5 hover:bg-white"
							title="钱包"
						>
							<Icon name="currencyDollar" size="sm" />
							<span class="text-xs font-semibold tracking-tight">{{ walletBalance || formatBilling(0, 2) }}</span>
						</router-link>
						<!-- Member Quota Capsule -->
					<router-link
						v-if="!canViewWallet && memberQuota"
						to="/tenant/personal-dashboard"
						class="topbar-wallet flex h-9 items-center gap-1.5 rounded-full bg-gray-50 border border-gray-200/60 px-3 text-gray-600 hover:bg-gray-100 hover:border-gray-300 hover:shadow-sm transition-all duration-200"
							title="额度"
						>
							<Icon name="chart" size="sm" />
							<span class="text-xs font-semibold tracking-tight">${{ memberQuota.limit > 0 ? (memberQuota.used / memberQuota.limit * 100).toFixed(0) : 0 }}%</span>
						</router-link>
						<!-- User Menu -->
						<div class="relative" data-user-menu>
						<button
							@click="toggleUserMenu"
							class="topbar-user-button flex items-center gap-2 rounded-xl px-2 py-1.5 transition-colors hover:bg-gray-100"
						>
							<div class="topbar-user-avatar flex h-8 w-8 items-center justify-center rounded-full bg-gradient-to-br from-primary-500 to-primary-600 text-white text-sm font-medium">
									{{ currentUser.username.charAt(0).toUpperCase() }}
								</div>
								<div class="hidden sm:block text-left">
									<p class="text-sm font-medium text-gray-700">{{ currentUser.username }}</p>
									<p class="text-xs text-gray-500">{{ currentUser.role }}</p>
								</div>
								<Icon name="chevronDown" size="sm" class="text-gray-400" />
							</button>

							<!-- Dropdown -->
							<transition name="fade">
								<div
									v-if="userMenuOpen"
									class="dropdown user-dropdown right-0 mt-2 w-56"
								>
									<div class="border-b border-gray-100 px-4 py-3 flex items-center">
										<div class="flex-1 min-w-0">
											<p class="text-sm font-medium text-gray-800 truncate">{{ currentUser.username }}</p>
											<p class="text-xs text-gray-500 truncate">{{ authStore.tenant?.name }}</p>
										</div>
										<router-link
											v-if="authStore.isOwner"
											to="/tenant/organization"
											class="ml-2 rounded-lg border border-gray-200 px-2 py-1 text-xs text-gray-500 hover:border-primary-300 hover:text-primary-500 transition-colors"
											@click.stop="closeUserMenu"
										>
											设置
										</router-link>
									</div>
									<router-link
										to="/tenant/profile"
										class="dropdown-item"
										@click="closeUserMenu"
									>
										<Icon name="user" size="sm" class="text-gray-400" />
										个人设置
									</router-link>
									<button
										@click="handleLogout"
										class="dropdown-item w-full text-red-600 hover:bg-red-50"
									>
										<Icon name="logout" size="sm" class="text-red-400" />
										退出登录
									</button>
								</div>
							</transition>
						</div>
					</div>
				</div>
			</header>

			<!-- Page Content -->
			<main
				class="tenant-main px-3 pb-5 pt-4 md:px-5 md:pb-6 lg:pt-5"
				:class="{ 'tenant-table-main': route.meta.viewportTable }"
			>
				<router-view v-slot="{ Component, route: currentRoute }">
					<RouteErrorBoundary :key="currentRoute.fullPath">
						<component :is="Component" />
					</RouteErrorBoundary>
				</router-view>
			</main>
		</div>
	</div>

	<!-- Announcement Detail Modal -->
	<Teleport to="body">
		<Transition name="fade">
			<div v-if="announceDetailItem" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm" @click.self="closeAnnouncementDetail">
				<div class="w-full max-w-3xl rounded-2xl bg-white shadow-2xl border border-gray-200 animate-scale-in" @click.stop>
					<div class="flex items-start gap-3 px-6 py-4 border-b border-gray-100">
						<div class="flex-1 min-w-0">
							<h3 class="text-lg font-semibold text-gray-900">{{ announceDetailItem.title }}</h3>
							<p class="text-xs text-gray-500 mt-0.5">{{ announceDetailItem.created_at }}</p>
						</div>
						<button @click="closeAnnouncementDetail" class="flex-shrink-0 rounded-lg p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors">
							<Icon name="x" size="md" />
						</button>
					</div>
					<div class="px-6 py-5 max-h-[60vh] overflow-y-auto">
						<div class="announcement-content prose prose-sm max-w-none text-gray-700" v-html="renderMarkdown(announceDetailItem.content)"></div>
					</div>
					<div class="px-6 py-3 border-t border-gray-100 flex justify-end">
						<button @click="closeAnnouncementDetail" class="btn btn-secondary btn-sm">关闭</button>
					</div>
				</div>
			</div>
		</Transition>
	</Teleport>

</template>

<style scoped>
.docs-nav-entry {
	display: inline-flex;
	height: 2.5rem;
	flex-shrink: 0;
	align-items: center;
	justify-content: center;
	gap: 0.5rem;
	border: 1px solid rgba(20, 184, 166, 0.28);
	border-radius: 9999px;
	background: rgba(255, 255, 255, 0.82);
	padding: 0 0.65rem;
	box-shadow: 0 8px 22px rgba(13, 148, 136, 0.12), inset 0 1px 0 rgba(255, 255, 255, 0.94);
	color: #0d9488;
	font-size: 0.75rem;
	font-weight: 700;
	transition: transform 180ms ease, border-color 180ms ease, background-color 180ms ease, box-shadow 180ms ease;
}

.docs-nav-entry:hover,
.docs-nav-entry-active {
	transform: translateY(-1px);
	border-color: rgba(20, 184, 166, 0.5);
	background: #fff;
	box-shadow: 0 10px 26px rgba(13, 148, 136, 0.18), inset 0 1px 0 #fff;
}

.docs-nav-badge {
	height: 1.25rem;
	align-items: center;
	border: 1px solid #fed7aa;
	border-radius: 9999px;
	background: #fff7ed;
	padding: 0 0.4rem;
	color: #ea580c;
	font-size: 0.5625rem;
	font-weight: 800;
}

.user-dropdown {
	background: #fff;
	backdrop-filter: none;
}

.fade-enter-active,
.fade-leave-active {
	transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
	opacity: 0;
}

@media (max-width: 639px) {
	.topbar-actions {
		gap: 0.375rem;
	}

	.topbar-actions :deep(svg) {
		height: 1rem;
		width: 1rem;
	}

	.docs-nav-entry,
	.topbar-icon-button {
		height: 2.25rem;
		width: 2.25rem;
	}

	.docs-nav-entry {
		padding: 0;
	}

	.topbar-wallet {
		height: 2.25rem;
		gap: 0.25rem;
		padding-right: 0.55rem;
		padding-left: 0.55rem;
	}

	.topbar-user-button {
		gap: 0.25rem;
		padding: 0.25rem;
	}

	.topbar-user-avatar {
		height: 1.75rem;
		width: 1.75rem;
		font-size: 0.75rem;
	}
}

@media (min-width: 1024px) {
	.docs-nav-entry {
		padding-right: 0.85rem;
		padding-left: 0.85rem;
	}
}

@media (prefers-reduced-motion: reduce) {
	.docs-nav-entry {
		transition: none;
	}
}
</style>

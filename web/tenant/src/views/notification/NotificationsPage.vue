<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'
import { useNotificationCount } from '@/composables/useNotificationCount'

interface Notification {
	id: number
	title: string
	content: string
	type: string
	is_read: boolean
	is_broadcast: number
	created_at: string
}

const notifications = ref<Notification[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = 20
const total = ref(0)
const activeTab = ref<'all' | 'unread'>('all')

const showExportDropdown = ref(false)
const { exporting, exportFile } = useExport({
	url: '/tenant/notifications/export',
})

const { unreadCount, decrement: decrementUnread, reset: resetUnread } = useNotificationCount()

const totalPages = computed(() => Math.ceil(total.value / pageSize))

const typeLabel: Record<string, string> = {
	security: '安全',
	billing: '账单',
	system: '系统',
	member: '成员',
	project: '项目',
}

const typeBadgeClass: Record<string, string> = {
	security: 'badge-danger',
	billing: 'badge-warning',
	system: 'badge-primary',
	member: 'badge-success',
	project: 'badge-purple',
}

async function fetchNotifications() {
	loading.value = true
	try {
		const params: any = { page: page.value, page_size: pageSize }
		if (activeTab.value === 'unread') params.is_read = false
		const res: any = await request.get('/tenant/notifications', { params })
		const raw = res.data?.data
		notifications.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
		total.value = raw?.total || 0
	} catch {
		notifications.value = []
		total.value = 0
	} finally {
		loading.value = false
	}
}

async function fetchUnreadCount() {
	try {
		const res: any = await request.get('/tenant/notifications/unread-count')
		unreadCount.value = res.data?.data?.count || 0
	} catch {
		resetUnread()
	}
}

async function markAsRead(id: number) {
	try {
		await request.post(`/tenant/notifications/${id}/read`)
		const n = notifications.value.find((item) => item.id === id)
		if (n) n.is_read = true
		decrementUnread()
	} catch {
	}
}

async function markAllRead() {
	try {
		await request.post('/tenant/notifications/read-all')
		notifications.value.forEach((n) => { n.is_read = true })
		resetUnread()
	} catch {
	}
}

const confirmDeleteId = ref<number | null>(null)

async function deleteNotification(n: Notification) {
	try {
		await request.delete(`/tenant/notifications/${n.id}`)
		notifications.value = notifications.value.filter((item) => item.id !== n.id)
		total.value--
		confirmDeleteId.value = null
	} catch {
	}
}

function switchTab(tab: 'all' | 'unread') {
	activeTab.value = tab
	page.value = 1
	fetchNotifications()
}

function handlePageChange(newPage: number) {
	page.value = newPage
	fetchNotifications()
}

onMounted(() => {
	fetchNotifications()
	fetchUnreadCount()
})
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="page-header flex items-center justify-between">
			<div>
				<h1 class="page-title">通知中心</h1>
				<p class="page-description">查看系统通知和消息</p>
			</div>
			<div class="flex items-center gap-2">
				<!-- Export dropdown -->
				<div class="relative inline-block">
					<button class="btn btn-secondary" :disabled="exporting" @click="showExportDropdown = !showExportDropdown">
						<svg v-if="!exporting" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3"/></svg>
						<svg v-else class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
						导出
					</button>
					<div v-if="showExportDropdown" class="absolute right-0 mt-2 w-36 bg-white rounded-xl border shadow-lg py-1 z-50">
						<div class="px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 cursor-pointer" @click="exportFile('csv'); showExportDropdown = false">导出 CSV</div>
						<div class="px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 cursor-pointer" @click="exportFile('xlsx'); showExportDropdown = false">导出 Excel</div>
					</div>
				</div>
				<button class="btn btn-secondary" @click="markAllRead" :disabled="unreadCount === 0">
					<Icon name="check" size="sm" />
					全部已读
				</button>
			</div>
		</div>

		<!-- Tabs -->
		<div class="flex items-center gap-3 mb-4">
			<button
				class="tab"
				:class="{ 'tab-active': activeTab === 'all' }"
				@click="switchTab('all')"
			>
				全部
			</button>
			<button
				class="tab"
				:class="{ 'tab-active': activeTab === 'unread' }"
				@click="switchTab('unread')"
			>
				未读
				<span v-if="unreadCount > 0" class="ml-1 inline-flex items-center justify-center h-4 min-w-[1rem] px-1 rounded-full bg-red-500 text-white text-[10px] font-medium">
					{{ unreadCount > 99 ? '99+' : unreadCount }}
				</span>
			</button>
		</div>

		<!-- Loading -->
		<div v-if="loading" class="card p-8 text-center">
			<div class="spinner mx-auto mb-3"></div>
			<p class="text-sm text-gray-500">加载中...</p>
		</div>

		<!-- Empty -->
		<div v-else-if="notifications.length === 0" class="empty-state card">
			<Icon name="bell" size="xl" class="empty-state-icon text-gray-300" />
			<p class="empty-state-title">暂无通知</p>
			<p class="empty-state-description">您的通知消息将显示在这里</p>
		</div>

		<!-- Notification List -->
		<div v-else class="space-y-2">
			<div
				v-for="n in notifications"
				:key="n.id"
				class="card transition-all duration-200"
				:class="{ 'border-l-4 border-l-primary-500': !n.is_read, 'opacity-70': n.is_read }"
			>
				<div class="px-6 py-4">
					<div class="flex items-start justify-between gap-3">
						<div class="flex items-start gap-3 flex-1 min-w-0" @click="!n.is_read && markAsRead(n.id)">
							<!-- Unread dot -->
							<div class="mt-1.5 flex-shrink-0">
								<div v-if="!n.is_read" class="h-2.5 w-2.5 rounded-full bg-primary-500"></div>
								<div v-else class="h-2.5 w-2.5 rounded-full bg-gray-300"></div>
							</div>

							<div class="flex-1 min-w-0">
								<div class="flex items-center gap-2 mb-1">
									<span class="text-sm font-medium" :class="n.is_read ? 'text-gray-600' : 'text-gray-900'">{{ n.title }}</span>
									<span v-if="n.type" class="badge" :class="typeBadgeClass[n.type] || 'badge-gray'">
										{{ typeLabel[n.type] || n.type }}
									</span>
								</div>
								<div class="text-sm text-gray-600 notification-content" v-html="n.content"></div>
							</div>
						</div>

						<div class="flex items-center gap-2 flex-shrink-0 mt-0.5">
							<!-- Delete button for read personal messages -->
							<template v-if="n.is_read && !n.is_broadcast">
								<button v-if="confirmDeleteId !== n.id" class="btn btn-ghost btn-sm text-red-400 hover:text-red-500 hover:bg-red-50" @click.stop="confirmDeleteId = n.id">
									<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0"/></svg>
								</button>
								<template v-else>
									<span class="text-xs text-gray-400">删除？</span>
									<button class="btn btn-danger btn-sm text-xs" @click.stop="deleteNotification(n)">确认</button>
									<button class="btn btn-secondary btn-sm text-xs" @click.stop="confirmDeleteId = null">取消</button>
								</template>
							</template>
							<span class="text-xs text-gray-400">
								{{ n.created_at ? new Date(n.created_at).toLocaleString() : '' }}
							</span>
						</div>
					</div>
				</div>
			</div>

			<!-- Pagination -->
			<div v-if="totalPages > 1" class="flex items-center justify-between px-1 pt-3">
				<span class="text-xs text-gray-500">共 {{ total }} 条通知</span>
				<div class="flex items-center gap-2">
					<button
						class="btn btn-ghost btn-sm"
						:disabled="page <= 1"
						@click="handlePageChange(page - 1)"
					>
						上一页
					</button>
					<span class="text-sm text-gray-600">{{ page }} / {{ totalPages }}</span>
					<button
						class="btn btn-ghost btn-sm"
						:disabled="page >= totalPages"
						@click="handlePageChange(page + 1)"
					>
						下一页
					</button>
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped>
.tabs {
	display: flex;
	gap: 0.25rem;
	padding: 0.25rem;
	border-radius: 0.75rem;
	background: #f3f4f6;
}
.tab {
	border-radius: 0.5rem;
	padding: 0.5rem 1rem;
	font-size: 0.875rem;
	font-weight: 500;
	color: #4b5563;
	transition: all 0.15s;
	cursor: pointer;
	border: none;
	background: transparent;
}
.tab-active {
	background: white;
	color: #111827;
	box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
}
.notification-content :deep(p) {
	margin-bottom: 0.5rem;
}
.notification-content :deep(p:last-child) {
	margin-bottom: 0;
}
.notification-content :deep(strong) {
	font-weight: 600;
	color: #111827;
}
</style>

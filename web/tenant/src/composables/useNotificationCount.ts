import { ref } from 'vue'
import request from '@/utils/request'

const unreadCount = ref(0)
let pollTimer: ReturnType<typeof setInterval> | null = null
let onNewCallback: ((count: number) => void) | null = null

async function fetchCount() {
	try {
		const res: any = await request.get('/tenant/notifications/unread-count', {
			_suppressErrorMsg: true,
		} as any)
		const newCount = res.data?.data?.unread_count || 0
		if (newCount > unreadCount.value && onNewCallback) {
			onNewCallback(newCount - unreadCount.value)
		}
		unreadCount.value = newCount
	} catch {
		// silently ignore
	}
}

function startPolling() {
	fetchCount()
	if (!pollTimer) {
		pollTimer = setInterval(fetchCount, 10 * 60 * 1000)
	}
}

function stopPolling() {
	if (pollTimer) {
		clearInterval(pollTimer)
		pollTimer = null
	}
}

function decrement() {
	if (unreadCount.value > 0) unreadCount.value--
}

function reset() {
	unreadCount.value = 0
}

function setOnNewNotification(cb: (newCount: number) => void) {
	onNewCallback = cb
}

export function useNotificationCount() {
	return {
		unreadCount,
		fetchCount,
		startPolling,
		stopPolling,
		decrement,
		reset,
		setOnNewNotification,
	}
}

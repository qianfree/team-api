import { ref } from 'vue'
import request from '@/utils/request'
import { createPoller } from '@/composables/usePolling'

const unreadCount = ref(0)
// 未读数轮询：页面隐藏时暂停，恢复可见时按剩余时间续排
const poller = createPoller(fetchCount, 10 * 60 * 1000)
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
	poller.start()
}

function stopPolling() {
	poller.stop()
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

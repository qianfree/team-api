import { ref } from 'vue'
import request from '@/utils/request'

const unreadCount = ref(0)
let onNewCallback: ((count: number) => void) | null = null

export function useNotificationCount() {
	let timer: ReturnType<typeof setInterval> | null = null
	let previousCount = -1

	async function fetchCount() {
		try {
			const res: any = await request.get('/tenant/notifications/unread-count', {
				_suppressErrorMsg: true,
			} as any)
			const newCount = res.data?.data?.count || 0
			const oldCount = unreadCount.value
			previousCount = oldCount
			unreadCount.value = newCount
			if (previousCount >= 0 && newCount > oldCount && onNewCallback) {
				onNewCallback(newCount - oldCount)
			}
		} catch {
			// silently ignore
		}
	}

	function startPolling(interval = 30_000) {
		fetchCount()
		timer = setInterval(fetchCount, interval)
	}

	function stopPolling() {
		if (timer) {
			clearInterval(timer)
			timer = null
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

	return {
		unreadCount,
		startPolling,
		stopPolling,
		fetchCount,
		decrement,
		reset,
		setOnNewNotification,
	}
}

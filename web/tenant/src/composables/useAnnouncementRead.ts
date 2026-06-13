import { ref, computed } from 'vue'

const STORAGE_KEY = 'tenant_read_announcements'

function loadReadIds(): Set<number> {
	try {
		const raw = localStorage.getItem(STORAGE_KEY)
		if (raw) return new Set(JSON.parse(raw))
	} catch { /* ignore */ }
	return new Set()
}

function saveReadIds(ids: Set<number>) {
	try {
		localStorage.setItem(STORAGE_KEY, JSON.stringify([...ids]))
	} catch { /* ignore */ }
}

const readIds = ref<Set<number>>(loadReadIds())

export function useAnnouncementRead(announcements: { value: { id: number }[] }) {
	const unreadCount = computed(() => {
		return announcements.value.filter(a => !readIds.value.has(a.id)).length
	})

	const unreadItems = computed(() => {
		return announcements.value.filter(a => !readIds.value.has(a.id))
	})

	function markAsRead(id: number) {
		readIds.value.add(id)
		saveReadIds(readIds.value)
	}

	function markAllRead() {
		for (const a of announcements.value) {
			readIds.value.add(a.id)
		}
		saveReadIds(readIds.value)
	}

	function isRead(id: number): boolean {
		return readIds.value.has(id)
	}

	return {
		unreadCount,
		unreadItems,
		markAsRead,
		markAllRead,
		isRead,
	}
}

import { ref } from 'vue'

export interface ToastItem {
	id: number
	type: 'success' | 'error' | 'warning' | 'info'
	message: string
}

export const toasts = ref<ToastItem[]>([])

let nextId = 0

function addToast(type: ToastItem['type'], message: string, duration = 3000) {
	const id = nextId++
	toasts.value.push({ id, type, message })
	if (duration > 0) {
		setTimeout(() => removeToast(id), duration)
	}
}

export function removeToast(id: number) {
	const idx = toasts.value.findIndex((t) => t.id === id)
	if (idx !== -1) toasts.value.splice(idx, 1)
}

export const toast = {
	success(message: string, duration?: number) { addToast('success', message, duration) },
	error(message: string, duration?: number) { addToast('error', message, duration) },
	warning(message: string, duration?: number) { addToast('warning', message, duration) },
	info(message: string, duration?: number) { addToast('info', message, duration) },
}

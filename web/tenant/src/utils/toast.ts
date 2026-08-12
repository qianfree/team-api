import { createDiscreteApi } from 'naive-ui'
import { themeOverrides } from './naiveTheme'

/**
 * 全局消息提示——基于 Naive UI message（createDiscreteApi 可在任意非组件上下文调用）。
 * 对外 API 签名保持与旧实现一致：toast.success/error/warning/info(message, duration?)。
 */
const { message } = createDiscreteApi(['message'], {
	configProviderProps: { themeOverrides },
})

export interface ToastItem {
	id: number
	type: 'success' | 'error' | 'warning' | 'info'
	message: string
}

let nextId = 0

function addToast(type: ToastItem['type'], msg: string, duration: number) {
	nextId += 1
	switch (type) {
		case 'success':
			message.success(msg, { duration })
			break
		case 'error':
			message.error(msg, { duration })
			break
		case 'warning':
			message.warning(msg, { duration })
			break
		case 'info':
			message.info(msg, { duration })
			break
	}
}

export function removeToast(_id: number) {
	// Naive message 由内部自动销毁，无需手动移除；保留签名以兼容旧调用
}

export const toast = {
	success(message: string, duration = 3000) { addToast('success', message, duration) },
	error(message: string, duration = 3000) { addToast('error', message, duration) },
	warning(message: string, duration = 3000) { addToast('warning', message, duration) },
	info(message: string, duration = 3000) { addToast('info', message, duration) },
}
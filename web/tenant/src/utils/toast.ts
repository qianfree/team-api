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

// Naive message 不在 aria-live 区域内，读屏用户听不到任何 toast；
// 用一个视觉隐藏的 polite live region 镜像播报每条消息
let liveRegion: HTMLDivElement | null = null

function announce(msg: string) {
	if (!liveRegion) {
		liveRegion = document.createElement('div')
		liveRegion.setAttribute('aria-live', 'polite')
		liveRegion.setAttribute('role', 'status')
		liveRegion.style.cssText =
			'position:absolute;width:1px;height:1px;padding:0;margin:-1px;overflow:hidden;clip:rect(0 0 0 0);white-space:nowrap;border:0'
		document.body.appendChild(liveRegion)
	}
	// 先清空再写入，连续两条相同文案时也能触发第二次播报
	liveRegion.textContent = ''
	requestAnimationFrame(() => {
		if (liveRegion) liveRegion.textContent = msg
	})
}

function addToast(type: ToastItem['type'], msg: string, duration: number) {
	nextId += 1
	announce(msg)
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
import { createDiscreteApi } from 'naive-ui'
import { themeOverrides } from './../utils/naiveTheme'

interface ConfirmOptions {
	title?: string
	message: string
	confirmText?: string
	cancelText?: string
	danger?: boolean
}

// createDiscreteApi 使 dialog 可在任意非组件上下文调用，且随 configProviderProps 对齐主题
const { dialog } = createDiscreteApi(['dialog'], {
	configProviderProps: { themeOverrides },
})

export function useConfirm() {
	/** 弹出确认框，返回 Promise：确认 resolve(true)，取消/关闭 resolve(false) */
	function confirm(opts: ConfirmOptions | string): Promise<boolean> {
		const options: ConfirmOptions = typeof opts === 'string' ? { message: opts } : opts
		return new Promise((resolve) => {
			const base = {
				title: options.title ?? '确认操作',
				content: options.message,
				positiveText: options.confirmText ?? '确认',
				negativeText: options.cancelText ?? '取消',
				onPositiveClick: () => resolve(true),
				onNegativeClick: () => resolve(false),
				onClose: () => resolve(false),
			}
			// danger 用 error 预设（红色确认按钮），否则用 warning 预设（黄色提示）
			if (options.danger) {
				dialog.error(base)
			} else {
				dialog.warning(base)
			}
		})
	}

	return { confirm }
}
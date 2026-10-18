/**
 * 为 arco Message 补充读屏播报：Message 弹层不在 aria-live 区域内，
 * 视障用户听不到任何操作反馈。视觉提示仍走 Message，这里只把文案
 * 镜像一份到视觉隐藏的 polite live region。
 * （tenant 侧在 utils/toast.ts 内置了同样机制）
 */
let liveRegion: HTMLDivElement | null = null

export function announce(msg: string) {
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

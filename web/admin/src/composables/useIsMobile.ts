import { ref, onMounted, onBeforeUnmount } from 'vue'

/**
 * 响应式断点检测：当前视口宽度 < breakpoint 时视为移动端。
 * 默认 768（对应 Tailwind 的 md 断点），超过则在桌面/移动之间实时切换。
 *
 * 直接返回 isMobile ref（无须解构）：
 *   const isMobile = useIsMobile()
 *   watch(isMobile, v => { ... })
 */
export function useIsMobile(breakpoint = 768) {
  const isMobile = ref(false)
  let mql: MediaQueryList | null = null

  function update(e: MediaQueryListEvent | MediaQueryList) {
    isMobile.value = e.matches
  }

  onMounted(() => {
    mql = window.matchMedia(`(max-width: ${breakpoint - 1}px)`)
    update(mql)
    mql.addEventListener('change', update)
  })

  onBeforeUnmount(() => {
    mql?.removeEventListener('change', update)
    mql = null
  })

  return isMobile
}

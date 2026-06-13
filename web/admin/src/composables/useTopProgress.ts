import { ref, nextTick } from 'vue'

/**
 * 顶部加载进度条（模块级单例）
 *
 * 参考 NProgress 的精髓：start() 后用指数收敛缓慢爬升、永远到不了 100%，
 * 营造「加载中」的视觉反馈；done() 跳到 100% 再淡出。
 *
 * 状态放在模块作用域（非函数内），保证多处调用共享同一份进度。
 */

const TICK_MS = 100
/** 指数收敛时间常数：越大爬升越慢 */
const TIME_CONSTANT = 1500
/** 运行中进度上限，永远到不了 100% */
const MAX_PROGRESS = 90
/** done() 后让宽度先滑到 100% 再淡出的过渡时间 */
const HIDE_DELAY_MS = 300
/** 绝对超时兜底：防止 done() 因故未被调用导致进度条卡死 */
const TIMEOUT_MS = 15_000

const progress = ref(0)
const visible = ref(false)
/** 是否启用宽度过渡；重置帧临时关闭，避免归零时从右向左缩回 */
const smooth = ref(true)

let tickTimer: ReturnType<typeof setInterval> | null = null
let hideTimer: ReturnType<typeof setTimeout> | null = null
let startTime = 0

function clearTickTimer(): void {
  if (tickTimer !== null) {
    clearInterval(tickTimer)
    tickTimer = null
  }
}

function clearHideTimer(): void {
  if (hideTimer !== null) {
    clearTimeout(hideTimer)
    hideTimer = null
  }
}

function start(): void {
  // 清掉上一次 done() 排程的隐藏，避免「刚 start 就被旧定时器隐藏」
  clearHideTimer()

  // 幂等：已在运行则直接返回，不重置进度（支持重定向链中途多次 start）
  if (visible.value) return

  // 关闭宽度过渡，让重置（上次残留的 100% → 0）瞬间完成、不可见
  smooth.value = false
  progress.value = 0
  visible.value = true

  // 下一帧恢复过渡并起步爬升
  nextTick(() => {
    smooth.value = true
    progress.value = 8 // 即时起步反馈，避免前 100ms 空白
    startTime = Date.now()

    clearTickTimer()
    tickTimer = setInterval(() => {
      const elapsed = Date.now() - startTime
      // 绝对超时兜底
      if (elapsed > TIMEOUT_MS) {
        done()
        return
      }
      // 用 elapsed 而非 tick 累加，避免后台 tab 定时器被 throttle 后进度漂移
      const next = MAX_PROGRESS * (1 - Math.exp(-elapsed / TIME_CONSTANT))
      progress.value = Math.min(next, 99)
    }, TICK_MS)
  })
}

function done(): void {
  clearTickTimer()
  // 幂等：已隐藏则直接返回（会被 afterEach / onError / 超时三处调用）
  if (!visible.value) return

  progress.value = 100
  clearHideTimer()
  hideTimer = setTimeout(() => {
    visible.value = false
    // 不归零 progress，保持 100% 满宽度淡出；下次 start 时再无过渡重置
    hideTimer = null
  }, HIDE_DELAY_MS)
}

export function useTopProgress() {
  return { progress, visible, smooth, start, done }
}

/**
 * 页面可见性感知的轮询工具
 *
 * 后台标签页的轮询请求既浪费带宽也无展示意义，浏览器自身的定时器节流
 * 又不可靠（部分平台放宽到每分钟一次，仍会产生请求）。统一改由
 * visibilitychange 显式控制：页面隐藏时停止排队（隐藏期间零请求），
 * 恢复可见时若已超过轮询间隔立即补拉一次，未到则按剩余时间续排。
 *
 * 内部用递归 setTimeout 而非 setInterval：上一次回调完成后才排下一次，
 * 慢响应下不会堆叠请求。每个调用点创建独立实例，与组件生命周期无关，
 * 组件外（模块级单例）也可使用。
 */

export interface PollerOptions {
  /** start() 时是否立即执行一次回调。默认 false，与 setInterval「先等一个完整间隔」的语义一致 */
  immediate?: boolean
}

export interface Poller {
  /** 启动轮询。幂等：已运行时调用无效果 */
  start(): void
  /** 停止轮询并移除可见性监听。幂等，可再次 start() 重新开始完整间隔 */
  stop(): void
}

export function createPoller(
  callback: () => void | Promise<unknown>,
  intervalMs: number,
  options?: PollerOptions,
): Poller {
  /** 是否处于运行中（start 后 stop 前） */
  let active = false
  /** 回调是否执行中，防重入 */
  let inFlight = false
  /** 上次回调的执行起点，兼作调度锚点（计算剩余等待时间） */
  let lastRunAt = 0
  let timer: ReturnType<typeof setTimeout> | null = null

  function clearTimer(): void {
    if (timer !== null) {
      clearTimeout(timer)
      timer = null
    }
  }

  function run(): void {
    // 防重入：可见性事件与到期定时器可能同时触发
    if (inFlight) return
    // 隐藏态兜底：定时器到期与 visibilitychange 存在事件顺序竞态，执行前再判一次
    if (document.hidden) return
    inFlight = true
    lastRunAt = Date.now()
    // 回调异常吞掉，保证后续调度不中断（调用点自身已有 try/catch，这里是双保险）
    Promise.resolve(callback())
      .catch(() => {})
      .finally(() => {
        inFlight = false
        if (active) schedule()
      })
  }

  /** 唯一的排队入口：所有续排路径都经过这里，保证同一时刻至多一个待触发定时器 */
  function schedule(): void {
    if (!active || document.hidden) return
    clearTimer()
    const remaining = intervalMs - (Date.now() - lastRunAt)
    if (remaining <= 0) {
      run()
      return
    }
    timer = setTimeout(run, remaining)
  }

  function onVisibilityChange(): void {
    if (!active) return
    if (document.hidden) {
      // 隐藏：停止排队。在途请求自然完成，完成后发现隐藏态不会再续排
      clearTimer()
    } else {
      // 恢复可见：已超间隔则立即补拉（remaining <= 0 走 run），否则按剩余时间续排
      schedule()
    }
  }

  function start(): void {
    if (active) return
    active = true
    // 锚点重置：stop 后再 start 视为全新周期，首个回调在完整间隔之后
    lastRunAt = Date.now()
    document.addEventListener('visibilitychange', onVisibilityChange)
    if (options?.immediate) {
      run()
    } else {
      schedule()
    }
  }

  function stop(): void {
    active = false
    clearTimer()
    document.removeEventListener('visibilitychange', onVisibilityChange)
  }

  return { start, stop }
}

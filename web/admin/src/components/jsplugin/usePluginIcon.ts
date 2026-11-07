// 插件品牌图标加载：图标端点挂在 AdminAuth 之后（仅认 Authorization 头），
// <img :src> 无法携带 Bearer token 必然 401——改经 axios 拉 blob 转 ObjectURL
// （与 FilesPage 的 local serve 预览同模式）。按 key+version 缓存并共享给
// 卡片/详情抽屉，避免同屏 N 张卡各拉一次。
import { ref, watch, type Ref } from 'vue'
import request from '@/utils/request'

// 模块级缓存：key ↔ ObjectURL（同版本图标内容不变，命中即免请求）
const iconURLCache = new Map<string, string>()
// 去重进行中的请求（并发挂载的多张同 key 卡共享一次往返）
const inflight = new Map<string, Promise<string | null>>()

async function fetchIconObjectURL(key: string, version: string): Promise<string | null> {
  const cacheKey = `${key}@${version}`
  const cached = iconURLCache.get(cacheKey)
  if (cached) return cached
  const pending = inflight.get(cacheKey)
  if (pending) return pending

  const p = (async () => {
    try {
      const res: any = await request.get(`/api/admin/js-plugins/${key}/icon`, {
        params: { v: version },
        responseType: 'blob',
        timeout: 15000,
        // 无图标/瞬时失败是常态而非异常：静默回落文字头像，不走全局错误 toast
        _suppressErrorMsg: true,
      } as any)
      const blob = res.data instanceof Blob ? res.data : new Blob([res.data])
      const url = URL.createObjectURL(blob)
      iconURLCache.set(cacheKey, url)
      return url
    } catch {
      return null // 404（无上传图标）/瞬时错误：组件回落非图片形态，不 toast 打扰
    } finally {
      inflight.delete(cacheKey)
    }
  })()
  inflight.set(cacheKey, p)
  return p
}

/**
 * 按插件（key + active_version）加载品牌图标 ObjectURL。
 * 返回 Ref：null = 未就绪/加载失败（调用方回落图标名/文字头像）。
 * ObjectURL 归模块级缓存所有（进程生命周期内复用，不 revoke）。
 */
export function usePluginIcon(plugin: Ref<{ key: string; active_version?: string } | null | undefined>): Ref<string | null> {
  const iconURL = ref<string | null>(null)

  watch(
    () => plugin.value ? `${plugin.value.key}@${plugin.value.active_version || ''}` : '',
    async (sig) => {
      if (!sig) {
        iconURL.value = null
        return
      }
      const [key, version] = sig.split('@')
      iconURL.value = await fetchIconObjectURL(key, version)
    },
    { immediate: true },
  )

  return iconURL
}

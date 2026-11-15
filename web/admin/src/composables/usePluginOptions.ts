import { ref } from 'vue'
import request from '@/utils/request'

/**
 * JS 任务插件渠道（type=43）绑定插件的下拉选项。
 * 懒加载 + 进程内去重：仅在渠道表单需要时拉取一次；不筛启用状态——
 * 绑定关系独立于插件运行状态（未启用插件也可先绑好，启用后即生效）。
 */
export function usePluginOptions() {
  const pluginOptions = ref<{ label: string; value: string }[]>([])
  const pluginOptionsLoaded = ref(false)

  async function loadPluginOptions(force = false) {
    if (pluginOptionsLoaded.value && !force) return
    try {
      const res: any = await request.get('/admin/js-plugins')
      const list = res.data?.data?.list || res.data?.list || []
      pluginOptions.value = list.map((p: any) => ({
        label: `${p.name}（${p.key}）`,
        value: p.key,
      }))
      pluginOptionsLoaded.value = true
    } catch {
      // 拦截器已提示；选项保持为空，表单校验会拦截提交
    }
  }

  return { pluginOptions, loadPluginOptions }
}

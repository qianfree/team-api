import { ref } from 'vue'
import request from '@/utils/request'

interface CrudListOptions {
  url: string
  pageSize?: number
  params?: Record<string, any>
}

export function useCrudList<T = any>(options: CrudListOptions) {
  const { url, pageSize = 20 } = options

  const data = ref<T[]>([]) as { value: T[] }
  const loading = ref(false)
  const refreshing = ref(false)
  const finished = ref(false)
  const error = ref('')
  const page = ref(1)
  const total = ref(0)

  async function fetchData(append = false): Promise<void> {
    if (!append) {
      page.value = 1
      finished.value = false
    }

    loading.value = true
    error.value = ''

    try {
      const params: Record<string, any> = {
        page: page.value,
        page_size: pageSize,
        ...options.params,
      }

      const { data: res } = await request.get(url, { params })

      const list = res.data?.list || []
      const totalCount = res.data?.total || 0

      if (append) {
        data.value = [...data.value, ...list]
      } else {
        data.value = list
      }

      total.value = totalCount
      finished.value = data.value.length >= totalCount
    } catch (err: any) {
      error.value = err?.apiError?.message || err?.message || '加载失败'
    } finally {
      loading.value = false
      refreshing.value = false
    }
  }

  async function onRefresh(): Promise<void> {
    refreshing.value = true
    await fetchData(false)
  }

  async function onLoad(): Promise<void> {
    if (finished.value) return
    page.value++
    await fetchData(true)
  }

  async function refresh(): Promise<void> {
    await fetchData(false)
  }

  return {
    data,
    loading,
    refreshing,
    finished,
    error,
    total,
    page,
    fetchData,
    onRefresh,
    onLoad,
    refresh,
  }
}

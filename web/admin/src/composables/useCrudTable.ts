import { ref, reactive } from 'vue'
import request from '@/utils/request'

interface CrudTableOptions {
  fetchUrl: string
  fetchParams?: () => Record<string, any>
  pageSize?: number
}

export function useCrudTable(options: CrudTableOptions) {
  const loading = ref(false)
  const data = ref<any[]>([])
  const pagination = reactive({
    current: 1,
    pageSize: options.pageSize ?? 20,
    total: 0,
    showPageSize: true,
    pageSizeOptions: [10, 20, 50],
  })

  async function fetchData() {
    loading.value = true
    try {
      const params: Record<string, any> = {
        page: pagination.current,
        page_size: pagination.pageSize,
        ...options.fetchParams?.(),
      }
      const res: any = await request.get(options.fetchUrl, { params })
      data.value = res.data?.list || res.data?.data || []
      pagination.total = res.data?.total || 0
    } catch {
      data.value = []
      pagination.total = 0
    } finally {
      loading.value = false
    }
  }

  function handlePageChange(page: number) {
    pagination.current = page
    fetchData()
  }

  function handlePageSizeChange(pageSize: number) {
    pagination.pageSize = pageSize
    pagination.current = 1
    fetchData()
  }

  function refresh() {
    pagination.current = 1
    fetchData()
  }

  return {
    loading,
    data,
    pagination,
    fetchData,
    handlePageChange,
    handlePageSizeChange,
    refresh,
  }
}

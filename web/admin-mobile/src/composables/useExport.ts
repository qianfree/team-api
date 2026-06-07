import { ref } from 'vue'
import request from '@/utils/request'
import { showToast } from 'vant'

interface ExportOptions {
  url: string
  format?: 'csv' | 'xlsx'
  params?: Record<string, any>
}

export function useExport() {
  const exporting = ref(false)

  async function doExport(options: ExportOptions): Promise<void> {
    exporting.value = true
    try {
      const params = {
        format: options.format || 'csv',
        ...options.params,
      }
      const { data: res } = await request.get(options.url, {
        params,
        responseType: 'blob',
        _suppressErrorMsg: true,
      } as any)

      // Create download link
      const blob = new Blob([res], {
        type: options.format === 'xlsx'
          ? 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
          : 'text/csv',
      })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `export.${options.format || 'csv'}`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)

      showToast('导出成功')
    } catch {
      showToast('导出失败')
    } finally {
      exporting.value = false
    }
  }

  return { exporting, doExport }
}

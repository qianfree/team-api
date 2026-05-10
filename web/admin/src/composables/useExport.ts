import { ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'

interface UseExportOptions {
	url: string
	getFilters?: () => Record<string, any>
}

export function useExport(options: UseExportOptions) {
	const exporting = ref(false)

	async function exportFile(format: 'csv' | 'xlsx') {
		exporting.value = true
		try {
			const params: Record<string, any> = {
				format,
				...options.getFilters?.(),
			}
			const response = await request.get(options.url, {
				params,
				responseType: 'blob',
				timeout: 300000,
			})

			const blob = response.data instanceof Blob
				? response.data
				: new Blob([response.data])

			// Check if server returned JSON error instead of file
			if (blob.type && blob.type.includes('application/json')) {
				const text = await blob.text()
				try {
					const json = JSON.parse(text)
					Message.error(json.message || '导出失败')
				} catch {
					Message.error('导出失败')
				}
				return
			}

			// Extract filename from Content-Disposition
			const disposition = response.headers?.['content-disposition']
			let filename = `export.${format === 'xlsx' ? 'xlsx' : 'csv'}`
			if (disposition) {
				const match = disposition.match(/filename\*=UTF-8''([^;\n]+)/)
				if (match) {
					filename = decodeURIComponent(match[1])
				} else {
					const simpleMatch = disposition.match(/filename="?([^";\n]+)"?/)
					if (simpleMatch) filename = simpleMatch[1]
				}
			}

			// Trigger download
			const url = URL.createObjectURL(blob)
			const a = document.createElement('a')
			a.href = url
			a.download = filename
			document.body.appendChild(a)
			a.click()
			document.body.removeChild(a)
			URL.revokeObjectURL(url)

			Message.success('导出成功')
		} catch {
			// Error already handled by interceptor for most cases
		} finally {
			exporting.value = false
		}
	}

	return { exporting, exportFile }
}

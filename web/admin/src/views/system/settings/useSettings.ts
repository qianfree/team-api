import { ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'

export function useSettings(category: () => string) {
	const formValues = ref<Record<string, any>>({})
	const loading = ref(false)
	const saving = ref(false)

	async function refresh() {
		if (!category()) return
		loading.value = true
		try {
			const res: any = await request.get(`/admin/settings/${category()}`)
			const items = res.data?.data?.list || []
			const vals: Record<string, any> = {}
			for (const item of items) {
				let val = item.value ?? item.default ?? ''
				// Normalize bool values to strings for consistent switch handling
				if (item.type === 'bool') {
					val = String(val)
				}
				vals[item.key] = val
			}
			formValues.value = vals
		} catch {
			formValues.value = {}
		} finally {
			loading.value = false
		}
	}

	async function save() {
		saving.value = true
		try {
			await request.put(`/admin/settings/${category()}`, { settings: formValues.value })
			Message.success('保存成功')
			await refresh()
		} catch {
			// error toast already shown by interceptor
		} finally {
			saving.value = false
		}
	}

	return { formValues, loading, saving, refresh, save }
}

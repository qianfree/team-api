import { ref, reactive, provide, inject, type InjectionKey } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'

const settingsFormKey: InjectionKey<Record<string, any>> = Symbol('settingsFormValues')

export function useSettings(category: () => string) {
	const formValues = reactive<Record<string, any>>({})
	const loading = ref(false)
	const saving = ref(false)

	provide(settingsFormKey, formValues)

	async function refresh() {
		if (!category()) return
		loading.value = true
		try {
			const res: any = await request.get(`/admin/settings/${category()}`)
			const items = res.data?.data?.list || []
			const vals: Record<string, any> = {}
			for (const item of items) {
				vals[item.key] = item.value ?? item.default ?? ''
			}
			Object.keys(formValues).forEach(k => delete formValues[k])
			Object.assign(formValues, vals)
		} catch {
			Object.keys(formValues).forEach(k => delete formValues[k])
		} finally {
			loading.value = false
		}
	}

	async function save() {
		saving.value = true
		try {
			await request.put(`/admin/settings/${category()}`, { settings: formValues })
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

export function useFormValues() {
	return inject(settingsFormKey)!
}

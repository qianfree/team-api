import { ref, reactive, provide, inject, type InjectionKey } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'
import { refreshCurrencySettings } from '@/composables/useCurrency'

const settingsFormKey: InjectionKey<Record<string, any>> = Symbol('settingsFormValues')

// 各分类保存前校验器：返回错误文案则中断保存（与后端 validateCrossFieldSettings 口径保持一致）
const categoryValidators: Record<string, (values: Record<string, any>) => string | null> = {
	general: (values) => {
		// 维护时长必须是 Go duration 格式（2h/30m/1h30m），否则后端 API 维护的 Retry-After 解析会回退默认值
		const v = (values['maintenance_duration'] ?? '').toString().trim()
		if (v && !/^(\d+(\.\d+)?(ms|s|m|h))+$/.test(v)) {
			return '维护时长格式不正确，请使用如 2h、30m、1h30m 的时长格式'
		}
		return null
	},
}

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
		// 先跑分类校验器，非法值直接拦截，避免把脏数据发到后端
		const validator = categoryValidators[category()]
		if (validator) {
			const errMsg = validator(formValues)
			if (errMsg) {
				Message.error(errMsg)
				return
			}
		}
		saving.value = true
		try {
			await request.put(`/admin/settings/${category()}`, { settings: formValues })
			Message.success('保存成功')
			// 汇率等公共配置变更后强制刷新，已打开页面立即按新值重渲染货币显示
			refreshCurrencySettings()
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

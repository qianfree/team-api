import { ref, readonly } from 'vue'
import request from '@/utils/request'

/**
 * 全局公共配置（/settings/public，无认证）。
 * 与租户端 usePublicSettings 镜像：模块级单例 + 30s TTL + 并发合并，
 * 管理后台用于本位币/汇率（货币显示）与站点名等公共配置。
 */
export interface PublicSettings {
	site_name: string
	site_description: string
	maintenance_mode: boolean
	maintenance_message: string
	maintenance_duration: string
	demo_mode: boolean
	demo_message: string
	[key: string]: unknown
}

const settings = ref<PublicSettings>({
	site_name: '',
	site_description: '',
	maintenance_mode: false,
	maintenance_message: '',
	maintenance_duration: '',
	demo_mode: false,
	demo_message: '',
})

let fetchPromise: Promise<void> | null = null
let lastFetchTime = 0
const CACHE_TTL = 30_000 // 30秒缓存

export function usePublicSettings() {
	async function fetchSettings(force = false): Promise<void> {
		const now = Date.now()
		if (!force && now - lastFetchTime < CACHE_TTL && fetchPromise === null) {
			return
		}

		if (fetchPromise) {
			return fetchPromise
		}

		fetchPromise = (async () => {
			try {
				const { data } = await request.get('/settings/public')
				if (data?.data?.settings) {
					Object.assign(settings.value, data.data.settings)
				}
				lastFetchTime = Date.now()
			} catch {
				// silent — non-critical
			} finally {
				fetchPromise = null
			}
		})()

		return fetchPromise
	}

	return {
		settings: readonly(settings),
		fetchSettings,
	}
}

import { ref, readonly } from 'vue'
import request from '@/utils/request'

export interface PublicSettings {
	maintenance_mode: boolean
	maintenance_message: string
	maintenance_duration: string
	register_enabled: boolean
	register_email_verification: boolean
	site_name: string
	site_description: string
	demo_mode: boolean
	demo_message: string
	[key: string]: unknown
}

const settings = ref<PublicSettings>({
	maintenance_mode: false,
	maintenance_message: '',
	maintenance_duration: '',
	register_enabled: true,
	register_email_verification: false,
	site_name: '',
	site_description: '',
	demo_mode: false,
	demo_message: '',
})

let fetchPromise: Promise<void> | null = null
let lastFetchTime = 0
const CACHE_TTL = 60_000

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

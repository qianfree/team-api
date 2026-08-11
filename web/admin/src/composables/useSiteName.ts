import { ref } from 'vue'
import axios from 'axios'

const siteName = ref('')
let fetchPromise: Promise<void> | null = null
let lastFetchTime = 0
const CACHE_TTL = 30_000 // 30秒缓存，确保设置保存后标题能及时更新

async function fetchSiteName(force = false): Promise<void> {
	const now = Date.now()
	if (!force && now - lastFetchTime < CACHE_TTL && fetchPromise === null) {
		return
	}

	if (fetchPromise) {
		return fetchPromise
	}

	fetchPromise = (async () => {
		try {
			const res = await axios.get('/api/settings/public', { timeout: 5000 })
			const settings = res.data?.data?.settings
			if (settings?.site_name) {
				siteName.value = settings.site_name
			}
			lastFetchTime = Date.now()
		} catch {
			// silent — fallback to default
		} finally {
			fetchPromise = null
		}
	})()

	return fetchPromise
}

export function useSiteName() {
	return { siteName, fetchSiteName }
}

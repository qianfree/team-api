import { ref } from 'vue'
import axios from 'axios'

const siteName = ref('')
let fetched = false

async function fetchSiteName(): Promise<void> {
	if (fetched) return
	fetched = true
	try {
		const res = await axios.get('/api/settings/public', { timeout: 5000 })
		const settings = res.data?.data?.settings
		if (settings?.site_name) {
			siteName.value = settings.site_name
		}
	} catch {
		// silent — fallback to default
	}
}

export function useSiteName() {
	return { siteName, fetchSiteName }
}

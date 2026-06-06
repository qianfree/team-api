import { ref, onMounted, computed } from 'vue'
import request from '@/utils/request'

export interface ApiKeyItem {
	id: number
	name: string
	key_prefix: string
	status: string
	scope: string
}

export function usePlaygroundApiKey() {
	const apiKeys = ref<ApiKeyItem[]>([])
	const selectedKeyId = ref<number | null>(null)
	const revealedKey = ref<string | null>(null)
	const loading = ref(true)
	const error = ref<string | null>(null)

	const selectedKey = computed(() =>
		apiKeys.value.find((k) => k.id === selectedKeyId.value) ?? null,
	)

	async function fetchKeys() {
		try {
			loading.value = true
			error.value = null
			const res = await request.get('/tenant/api-keys', {
				params: { key_type: 'personal', page: 1, page_size: 100 },
			})
			const list: ApiKeyItem[] = res.data?.data?.list ?? []
			// 只保留 active 状态的 key
			apiKeys.value = list.filter((k: ApiKeyItem) => k.status === 'active')

			if (apiKeys.value.length > 0) {
				selectedKeyId.value = apiKeys.value[0].id
				await revealSelectedKey()
			} else {
				error.value = '没有可用的 API Key，请先创建'
			}
		} catch {
			error.value = '获取 API Key 列表失败'
		} finally {
			loading.value = false
		}
	}

	async function revealSelectedKey() {
		if (!selectedKeyId.value) {
			revealedKey.value = null
			return
		}
		try {
			const res = await request.get(`/tenant/api-keys/${selectedKeyId.value}/value`)
			revealedKey.value = res.data?.data?.key ?? null
		} catch {
			revealedKey.value = null
			error.value = '获取 API Key 失败'
		}
	}

	async function selectKey(id: number) {
		selectedKeyId.value = id
		await revealSelectedKey()
	}

	onMounted(fetchKeys)

	return {
		apiKeys,
		selectedKeyId,
		selectedKey,
		revealedKey,
		loading,
		error,
		selectKey,
	}
}

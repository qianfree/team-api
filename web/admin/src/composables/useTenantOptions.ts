import { ref, onUnmounted } from 'vue'
import request from '@/utils/request'

// 租户下拉选项：远程搜索专用轻量接口 /admin/tenants/select（仅返回 id/name/code），
// 300ms 防抖。value 为租户 ID（下发 tenant_id 筛选参数），label 为「名称（代码）」。
// ASelect 需配 allow-search + :filter-option="false"（关闭本地过滤，交给服务端搜索）。
export function useTenantOptions() {
	const tenantOptions = ref<{ label: string; value: number }[]>([])
	let searchTimer: ReturnType<typeof setTimeout> | null = null

	async function fetchTenantOptions(keyword = '') {
		try {
			const res: any = await request.get('/admin/tenants/select', {
				params: { page: 1, page_size: 50, keyword }
			})
			const list = res.data?.data?.list || []
			tenantOptions.value = list.map((t: any) => ({
				label: `${t.name}（${t.code}）`,
				value: t.id,
			}))
		} catch {
			tenantOptions.value = []
		}
	}

	function handleTenantSearch(value: string) {
		if (searchTimer) clearTimeout(searchTimer)
		searchTimer = setTimeout(() => fetchTenantOptions(value), 300)
	}

	onUnmounted(() => {
		if (searchTimer) clearTimeout(searchTimer)
	})

	return { tenantOptions, fetchTenantOptions, handleTenantSearch }
}

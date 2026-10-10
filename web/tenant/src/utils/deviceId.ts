/**
 * 持久设备 ID：首次生成 UUID 存 localStorage，登录/2FA 验证时随请求上报，
 * 服务端用于「新设备登录」识别（无设备 ID 时退化为 UA 哈希指纹）。
 */
const STORAGE_KEY = 'team_tenant_device_id'

function randomUUID(): string {
	// crypto.randomUUID 仅安全上下文（HTTPS / localhost）可用，HTTP 部署时降级手工拼 v4
	if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
		return crypto.randomUUID()
	}
	const bytes = new Uint8Array(16)
	crypto.getRandomValues(bytes)
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
	return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

export function getDeviceId(): string {
	try {
		let id = localStorage.getItem(STORAGE_KEY)
		if (!id) {
			id = randomUUID()
			localStorage.setItem(STORAGE_KEY, id)
		}
		return id
	} catch {
		// localStorage 不可用（隐私模式等）时返回空串，服务端退化为 UA 指纹
		return ''
	}
}

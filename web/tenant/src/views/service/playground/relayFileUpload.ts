import type { AxiosInstance } from 'axios'
import type { Attachment } from './useAttachments'

/**
 * /v1/files 素材上传通道（Playground 视频 Tab 专用）。
 *
 * 网关配置了云对象存储（/settings/public 的 relay_files_upload_enabled）时，
 * 参考素材优先经此处上传换取 file_id，请求里以 references[].file_id 引用——
 * 网关会解析成对象存储签名 URL 发给上游，避免 base64 内联撑爆请求体、
 * 以及部分上游（如 Seedance 参考视频）明确拒绝 base64 的问题。
 * 上传失败不阻断提交：图片/音频回退 dataUrl 内联，视频无法内联则留在 deferred。
 */

/** 上传超时：素材最大 5MB + 弱网上传带宽余量 */
const UPLOAD_TIMEOUT_MS = 120_000

/**
 * uploadRelayFile 把单个附件上传到 /v1/files，成功返回 file_id（file-xxx）。
 *
 * file_id 缓存在附件对象上（att.fileId），同一附件多次提交不重复上传；
 * 任何失败返回 null 由调用方决定回退策略，此处不弹错不重试。
 */
export async function uploadRelayFile(api: AxiosInstance, att: Attachment): Promise<string | null> {
	if (att.fileId) return att.fileId
	if (!att.blob) return null

	const form = new FormData()
	// filename 保留原始扩展名：后端类型白名单在 MIME 缺失/失真时按扩展名兜底
	form.append('file', att.blob, att.fileName)
	form.append('purpose', 'user_data')

	try {
		// /v1/files 返回原生 OpenAI File 对象（非统一响应格式），id 即 file_id。
		// 实例未设默认 Content-Type（见 playgroundApi.ts），FormData 由浏览器补
		// multipart boundary；若带 application/json 头，axios 会把 FormData JSON 化导致上传失败
		const res = await api.post('/v1/files', form, { timeout: UPLOAD_TIMEOUT_MS })
		const id = res.data?.id
		if (typeof id === 'string' && id) {
			att.fileId = id
			return id
		}
		return null
	} catch {
		return null
	}
}

/**
 * uploadVideoAttachments 批量上传视频 Tab 的参考素材，返回 附件id → file_id 映射。
 *
 * 并发上传（附件数上限 6，对后端无压力）；单个失败只影响该素材的引用形态。
 */
export async function uploadVideoAttachments(
	api: AxiosInstance,
	attachments: Attachment[],
): Promise<Map<number, string>> {
	const fileIds = new Map<number, string>()
	await Promise.all(
		attachments.map(async att => {
			const fileId = await uploadRelayFile(api, att)
			if (fileId) fileIds.set(att.id, fileId)
		}),
	)
	return fileIds
}

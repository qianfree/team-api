import { ref, computed, onUnmounted } from 'vue'
import { toast } from '@/utils/toast'

/**
 * Playground 附件管理
 *
 * 附件全部以 base64 data URL 内联进请求体（对话走 content parts，视频走 input_reference），
 * 不落对象存储。因此体积是硬约束：后端 GoFrame server 默认 ClientMaxBodySize 为 8MB，
 * base64 还要膨胀 4/3，这里按 6MB 的安全水位做单个与合计两道闸门。
 */

export type AttachmentKind = 'image' | 'audio' | 'video'

export interface Attachment {
	id: number
	kind: AttachmentKind
	/** 引用名，如「图片1」「音频1」，即 [@图片1] 标记里的那部分 */
	name: string
	/** 原始文件名，用于 tooltip 展示 */
	fileName: string
	/** 完整 data URL（data:<mime>;base64,<载荷>），直接进请求体 */
	dataUrl: string
	/** 本地预览地址（objectURL），仅供 <img>/<video> 展示，用后必须 revoke */
	previewUrl: string
	mimeType: string
	/** data URL 的字节长度，用于体积闸门 */
	bytes: number
}

/** 单个附件上限：图片 5MB（超限先压缩），音视频 4MB（不压缩，超限直接拒绝） */
const MAX_IMAGE_BYTES = 5 * 1024 * 1024
const MAX_MEDIA_BYTES = 4 * 1024 * 1024
/** 所有附件 data URL 合计上限，与后端 8MB body 上限留出余量 */
export const MAX_TOTAL_BYTES = 6 * 1024 * 1024

/** 图片压缩目标：最长边与 JPEG 质量 */
const COMPRESS_MAX_EDGE = 2048
const COMPRESS_QUALITY = 0.85

const KIND_LABEL: Record<AttachmentKind, string> = {
	image: '图片',
	audio: '音频',
	video: '视频',
}

export function formatBytes(bytes: number): string {
	if (bytes < 1024) return `${bytes} B`
	if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
	return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function detectKind(file: File): AttachmentKind | null {
	if (file.type.startsWith('image/')) return 'image'
	if (file.type.startsWith('audio/')) return 'audio'
	if (file.type.startsWith('video/')) return 'video'
	return null
}

function readAsDataUrl(blob: Blob): Promise<string> {
	return new Promise((resolve, reject) => {
		const reader = new FileReader()
		reader.onload = () => resolve(String(reader.result || ''))
		reader.onerror = () => reject(new Error('读取文件失败'))
		reader.readAsDataURL(blob)
	})
}

/**
 * compressImage 把超限图片降采样后重编码为 JPEG。
 * GIF 不压缩（canvas 重编码会丢掉动画），超限直接判失败。
 */
async function compressImage(file: File): Promise<Blob | null> {
	if (file.type === 'image/gif') return null

	const bitmapUrl = URL.createObjectURL(file)
	try {
		const img = await new Promise<HTMLImageElement>((resolve, reject) => {
			const el = new Image()
			el.onload = () => resolve(el)
			el.onerror = () => reject(new Error('图片解码失败'))
			el.src = bitmapUrl
		})

		const scale = Math.min(1, COMPRESS_MAX_EDGE / Math.max(img.width, img.height))
		const canvas = document.createElement('canvas')
		canvas.width = Math.round(img.width * scale)
		canvas.height = Math.round(img.height * scale)
		const ctx = canvas.getContext('2d')
		if (!ctx) return null
		ctx.drawImage(img, 0, 0, canvas.width, canvas.height)

		return await new Promise<Blob | null>(resolve => {
			canvas.toBlob(blob => resolve(blob), 'image/jpeg', COMPRESS_QUALITY)
		})
	} catch {
		return null
	} finally {
		URL.revokeObjectURL(bitmapUrl)
	}
}

export interface UseAttachmentsOptions {
	/**
	 * 允许的附件类型，未列出的类型会被拒绝。
	 * 传 getter 可跟随模型能力动态收放（切到非 vision 模型后就不该再收图片）。
	 */
	kinds: AttachmentKind[] | (() => AttachmentKind[])
	/** 附件数量上限 */
	max?: number
}

export function useAttachments(options: UseAttachmentsOptions) {
	const list = ref<Attachment[]>([])
	const max = options.max ?? 9

	let nextId = 1
	// 各类型独立递增。删除后不重排号——正文里可能已经写了 [@图片2]，
	// 重排会让这些标记悄悄指向另一张图，比留个号码空档危险得多。
	const counters: Record<AttachmentKind, number> = { image: 0, audio: 0, video: 0 }

	const kinds = computed(() =>
		typeof options.kinds === 'function' ? options.kinds() : options.kinds,
	)
	const totalBytes = computed(() => list.value.reduce((sum, a) => sum + a.bytes, 0))
	const accept = computed(() => kinds.value.map(k => `${k}/*`).join(','))

	async function addFiles(files: File[] | FileList) {
		const incoming = Array.from(files)
		for (const file of incoming) {
			if (list.value.length >= max) {
				toast.warning(`最多上传 ${max} 个附件`)
				return
			}
			await addOne(file)
		}
	}

	async function addOne(file: File) {
		const kind = detectKind(file)
		if (!kind || !kinds.value.includes(kind)) {
			toast.error(`不支持的文件类型：${file.name}`)
			return
		}

		let source: Blob = file
		let mimeType = file.type

		if (kind === 'image' && file.size > MAX_IMAGE_BYTES) {
			const compressed = await compressImage(file)
			if (!compressed || compressed.size > MAX_IMAGE_BYTES) {
				toast.error(`${file.name} 超过 ${formatBytes(MAX_IMAGE_BYTES)}，压缩后仍过大`)
				return
			}
			source = compressed
			mimeType = compressed.type
		} else if (kind !== 'image' && file.size > MAX_MEDIA_BYTES) {
			toast.error(`${file.name} 超过 ${formatBytes(MAX_MEDIA_BYTES)}`)
			return
		}

		let dataUrl: string
		try {
			dataUrl = await readAsDataUrl(source)
		} catch {
			toast.error(`${file.name} 读取失败`)
			return
		}

		const bytes = dataUrl.length
		if (totalBytes.value + bytes > MAX_TOTAL_BYTES) {
			toast.error(`附件合计超过 ${formatBytes(MAX_TOTAL_BYTES)}，请先删除部分附件`)
			return
		}

		counters[kind] += 1
		list.value.push({
			id: nextId++,
			kind,
			name: `${KIND_LABEL[kind]}${counters[kind]}`,
			fileName: file.name,
			dataUrl,
			previewUrl: URL.createObjectURL(source),
			mimeType,
			bytes,
		})
	}

	function remove(id: number) {
		const idx = list.value.findIndex(a => a.id === id)
		if (idx === -1) return
		URL.revokeObjectURL(list.value[idx].previewUrl)
		list.value.splice(idx, 1)
	}

	function clear() {
		list.value.forEach(a => URL.revokeObjectURL(a.previewUrl))
		list.value = []
	}

	// Playground 的 tab 由 KeepAlive 缓存，组件不会随切换销毁；
	// 但路由离开时仍会卸载，此处兜底释放 objectURL，避免泄漏。
	onUnmounted(clear)

	return { list, totalBytes, accept, max, addFiles, remove, clear }
}

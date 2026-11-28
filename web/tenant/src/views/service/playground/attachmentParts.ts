import type { Attachment, AttachmentKind } from './useAttachments'

/**
 * 把「正文 + 附件」编译成各端点要的请求形态。
 *
 * 正文里的 [@图片1] 是位置锚点：编译时换成纯文字标签「图片1」留在句子里
 * （多图场景下模型能用名字指代），同时在该位置插入对应的 content part。
 * 没被引用的附件追加到末尾——上传了不打 @ 也要能发出去。
 * 引用了已删除的附件时退化成纯文字，不报错打断输入。
 */

export interface ContentPart {
	type: 'text' | 'image_url' | 'input_audio'
	text?: string
	image_url?: { url: string }
	input_audio?: { data: string; format: string }
}

export interface AttachmentPreview {
	name: string
	kind: AttachmentKind
	/** 直接用 data URL 展示：objectURL 在附件被清空时就 revoke 了，消息气泡不能依赖它 */
	url: string
}

export interface CompiledChat {
	/** 气泡里展示的文本（标记已换成文字标签） */
	displayText: string
	/** 发给 /v1/chat/completions 的 content parts */
	parts: ContentPart[]
	previews: AttachmentPreview[]
}

/** references[] 素材项（/v1/videos 参考素材唯一通道）。url 与 file_id 二选一：
 *  file_id 为 /v1/files 上传后的引用（网关解析为对象存储签名 URL），url 为 base64 内联回退 */
export interface VideoReferenceItem {
	role: 'first_frame' | 'reference_image' | 'reference_video' | 'reference_audio'
	url?: string
	file_id?: string
}

export interface CompiledVideo {
	/** 发给 /v1/videos 的提示词（标记已换成文字标签） */
	prompt: string
	/** 全部素材经 references[] 以 reference_* 参考形式发出（图片不走 first_frame 首帧角色） */
	references: VideoReferenceItem[]
	/** 未随请求发出的素材（视频：上传通道不可用/失败，上游普遍不收 base64） */
	deferred: Attachment[]
}

const MENTION_RE = /\[@([^\]\n]+)\]/g

/** stripMentions 把 [@图片1] 还原成纯文字标签「图片1」 */
export function stripMentions(text: string): string {
	return text.replace(MENTION_RE, (_, name: string) => name)
}

/** hasMention 判断正文是否引用了某个附件 */
export function hasMention(text: string, name: string): boolean {
	return text.includes(`[@${name}]`)
}

/** toMediaPart 把附件转成对应协议的 content part；不支持的类型返回 null */
function toMediaPart(att: Attachment): ContentPart | null {
	if (att.kind === 'image') {
		return { type: 'image_url', image_url: { url: att.dataUrl } }
	}
	if (att.kind === 'audio') {
		// OpenAI input_audio 要的是裸 base64 + 格式名，不是完整 data URL
		const base64 = att.dataUrl.slice(att.dataUrl.indexOf('base64,') + 'base64,'.length)
		const format = att.mimeType.split('/')[1]?.split(';')[0] || 'mp3'
		return { type: 'input_audio', input_audio: { data: base64, format } }
	}
	return null
}

export function compileChatParts(text: string, attachments: Attachment[]): CompiledChat {
	const byName = new Map(attachments.map(a => [a.name, a]))
	const referenced = new Set<number>()

	const parts: ContentPart[] = []
	let buf = ''

	const flush = () => {
		if (buf.trim()) parts.push({ type: 'text', text: buf })
		buf = ''
	}

	let cursor = 0
	MENTION_RE.lastIndex = 0
	for (let m = MENTION_RE.exec(text); m !== null; m = MENTION_RE.exec(text)) {
		buf += text.slice(cursor, m.index) + m[1]
		cursor = m.index + m[0].length

		const att = byName.get(m[1])
		if (!att) continue // 失效标记：只留文字，继续累积

		const part = toMediaPart(att)
		if (!part) continue
		flush()
		parts.push(part)
		referenced.add(att.id)
	}
	buf += text.slice(cursor)
	flush()

	for (const att of attachments) {
		if (referenced.has(att.id)) continue
		const part = toMediaPart(att)
		if (part) parts.push(part)
	}

	return {
		displayText: stripMentions(text),
		parts,
		previews: attachments.map(a => ({ name: a.name, kind: a.kind, url: a.dataUrl })),
	}
}

/**
 * compileVideoPrompt 把「正文 + 附件」编译成 /v1/videos 请求载荷。
 *
 * 参考素材统一经 references[] 发出，图片一律走参考模式 role reference_image —— 不做
 * 「单图自动转首帧 first_frame」：首帧与参考图是上游两种不同语义（MiniMax H3 / grsai 等
 * 上游的首尾帧必须成对提供且与参考图互斥，单图转首帧会触发其成对校验报错），用户上传的
 * 就是参考图，应保持参考语义。仅支持首帧图生视频的模型会收到明确 400，属预期行为。
 *
 * 素材引用形态（fileIds 参数）：
 * - 经 /v1/files 上传成功的附件以 file_id 引用（网关解析为对象存储签名 URL 发上游）；
 * - 未上传/失败的图片与音频回退 base64 data URL 内联；
 * - 视频素材上游普遍不收 base64（Seedance 明确拒绝）：无 file_id 时归入 deferred 提示。
 */
export function compileVideoPrompt(
	text: string,
	attachments: Attachment[],
	fileIds?: Map<number, string>,
): CompiledVideo {
	const images = attachments.filter(a => a.kind === 'image')
	const audios = attachments.filter(a => a.kind === 'audio')
	const videos = attachments.filter(a => a.kind === 'video')
	const sentVideos = videos.filter(v => fileIds?.has(v.id))
	const deferred = videos.filter(v => !fileIds?.has(v.id))

	// 引用形态：有 file_id 用上传引用，否则 base64 内联（仅图片/音频可内联）
	const refOf = (att: Attachment): Pick<VideoReferenceItem, 'url' | 'file_id'> => {
		const fileId = fileIds?.get(att.id)
		return fileId ? { file_id: fileId } : { url: att.dataUrl }
	}

	const references: VideoReferenceItem[] = []
	for (const img of images) references.push({ role: 'reference_image', ...refOf(img) })
	for (const au of audios) references.push({ role: 'reference_audio', ...refOf(au) })
	for (const vd of sentVideos) references.push({ role: 'reference_video', ...refOf(vd) })
	return { prompt: stripMentions(text), references, deferred }
}

export interface CompiledImageReferences {
	/** 参考图 data URL 列表（按上传顺序）；图片 Tab 只收图片附件，此处恒为纯图片 */
	images: string[]
}

/**
 * compileImageReferences 把图片附件编译成 /v1/images/generations 的参考图载荷。
 * 图片生成没有 [@图片1] 位置语义（参考图是整单生效的素材而非句内锚点），
 * 因此不做标记解析，全部图片附件按序发出。
 */
export function compileImageReferences(attachments: Attachment[]): CompiledImageReferences {
	return {
		images: attachments.filter(a => a.kind === 'image').map(a => a.dataUrl),
	}
}

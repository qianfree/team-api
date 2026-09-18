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

export interface CompiledVideo {
	/** 发给 /v1/videos 的提示词（标记已换成文字标签） */
	prompt: string
	/** 首帧参考图的 data URL，无图时为空串 */
	inputReference: string
	/** 本期协议携带不了、未随请求发出的素材 */
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

export function compileVideoPrompt(text: string, attachments: Attachment[]): CompiledVideo {
	// /v1/videos 的 input_reference 只能带一张图，其余素材本期发不出去
	const firstImage = attachments.find(a => a.kind === 'image')
	return {
		prompt: stripMentions(text),
		inputReference: firstImage?.dataUrl || '',
		deferred: attachments.filter(a => a.id !== firstImage?.id),
	}
}

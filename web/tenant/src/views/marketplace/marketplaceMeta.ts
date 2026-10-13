import type { MarketplaceModel, TimePriceItem } from '@/api/marketplace'
import { formatBilling } from '@/composables/useCurrency'

// 分类元数据：卡片磁贴渐变、筛选 pills 与各类徽章共用同一份配置
export interface CategoryMeta {
	key: string
	// null 表示「全部」，请求时不传 category
	value: string | null
	// 完整名称：卡片类别徽章 / 抽屉 / 规格行
	label: string
	// 两个字：筛选 pills 用
	shortLabel: string
	// hover 提示
	description: string
	// Icon.vue 内置图标名
	icon: string
	// 渐变起止色
	from: string
	to: string
	// 发光阴影 RGB（"r, g, b"），供 rgba() 拼接 box-shadow
	glow: string
}

// 色值与全局 .icon-tile-* 系列保持一致，chat 占据主色 teal
export const categoryMetaList: CategoryMeta[] = [
	{ key: 'all', value: null, label: '全部模型', shortLabel: '全部', description: '浏览完整模型目录', icon: 'grid', from: '#2dd4bf', to: '#0d9488', glow: '20, 184, 166' },
	{ key: 'chat', value: 'chat', label: '对话与推理', shortLabel: '对话', description: '文本生成、推理与工具调用', icon: 'chat', from: '#2dd4bf', to: '#0d9488', glow: '20, 184, 166' },
	{ key: 'embedding', value: 'embedding', label: '向量嵌入', shortLabel: '向量', description: '语义检索与知识库构建', icon: 'cube', from: '#a78bfa', to: '#8b5cf6', glow: '139, 92, 246' },
	{ key: 'image', value: 'image', label: '图像生成', shortLabel: '图像', description: '文生图、图像理解与编辑', icon: 'eye', from: '#f472b6', to: '#ec4899', glow: '236, 72, 153' },
	{ key: 'audio', value: 'audio', label: '音频处理', shortLabel: '音频', description: '语音识别、合成与音乐生成', icon: 'bell', from: '#fbbf24', to: '#f59e0b', glow: '245, 158, 11' },
	{ key: 'video', value: 'video', label: '视频生成', shortLabel: '视频', description: '文生视频与视频处理', icon: 'film', from: '#fb7185', to: '#f43f5e', glow: '244, 63, 94' },
	{ key: 'rerank', value: 'rerank', label: '重排序', shortLabel: '重排', description: '检索结果相关性排序', icon: 'trendingUp', from: '#22d3ee', to: '#06b6d4', glow: '6, 182, 212' },
]

// 未知分类的渲染兜底：仅用于展示，绝不进入筛选 pills
export const fallbackCategoryMeta: CategoryMeta = {
	key: 'other', value: null, label: '其他模型', shortLabel: '其他', description: '其他模型能力', icon: 'bolt', from: '#d1d5db', to: '#9ca3af', glow: '156, 163, 175',
}

export function getCategoryMeta(category?: string | null): CategoryMeta {
	if (!category) return categoryMetaList[0]
	return categoryMetaList.find(item => item.value === category) || fallbackCategoryMeta
}

// 能力中文标签：合并控制台模型页的映射，缺失的 key 回退为可读文本
const capabilityLabels: Record<string, string> = {
	streaming: '流式输出',
	function_calling: '工具调用',
	tool_calling: '工具调用',
	parallel_function_calling: '并行工具调用',
	tool_choice: '工具选择',
	vision: '视觉理解',
	multimodal: '多模态',
	json_mode: 'JSON 模式',
	response_schema: '结构化输出',
	system_messages: '系统消息',
	reasoning: '深度思考',
	web_search: '联网搜索',
	image_generation: '图像生成',
	prompt_caching: '提示词缓存',
	audio: '音频能力',
	audio_input: '音频输入',
	audio_output: '音频输出',
	pdf_input: 'PDF 输入',
	embedding_image: '图像嵌入',
}

// capabilities 是 { key: boolean } 形态，只取为真的项并转成中文标签
export function getCapabilityList(model: MarketplaceModel, limit = 4): string[] {
	if (!model.capabilities || typeof model.capabilities !== 'object') return []
	return Object.entries(model.capabilities)
		.filter(([, value]) => value === true || value === 1 || value === 'true')
		.map(([key]) => capabilityLabels[key] || key.replaceAll('_', ' '))
		.filter((item, index, list) => list.indexOf(item) === index)
		.slice(0, limit)
}

export function formatTokens(tokens?: number | null): string {
	const value = Number(tokens || 0)
	if (!value) return '—'
	if (value >= 1_000_000) return `${Number((value / 1_000_000).toFixed(1))}M`
	if (value >= 1_000) return `${Number((value / 1_000).toFixed(1))}K`
	return value.toLocaleString('en-US')
}

export function formatPrice(price?: number | null): string {
	const value = Number(price)
	if (!Number.isFinite(value)) return '—'
	// 本位币直显：最多保留 6 位小数（与后端 NUMERIC(20,10) 精度对齐），自动去掉末尾的 0
	return formatBilling(value, 6)
}

export function billingModeLabel(mode?: string | null): string {
	if (mode === 'per_request') return '按次计费'
	if (mode === 'tiered') return '阶梯计费'
	if (mode === 'per_second') return '按秒计费'
	if (mode === 'special') return '特殊计费'
	return '按量计费'
}

// ── 时段价目展示辅助（价格为后端换算好的展示价，前端直接渲染） ──

function daysLabel(days?: number[] | null): string {
	if (!days || days.length === 0) return '每天'
	const sorted = [...days].sort((a, b) => a - b)
	if (sorted.join(',') === '1,2,3,4,5') return '工作日'
	if (sorted.join(',') === '6,7') return '周末'
	const names = ['', '周一', '周二', '周三', '周四', '周五', '周六', '周日']
	return sorted.map((d) => names[d] || String(d)).join('、')
}

export function timeWindow(tp: TimePriceItem): string {
	const parts: string[] = [daysLabel(tp.days)]
	if (tp.start_time && tp.end_time) parts.push(`${tp.start_time}~${tp.end_time}`)
	else parts.push('全天')
	if (tp.valid_from) parts.push(tp.valid_to ? `${tp.valid_from}~${tp.valid_to}` : `${tp.valid_from} 起`)
	return parts.join(' · ')
}

export function timePriceText(model: MarketplaceModel, tp: TimePriceItem): string {
	if (model.billing_mode === 'per_request') return `${formatPrice(tp.per_request_price)} /次`
	// per_second / special：时段价后端按每秒基准价换算，per_second_price 字段下发
	if (model.billing_mode === 'per_second' || model.billing_mode === 'special') {
		return `${formatPrice(tp.per_second_price)} /秒`
	}
	return `输入 ${formatPrice(tp.input_price)} · 输出 ${formatPrice(tp.output_price)}${model.billing_mode === 'tiered' ? ' 起' : ''}`
}

// ── 模型卡片：按品类/计费模式自适应（价格锚是全卡唯一大号元素） ──

export interface CardPriceSlot {
	label: string
	value: string
	/** 单位附注（11px 灰） */
	unit: string
	primary: boolean
}

export interface CardView {
	price: {
		slots: CardPriceSlot[]
		/** tiered=首档价、per_second=多规格 → 角标「起」 */
		fromNote: boolean
		/** 配置了分时段价 → 价格区角落显示时钟提示 */
		hasTimePrices: boolean
	}
	/** 规格行片段（等宽值），如 [上下文 400K · 最大输出 64K] / [480p · 720p] */
	specSegments: { label: string; value: string }[]
}

// per_second 矩阵展示档摘要：取非 "*" 的规格键（字典序），最多两档
function perSecondSpecs(prices: Record<string, number> | null | undefined): string[] {
	if (!prices) return []
	return Object.keys(prices)
		.filter(k => k !== '*')
		.sort()
		.slice(0, 2)
}

// per_second 价格锚：兜底档 "*" 优先，无则取最低档（多档时 from=true）
function perSecondAnchor(prices: Record<string, number> | null | undefined): { value: number; from: boolean } {
	if (!prices) return { value: 0, from: false }
	if (prices['*'] != null && prices['*'] > 0) return { value: prices['*'], from: false }
	const positive = Object.values(prices).filter(v => v > 0)
	if (!positive.length) return { value: 0, from: false }
	return { value: Math.min(...positive), from: Object.keys(prices).length > 1 }
}

export function getCardView(model: MarketplaceModel): CardView {
	const mode = model.billing_mode || 'token'
	const hasTimePrices = !!model.time_prices?.length

	// 按次计费（图像等）：单价锚 + tags 作规格行
	if (mode === 'per_request') {
		return {
			price: {
				slots: [{ label: '按次计费', value: formatPrice(model.per_request_price), unit: '/ 次', primary: true }],
				fromNote: false,
				hasTimePrices,
			},
			specSegments: (model.tags || []).slice(0, 2).map(t => ({ label: t, value: '' })),
		}
	}

	// 按秒计费（视频等）：每秒价锚 + 规格档摘要
	if (mode === 'per_second' || mode === 'special') {
		const anchor = perSecondAnchor(model.per_second_prices)
		return {
			price: {
				slots: [{ label: '按秒计费', value: formatPrice(anchor.value), unit: '/ 秒', primary: true }],
				fromNote: anchor.from,
				hasTimePrices,
			},
			specSegments: perSecondSpecs(model.per_second_prices).map(s => ({ label: s, value: '' })),
		}
	}

	// token / tiered：输入+输出双价锚（向量/重排类只显示输入价）
	const isEmbeddingLike = model.category === 'embedding' || model.category === 'rerank'
	const slots: CardPriceSlot[] = [
		{ label: '输入', value: formatPrice(model.input_price), unit: '/1M tokens', primary: true },
	]
	if (!isEmbeddingLike) {
		slots.push({ label: '输出', value: formatPrice(model.output_price), unit: '/1M tokens', primary: false })
	}

	// 规格行：chat 类给上下文/输出硬参数；无硬参数时 tags 兜底
	const specSegments: { label: string; value: string }[] = []
	if (model.max_context_tokens) specSegments.push({ label: '上下文', value: formatTokens(model.max_context_tokens) })
	if (model.max_output_tokens && !isEmbeddingLike) specSegments.push({ label: '输出', value: formatTokens(model.max_output_tokens) })
	if (!specSegments.length) {
		for (const tag of (model.tags || []).slice(0, 2)) specSegments.push({ label: tag, value: '' })
	}

	return {
		price: { slots, fromNote: mode === 'tiered', hasTimePrices },
		specSegments,
	}
}

// ── 卡片状态徽章：至多一枚，优先级 折扣 > NEW(运营标记) > 推荐 ──

export type CardBadgeType = 'deal' | 'new' | 'rec'

export function getCardBadge(model: MarketplaceModel): { type: CardBadgeType; text: string } | null {
	if (model.discount_label) return { type: 'deal', text: model.discount_label }
	if (model.is_new) return { type: 'new', text: 'NEW' }
	if (model.is_recommended) return { type: 'rec', text: '推荐' }
	return null
}

// ── 筛选/排序常量 ──

export const marketplaceSortOptions = [
	{ value: '', label: '默认排序' },
	{ value: 'newest', label: '最新上架' },
	{ value: 'price_asc', label: '价格低到高' },
	{ value: 'price_desc', label: '价格高到低' },
	{ value: 'context_desc', label: '上下文最长' },
]

export const marketplaceBillingOptions = [
	{ value: '', label: '全部' },
	{ value: 'token', label: '按量计费' },
	{ value: 'per_request', label: '按次计费' },
	{ value: 'per_second', label: '按秒计费' },
	{ value: 'tiered', label: '阶梯计费' },
]

/** 能力键 → 筛选选项标签（键即后端 capabilities JSONB 键） */
export function capabilityOptionLabel(key: string): string {
	return capabilityLabels[key] || key.replaceAll('_', ' ')
}

// ── 上架时间展示 ──

/** 相对时间：7 天内「N 天前」，更早显示日期（新上线条与详情用） */
export function relativeCreatedAt(dateStr?: string | null): string {
	if (!dateStr) return ''
	const date = new Date(`${dateStr}T00:00:00`)
	if (Number.isNaN(date.getTime())) return dateStr
	const days = Math.floor((Date.now() - date.getTime()) / 86_400_000)
	if (days <= 0) return '今天'
	if (days < 7) return `${days} 天前`
	if (days < 30) return `${Math.floor(days / 7)} 周前`
	return date.toLocaleDateString('zh-CN', { month: 'long', day: 'numeric' })
}

// ── 快速接入代码块（详情抽屉） ──

export interface QuickStartSnippet {
	key: 'curl' | 'python'
	label: string
	code: string
}

/** 接入示例：baseUrl 为当前部署的网关地址（window.location.origin + /v1） */
export function quickStartSnippets(modelId: string, baseUrl: string): QuickStartSnippet[] {
	return [
		{
			key: 'curl',
			label: 'cURL',
			code: [
				`curl ${baseUrl}/chat/completions \\`,
				`  -H "Authorization: Bearer $API_KEY" \\`,
				`  -H "Content-Type: application/json" \\`,
				`  -d '{`,
				`    "model": "${modelId}",`,
				`    "messages": [{"role": "user", "content": "Hello"}]`,
				`  }'`,
			].join('\n'),
		},
		{
			key: 'python',
			label: 'Python',
			code: [
				`from openai import OpenAI`,
				``,
				`client = OpenAI(`,
				`    base_url="${baseUrl}",`,
				`    api_key="$API_KEY",`,
				`)`,
				``,
				`resp = client.chat.completions.create(`,
				`    model="${modelId}",`,
				`    messages=[{"role": "user", "content": "Hello"}],`,
				`)`,
				`print(resp.choices[0].message.content)`,
			].join('\n'),
		},
	]
}

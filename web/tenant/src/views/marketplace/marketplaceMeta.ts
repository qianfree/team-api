import type { MarketplaceModel, TimePriceItem } from '@/api/marketplace'

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
	// 最多保留 6 位小数（与后端 NUMERIC(20,10) 精度对齐），自动去掉末尾的 0
	return `$${Number(value.toFixed(6))}`
}

export function billingModeLabel(mode?: string | null): string {
	if (mode === 'per_request') return '按次计费'
	if (mode === 'tiered') return '阶梯计费'
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
	return `输入 ${formatPrice(tp.input_price)} · 输出 ${formatPrice(tp.output_price)}${model.billing_mode === 'tiered' ? ' 起' : ''}`
}

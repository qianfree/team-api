// 厂商元数据：与后端 mdl_models.vendor 枚举对齐（api/admin/v1/model.go、api/tenant/v1/marketplace.go
// 的 v:"in:" 列表，以及 internal/logic/admin/model.go 的 validModelVendors）。
// 新增厂商时需同步：后端枚举校验、本文件、web/admin/src/constants/vendor.ts，并补 src/assets/vendors/{slug}.svg。
// 注意与渠道供应商（chn_channels.type）是两个维度：厂商 = 模型的研发公司。

export interface VendorMeta {
	// 枚举值（入库值）
	value: string
	// 显示名
	label: string
	// 品牌色（logo 芯片底色）
	color: string
	// logo 本身是多色图形（如 cohere 三色 mark），芯片用浅色底展示彩色 logo
	colored?: boolean
}

export const vendorMetaList: VendorMeta[] = [
	{ value: 'openai', label: 'OpenAI', color: '#10a37f' },
	{ value: 'anthropic', label: 'Anthropic', color: '#d97757' },
	{ value: 'google', label: 'Google', color: '#4285f4' },
	{ value: 'xai', label: 'xAI', color: '#000000' },
	{ value: 'mistral', label: 'Mistral', color: '#fa520f' },
	{ value: 'cohere', label: 'Cohere', color: '#39594d', colored: true },
	{ value: 'meta', label: 'Meta', color: '#0866ff' },
	{ value: 'alibaba', label: '阿里巴巴', color: '#615ced' }, // 图标用通义千问 mark，底色取千问品牌紫
	{ value: 'bytedance', label: '字节跳动', color: '#325ab4' },
	{ value: 'deepseek', label: '深度求索', color: '#4d6bfe' },
	{ value: 'zhipu', label: '智谱AI', color: '#3e5cfa' },
	{ value: 'moonshot', label: '月之暗面', color: '#16191e' },
	{ value: 'minimax', label: 'MiniMax', color: '#1f1f1f' },
	{ value: 'baidu', label: '百度', color: '#2932e1' },
	{ value: 'tencent', label: '腾讯', color: '#0052d9' },
	{ value: 'xunfei', label: '科大讯飞', color: '#0f4cfd' },
	{ value: 'kuaishou', label: '快手', color: '#ff5000' },
	{ value: 'midjourney', label: 'Midjourney', color: '#111111' },
	{ value: 'suno', label: 'Suno', color: '#111111' },
]

// 未收录厂商（vendor 非空但字典缺失）与未设置厂商（空串/null）的展示兜底
export function getVendorMeta(vendor?: string | null): VendorMeta | null {
	if (!vendor) return null
	return vendorMetaList.find(item => item.value === vendor) ?? null
}

// 厂商显示名：字典命中取 label，未收录回退原值，未设置返回「未分类」
export function vendorLabel(vendor?: string | null): string {
	if (!vendor) return '未分类'
	return getVendorMeta(vendor)?.label ?? vendor
}

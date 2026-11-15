// 渠道供应商类型共享常量（管理后台渠道列表/详情/创建共用）
// value 与后端 chn_channels.type 数值一一对应；新增供应商时在此统一维护。
// 注意：后端 internal/logic/admin/channel.go 的 providerTypeNames 仅覆盖部分类型，
// 选到后端缺失的类型时详情页 type_name 会显示 Unknown(xx)，属既有展示问题。

export const providerTypeOptions: { label: string; value: number }[] = [
  { label: 'OpenAI', value: 1 }, { label: 'Claude (Anthropic)', value: 2 },
  { label: 'Gemini (Google)', value: 3 }, { label: 'Ali (百炼)', value: 4 },
  { label: 'Tencent (混元)', value: 6 },
  { label: 'Zhipu (智谱)', value: 7 }, { label: 'DeepSeek', value: 8 },
  { label: 'Moonshot', value: 9 }, { label: 'Volcengine (火山)', value: 10 },
  { label: 'AWS Bedrock', value: 11 }, { label: 'Azure OpenAI', value: 12 },
  { label: 'Vertex AI', value: 13 },
  { label: 'Mistral', value: 15 }, { label: 'xAI (Grok)', value: 16 },
  { label: 'Lingyi (零一万物)', value: 18 },
  { label: 'Baidu V2', value: 19 }, { label: 'Cloudflare Workers AI', value: 20 },
  { label: 'Ollama', value: 22 },
  { label: 'SiliconFlow (硅基流动)', value: 25 }, { label: 'Xunfei (讯飞)', value: 26 },
  { label: 'OpenRouter', value: 27 }, { label: 'XInference', value: 28 },
  { label: 'MiniMax', value: 29 }, { label: 'Submodel', value: 30 },
  // 32 = Coze、33 = Dify：渠道已移除，编号永久保留不复用
  { label: 'Jimeng (即梦)', value: 34 },
  { label: 'Codex', value: 35 },
  { label: 'New API', value: 41 }, { label: 'Sub2API', value: 42 },
  { label: 'JS 任务插件', value: 43 },
]

// value -> label 映射，便于表格/详情按数值取展示名
export const providerTypeName: Record<number, string> = {}
providerTypeOptions.forEach(o => { providerTypeName[o.value] = o.label })

// 供应商分类（创建渠道表单顶部的模式切换胶囊），三类划分：
// 三大协议 = OpenAI / Claude / Gemini 三家基础协议；插件 = JS 任务插件；
// 模型厂商 = 其余全部内置厂商，由「全量 - 三大协议 - 插件」自动推导，新增供应商无需登记归类。
const basicProviderTypes = [1, 2, 3]
const pluginProviderTypes = [43]
export const providerCategoryMap: Record<string, { label: string; types: number[] }> = {
  basic: { label: '三大协议', types: basicProviderTypes },
  builtin: {
    label: '模型厂商',
    types: providerTypeOptions.map(o => o.value).filter(v => !basicProviderTypes.includes(v) && !pluginProviderTypes.includes(v)),
  },
  plugin: { label: '插件', types: pluginProviderTypes },
}

// 按分类过滤供应商类型选项（未知分类返回全量）
export function filterTypeOptionsByCategory(category: string) {
  const cat = providerCategoryMap[category]
  if (!cat) return providerTypeOptions
  const allow = new Set(cat.types)
  return providerTypeOptions.filter(o => allow.has(o.value))
}

// base_url 填写形态：后端转发前会把地址归一化成「站点根」（adaptor 自拼 /v1/... 等版本段+端点）。
// ''(自动)=按供应商已知版本段自动剥离（推荐，覆盖粘贴官方文档地址的场景）；
// root=填的是域名/站点根；versioned=填的地址已含版本前缀（如 /v1、/v1beta、/api/paas/v4）
export const baseURLTypeOptions: { label: string; value: string }[] = [
  { label: '自动', value: '' },
  { label: '域名', value: 'root' },
  { label: '带版本前缀', value: 'versioned' },
]

// Arco Select 的 allow-search 自定义过滤：小写包含匹配
export function filterProviderOption(inputValue: string, option: { label: string; value: number }) {
  const input = inputValue.toLowerCase()
  return option.label.toLowerCase().includes(input)
}

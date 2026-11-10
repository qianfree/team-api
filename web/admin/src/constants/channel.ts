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

// 供应商分类（创建渠道表单顶部的模式切换胶囊）：
// 每个类型恰好归属一类；all 为空数组表示不过滤。新增供应商时在此登记归类。
export const providerCategoryMap: Record<string, { label: string; types: number[] }> = {
  all: { label: '全部', types: [] },
  overseas: { label: '海外模型', types: [1, 2, 3, 15, 16, 35] },
  china: { label: '国产大模型', types: [4, 6, 7, 8, 9, 10, 18, 19, 26, 29] },
  cloud: { label: '云平台', types: [11, 12, 13, 20] },
  aggregator: { label: '聚合与自部署', types: [22, 25, 27, 28, 30, 41, 42] },
  media: { label: '媒体生成', types: [34] },
  jsplugin: { label: 'JS 插件', types: [43] },
}

// 按分类过滤供应商类型选项（all / 未知分类返回全量）
export function filterTypeOptionsByCategory(category: string) {
  const cat = providerCategoryMap[category]
  if (!cat || cat.types.length === 0) return providerTypeOptions
  const allow = new Set(cat.types)
  return providerTypeOptions.filter(o => allow.has(o.value))
}

// Arco Select 的 allow-search 自定义过滤：小写包含匹配
export function filterProviderOption(inputValue: string, option: { label: string; value: number }) {
  const input = inputValue.toLowerCase()
  return option.label.toLowerCase().includes(input)
}

// 模型研发厂商共享常量（管理后台模型管理列表/筛选/表单共用）
// value 与后端 mdl_models.vendor 枚举一一对应（api/admin/v1/model.go 的 v:"in:" 校验、
// internal/logic/admin/model.go 的 validModelVendors）。
// 新增厂商时需同步：后端枚举校验、租户端 web/tenant/src/constants/vendor.ts、本文件。
// 注意与渠道供应商类型（constants/channel.ts，chn_channels.type）是两个维度：厂商 = 模型的研发公司，
// 例如 Bedrock 上的 Claude 厂商仍是 anthropic。

export const vendorOptions: { label: string; value: string }[] = [
  { label: 'OpenAI', value: 'openai' },
  { label: 'Anthropic', value: 'anthropic' },
  { label: 'Google', value: 'google' },
  { label: 'xAI', value: 'xai' },
  { label: 'Mistral', value: 'mistral' },
  { label: 'Cohere', value: 'cohere' },
  { label: 'Meta', value: 'meta' },
  { label: '阿里巴巴', value: 'alibaba' },
  { label: '字节跳动', value: 'bytedance' },
  { label: '深度求索', value: 'deepseek' },
  { label: '智谱AI', value: 'zhipu' },
  { label: '月之暗面', value: 'moonshot' },
  { label: 'MiniMax', value: 'minimax' },
  { label: '百度', value: 'baidu' },
  { label: '腾讯', value: 'tencent' },
  { label: '科大讯飞', value: 'xunfei' },
  { label: '快手', value: 'kuaishou' },
  { label: 'Midjourney', value: 'midjourney' },
  { label: 'Suno', value: 'suno' },
]

// value -> label 映射，表格列展示用
export const vendorLabelMap: Record<string, string> = {}
vendorOptions.forEach(o => { vendorLabelMap[o.value] = o.label })

// value -> Arco Tag 预设色映射（淡底深字，取接近品牌色的预设）
export const vendorTagColor: Record<string, string> = {
  openai: 'green',
  anthropic: 'orange',
  google: 'arcoblue',
  xai: 'gray',
  mistral: 'orangered',
  cohere: 'cyan',
  meta: 'blue',
  alibaba: 'purple', // 图标用通义千问 mark（品牌紫），Tag 色随之取紫
  bytedance: 'arcoblue',
  deepseek: 'blue',
  zhipu: 'purple',
  moonshot: 'gray',
  minimax: 'red',
  baidu: 'blue',
  tencent: 'cyan',
  xunfei: 'arcoblue',
  kuaishou: 'orangered',
  midjourney: 'gray',
  suno: 'purple',
}

// Arco Select 的 allow-search 自定义过滤：小写包含匹配
export function filterVendorOption(inputValue: string, option: { label: string; value: string }) {
  const input = inputValue.toLowerCase()
  return option.label.toLowerCase().includes(input)
}

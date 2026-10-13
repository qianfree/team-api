import request from '@/utils/request'

export interface TimePriceItem {
  name: string
  days?: number[] | null
  start_time?: string
  end_time?: string
  valid_from?: string
  valid_to?: string
  multiplier: number
  input_price?: number | null
  output_price?: number | null
  per_request_price?: number | null
  /** 按秒/特殊计费的时段换算价（每秒基准价 × 时段乘数） */
  per_second_price?: number | null
}

export interface MarketplaceModel {
  model_id: string
  model_name: string
  category: string
  /** 研发厂商枚举（openai/anthropic/alibaba 等，空串=未分类） */
  vendor?: string | null
  description: string
  max_context_tokens: number
  max_output_tokens: number
  billing_mode?: string
  input_price: number
  output_price: number
  per_request_price?: number
  cache_read_price?: number
  cache_creation_price?: number
  /** 按秒/特殊计费的输出每秒单价矩阵（规格→单价，"*" 为兜底档） */
  per_second_prices?: Record<string, number> | null
  discount_label?: string | null
  price_change_note?: string | null
  time_prices?: TimePriceItem[] | null
  tags: string[]
  capabilities: Record<string, any>
  /** 推荐位（运营手动勾选） */
  is_recommended?: boolean
  /** 上新位（运营手动勾选） */
  is_new?: boolean
  /** 上架日期 YYYY-MM-DD（新上线条排序与详情展示用） */
  created_at?: string
}

/** 析取分面计数：每个维度在「应用其余维度筛选后」的剩余集合内计数，0=该选项点不出结果 */
export interface MarketplaceFacets {
  categories: Record<string, number>
  vendors: Record<string, number>
  capabilities: Record<string, number>
  billing_modes: Record<string, number>
}

export interface MarketplaceListParams {
  keyword?: string
  category?: string
  categories?: string
  vendor?: string
  vendors?: string
  capabilities?: string
  billing_mode?: string
  sort?: string
  page: number
  page_size: number
}

export interface MarketplaceListResponse {
  list: MarketplaceModel[]
  total: number
  page: number
  page_size: number
  /** 当前条件下有模型的厂商去重列表（facets 不可用时的降级渲染数据源） */
  vendors?: string[] | null
  /** 析取分面计数（目录超阈值时各维度为空对象，前端隐藏计数） */
  facets?: MarketplaceFacets | null
}

export interface MarketplaceShowcaseResponse {
  recommended: MarketplaceModel[]
  newest: MarketplaceModel[]
}

export const getMarketplaceModels = (params: MarketplaceListParams) => {
  return request.get<{ data: MarketplaceListResponse }>('/tenant/marketplace/models', { params }).then(res => res.data.data)
}

export const getMarketplaceModelDetail = (modelId: string) => {
  return request.get<{ data: MarketplaceModel }>(`/tenant/marketplace/models/${modelId}`).then(res => res.data.data)
}

export const getMarketplaceShowcase = () => {
  return request.get<{ data: MarketplaceShowcaseResponse }>('/tenant/marketplace/showcase').then(res => res.data.data)
}

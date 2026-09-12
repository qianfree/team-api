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
  discount_label?: string | null
  price_change_note?: string | null
  time_prices?: TimePriceItem[] | null
  tags: string[]
  capabilities: Record<string, any>
}

export interface MarketplaceListParams {
  keyword?: string
  category?: string
  vendor?: string
  page: number
  page_size: number
}

export interface MarketplaceListResponse {
  list: MarketplaceModel[]
  total: number
  page: number
  page_size: number
  /** 当前条件下有模型的厂商去重列表（前端只渲染这些厂商筛选项） */
  vendors?: string[] | null
}

export const getMarketplaceModels = (params: MarketplaceListParams) => {
  return request.get<{ data: MarketplaceListResponse }>('/tenant/marketplace/models', { params }).then(res => res.data.data)
}

export const getMarketplaceModelDetail = (modelId: string) => {
  return request.get<{ data: MarketplaceModel }>(`/tenant/marketplace/models/${modelId}`).then(res => res.data.data)
}

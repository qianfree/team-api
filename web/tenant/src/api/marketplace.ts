import request from '@/utils/request'

export interface MarketplaceModel {
  model_id: string
  model_name: string
  category: string
  description: string
  max_context_tokens: number
  max_output_tokens: number
  input_price: number
  output_price: number
  tags: string[]
  capabilities: Record<string, any>
}

export interface MarketplaceListParams {
  keyword?: string
  category?: string
  page: number
  page_size: number
}

export interface MarketplaceListResponse {
  list: MarketplaceModel[]
  total: number
  page: number
  page_size: number
}

export const getMarketplaceModels = (params: MarketplaceListParams) => {
  return request.get<{ data: MarketplaceListResponse }>('/tenant/marketplace/models', { params }).then(res => res.data.data)
}

export const getMarketplaceModelDetail = (modelId: string) => {
  return request.get<{ data: MarketplaceModel }>(`/tenant/marketplace/models/${modelId}`).then(res => res.data.data)
}

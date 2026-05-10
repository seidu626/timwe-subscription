export interface AdminProduct {
  id: number;
  product_id: string;
  name: string;
  price_point_id: number;
  price_point_value: number;
  short_code: string;
  created_at: string;
}

export interface ProductListResponse {
  products: AdminProduct[];
  total_count: number;
  page: number;
  page_size: number;
}

export interface ProductFilters {
  page?: number;
  page_size?: number;
  q?: string;
  short_code?: string;
}

export interface ProductMutationPayload {
  product_id: string;
  name: string;
  price_point_id: number;
  price_point_value: number;
  short_code: string;
  performed_by?: string;
}

export interface ProductBatchPayload {
  products: ProductMutationPayload[];
  performed_by?: string;
}

export interface ProductDependencyCounts {
  campaign_count: number;
  subscription_count: number;
}

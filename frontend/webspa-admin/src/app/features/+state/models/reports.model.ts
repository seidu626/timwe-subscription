// reports.model.ts

export interface ReportFilters {
  start_date: string;
  end_date: string;
  campaign_slug?: string;
  country?: string;
}

export interface KPIsResponse {
  filters: ReportFilters;
  landing_views: number;
  landing_clicks: number;
  transactions: number;
  subscribed: number;
  charged: number;
  estimated_revenue: number;
  view_to_click_rate: number;
  click_to_transaction_rate: number;
  transaction_to_sub_rate: number;
  sub_to_charged_rate: number;
  overall_conversion_rate: number;
}

export interface FunnelStage {
  name: string;
  count: number;
  dropoff_percent: number;
}

export interface AcquisitionFunnelResponse {
  filters: ReportFilters;
  stages: FunnelStage[];
}

export interface CampaignPerformance {
  campaign_slug: string;
  country: string;
  landing_views: number;
  transactions: number;
  subscribed: number;
  charged: number;
  estimated_revenue: number;
  conversion_rate: number;
}

export interface CampaignPerformanceResponse {
  filters: ReportFilters;
  campaigns: CampaignPerformance[] | null;
}

export interface TimeSeriesPoint {
  timestamp: string;
  landing_views: number;
  transactions: number;
  subscribed: number;
  charged: number;
  estimated_revenue: number;
}

export interface TimeSeriesResponse {
  filters: ReportFilters;
  interval: string;
  data_points: TimeSeriesPoint[];
}

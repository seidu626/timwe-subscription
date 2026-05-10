import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { ChartData, ChartOptions } from 'chart.js';
import {
  KPIsResponse,
  AcquisitionFunnelResponse,
  CampaignPerformanceResponse,
  TimeSeriesResponse,
  CampaignPerformance
} from '../+state/models/reports.model';
import { ReportsApiService } from '../+state/services/reports-api.service';
import { CampaignService } from '../+state/services/campaign.service';
import { Campaign } from '../+state/models/campaign.model';
import { forkJoin } from 'rxjs';

@Component({
  selector: 'app-reports',
  templateUrl: './reports.component.html',
  styleUrls: ['./reports.component.scss']
})
export class ReportsComponent implements OnInit {
  filterForm!: FormGroup;

  loading = false;
  exporting = false;
  error: string | null = null;

  // Data
  kpis: KPIsResponse | null = null;
  funnel: AcquisitionFunnelResponse | null = null;
  campaignPerformance: CampaignPerformanceResponse | null = null;
  timeSeries: TimeSeriesResponse | null = null;
  campaigns: Campaign[] = [];

  // Table columns
  displayedColumns: string[] = [
    'campaign_slug',
    'country',
    'landing_views',
    'transactions',
    'subscribed',
    'charged',
    'estimated_revenue',
    'conversion_rate'
  ];

  // Chart configuration
  chartData: ChartData = {
    labels: [],
    datasets: []
  };

  chartOptions: ChartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        display: true,
        position: 'top'
      },
      tooltip: {
        mode: 'index',
        intersect: false
      }
    },
    scales: {
      x: {
        display: true,
        title: {
          display: true,
          text: 'Date'
        }
      },
      y: {
        display: true,
        title: {
          display: true,
          text: 'Count'
        },
        beginAtZero: true
      },
      y1: {
        display: true,
        position: 'right',
        title: {
          display: true,
          text: 'Revenue ($)'
        },
        beginAtZero: true,
        grid: {
          drawOnChartArea: false
        }
      }
    }
  };

  constructor(
    private fb: FormBuilder,
    private reportsApi: ReportsApiService,
    private campaignService: CampaignService
  ) {}

  ngOnInit(): void {
    // Initialize filters with default date range (last 30 days)
    const endDate = new Date();
    const startDate = new Date();
    startDate.setDate(startDate.getDate() - 30);

    this.filterForm = this.fb.group({
      startDate: [this.formatDate(startDate)],
      endDate: [this.formatDate(endDate)],
      campaignSlug: [''],
      interval: ['daily']
    });

    this.loadCampaigns();
    this.loadReports();
  }

  loadCampaigns(): void {
    this.campaignService.getCampaigns().subscribe({
      next: (campaigns) => {
        this.campaigns = campaigns;
      },
      error: (err) => {
        console.error('Failed to load campaigns for filter:', err);
      }
    });
  }

  loadReports(): void {
    this.loading = true;
    this.error = null;

    const filters = {
      startDate: this.filterForm.get('startDate')?.value,
      endDate: this.filterForm.get('endDate')?.value,
      campaignSlug: this.filterForm.get('campaignSlug')?.value || undefined,
      interval: this.filterForm.get('interval')?.value || 'daily'
    };

    // Load all reports in parallel
    forkJoin({
      kpis: this.reportsApi.getKPIs(filters),
      funnel: this.reportsApi.getAcquisitionFunnel(filters),
      campaignPerformance: this.reportsApi.getCampaignPerformance(filters),
      timeSeries: this.reportsApi.getTimeSeries(filters)
    }).subscribe({
      next: (results) => {
        this.kpis = results.kpis;
        this.funnel = results.funnel;
        this.campaignPerformance = results.campaignPerformance;
        this.timeSeries = results.timeSeries;
        this.updateChartData();
        this.loading = false;
      },
      error: (err) => {
        console.error('Failed to load reports:', err);
        this.error = err.status === 401
          ? 'Unauthorized. Please log in again with Auth0.'
          : 'Failed to load reports. Please try again.';
        this.loading = false;
      }
    });
  }

  /**
   * Update chart data from time series response
   */
  updateChartData(): void {
    if (!this.timeSeries?.data_points?.length) {
      this.chartData = { labels: [], datasets: [] };
      return;
    }

    const labels = this.timeSeries.data_points.map(pt => {
      const date = new Date(pt.timestamp);
      return this.filterForm.get('interval')?.value === 'hourly'
        ? date.toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit' })
        : date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
    });

    this.chartData = {
      labels,
      datasets: [
        {
          label: 'Landing Views',
          data: this.timeSeries.data_points.map(pt => pt.landing_views),
          borderColor: '#3399ff',
          backgroundColor: 'rgba(51, 153, 255, 0.1)',
          borderWidth: 2,
          fill: true,
          tension: 0.4,
          yAxisID: 'y'
        },
        {
          label: 'Transactions',
          data: this.timeSeries.data_points.map(pt => pt.transactions),
          borderColor: '#f9b115',
          backgroundColor: 'rgba(249, 177, 21, 0.1)',
          borderWidth: 2,
          fill: false,
          tension: 0.4,
          yAxisID: 'y'
        },
        {
          label: 'Subscribed',
          data: this.timeSeries.data_points.map(pt => pt.subscribed),
          borderColor: '#2eb85c',
          backgroundColor: 'rgba(46, 184, 92, 0.1)',
          borderWidth: 2,
          fill: false,
          tension: 0.4,
          yAxisID: 'y'
        },
        {
          label: 'Charged',
          data: this.timeSeries.data_points.map(pt => pt.charged),
          borderColor: '#e55353',
          backgroundColor: 'rgba(229, 83, 83, 0.1)',
          borderWidth: 2,
          fill: false,
          tension: 0.4,
          yAxisID: 'y'
        },
        {
          label: 'Revenue ($)',
          data: this.timeSeries.data_points.map(pt => pt.estimated_revenue),
          borderColor: '#9da5b1',
          backgroundColor: 'rgba(157, 165, 177, 0.1)',
          borderWidth: 2,
          borderDash: [5, 5],
          fill: false,
          tension: 0.4,
          yAxisID: 'y1'
        }
      ]
    };
  }

  applyFilters(): void {
    this.loadReports();
  }

  setDateRange(days: number): void {
    const endDate = new Date();
    const startDate = new Date();
    startDate.setDate(startDate.getDate() - days);

    this.filterForm.patchValue({
      startDate: this.formatDate(startDate),
      endDate: this.formatDate(endDate)
    });

    this.loadReports();
  }

  formatDate(date: Date): string {
    return date.toISOString().split('T')[0];
  }

  formatNumber(value: number): string {
    if (value >= 1000000) {
      return (value / 1000000).toFixed(1) + 'M';
    }
    if (value >= 1000) {
      return (value / 1000).toFixed(1) + 'K';
    }
    return value.toString();
  }

  formatCurrency(value: number): string {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2
    }).format(value);
  }

  formatPercent(value: number): string {
    return value.toFixed(1) + '%';
  }

  getFunnelWidth(stage: { count: number }, maxCount: number): string {
    if (maxCount === 0) return '100%';
    const percent = Math.max((stage.count / maxCount) * 100, 10);
    return percent + '%';
  }

  getMaxFunnelCount(): number {
    if (!this.funnel?.stages?.length) return 0;
    return Math.max(...this.funnel.stages.map(s => s.count));
  }

  /**
   * Export campaign performance data as CSV
   */
  exportToCSV(): void {
    this.exporting = true;

    const filters = {
      startDate: this.filterForm.get('startDate')?.value,
      endDate: this.filterForm.get('endDate')?.value,
      campaignSlug: this.filterForm.get('campaignSlug')?.value || undefined
    };

    this.reportsApi.exportCampaignPerformanceCSV(filters).subscribe({
      next: (blob) => {
        // Generate filename with date range
        const startDate = filters.startDate || 'all';
        const endDate = filters.endDate || 'all';
        const filename = `campaign-performance_${startDate}_to_${endDate}.csv`;

        // Create download link and trigger download
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = filename;
        link.click();
        window.URL.revokeObjectURL(url);

        this.exporting = false;
      },
      error: (err) => {
        console.error('Failed to export CSV:', err);
        this.error = err.status === 401
          ? 'Unauthorized. Please log in again with Auth0.'
          : 'Failed to export CSV. Please try again.';
        this.exporting = false;
      }
    });
  }
}

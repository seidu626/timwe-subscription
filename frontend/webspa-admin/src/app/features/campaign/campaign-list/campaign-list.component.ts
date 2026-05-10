import { Component, OnInit, ViewChild } from '@angular/core';
import { MatTableDataSource } from '@angular/material/table';
import { MatPaginator } from '@angular/material/paginator';
import { MatSort } from '@angular/material/sort';
import { MatDialog } from '@angular/material/dialog';
import { MatSnackBar } from '@angular/material/snack-bar';
import { Router } from '@angular/router';
import { Campaign, FlowType } from '../../+state/models/campaign.model';
import { CampaignService, CloneCampaignRequest } from '../../+state/services/campaign.service';
import { CampaignCloneDialogComponent } from './campaign-clone-dialog.component';

@Component({
  selector: 'app-campaign-list',
  templateUrl: './campaign-list.component.html',
  styleUrls: ['./campaign-list.component.scss']
})
export class CampaignListComponent implements OnInit {
  campaigns: Campaign[] = [];
  loading = false;
  error: string | null = null;

  displayedColumns: string[] = [
    'slug',
    'country',
    'language',
    'flow_type',
    'price',
    'enabled',
    'updated_at',
    'actions'
  ];

  filters = {
    enabled: '',
    country: ''
  };

  dataSource = new MatTableDataSource<Campaign>([]);

  @ViewChild(MatPaginator) paginator!: MatPaginator;
  @ViewChild(MatSort) sort!: MatSort;

  constructor(
    private campaignService: CampaignService,
    private router: Router,
    private dialog: MatDialog,
    private snackBar: MatSnackBar
  ) {}

  ngOnInit(): void {
    this.loadCampaigns();
  }

  ngAfterViewInit() {
    this.dataSource.paginator = this.paginator;
    this.dataSource.sort = this.sort;
  }

  loadCampaigns(): void {
    this.loading = true;
    this.error = null;

    const filters: { enabled?: boolean; country?: string } = {};
    if (this.filters.enabled === 'true') {
      filters.enabled = true;
    } else if (this.filters.enabled === 'false') {
      filters.enabled = false;
    }
    if (this.filters.country) {
      filters.country = this.filters.country;
    }

    this.campaignService.getCampaigns(Object.keys(filters).length > 0 ? filters : undefined)
      .subscribe({
        next: (campaigns) => {
          this.campaigns = campaigns;
          this.dataSource.data = campaigns;
          this.loading = false;
        },
        error: (err) => {
          console.error('Failed to load campaigns:', err);
          this.error = err.status === 401
            ? 'Unauthorized. Please log in again with Auth0.'
            : 'Failed to load campaigns. Please try again.';
          this.loading = false;
        }
      });
  }

  applyFilters(): void {
    this.loadCampaigns();
  }

  clearFilters(): void {
    this.filters = { enabled: '', country: '' };
    this.loadCampaigns();
  }

  onCreate(): void {
    this.router.navigate(['/campaign/create']);
  }

  onEdit(campaign: Campaign): void {
    this.router.navigate(['/campaign/edit', campaign.slug]);
  }

  onToggleEnabled(campaign: Campaign): void {
    const newEnabled = !campaign.enabled;
    this.campaignService.setEnabled(campaign.slug, newEnabled).subscribe({
      next: (updated) => {
        const index = this.campaigns.findIndex(c => c.slug === campaign.slug);
        if (index !== -1) {
          this.campaigns[index] = updated;
          this.dataSource.data = [...this.campaigns];
        }
      },
      error: (err) => {
        console.error('Failed to toggle enabled:', err);
        this.error = 'Failed to update campaign status.';
      }
    });
  }

  onPreview(campaign: Campaign): void {
    const url = (campaign.landing_page_urls && campaign.landing_page_urls.length > 0)
      ? campaign.landing_page_urls[0]
      : this.campaignService.getLandingPageUrl(campaign.slug);
    window.open(url, '_blank');
  }

  onClone(campaign: Campaign): void {
    const dialogRef = this.dialog.open(CampaignCloneDialogComponent, {
      width: '520px',
      maxWidth: '95vw',
      data: { sourceSlug: campaign.slug }
    });

    dialogRef.afterClosed().subscribe((payload?: CloneCampaignRequest) => {
      if (!payload) {
        return;
      }

      this.campaignService.cloneCampaign(campaign.slug, payload).subscribe({
        next: (cloned) => {
          this.loadCampaigns();
          const snackRef = this.snackBar.open(
            `Campaign copied as ${cloned.slug}`,
            'Edit',
            { duration: 5000 }
          );
          snackRef.onAction().subscribe(() => {
            this.router.navigate(['/campaign/edit', cloned.slug]);
          });
        },
        error: (err) => {
          console.error('Failed to clone campaign:', err);
          this.error = this.getCloneErrorMessage(err);
        }
      });
    });
  }

  private getCloneErrorMessage(err: any): string {
    if (err?.status === 404) {
      return 'Source campaign not found.';
    }
    if (err?.status === 409) {
      return 'New slug already exists. Choose a different slug.';
    }
    if (err?.status === 400) {
      return err?.error || 'Invalid clone request.';
    }
    return 'Failed to copy campaign. Please try again.';
  }

  getFlowTypeLabel(flowType: FlowType): string {
    const labels: Record<FlowType, string> = {
      'CLICK_TO_SMS': 'Click to SMS',
      'OTP': 'OTP',
      'REDIRECT': 'Redirect',
      'MIXED': 'Mixed'
    };
    return labels[flowType] || flowType;
  }

  applyFilter(event: Event): void {
    const filterValue = (event.target as HTMLInputElement).value;
    this.dataSource.filter = filterValue.trim().toLowerCase();
  }
}

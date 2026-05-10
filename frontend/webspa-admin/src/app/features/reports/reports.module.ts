import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { ReportsRoutingModule } from './reports-routing.module';
import { ReportsComponent } from './reports.component';
import { SharedModule } from '../../shared/shared.module';
import { MaterialModule } from '../../shared/material.module';
import { ReportsApiService } from '../+state/services/reports-api.service';
import { CampaignService } from '../+state/services/campaign.service';
import { DataService, LoggerService } from '../../core/services';
import { ChartjsModule } from '@coreui/angular-chartjs';

@NgModule({
  declarations: [
    ReportsComponent
  ],
  imports: [
    CommonModule,
    FormsModule,
    ReactiveFormsModule,
    ReportsRoutingModule,
    SharedModule,
    MaterialModule,
    ChartjsModule
  ],
  providers: [
    DataService,
    LoggerService,
    ReportsApiService,
    CampaignService
  ]
})
export class ReportsModule { }

import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { CampaignRoutingModule } from './campaign-routing.module';
import { CampaignListComponent } from './campaign-list/campaign-list.component';
import { CampaignFormComponent } from './campaign-form/campaign-form.component';
import { CampaignCloneDialogComponent } from './campaign-list/campaign-clone-dialog.component';
import { SharedModule } from '../../shared/shared.module';
import { MaterialModule } from '../../shared/material.module';
import { CampaignService } from '../+state/services/campaign.service';
import { DataService, LoggerService } from '../../core/services';

@NgModule({
  declarations: [
    CampaignListComponent,
    CampaignFormComponent,
    CampaignCloneDialogComponent
  ],
  imports: [
    CommonModule,
    FormsModule,
    ReactiveFormsModule,
    CampaignRoutingModule,
    SharedModule,
    MaterialModule
  ],
  providers: [
    DataService,
    LoggerService,
    CampaignService
  ]
})
export class CampaignModule { }

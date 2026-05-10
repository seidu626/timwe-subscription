import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { CampaignListComponent } from './campaign-list/campaign-list.component';
import { CampaignFormComponent } from './campaign-form/campaign-form.component';
import { pendingChangesGuard } from '../../core/guards/pending-changes.guard';

const routes: Routes = [
  { 
    path: '', 
    component: CampaignListComponent,
    data: { title: 'Campaigns' }
  },
  { 
    path: 'create', 
    component: CampaignFormComponent,
    canDeactivate: [pendingChangesGuard],
    data: { title: 'Create Campaign' }
  },
  { 
    path: 'edit/:slug', 
    component: CampaignFormComponent,
    canDeactivate: [pendingChangesGuard],
    data: { title: 'Edit Campaign' }
  }
];

@NgModule({
  imports: [RouterModule.forChild(routes)],
  exports: [RouterModule]
})
export class CampaignRoutingModule { }

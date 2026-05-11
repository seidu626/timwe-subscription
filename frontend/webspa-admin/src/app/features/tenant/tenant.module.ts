import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { TenantRoutingModule } from './tenant-routing.module';
import { TenantListComponent } from './tenant-list/tenant-list.component';
import { SharedModule } from '../../shared/shared.module';
import { MaterialModule } from '../../shared/material.module';

@NgModule({
  declarations: [
    TenantListComponent
  ],
  imports: [
    CommonModule,
    FormsModule,
    TenantRoutingModule,
    SharedModule,
    MaterialModule
  ]
})
export class TenantModule {}

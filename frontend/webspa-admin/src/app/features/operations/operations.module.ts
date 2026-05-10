import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { OperationsRoutingModule } from './operations-routing.module';
import { OperationsDashboardComponent } from './operations-dashboard/operations-dashboard.component';
import { SharedModule } from '../../shared/shared.module';
import { MaterialModule } from '../../shared/material.module';

@NgModule({
  declarations: [
    OperationsDashboardComponent
  ],
  imports: [
    CommonModule,
    FormsModule,
    ReactiveFormsModule,
    OperationsRoutingModule,
    SharedModule,
    MaterialModule
  ]
})
export class OperationsModule {}

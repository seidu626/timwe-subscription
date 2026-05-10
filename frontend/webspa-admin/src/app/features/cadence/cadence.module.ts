import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { CadenceRoutingModule } from './cadence-routing.module';
import { CadenceComponent } from './cadence.component';
import { SharedModule } from '../../shared/shared.module';
import { MaterialModule } from '../../shared/material.module';

@NgModule({
  declarations: [CadenceComponent],
  imports: [
    CommonModule,
    FormsModule,
    ReactiveFormsModule,
    CadenceRoutingModule,
    SharedModule,
    MaterialModule,
  ],
})
export class CadenceModule {}


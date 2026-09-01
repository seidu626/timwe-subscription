import { CommonModule } from '@angular/common';
import { NgModule } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MaterialModule } from '../../shared/material.module';
import { MSISDNCatalogRoutingModule } from './msisdn-catalog-routing.module';
import { MSISDNCatalogComponent } from './msisdn-catalog.component';

@NgModule({
  declarations: [MSISDNCatalogComponent],
  imports: [CommonModule, FormsModule, MaterialModule, MSISDNCatalogRoutingModule]
})
export class MSISDNCatalogModule {}

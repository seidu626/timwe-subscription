import { CommonModule } from '@angular/common';
import { NgModule } from '@angular/core';
import { ReactiveFormsModule } from '@angular/forms';
import { MaterialModule } from '../../shared/material.module';
import { SmsTemplatesRoutingModule } from './sms-templates-routing.module';
import { SmsTemplatesComponent } from './sms-templates.component';

@NgModule({
  declarations: [SmsTemplatesComponent],
  imports: [CommonModule, ReactiveFormsModule, MaterialModule, SmsTemplatesRoutingModule],
})
export class SmsTemplatesModule {}

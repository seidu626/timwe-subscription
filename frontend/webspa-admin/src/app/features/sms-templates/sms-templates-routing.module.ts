import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { SmsTemplatesComponent } from './sms-templates.component';

const routes: Routes = [{ path: '', component: SmsTemplatesComponent }];

@NgModule({ imports: [RouterModule.forChild(routes)], exports: [RouterModule] })
export class SmsTemplatesRoutingModule {}

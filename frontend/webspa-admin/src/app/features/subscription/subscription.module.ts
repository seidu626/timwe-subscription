import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SubscriptionListComponent } from './subscription-list/subscription-list.component';
import { SubscriptionRoutingModule } from './subscription-routing.module';
import { SharedModule } from '../../shared/shared.module';
import { FormsModule } from '@angular/forms';
import { SubscriptionService } from '../+state/services/subscription.service';
import { MaterialModule } from '../../shared/material.module';
import { DataService,  LoggerService } from '../../core/services';

@NgModule({
  declarations: [
    SubscriptionListComponent
  ],
  imports: [
    CommonModule,
    FormsModule,
    SubscriptionRoutingModule,
    SharedModule,
    MaterialModule
  ],            
  providers: [
    DataService,
    LoggerService,
    SubscriptionService
  ]
})
export class SubscriptionModule { }

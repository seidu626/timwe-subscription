import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SharedModule } from '../../shared/shared.module';
import { FormsModule } from '@angular/forms';
import { SubscriptionService } from '../+state/services/subscription.service';
import { MaterialModule } from '../../shared/material.module';
import { DataService,  LoggerService } from '../../core/services';
import { NotificationListComponent } from './notification-list/notification-list.component';
import { NotificationRoutingModule } from './notification-routing.module';

@NgModule({
  declarations: [
    NotificationListComponent
  ],
  imports: [
    CommonModule,
    FormsModule,
    NotificationRoutingModule,
    SharedModule,
    MaterialModule
  ],            
  providers: [
    DataService,
    LoggerService,
    SubscriptionService
  ]
})
export class NotificationModule { }

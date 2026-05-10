import { CommonModule } from '@angular/common';
import { NgModule, Optional, SkipSelf, ErrorHandler, APP_INITIALIZER } from '@angular/core';
import {
  HTTP_INTERCEPTORS
} from '@angular/common/http';
import { FormsModule } from '@angular/forms';

import {
  ROUTE_ANIMATIONS_ELEMENTS,
  routeAnimations
} from './animations/route.animations';
import { AnimationsService } from './animations/animations.service';
import { AppErrorHandler } from './error-handler/app-error-handler.service';
import { LocalStorageService } from './local-storage/local-storage.service';
import { HttpErrorInterceptor } from './http-interceptors/http-error.interceptor';
import { TenantWorkspaceInterceptor } from './http-interceptors/tenant-workspace.interceptor';
import { NotificationService } from './notifications/notification.service';
import { DataService, ErrorService,  LoggerService } from './services';
import { SecurityService } from './services/security.service';
import { VersionCheckService } from './utils/version-check.service';
import { MaterialModule } from '../shared/material.module';
import { ReactiveStorageModule } from './reactive-storage';
import { CdkStepper } from '@angular/cdk/stepper';
import { EventBusService } from './events/event-bus.service';
import { ScriptService } from "./services/script.service";
import { NgProgressbar } from 'ngx-progressbar'; 

 
export {
  routeAnimations,
  LocalStorageService,
  ROUTE_ANIMATIONS_ELEMENTS,
  AnimationsService,
  NotificationService
};


@NgModule({
  imports: [
    // angular
    CommonModule,
    FormsModule,

    ReactiveStorageModule.setLocalStorageKeys(['empty', '']),

    // material
    // MaterialModule,
  ],
  declarations: [],
  providers: [
    { provide: HTTP_INTERCEPTORS, useClass: TenantWorkspaceInterceptor, multi: true },
    { provide: HTTP_INTERCEPTORS, useClass: HttpErrorInterceptor, multi: true },
    { provide: ErrorHandler, useClass: AppErrorHandler },
    NgProgressbar,
    CdkStepper,
    DataService, LoggerService, VersionCheckService,
    SecurityService, LocalStorageService, AnimationsService, EventBusService, ErrorService
  ],
  exports: [
    // angular
    FormsModule,
    MaterialModule
  ]
})
export class CoreModule {
  constructor(
    @Optional()
    @SkipSelf()
    parentModule: CoreModule
  ) {
    if (parentModule) {
      throw new Error('CoreModule is already loaded. Import only in AppModule');
    }
  }
}

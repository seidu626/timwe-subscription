import { ErrorHandler, NgModule } from '@angular/core';
import { BrowserModule } from '@angular/platform-browser';
import * as Sentry from '@sentry/angular';

import { AppComponent } from './app/app.component';

@NgModule({
  imports: [
    BrowserModule,
    AppComponent,
  ],
  providers: [{ provide: ErrorHandler, useValue: Sentry.createErrorHandler() }],
  bootstrap: [AppComponent],
})
export class AppModule {}

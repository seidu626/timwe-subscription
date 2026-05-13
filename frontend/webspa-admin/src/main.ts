/// <reference types="@angular/localize" />

import * as Sentry from '@sentry/angular-ivy';
import { bootstrapApplication } from '@angular/platform-browser';
import { ErrorHandler } from '@angular/core';

import { AppComponent } from './app/app.component';
import { appConfig } from './app/app.config';
import { environment } from './environments/environment';

Sentry.init({
  dsn: environment.sentryDsn,
  environment: environment.sentryEnvironment,
  release: environment.sentryRelease,
  sendDefaultPii: false,
  tracesSampleRate: 1.0,
});

bootstrapApplication(AppComponent, {
  ...appConfig,
  providers: [
    ...(appConfig.providers ?? []),
    { provide: ErrorHandler, useValue: Sentry.createErrorHandler() },
  ],
})
  .catch(err => console.error(err));

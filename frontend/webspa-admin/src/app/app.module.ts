import { ErrorHandler, NgModule } from '@angular/core';
import { BrowserModule } from '@angular/platform-browser';
import { SentryErrorHandler, SentryModule } from '@sentry/angular-ivy';

import { AppComponent } from './app/app.component';
import { environment } from '../environments/environment';

@NgModule({
  imports: [
    BrowserModule,
    AppComponent,
    SentryModule.forRoot({
      dsn: environment.sentryDsn,
      environment: environment.sentryEnvironment,
      release: environment.sentryRelease,
      sendDefaultPii: false,
    }),
  ],
  providers: [{ provide: ErrorHandler, useClass: SentryErrorHandler }],
  bootstrap: [AppComponent],
})
export class AppModule {}

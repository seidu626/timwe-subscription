import { Injectable, ErrorHandler, NgZone, Injector } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { NotificationService } from '../notifications/notification.service';
import { DataService, LoggerService, ErrorService } from '../services';

/** Application-wide error handler that adds a UI notification to the error handling
 * provided by the default Angular ErrorHandler.
 */
@Injectable()
export class AppErrorHandler extends ErrorHandler {
  // Error handling is important and needs to be loaded first.
  // Because of this we should manually inject the services with Injector.
  constructor(private injector: Injector) { super(); }

  override handleError(error: Error | HttpErrorResponse) {
    const errorService = this.injector.get(ErrorService);
    const logger = this.injector.get(LoggerService);
    const notifier = this.injector.get(NotificationService);

    let message;
    let stackTrace;

    if (error instanceof HttpErrorResponse) {
      // Server Error
      message = errorService.getServerMessage(error);
      stackTrace = errorService.getServerStack(error);
      if (error.status == 401) {
        notifier.error(message);
      }
    } else {
      // Client Error
      message = errorService.getClientMessage(error);
      stackTrace = errorService.getClientStack(error);

      // Additional logging for undefined properties or specific errors
      if (error.message.includes("Cannot read properties of undefined")) {
        notifier.error("A critical error occurred. Please try again later.");
      } else {
        notifier.error(message);
      }
    }

    // Always log errors
    logger.error(message, stackTrace);
    console.error(error);
    super.handleError(error);
  }

}

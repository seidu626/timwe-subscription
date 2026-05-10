import { Injectable } from '@angular/core';
import { MatSnackBar, MatSnackBarConfig } from '@angular/material/snack-bar';

@Injectable({
  providedIn: 'root',
})
export class NotificationService {
  constructor(private snackBar: MatSnackBar) {}

  private showNotification(
    message: string,
    action: string = '',
    config: MatSnackBarConfig = {}
  ) {
    this.snackBar.open(message, action, config);
  }

  default(message: string) {
    this.showNotification(message, '', {
      duration: 600,
      horizontalPosition: 'right',
      verticalPosition: 'top',
      panelClass: ['default-notification'],
    });
  }

  info(message: string) {
    this.showNotification(message, '', {
      duration: 600,
      horizontalPosition: 'center',
      verticalPosition: 'top',
      panelClass: ['info-notification'],
    });
  }

  success(message: string) {
    this.showNotification(message, 'Close', {
      duration: 600,
      horizontalPosition: 'center',
      verticalPosition: 'top',
      panelClass: ['success-notification'],
    });
  }

  warn(message: string) {
    this.showNotification(message, '', {
      duration: 600,
      horizontalPosition: 'right',
      verticalPosition: 'top',
      panelClass: ['warn-notification'],
    });
  }

  error(message: string) {
    this.showNotification(message, 'Close', {
      duration: 600,
      horizontalPosition: 'right',
      verticalPosition: 'bottom',
      panelClass: ['error-notification'],
    });
  }
}

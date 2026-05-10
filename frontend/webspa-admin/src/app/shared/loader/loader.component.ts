import { Component } from '@angular/core';
import { LoaderService } from './loader.service';  // Import the loader service
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-loader',
  standalone: true,  // Ensure the component is standalone
  imports: [MatProgressBarModule, CommonModule],
  template: `
    <mat-progress-bar 
      *ngIf="isLoading$ | async"
      mode="indeterminate">
    </mat-progress-bar>
  `,
  styles: [`
    mat-progress-bar {
      position: fixed;
      top: 0;
      left: 0;
      width: 100%;
      z-index: 1000;  /* Ensure it's on top */
    }
  `]  // Remove the trailing comma here
})
export class LoaderComponent {
  isLoading$ = this.loaderService.isLoading$;  // Subscribe to loader observable

  constructor(private loaderService: LoaderService) {}
}

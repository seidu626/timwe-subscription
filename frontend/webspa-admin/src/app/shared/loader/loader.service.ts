import { Injectable } from '@angular/core';
import { BehaviorSubject } from 'rxjs';

@Injectable({
  providedIn: 'root',  // Ensure the service is available globally
})
export class LoaderService {
  private isLoadingSubject = new BehaviorSubject<boolean>(false); // Observable to track loader state
  public isLoading$ = this.isLoadingSubject.asObservable(); // Exposed observable for components to subscribe

  constructor() {}

  show() {
    this.isLoadingSubject.next(true);  // Start showing the loader
  }

  hide() {
    this.isLoadingSubject.next(false);  // Hide the loader
  }
}

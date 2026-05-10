import { Injectable, inject } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { Observable } from 'rxjs';
import { Router } from '@angular/router';
import { AuthService, User } from '@auth0/auth0-angular';

/**
 * Security Service
 * Provides authentication-related utilities.
 * Primary authentication is handled by Auth0.
 */
@Injectable()
export class SecurityService {
  private router = inject(Router);
  private auth = inject(AuthService);

  /**
   * Observable of the current authenticated user.
   * Returns null/undefined if not authenticated.
   */
  get user$(): Observable<User | null | undefined> {
    return this.auth.user$;
  }

  /**
   * Observable indicating if the user is authenticated.
   */
  get isAuthenticated$(): Observable<boolean> {
    return this.auth.isAuthenticated$;
  }

  /**
   * Observable indicating if Auth0 is loading (e.g., checking session).
   */
  get isLoading$(): Observable<boolean> {
    return this.auth.isLoading$;
  }

  /**
   * Initiates Auth0 login flow.
   */
  login(): void {
    this.auth.loginWithRedirect();
  }

  /**
   * Initiates Auth0 logout flow.
   */
  logout(): void {
    this.auth.logout({ 
      logoutParams: { 
        returnTo: window.location.origin 
      } 
    });
  }

  /**
   * Handles HTTP errors by navigating to appropriate error pages.
   */
  handleError(error: any): void {
    console.error('Security error:', error);
    if (error instanceof HttpErrorResponse) {
      if (error.status === 403) {
        this.router.navigate(['/403']);
      } else if (error.status === 401) {
        // Store redirect URL and navigate to login
        sessionStorage.setItem('auth_redirect_url', this.router.url);
        this.router.navigate(['/login']);
      }
    }
  }
}

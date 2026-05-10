import { Component, OnInit, inject } from '@angular/core';
import { CommonModule, NgStyle } from '@angular/common';
import { IconDirective } from '@coreui/icons-angular';
import { ContainerComponent, RowComponent, ColComponent, CardGroupComponent, TextColorDirective, CardComponent, CardBodyComponent, FormDirective, InputGroupComponent, InputGroupTextDirective, FormControlDirective, ButtonDirective, SpinnerComponent } from '@coreui/angular';
import { AuthService } from '@auth0/auth0-angular';
import { Router } from '@angular/router';
import { Observable } from 'rxjs';

@Component({
    selector: 'app-login',
    templateUrl: './login.component.html',
    styleUrls: ['./login.component.scss'],
    standalone: true,
    imports: [
      CommonModule,
      ContainerComponent, 
      RowComponent, 
      ColComponent, 
      CardGroupComponent, 
      TextColorDirective, 
      CardComponent, 
      CardBodyComponent, 
      FormDirective, 
      InputGroupComponent, 
      InputGroupTextDirective, 
      IconDirective, 
      FormControlDirective, 
      ButtonDirective, 
      NgStyle,
      SpinnerComponent
    ]
})
export class LoginComponent implements OnInit {
  private auth = inject(AuthService);
  private router = inject(Router);

  isLoading$: Observable<boolean> = this.auth.isLoading$;
  error$: Observable<Error | null> = this.auth.error$;

  ngOnInit(): void {
    // Check if user is already authenticated and redirect
    this.auth.isAuthenticated$.subscribe(isAuthenticated => {
      if (isAuthenticated) {
        // Check for stored redirect URL
        const redirectUrl = sessionStorage.getItem('auth_redirect_url');
        if (redirectUrl) {
          sessionStorage.removeItem('auth_redirect_url');
          this.router.navigateByUrl(redirectUrl);
        } else {
          this.router.navigate(['/dashboard']);
        }
      }
    });
  }

  login(): void {
    this.auth.loginWithRedirect({
      appState: { 
        target: sessionStorage.getItem('auth_redirect_url') || '/dashboard' 
      }
    });
  }

}

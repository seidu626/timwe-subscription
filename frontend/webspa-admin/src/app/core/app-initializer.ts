import { AuthService } from './services/auth.service';


export function appInitializer(authService: AuthService) {
  return () =>
    new Promise((resolve: any) => {
      console.log('refresh token on app start up')
      authService.refreshToken().subscribe(
        (resp: any) => {
          if(resp) {
            console.log(resp, 'refresh successful');
            return;
          }
         
          console.log(resp, 'not authenticated');
        },
        err => {
          console.log(err);
        }
      ).add(resolve);
    });
}

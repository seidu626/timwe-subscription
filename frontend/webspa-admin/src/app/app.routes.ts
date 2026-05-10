import { Routes } from '@angular/router';
import { DefaultLayoutComponent } from './layout';
import { authGuard, publicOnlyGuard } from './core/guards/auth.guard';
import { tenantWorkspaceGuard } from './core/guards/tenant-workspace.guard';

export const routes: Routes = [
  {
    path: '',
    redirectTo: 'dashboard',
    pathMatch: 'full'
  },
  {
    path: '',
    component: DefaultLayoutComponent,
    canActivate: [authGuard],
    canActivateChild: [tenantWorkspaceGuard],
    data: {
      title: 'Home'
    },
    children: [
      {
        path: 'dashboard',
        loadChildren: () => import('./views/dashboard/routes').then((m) => m.routes)
      },
      { path: 'subscription', loadChildren: () => import('./features/subscription/subscription.module').then(m => m.SubscriptionModule) },
      { path: 'notification', loadChildren: () => import('./features/notification/notification.module').then(m => m.NotificationModule) },
      { path: 'campaign', loadChildren: () => import('./features/campaign/campaign.module').then(m => m.CampaignModule) },
      { path: 'cadence', loadChildren: () => import('./features/cadence/cadence.module').then(m => m.CadenceModule) },
      { path: 'reports', loadChildren: () => import('./features/reports/reports.module').then(m => m.ReportsModule) },
      { path: 'postback', loadChildren: () => import('./features/postback/postback.module').then(m => m.PostbackModule) },
      { path: 'transactions', loadChildren: () => import('./features/transaction/transaction.module').then(m => m.TransactionModule) },
      { path: 'products', loadChildren: () => import('./features/product/product.module').then(m => m.ProductModule) },
      { path: 'userbase', loadChildren: () => import('./features/userbase/userbase.module').then(m => m.UserbaseModule) },
      { path: 'operations', loadChildren: () => import('./features/operations/operations.module').then(m => m.OperationsModule) },
      { path: 'settings', loadChildren: () => import('./features/settings/settings.module').then(m => m.SettingsModule) },
      {
        path: 'theme',
        loadChildren: () => import('./views/theme/routes').then((m) => m.routes)
      },
      {
        path: 'base',
        loadChildren: () => import('./views/base/routes').then((m) => m.routes)
      },
      {
        path: 'buttons',
        loadChildren: () => import('./views/buttons/routes').then((m) => m.routes)
      },
      {
        path: 'forms',
        loadChildren: () => import('./views/forms/routes').then((m) => m.routes)
      },
      {
        path: 'icons',
        loadChildren: () => import('./views/icons/routes').then((m) => m.routes)
      },
      {
        path: 'notifications',
        loadChildren: () => import('./views/notifications/routes').then((m) => m.routes)
      },
      {
        path: 'widgets',
        loadChildren: () => import('./views/widgets/routes').then((m) => m.routes)
      },
      {
        path: 'charts',
        loadChildren: () => import('./views/charts/routes').then((m) => m.routes)
      },
      {
        path: 'pages',
        loadChildren: () => import('./views/pages/routes').then((m) => m.routes)
      }
    ]
  },
  {
    path: '403',
    loadComponent: () => import('./views/pages/page403/page403.component').then(m => m.Page403Component),
    data: {
      title: 'Workspace unavailable'
    }
  },
  {
    path: '404',
    loadComponent: () => import('./views/pages/page404/page404.component').then(m => m.Page404Component),
    data: {
      title: 'Page 404'
    }
  },
  {
    path: '500',
    loadComponent: () => import('./views/pages/page500/page500.component').then(m => m.Page500Component),
    data: {
      title: 'Page 500'
    }
  },
  {
    path: 'login',
    loadComponent: () => import('./views/pages/login/login.component').then(m => m.LoginComponent),
    canActivate: [publicOnlyGuard],
    data: {
      title: 'Login Page'
    }
  },
  {
    path: 'register',
    loadComponent: () => import('./views/pages/register/register.component').then(m => m.RegisterComponent),
    data: {
      title: 'Register Page'
    }
  },
  { path: '**', redirectTo: '404' }
];

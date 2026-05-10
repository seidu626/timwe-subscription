import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { OperationsDashboardComponent } from './operations-dashboard/operations-dashboard.component';

const routes: Routes = [
  {
    path: '',
    component: OperationsDashboardComponent,
    data: {
      title: 'Operations'
    }
  }
];

@NgModule({
  imports: [RouterModule.forChild(routes)],
  exports: [RouterModule]
})
export class OperationsRoutingModule {}

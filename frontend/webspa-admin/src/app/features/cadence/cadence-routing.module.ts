import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { CadenceComponent } from './cadence.component';

const routes: Routes = [
  {
    path: '',
    component: CadenceComponent,
  },
];

@NgModule({
  imports: [RouterModule.forChild(routes)],
  exports: [RouterModule],
})
export class CadenceRoutingModule {}


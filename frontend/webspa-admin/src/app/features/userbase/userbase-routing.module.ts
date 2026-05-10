import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { UserbaseListComponent } from './userbase-list/userbase-list.component';

const routes: Routes = [
  {
    path: '',
    component: UserbaseListComponent,
    data: {
      title: 'Userbase'
    }
  }
];

@NgModule({
  imports: [RouterModule.forChild(routes)],
  exports: [RouterModule]
})
export class UserbaseRoutingModule {}

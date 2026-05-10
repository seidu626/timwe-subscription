import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { PostbackLookupComponent } from './postback-lookup/postback-lookup.component';

const routes: Routes = [
  {
    path: '',
    component: PostbackLookupComponent,
    data: {
      title: 'Postback Lookup'
    }
  }
];

@NgModule({
  imports: [RouterModule.forChild(routes)],
  exports: [RouterModule]
})
export class PostbackRoutingModule { }

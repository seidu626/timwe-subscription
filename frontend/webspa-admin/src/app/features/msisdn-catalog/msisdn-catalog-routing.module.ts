import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { MSISDNCatalogComponent } from './msisdn-catalog.component';

const routes: Routes = [{ path: '', component: MSISDNCatalogComponent, data: { title: 'MSISDN Catalog' } }];

@NgModule({ imports: [RouterModule.forChild(routes)], exports: [RouterModule] })
export class MSISDNCatalogRoutingModule {}

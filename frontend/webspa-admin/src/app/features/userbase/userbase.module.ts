import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { UserbaseRoutingModule } from './userbase-routing.module';
import { UserbaseListComponent } from './userbase-list/userbase-list.component';
import { SharedModule } from '../../shared/shared.module';
import { MaterialModule } from '../../shared/material.module';

@NgModule({
  declarations: [
    UserbaseListComponent
  ],
  imports: [
    CommonModule,
    FormsModule,
    ReactiveFormsModule,
    UserbaseRoutingModule,
    SharedModule,
    MaterialModule
  ]
})
export class UserbaseModule {}

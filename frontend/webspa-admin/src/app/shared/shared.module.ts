import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { ValidationSummaryComponent } from './validation-summary/validation-summary.component';
import { ErrorDialogComponent } from './error-dialog/error-dialog.component';
import { LoaderComponent } from './loader/loader.component';
import { LoaderService } from './loader/loader.service';
import { MaterialModule } from './material.module';

@NgModule({
  imports: [
    CommonModule,
    FormsModule,
    FormsModule,
    MaterialModule,
    ReactiveFormsModule,
  ],
  declarations: [
    ValidationSummaryComponent,
    ErrorDialogComponent,
  ],
  providers: [ LoaderService],
  exports: [
    ValidationSummaryComponent,
    ErrorDialogComponent,
  ]
})
export class SharedModule {
}

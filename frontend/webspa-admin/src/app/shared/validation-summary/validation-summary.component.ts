import { Component, OnInit, Input } from '@angular/core';
import { NgForm, FormGroup } from '@angular/forms';

@Component({
  // tslint:disable-next-line:component-selector
  selector: 'validation-summary',
  templateUrl: './validation-summary.component.html',
  styleUrls: ['./validation-summary.component.css']
})
export class ValidationSummaryComponent implements OnInit {
  @Input() form!: FormGroup;
  errors: string[] = [];

  constructor() { }

  ngOnInit() {
    if (this.form instanceof FormGroup === false) {
      throw new Error('You must supply the validation summary with an FormGroup.');
    }
    this.form.statusChanges.subscribe(status => {
      this.resetErrorMessages();
      this.generateErrorMessages(this.form);
    });
  }

  resetErrorMessages() {
    this.errors.length = 0;
  }

  generateErrorMessages(formGroup: FormGroup) {
    Object.keys(formGroup.controls).forEach(controlName => {
      const control = formGroup.controls[controlName];
      const errors = control.errors;
      if (errors === null || errors['count'] === 0) {
        return;
      }
      // Handle the 'required' case
      if (errors['required']) {
        this.errors.push(`${controlName} is required`);
      }
      // Handle 'minlength' case
      if (errors['minlength']) {
        this.errors.push(`${controlName} minimum length is ${errors['minlength'].requiredLength}.`);
      }
      // Handle custom messages.
      if (errors['message']) {
        this.errors.push(`${controlName} ${errors['message']}`);
      }
    });
  }
}

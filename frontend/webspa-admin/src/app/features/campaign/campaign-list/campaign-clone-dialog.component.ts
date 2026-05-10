import { Component, Inject } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { CloneCampaignRequest } from '../../+state/services/campaign.service';

export interface CampaignCloneDialogData {
  sourceSlug: string;
}

@Component({
  selector: 'app-campaign-clone-dialog',
  templateUrl: './campaign-clone-dialog.component.html',
  styleUrls: ['./campaign-clone-dialog.component.scss']
})
export class CampaignCloneDialogComponent {
  form: FormGroup;

  private readonly slugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

  constructor(
    private fb: FormBuilder,
    private dialogRef: MatDialogRef<CampaignCloneDialogComponent, CloneCampaignRequest | undefined>,
    @Inject(MAT_DIALOG_DATA) public data: CampaignCloneDialogData
  ) {
    this.form = this.fb.group({
      new_slug: [`${data.sourceSlug}-copy`, [Validators.required, Validators.pattern(this.slugPattern)]],
      created_by: ['']
    });
  }

  cancel(): void {
    this.dialogRef.close(undefined);
  }

  submit(): void {
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }

    const raw = this.form.getRawValue();
    const newSlug = (raw.new_slug || '').trim();
    const createdBy = (raw.created_by || '').trim();

    const payload: CloneCampaignRequest = {
      new_slug: newSlug,
      created_by: createdBy || undefined
    };

    this.dialogRef.close(payload);
  }

  get slugError(): string {
    const control = this.form.get('new_slug');
    if (!control || !control.touched) {
      return '';
    }
    if (control.hasError('required')) {
      return 'New slug is required';
    }
    if (control.hasError('pattern')) {
      return 'Use lowercase letters, numbers, and hyphens only';
    }
    return '';
  }
}

import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { CadenceApiService } from '../+state/services/cadence-api.service';
import {
  CadenceContentItem,
  CadenceCsvImportResult,
  CadenceSeries,
  CadenceScheduleRule,
} from '../+state/models/cadence.model';

const LINK_URL_PATTERN = /^https?:\/\/.+/i;

export interface CadenceSelectOption {
  value: string;
  label: string;
  help?: string;
}

const DELIVERY_CHANNEL_OPTIONS: CadenceSelectOption[] = [
  { value: 'USER_PREF', label: 'Subscriber preference (default)' },
  { value: 'SMS', label: 'Always SMS' },
  { value: 'PUSH', label: 'Push to app, SMS fallback' },
];

const CONTENT_KIND_OPTIONS: CadenceSelectOption[] = [
  { value: 'TEXT', label: 'Text message' },
  { value: 'LINK', label: 'Link / service resource' },
];

const MODE_OPTIONS: CadenceSelectOption[] = [
  { value: 'SEQUENTIAL', label: 'Sequential (fixed order)' },
  { value: 'POOL', label: 'Pool (randomized)' },
];

const RULE_KIND_OPTIONS: CadenceSelectOption[] = [
  { value: 'DAILY', label: 'Daily' },
  { value: 'WEEKLY', label: 'Weekly (choose days)' },
  { value: 'EVERY_N_DAYS', label: 'Every N days' },
];

const CATCHUP_MODE_OPTIONS: CadenceSelectOption[] = [
  { value: 'THROTTLE', label: 'Throttle (spread out catch-up sends)' },
  { value: 'SEND', label: 'Send immediately' },
  { value: 'SKIP', label: 'Skip missed sends' },
];

const WEEKDAY_OPTIONS: Array<{ bit: number; label: string }> = [
  { bit: 1, label: 'Mon' },
  { bit: 2, label: 'Tue' },
  { bit: 4, label: 'Wed' },
  { bit: 8, label: 'Thu' },
  { bit: 16, label: 'Fri' },
  { bit: 32, label: 'Sat' },
  { bit: 64, label: 'Sun' },
];

@Component({
  selector: 'app-cadence',
  templateUrl: './cadence.component.html',
  styleUrls: ['./cadence.component.scss'],
})
export class CadenceComponent implements OnInit {
  readonly deliveryChannelOptions = DELIVERY_CHANNEL_OPTIONS;
  readonly contentKindOptions = CONTENT_KIND_OPTIONS;
  readonly modeOptions = MODE_OPTIONS;
  readonly ruleKindOptions = RULE_KIND_OPTIONS;
  readonly catchupModeOptions = CATCHUP_MODE_OPTIONS;
  readonly weekdayOptions = WEEKDAY_OPTIONS;
  loading = false;
  error: string | null = null;

  series: CadenceSeries[] = [];
  selectedSeriesId: number | null = null;
  selectedSeries: CadenceSeries | null = null;

  rule: CadenceScheduleRule | null = null;
  contentItems: CadenceContentItem[] = [];

  createSeriesForm!: FormGroup;
  ruleForm!: FormGroup;
  contentForm!: FormGroup;

  importDryRun = true;
  importFile: File | null = null;
  importResult: CadenceCsvImportResult | null = null;
  importing = false;

  publishVersionInput: number = 1;
  publishingVersion = false;
  publishResult: { previous_version: number; published_version: number } | null = null;

  displayedContentColumns: string[] = [
    'content_version',
    'seq_no',
    'is_active',
    'content_kind',
    'message_text',
    'link',
  ];

  constructor(
    private cadenceApi: CadenceApiService,
    private fb: FormBuilder
  ) {}

  ngOnInit(): void {
    this.createSeriesForm = this.fb.group({
      partner_role_id: [null, [Validators.required, Validators.min(1)]],
      product_id: [null, [Validators.required, Validators.min(1)]],
      name: ['', [Validators.required]],
      mode: ['SEQUENTIAL', [Validators.required]],
      delivery_channel: ['USER_PREF', [Validators.required]],
    });

    this.ruleForm = this.fb.group({
      rule_kind: ['DAILY', [Validators.required]],
      preferred_time: ['09:00', [Validators.required]],
      days_of_week: [0],
      n_days: [0],
      send_start_time: ['08:00', [Validators.required]],
      send_end_time: ['20:00', [Validators.required]],
      timezone: ['Africa/Accra', [Validators.required]],
      max_per_day: [1, [Validators.required, Validators.min(1)]],
      catchup_mode: ['THROTTLE', [Validators.required]],
    });

    this.contentForm = this.fb.group({
      content_version: [1, [Validators.required, Validators.min(1)]],
      seq_no: [1, [Validators.required, Validators.min(1)]],
      is_active: [true],
      message_text: ['', [Validators.required]],
      content_kind: ['TEXT', [Validators.required]],
      link_url: [''],
      cta_label: ['', [Validators.maxLength(40)]],
    });

    this.contentForm.get('content_kind')?.valueChanges.subscribe((kind) => {
      this.applyContentKindValidators(kind);
    });
    this.applyContentKindValidators(this.contentForm.get('content_kind')?.value);

    this.loadSeries();
  }

  private applyContentKindValidators(kind: string): void {
    const linkUrlControl = this.contentForm.get('link_url');
    const ctaLabelControl = this.contentForm.get('cta_label');
    if (kind === 'LINK') {
      linkUrlControl?.setValidators([Validators.required, Validators.pattern(LINK_URL_PATTERN)]);
    } else {
      linkUrlControl?.setValue('');
      linkUrlControl?.setValidators([]);
      ctaLabelControl?.setValue('');
    }
    linkUrlControl?.updateValueAndValidity({ emitEvent: false });
  }

  get contentKind(): string {
    return this.contentForm?.get('content_kind')?.value || 'TEXT';
  }

  get ruleKind(): string {
    return this.ruleForm?.get('rule_kind')?.value || 'DAILY';
  }

  isWeekdaySelected(bit: number): boolean {
    const mask = Number(this.ruleForm?.get('days_of_week')?.value) || 0;
    return (mask & bit) !== 0;
  }

  toggleWeekday(bit: number): void {
    const control = this.ruleForm.get('days_of_week');
    const mask = Number(control?.value) || 0;
    control?.setValue(mask ^ bit);
  }

  loadSeries(): void {
    this.loading = true;
    this.error = null;
    this.cadenceApi.listSeries({ limit: 500 }).subscribe({
      next: (res) => {
        this.series = res.series || [];
        this.loading = false;
      },
      error: (err) => {
        console.error('Failed to load cadence series:', err);
        this.error = err.status === 401
          ? 'Unauthorized. Please log in again with Auth0.'
          : 'Failed to load cadence series.';
        this.loading = false;
      },
    });
  }

  onSeriesSelected(seriesId: number): void {
    this.selectedSeriesId = seriesId;
    this.selectedSeries = null;
    this.rule = null;
    this.contentItems = [];
    this.importResult = null;

    if (!seriesId) {
      return;
    }

    this.loading = true;
    this.error = null;

    this.cadenceApi.getSeries(seriesId).subscribe({
      next: (s) => {
        this.selectedSeries = s;
        this.loading = false;
        this.loadRule(seriesId);
        this.loadContent(seriesId);
      },
      error: (err) => {
        console.error('Failed to load series:', err);
        this.error = 'Failed to load series.';
        this.loading = false;
      },
    });
  }

  loadRule(seriesId: number): void {
    this.cadenceApi.getRule(seriesId).subscribe({
      next: (rule) => {
        this.rule = rule;
        this.ruleForm.patchValue({
          rule_kind: rule.rule_kind,
          preferred_time: this.extractClock(rule.preferred_time),
          days_of_week: rule.days_of_week,
          n_days: rule.n_days,
          send_start_time: this.extractClock(rule.send_start_time),
          send_end_time: this.extractClock(rule.send_end_time),
          timezone: rule.timezone,
          max_per_day: rule.max_per_day,
          catchup_mode: rule.catchup_mode,
        });
      },
      error: (err) => {
        // Rule might not exist yet; keep the form defaults.
        console.warn('Rule not found or failed to load:', err);
      },
    });
  }

  saveRule(): void {
    if (!this.selectedSeriesId) return;
    if (this.ruleForm.invalid) return;

    const payload = this.ruleForm.value;
    this.cadenceApi.putRule(this.selectedSeriesId, payload).subscribe({
      next: () => {
        this.loadRule(this.selectedSeriesId!);
      },
      error: (err) => {
        console.error('Failed to save rule:', err);
        this.error = 'Failed to save schedule rule.';
      },
    });
  }

  loadContent(seriesId: number): void {
    this.cadenceApi.listContent(seriesId, { limit: 1000 }).subscribe({
      next: (res) => {
        this.contentItems = res.items || [];
      },
      error: (err) => {
        console.error('Failed to load content:', err);
        this.error = 'Failed to load content items.';
      },
    });
  }

  saveContent(): void {
    if (!this.selectedSeriesId) return;
    if (this.contentForm.invalid) return;
    const formValue = this.contentForm.value;
    const payload = {
      content_version: formValue.content_version,
      seq_no: formValue.seq_no,
      message_text: formValue.message_text,
      is_active: formValue.is_active,
      content_kind: formValue.content_kind,
      // Link fields only apply to LINK content; never submitted for TEXT.
      ...(formValue.content_kind === 'LINK'
        ? { link_url: formValue.link_url, cta_label: formValue.cta_label || undefined }
        : {}),
    };
    this.cadenceApi.upsertContent(this.selectedSeriesId, payload).subscribe({
      next: () => {
        this.loadContent(this.selectedSeriesId!);
      },
      error: (err) => {
        console.error('Failed to save content:', err);
        this.error = 'Failed to save content item.';
      },
    });
  }

  createSeries(): void {
    if (this.createSeriesForm.invalid) return;
    const payload = this.createSeriesForm.value;
    this.cadenceApi.upsertSeries(payload).subscribe({
      next: (s) => {
        this.loadSeries();
        this.onSeriesSelected(s.id);
      },
      error: (err) => {
        console.error('Failed to create series:', err);
        this.error = 'Failed to create series.';
      },
    });
  }

  onFilePicked(evt: Event): void {
    const input = evt.target as HTMLInputElement;
    if (!input.files?.length) {
      this.importFile = null;
      return;
    }
    this.importFile = input.files[0];
  }

  runImport(dryRun: boolean): void {
    if (!this.importFile) {
      this.error = 'Please choose a CSV file to import.';
      return;
    }
    this.importing = true;
    this.error = null;
    this.importResult = null;
    this.cadenceApi.importCsv(this.importFile, dryRun).subscribe({
      next: (res) => {
        this.importResult = res;
        this.importing = false;
        if (!dryRun && this.selectedSeriesId) {
          this.loadContent(this.selectedSeriesId);
          this.loadSeries();
        }
      },
      error: (err) => {
        console.error('CSV import failed:', err);
        this.importResult = err?.error || null;
        this.error = 'CSV import failed. Check the result/error details.';
        this.importing = false;
      },
    });
  }

  onPublishVersion(): void {
    if (!this.selectedSeriesId || this.publishVersionInput <= 0) {
      this.error = 'Select a series and enter a valid content version to publish.';
      return;
    }
    this.publishingVersion = true;
    this.publishResult = null;
    this.error = null;

    this.cadenceApi.publishVersion(this.selectedSeriesId, this.publishVersionInput).subscribe({
      next: (res) => {
        this.publishResult = {
          previous_version: res.previous_version,
          published_version: res.published_version,
        };
        this.publishingVersion = false;
        // Refresh series to get updated content_version
        if (this.selectedSeriesId) {
          this.cadenceApi.getSeries(this.selectedSeriesId).subscribe({
            next: (s) => {
              this.selectedSeries = s;
            },
          });
          this.loadSeries();
        }
      },
      error: (err) => {
        console.error('Publish version failed:', err);
        this.error = err?.error?.error || 'Failed to publish content version.';
        this.publishingVersion = false;
      },
    });
  }

  getAvailableVersions(): number[] {
    const versions = new Set<number>();
    for (const item of this.contentItems) {
      versions.add(item.content_version);
    }
    return Array.from(versions).sort((a, b) => a - b);
  }

  getDeliveryChannelLabel(channel: string | null | undefined): string {
    return DELIVERY_CHANNEL_OPTIONS.find((o) => o.value === channel)?.label || channel || '-';
  }

  getContentKindLabel(kind: string | null | undefined): string {
    return CONTENT_KIND_OPTIONS.find((o) => o.value === kind)?.label || kind || 'Text message';
  }

  getModeLabel(mode: string | null | undefined): string {
    return MODE_OPTIONS.find((o) => o.value === mode)?.label || mode || '-';
  }

  private extractClock(value: string): string {
    // Handles either HH:MM(:SS) or RFC3339 time (e.g. 2000-01-01T09:00:00Z)
    if (!value) return '';
    if (value.includes('T')) {
      const t = value.split('T')[1] || '';
      return (t.split('Z')[0] || '').slice(0, 5);
    }
    return value.slice(0, 5);
  }
}


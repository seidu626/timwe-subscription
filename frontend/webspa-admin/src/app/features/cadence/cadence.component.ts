import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { CadenceApiService } from '../+state/services/cadence-api.service';
import {
  CadenceCloneResult,
  CadenceContentImpact,
  CadenceContentItem,
  CadenceCsvImportResult,
  CadenceSeries,
  CadenceSeriesHealth,
  CadenceSeriesPreview,
  CadenceScheduleRule,
} from '../+state/models/cadence.model';

const LINK_URL_PATTERN = /^https?:\/\/.+/i;

// GSM-7 basic character set; anything outside forces UCS-2 encoding.
const GSM7_BASIC = /^[@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞÆæßÉ !"#¤%&'()*+,\-./0-9:;<=>?¡A-ZÄÖÑÜ§¿a-zäöñüà]*$/;
// These GSM-7 extension characters are allowed but cost two septets each.
const GSM7_EXTENDED = new Set(['^', '{', '}', '\\', '[', ']', '~', '|', '€']);

export interface SmsSegmentInfo {
  chars: number;
  segments: number;
  encoding: 'GSM-7' | 'Unicode';
  perSegment: number;
}

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

  health: CadenceSeriesHealth[] = [];
  healthLoading = false;
  healthError: string | null = null;
  displayedHealthColumns: string[] = [
    'status',
    'name',
    'subscribers',
    'sent_24h',
    'failed_24h',
    'last_sent',
    'next_due',
  ];

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
  preview: CadenceSeriesPreview | null = null;
  previewLoading = false;
  previewError: string | null = null;
  publishResult: { previous_version: number; published_version: number } | null = null;

  reactivateResumable: number | null = null;
  reactivateBusy = false;
  reactivateError: string | null = null;
  reactivateResult: { resumed_states: number } | null = null;

  displayedContentColumns: string[] = [
    'content_version',
    'seq_no',
    'is_active',
    'content_kind',
    'message_text',
    'link',
    'actions',
  ];

  editingItem: CadenceContentItem | null = null;
  contentSaving = false;
  contentError: string | null = null;
  togglingItemId: number | null = null;

  liveImpact: CadenceContentImpact | null = null;
  impactLoading = false;

  cloneBusy = false;
  cloneResult: CadenceCloneResult | null = null;
  cloneError: string | null = null;

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

  loadHealth(): void {
    this.healthLoading = true;
    this.healthError = null;
    this.cadenceApi.seriesHealth().subscribe({
      next: (res) => {
        this.health = res.series || [];
        this.healthLoading = false;
      },
      error: (err) => {
        console.error('Failed to load series health:', err);
        this.healthError = err.status === 401
          ? 'Unauthorized. Please log in again with Auth0.'
          : 'Failed to load delivery health.';
        this.healthLoading = false;
      },
    });
  }

  // Status is derived from the last 24h of outbox activity so a series that
  // silently stops sending (the May-August blackout mode) surfaces as
  // critical, not merely idle.
  healthStatus(row: CadenceSeriesHealth): 'healthy' | 'warning' | 'critical' | 'idle' | 'inactive' {
    if (!row.is_active) return 'inactive';
    const total = row.sent_24h + row.failed_24h;
    if (total === 0) {
      const overdue = row.next_due_at && new Date(row.next_due_at).getTime() < Date.now() - 60 * 60 * 1000;
      return row.active_states > 0 && overdue ? 'critical' : 'idle';
    }
    const failureRatio = row.failed_24h / total;
    if (failureRatio >= 0.5) return 'critical';
    if (failureRatio > 0.05) return 'warning';
    return 'healthy';
  }

  healthStatusLabel(row: CadenceSeriesHealth): string {
    switch (this.healthStatus(row)) {
      case 'healthy': return 'Healthy';
      case 'warning': return 'Degraded';
      case 'critical': return 'Failing';
      case 'idle': return 'Idle';
      default: return 'Inactive';
    }
  }

  healthTooltip(row: CadenceSeriesHealth): string {
    const parts = [
      `7 days: ${row.sent_7d.toLocaleString()} sent, ${row.failed_7d.toLocaleString()} failed`,
      `All time: ${row.sent_total.toLocaleString()} sent, ${row.failed_total.toLocaleString()} failed`,
    ];
    if (row.last_error) {
      parts.push(`Last error: ${row.last_error}`);
    }
    return parts.join('\n');
  }

  loadSeries(): void {
    this.loading = true;
    this.error = null;
    this.loadHealth();
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
    this.preview = null;
    this.previewError = null;
    this.reactivateResumable = null;
    this.reactivateError = null;
    this.reactivateResult = null;
    this.cancelEditContent();
    this.contentError = null;
    this.liveImpact = null;
    this.cloneResult = null;
    this.cloneError = null;

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
        this.loadLiveImpact(seriesId, s.content_version);
      },
      error: (err) => {
        console.error('Failed to load series:', err);
        this.error = 'Failed to load series.';
        this.loading = false;
      },
    });
  }

  checkReactivate(): void {
    if (!this.selectedSeriesId) return;
    this.reactivateBusy = true;
    this.reactivateError = null;
    this.reactivateResult = null;
    this.cadenceApi.reactivateSeries(this.selectedSeriesId, true).subscribe({
      next: (res) => {
        this.reactivateResumable = res.resumable_states ?? 0;
        this.reactivateBusy = false;
      },
      error: (err) => {
        console.error('Failed to check reactivation:', err);
        this.reactivateError = 'Failed to check what reactivation would resume.';
        this.reactivateBusy = false;
      },
    });
  }

  cancelReactivate(): void {
    this.reactivateResumable = null;
    this.reactivateError = null;
  }

  confirmReactivate(): void {
    if (!this.selectedSeriesId) return;
    this.reactivateBusy = true;
    this.reactivateError = null;
    this.cadenceApi.reactivateSeries(this.selectedSeriesId, false).subscribe({
      next: (res) => {
        this.reactivateBusy = false;
        this.reactivateResumable = null;
        this.onSeriesSelected(this.selectedSeriesId!);
        // onSeriesSelected resets the panel state; restore the result banner.
        this.reactivateResult = { resumed_states: res.resumed_states ?? 0 };
      },
      error: (err) => {
        console.error('Failed to reactivate series:', err);
        this.reactivateError = err?.error?.error === 'no schedule rule: define one before reactivating'
          ? 'No schedule rule is defined. Save a schedule rule first, then reactivate.'
          : 'Failed to reactivate the series.';
        this.reactivateBusy = false;
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
    const formValue = this.contentForm.getRawValue();
    this.contentSaving = true;
    this.contentError = null;

    if (this.editingItem) {
      const patch = {
        message_text: formValue.message_text,
        content_kind: formValue.content_kind,
        is_active: formValue.is_active,
        // Link fields only apply to LINK content; never submitted for TEXT.
        ...(formValue.content_kind === 'LINK'
          ? { link_url: formValue.link_url, cta_label: formValue.cta_label || null }
          : {}),
      };
      this.cadenceApi.patchContent(this.selectedSeriesId, this.editingItem.id, patch).subscribe({
        next: () => {
          this.contentSaving = false;
          this.cancelEditContent();
          this.loadContent(this.selectedSeriesId!);
        },
        error: (err) => {
          console.error('Failed to update content:', err);
          this.contentError = err?.error?.error || 'Failed to update content item.';
          this.contentSaving = false;
        },
      });
      return;
    }

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
        this.contentSaving = false;
        this.loadContent(this.selectedSeriesId!);
      },
      error: (err) => {
        console.error('Failed to save content:', err);
        this.contentError = err?.error?.error || 'Failed to save content item.';
        this.contentSaving = false;
      },
    });
  }

  /** Load a row into the form for in-place editing (version/seq stay fixed). */
  startEditItem(item: CadenceContentItem): void {
    this.editingItem = item;
    this.contentError = null;
    this.contentForm.patchValue({
      content_version: item.content_version,
      seq_no: item.seq_no,
      is_active: item.is_active,
      message_text: item.message_text,
      content_kind: item.content_kind || 'TEXT',
      link_url: item.link_url || '',
      cta_label: item.cta_label || '',
    });
    // Version and sequence are immutable on edit: renumbering a published
    // version would skip or repeat messages for mid-series subscribers.
    this.contentForm.get('content_version')?.disable();
    this.contentForm.get('seq_no')?.disable();
  }

  cancelEditContent(): void {
    this.editingItem = null;
    this.contentForm?.get('content_version')?.enable();
    this.contentForm?.get('seq_no')?.enable();
  }

  /** Flip a row's active flag directly from the table. */
  toggleItemActive(item: CadenceContentItem): void {
    if (!this.selectedSeriesId || this.togglingItemId) return;
    this.togglingItemId = item.id;
    this.contentError = null;
    this.cadenceApi.patchContent(this.selectedSeriesId, item.id, { is_active: !item.is_active }).subscribe({
      next: () => {
        this.togglingItemId = null;
        this.loadContent(this.selectedSeriesId!);
      },
      error: (err) => {
        console.error('Failed to toggle content item:', err);
        this.contentError = err?.error?.error || 'Failed to toggle content item.';
        this.togglingItemId = null;
      },
    });
  }

  loadLiveImpact(seriesId: number, contentVersion: number): void {
    this.impactLoading = true;
    this.cadenceApi.contentImpact(seriesId, contentVersion).subscribe({
      next: (res) => {
        this.liveImpact = res;
        this.impactLoading = false;
      },
      error: (err) => {
        console.warn('Failed to load content impact:', err);
        this.liveImpact = null;
        this.impactLoading = false;
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

  /** Simulate the next sends for the version selected in the publish picker. */
  loadPreview(): void {
    if (!this.selectedSeriesId) return;
    this.previewLoading = true;
    this.previewError = null;
    this.preview = null;
    this.cadenceApi.previewSeries(this.selectedSeriesId, {
      count: 7,
      contentVersion: this.publishVersionInput > 0 ? this.publishVersionInput : undefined,
    }).subscribe({
      next: (res) => {
        this.preview = res;
        this.previewLoading = false;
      },
      error: (err) => {
        this.previewError = err?.error?.error || 'Failed to load preview. Define a schedule rule first.';
        this.previewLoading = false;
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
              this.loadLiveImpact(s.id, s.content_version);
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

  /** True when the version in the content form is the published (live) one. */
  get formTargetsLiveVersion(): boolean {
    if (!this.selectedSeries) return false;
    const version = Number(this.contentForm?.getRawValue()?.content_version) || 0;
    return version === this.selectedSeries.content_version;
  }

  /** In add mode, the existing item that a save would overwrite (same version+seq). */
  get addOverwriteTarget(): CadenceContentItem | null {
    if (this.editingItem || !this.contentForm) return null;
    const value = this.contentForm.getRawValue();
    const version = Number(value.content_version) || 0;
    const seq = Number(value.seq_no) || 0;
    return this.contentItems.find((i) => i.content_version === version && i.seq_no === seq) || null;
  }

  /** Copy an existing version's items into a fresh draft version for staged edits. */
  cloneVersion(fromVersion: number): void {
    if (!this.selectedSeriesId || this.cloneBusy) return;
    this.cloneBusy = true;
    this.cloneError = null;
    this.cloneResult = null;
    this.cadenceApi.cloneVersion(this.selectedSeriesId, fromVersion).subscribe({
      next: (res) => {
        this.cloneBusy = false;
        this.cloneResult = res;
        this.publishVersionInput = res.to_version;
        this.loadContent(this.selectedSeriesId!);
      },
      error: (err) => {
        console.error('Failed to clone version:', err);
        this.cloneError = err?.error?.error || 'Failed to clone content version.';
        this.cloneBusy = false;
      },
    });
  }

  /** Segment accounting for the SMS the subscriber will actually receive. */
  get formSmsInfo(): SmsSegmentInfo | null {
    if (!this.contentForm) return null;
    const value = this.contentForm.getRawValue();
    let text: string = (value.message_text || '').trim();
    if (!text) return null;
    if (value.content_kind === 'LINK' && (value.link_url || '').trim()) {
      // The sender appends the link after a space at send time.
      text = `${text} ${(value.link_url || '').trim()}`;
    }
    return this.smsSegmentInfo(text);
  }

  smsSegmentInfo(text: string): SmsSegmentInfo {
    let septets = 0;
    let gsm7 = true;
    for (const ch of text) {
      if (GSM7_EXTENDED.has(ch)) {
        septets += 2;
      } else if (GSM7_BASIC.test(ch)) {
        septets += 1;
      } else {
        gsm7 = false;
        break;
      }
    }
    if (gsm7) {
      const segments = septets <= 160 ? 1 : Math.ceil(septets / 153);
      return { chars: septets, segments, encoding: 'GSM-7', perSegment: segments === 1 ? 160 : 153 };
    }
    const chars = [...text].length;
    const segments = chars <= 70 ? 1 : Math.ceil(chars / 67);
    return { chars, segments, encoding: 'Unicode', perSegment: segments === 1 ? 70 : 67 };
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


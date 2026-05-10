import { Component, HostListener, OnInit, OnDestroy, ElementRef, ViewChild } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { MatSnackBar } from '@angular/material/snack-bar';
import { Campaign, FlowType, CampaignCreateRequest, CampaignUpdateRequest } from '../../+state/models/campaign.model';
import {
  CampaignService,
  PresignBackgroundUploadResponse
} from '../../+state/services/campaign.service';
import { PendingChangesAware } from '../../../core/guards/pending-changes.guard';

@Component({
  selector: 'app-campaign-form',
  templateUrl: './campaign-form.component.html',
  styleUrls: ['./campaign-form.component.scss']
})
export class CampaignFormComponent implements OnInit, OnDestroy, PendingChangesAware {
  private readonly defaultLPCopy = JSON.stringify({
    en: {
      heroTitle: 'Subscribe to unlock premium content.',
      heDescription: 'To continue, tap Subscribe.',
      heCta: 'Subscribe',
      heModalTitle: 'Almost there. Please confirm to continue.',
      heModalConfirm: 'Confirm',
      msisdnDescription: 'Enter your mobile number to receive your PIN code.',
      msisdnPlaceholder: 'Mobile number (9 digits)',
      msisdnCta: 'Subscribe',
      otpDescription: 'Enter the 4-digit PIN sent to your phone.',
      otpPlaceholder: '4-digit PIN',
      otpCta: 'Confirm',
      successTitle: 'Subscription successful',
      successBody: 'You will receive a text message with your access details.',
      consentPrefix: 'I agree to the',
      consentTerms: 'Terms and Conditions',
      termsHeading: 'Terms and Conditions',
      legal:
        'Your subscription renews automatically until cancelled. You must be 18+ years old or have parental permission to use this service.',
      phoneRequired: 'Phone number is required.',
      phoneInvalid: 'Enter a valid 9-digit mobile number.',
      otpInvalid: 'PIN must be exactly 4 digits.',
      consentRequired: 'You must accept terms to continue.'
    }
  }, null, 2);

  form!: FormGroup;
  isEditMode = false;
  loading = false;
  submitting = false;
  error: string | null = null;
  slug: string | null = null;
  backgroundUploadInProgress = false;
  backgroundUploadError: string | null = null;
  private existingTrackingConfig: any = {};

  readonly maxBackgroundSizeBytes = 2 * 1024 * 1024;
  readonly allowedBackgroundMimeTypes = ['image/jpeg', 'image/png', 'image/webp'];
  readonly themeColorRegex = /^#[0-9A-Fa-f]{6}$/;

  flowTypes: { value: FlowType; label: string }[] = [
    { value: 'CLICK_TO_SMS', label: 'Click to SMS' },
    { value: 'OTP', label: 'OTP' },
    { value: 'REDIRECT', label: 'Redirect' },
    { value: 'MIXED', label: 'Mixed' }
  ];

  billingCycles = ['daily', 'weekly', 'biweekly', 'monthly'];

  // Collapsible advanced sections
  advancedSectionState: Record<string, boolean> = {
    landing: true,
    attribution: true,
    guardrails: false
  };

  // Section navigation
  readonly formSections = [
    { id: 'section-basic', label: 'Basic Info', icon: 'campaign' },
    { id: 'section-product', label: 'Product', icon: 'inventory_2' },
    { id: 'section-flow', label: 'Flow', icon: 'alt_route' },
    { id: 'section-pricing', label: 'Pricing', icon: 'payments' },
    { id: 'section-compliance', label: 'Compliance', icon: 'verified_user' },
    { id: 'section-advanced', label: 'Advanced', icon: 'tune' }
  ];
  activeSectionId = 'section-basic';
  private scrollObserver?: IntersectionObserver;

  // JSON field validation
  jsonFieldValidity: Record<string, boolean | null> = {
    attribution_mapping: null,
    postback_rules: null,
    throttles: null,
    lp_copy: null
  };

  // Postback rules visual editor
  postbackEntries: { event: string; provider: string; method: string; url: string }[] = [];
  postbackRawMode = false;
  readonly postbackEvents = ['conversion', 'subscribed'];
  readonly postbackProviders = ['mobplus', 'generic', 'level23'];
  readonly postbackMethods = ['GET', 'POST'];
  readonly postbackVariables = [
    '{click_id}', '{transaction_id}', '{campaign_slug}', '{msisdn_hash}',
    '{payout}', '{status}', '{pub_id}', '{sub1}', '{sub2}', '{sub3}',
    '{offer_id}', '{campaign_id}', '{aff_id}', '{adv_id}'
  ];

  constructor(
    private fb: FormBuilder,
    private campaignService: CampaignService,
    private route: ActivatedRoute,
    private router: Router,
    private snackBar: MatSnackBar,
    private elementRef: ElementRef
  ) {}

  ngOnInit(): void {
    this.initForm();

    this.slug = this.route.snapshot.paramMap.get('slug');
    if (this.slug) {
      this.isEditMode = true;
      this.loadCampaign(this.slug);
    }

    // Watch for flow_type changes to update validation
    this.form.get('flow_type')?.valueChanges.subscribe((flowType: FlowType) => {
      this.updateClickToSmsValidation(flowType);
    });

    // Watch JSON fields for live validation
    for (const fieldName of Object.keys(this.jsonFieldValidity)) {
      this.form.get(fieldName)?.valueChanges.subscribe((value: string) => {
        this.jsonFieldValidity[fieldName] = this.validateJson(value);
      });
    }

    // Set up section scroll observer
    this.initScrollObserver();
  }

  ngOnDestroy(): void {
    this.scrollObserver?.disconnect();
  }

  private initScrollObserver(): void {
    if (typeof IntersectionObserver === 'undefined') return;

    setTimeout(() => {
      const sections = this.elementRef.nativeElement.querySelectorAll('[data-section-id]');
      if (!sections.length) return;

      this.scrollObserver = new IntersectionObserver(
        (entries) => {
          for (const entry of entries) {
            if (entry.isIntersecting) {
              this.activeSectionId = (entry.target as HTMLElement).dataset['sectionId'] || '';
            }
          }
        },
        { rootMargin: '-20% 0px -60% 0px', threshold: 0 }
      );

      sections.forEach((el: Element) => this.scrollObserver!.observe(el));
    }, 500);
  }

  scrollToSection(sectionId: string): void {
    const el = this.elementRef.nativeElement.querySelector(`[data-section-id="${sectionId}"]`);
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  }

  toggleAdvancedSection(key: string): void {
    this.advancedSectionState[key] = !this.advancedSectionState[key];
  }

  get themeColorSwatch(): string {
    const val = (this.form.get('theme_color')?.value || '').trim();
    return this.themeColorRegex.test(val) ? val : '';
  }

  get readinessPercent(): number {
    const items = this.completionItems;
    if (!items.length) return 0;
    const done = items.filter(i => i.complete).length;
    return Math.round((done / items.length) * 100);
  }

  private validateJson(value: string): boolean | null {
    if (!value || !value.trim()) return null;
    try {
      JSON.parse(value);
      return true;
    } catch {
      return false;
    }
  }

  @HostListener('window:beforeunload', ['$event'])
  handleBeforeUnload(event: BeforeUnloadEvent): void {
    if (!this.hasUnsavedChanges()) {
      return;
    }

    event.preventDefault();
    event.returnValue = '';
  }

  private initForm(): void {
    this.form = this.fb.group({
      // Basic Info
      slug: ['', [Validators.required, Validators.pattern(/^[a-z0-9]+(?:-[a-z0-9]+)*$/)]],
      language: ['en', Validators.required],
      country: ['', [Validators.required, Validators.maxLength(10)]],
      operator: [''],

      // Product Mapping
      offer_product_id: [null, [Validators.required, Validators.min(1)]],
      pricepoint_id: [null],
      partner_role_id: [null],

      // Flow Configuration
      flow_type: ['OTP', Validators.required],
      short_code: [''],
      sms_keyword: [''],

      // Pricing
      price: [null],
      billing_cycle: [''],

      // Compliance
      terms_url: [''],
      inline_terms_text: [''],
      consent_required: [true],
      consent_version: ['1.0'],

      // Advanced (JSON fields)
      attribution_mapping: ['{}'],
      postback_rules: ['{}'],
      throttles: ['{"per_msisdn_per_day": 3, "per_ip_per_day": 10}'],
      allowed_referrers: [''],
      allowed_sources: [''],
      landing_page_urls: [''],
      lp_copy: [this.defaultLPCopy],
      theme_color: ['', [Validators.pattern(this.themeColorRegex)]],
      background_image_url: ['', [Validators.pattern(/^https?:\/\/.+$/i)]],

      // Status
      enabled: [false]
    });
  }

  private updateClickToSmsValidation(flowType: FlowType): void {
    const shortCodeControl = this.form.get('short_code');
    const smsKeywordControl = this.form.get('sms_keyword');

    if (flowType === 'CLICK_TO_SMS') {
      shortCodeControl?.setValidators([Validators.required]);
      smsKeywordControl?.setValidators([Validators.required]);
    } else {
      shortCodeControl?.clearValidators();
      smsKeywordControl?.clearValidators();
    }

    shortCodeControl?.updateValueAndValidity();
    smsKeywordControl?.updateValueAndValidity();
  }

  private loadCampaign(slug: string): void {
    this.loading = true;
    this.campaignService.getCampaignBySlug(slug).subscribe({
      next: (campaign) => {
        this.patchForm(campaign);
        this.loading = false;
        // Disable slug editing
        this.form.get('slug')?.disable();
      },
      error: (err) => {
        console.error('Failed to load campaign:', err);
        this.error = 'Failed to load campaign.';
        this.loading = false;
      }
    });
  }

  private patchForm(campaign: Campaign): void {
    this.existingTrackingConfig = campaign.tracking_config
      ? JSON.parse(JSON.stringify(campaign.tracking_config))
      : {};
    const visual = campaign.tracking_config?.visual || {};

    this.form.patchValue({
      slug: campaign.slug,
      language: campaign.language,
      country: campaign.country,
      operator: campaign.operator || '',
      offer_product_id: campaign.offer_product_id,
      pricepoint_id: campaign.pricepoint_id,
      partner_role_id: campaign.partner_role_id,
      flow_type: campaign.flow_type,
      short_code: campaign.short_code || '',
      sms_keyword: campaign.sms_keyword || '',
      price: campaign.price,
      billing_cycle: campaign.billing_cycle || '',
      terms_url: campaign.terms_url || '',
      inline_terms_text: campaign.inline_terms_text || '',
      consent_required: campaign.consent_required,
      consent_version: campaign.consent_version || '',
      attribution_mapping: campaign.attribution_mapping
        ? JSON.stringify(campaign.attribution_mapping, null, 2)
        : '{}',
      postback_rules: campaign.postback_rules
        ? JSON.stringify(campaign.postback_rules, null, 2)
        : '{}',
      throttles: campaign.throttles
        ? JSON.stringify(campaign.throttles, null, 2)
        : '{}',
      allowed_referrers: (campaign.allowed_referrers || []).join('\n'),
      allowed_sources: (campaign.allowed_sources || []).join('\n'),
      landing_page_urls: (campaign.landing_page_urls || []).join('\n'),
      lp_copy: campaign.lp_copy
        ? JSON.stringify(campaign.lp_copy, null, 2)
        : this.defaultLPCopy,
      theme_color: visual.theme_color || '',
      background_image_url: visual.background_image_url || '',
      enabled: campaign.enabled
    });
    this.parsePostbackRulesToEntries(campaign.postback_rules);
    this.form.markAsPristine();
    this.form.markAsUntouched();
  }

  onSubmit(): void {
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }

    this.submitting = true;
    this.error = null;

    const formValue = this.form.getRawValue();
    const payload = this.buildPayload(formValue);

    const request$ = this.isEditMode && this.slug
      ? this.campaignService.updateCampaign(this.slug, payload as CampaignUpdateRequest)
      : this.campaignService.createCampaign(payload as CampaignCreateRequest);

    request$.subscribe({
      next: () => {
        this.submitting = false;
        this.snackBar.open(
          this.isEditMode ? 'Campaign updated successfully' : 'Campaign created successfully',
          'View List',
          { duration: 4000, panelClass: 'snackbar-success', horizontalPosition: 'center', verticalPosition: 'bottom' }
        );
        this.router.navigate(['/campaign']);
      },
      error: (err) => {
        console.error('Failed to save campaign:', err);
        this.error = err.error?.message || 'Failed to save campaign. Please check your input.';
        this.submitting = false;
        this.snackBar.open(
          'Save failed — check the form for errors',
          'Dismiss',
          { duration: 5000, panelClass: 'snackbar-error', horizontalPosition: 'center', verticalPosition: 'bottom' }
        );
      }
    });
  }

  private buildPayload(formValue: any): CampaignCreateRequest | CampaignUpdateRequest {
    const payload: any = {
      language: formValue.language,
      country: formValue.country.toUpperCase(),
      operator: formValue.operator || undefined,
      offer_product_id: formValue.offer_product_id,
      pricepoint_id: formValue.pricepoint_id || undefined,
      partner_role_id: formValue.partner_role_id || undefined,
      flow_type: formValue.flow_type,
      short_code: formValue.short_code || undefined,
      sms_keyword: formValue.sms_keyword || undefined,
      price: formValue.price || undefined,
      billing_cycle: formValue.billing_cycle || undefined,
      terms_url: formValue.terms_url || undefined,
      inline_terms_text: formValue.inline_terms_text || undefined,
      consent_required: formValue.consent_required,
      consent_version: formValue.consent_version || undefined,
      enabled: formValue.enabled
    };
    if (this.shouldIncludeTrackingConfig(formValue)) {
      const trackingConfig = this.buildTrackingConfig(formValue);
      payload.tracking_config = trackingConfig || {};
    }

    // Parse JSON fields
    try {
      payload.attribution_mapping = formValue.attribution_mapping
        ? JSON.parse(formValue.attribution_mapping)
        : undefined;
    } catch {
      payload.attribution_mapping = undefined;
    }

    try {
      payload.postback_rules = formValue.postback_rules
        ? JSON.parse(formValue.postback_rules)
        : undefined;
    } catch {
      payload.postback_rules = undefined;
    }

    try {
      payload.throttles = formValue.throttles
        ? JSON.parse(formValue.throttles)
        : undefined;
    } catch {
      payload.throttles = undefined;
    }

    try {
      payload.lp_copy = formValue.lp_copy
        ? JSON.parse(formValue.lp_copy)
        : undefined;
    } catch {
      payload.lp_copy = undefined;
    }

    // Parse array fields (newline separated)
    if (formValue.allowed_referrers) {
      payload.allowed_referrers = formValue.allowed_referrers
        .split('\n')
        .map((s: string) => s.trim())
        .filter((s: string) => s);
    }

    if (formValue.allowed_sources) {
      payload.allowed_sources = formValue.allowed_sources
        .split('\n')
        .map((s: string) => s.trim())
        .filter((s: string) => s);
    }

    if (formValue.landing_page_urls) {
      const urls = formValue.landing_page_urls
        .split('\n')
        .map((s: string) => s.trim())
        .filter((s: string) => s);

      // de-dupe while preserving order
      const seen = new Set<string>();
      payload.landing_page_urls = urls.filter((u: string) => {
        if (seen.has(u)) return false;
        seen.add(u);
        return true;
      });
    }

    // Add slug for create
    if (!this.isEditMode) {
      payload.slug = formValue.slug;
    }

    return payload;
  }

  private buildTrackingConfig(formValue: any): any {
    const base = this.extractKnownTrackingConfig(this.existingTrackingConfig);

    const themeColor = (formValue.theme_color || '').trim();
    const backgroundImageUrl = (formValue.background_image_url || '').trim();

    if (base.visual) {
      delete base.visual;
    }

    const visual: any = {};
    if (themeColor) {
      visual.theme_color = themeColor;
    }
    if (backgroundImageUrl) {
      visual.background_image_url = backgroundImageUrl;
    }

    if (Object.keys(visual).length > 0) {
      base.visual = visual;
    }

    return Object.keys(base).length > 0 ? base : undefined;
  }

  private extractKnownTrackingConfig(source: any): any {
    const known: any = {};
    if (source?.pixels) {
      known.pixels = JSON.parse(JSON.stringify(source.pixels));
    }
    if (source?.attribution) {
      known.attribution = JSON.parse(JSON.stringify(source.attribution));
    }
    if (source?.custom_events) {
      known.custom_events = JSON.parse(JSON.stringify(source.custom_events));
    }
    return known;
  }

  private shouldIncludeTrackingConfig(formValue: any): boolean {
    const existingThemeColor = (this.existingTrackingConfig?.visual?.theme_color || '').trim();
    const existingBackgroundImageUrl = (this.existingTrackingConfig?.visual?.background_image_url || '').trim();
    const nextThemeColor = (formValue.theme_color || '').trim();
    const nextBackgroundImageUrl = (formValue.background_image_url || '').trim();
    return existingThemeColor !== nextThemeColor || existingBackgroundImageUrl !== nextBackgroundImageUrl;
  }

  async onBackgroundFileSelected(event: Event): Promise<void> {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) {
      return;
    }

    this.backgroundUploadError = null;

    if (!this.allowedBackgroundMimeTypes.includes(file.type)) {
      this.backgroundUploadError = `Unsupported file type. Use: ${this.allowedBackgroundMimeTypes.join(', ')}`;
      input.value = '';
      return;
    }

    if (file.size > this.maxBackgroundSizeBytes) {
      this.backgroundUploadError = `File too large. Max size is ${Math.floor(this.maxBackgroundSizeBytes / (1024 * 1024))}MB.`;
      input.value = '';
      return;
    }

    const campaignSlug = this.getRawSlug();
    if (!campaignSlug || !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(campaignSlug)) {
      this.backgroundUploadError = 'Set a valid campaign slug before uploading a background image.';
      input.value = '';
      return;
    }

    this.backgroundUploadInProgress = true;
    this.submitting = false;

    this.campaignService.presignBackgroundUpload({
      campaign_slug: campaignSlug,
      file_name: file.name,
      content_type: file.type,
      size_bytes: file.size
    }).subscribe({
      next: async (presign: PresignBackgroundUploadResponse) => {
        try {
          await this.uploadFileToPresignedUrl(file, presign.upload_url);
          this.form.patchValue({ background_image_url: presign.asset_url });
          this.form.get('background_image_url')?.markAsDirty();
          this.form.get('background_image_url')?.markAsTouched();
        } catch {
          this.backgroundUploadError = 'Upload failed. Please try again.';
        } finally {
          this.backgroundUploadInProgress = false;
          input.value = '';
        }
      },
      error: (err) => {
        this.backgroundUploadError = err.error?.message || 'Failed to initialize upload.';
        this.backgroundUploadInProgress = false;
        input.value = '';
      }
    });
  }

  removeBackgroundImage(): void {
    this.form.patchValue({ background_image_url: '' });
    this.form.get('background_image_url')?.markAsDirty();
    this.backgroundUploadError = null;
  }

  get backgroundPreviewUrl(): string {
    return (this.form.get('background_image_url')?.value || '').trim();
  }

  private getRawSlug(): string {
    return (this.form.getRawValue().slug || '').trim();
  }

  private async uploadFileToPresignedUrl(file: File, uploadUrl: string): Promise<void> {
    const response = await fetch(uploadUrl, {
      method: 'PUT',
      headers: {
        'Content-Type': file.type
      },
      body: file
    });
    if (!response.ok) {
      throw new Error(`Upload failed with status ${response.status}`);
    }
  }

  onCancel(): void {
    if (!this.canDiscardChanges()) {
      return;
    }

    this.router.navigate(['/campaign']);
  }

  canDiscardChanges(): boolean {
    if (!this.hasUnsavedChanges()) {
      return true;
    }

    return window.confirm('You have unsaved changes. Leave this page and discard them?');
  }

  hasUnsavedChanges(): boolean {
    return !!this.form && (this.form.dirty || this.backgroundUploadInProgress);
  }

  getFlowTypeLabel(flowType?: FlowType): string {
    const labels: Record<FlowType, string> = {
      CLICK_TO_SMS: 'Click to SMS',
      OTP: 'OTP',
      REDIRECT: 'Redirect',
      MIXED: 'Mixed'
    };

    return flowType ? labels[flowType] || flowType : 'Not Set';
  }

  get statusLabel(): string {
    return this.form.get('enabled')?.value ? 'Enabled' : 'Draft';
  }

  get statusDescription(): string {
    return this.form.get('enabled')?.value
      ? 'Campaign is active and visible to landing pages.'
      : 'Campaign is disabled and hidden from landing pages.';
  }

  get postbackRuleCount(): number {
    return this.postbackEntries.length;
  }

  get landingPageUrlCount(): number {
    return this.countNonEmptyLines(this.form.get('landing_page_urls')?.value);
  }

  get allowedReferrerCount(): number {
    return this.countNonEmptyLines(this.form.get('allowed_referrers')?.value);
  }

  get allowedSourceCount(): number {
    return this.countNonEmptyLines(this.form.get('allowed_sources')?.value);
  }

  get completionItems(): Array<{ label: string; hint: string; complete: boolean }> {
    return [
      {
        label: 'Campaign Basics',
        hint: 'Slug, locale, operator, and product mapping are set.',
        complete: this.hasTextValue('slug')
          && this.hasTextValue('country')
          && this.hasTextValue('language')
          && !!this.form.get('offer_product_id')?.value
      },
      {
        label: 'Flow Setup',
        hint: 'The subscriber journey has the required flow inputs.',
        complete: this.isFlowConfigurationComplete()
      },
      {
        label: 'Compliance Copy',
        hint: 'Terms or inline consent text are available for the landing page.',
        complete: this.hasTextValue('terms_url') || this.hasTextValue('inline_terms_text')
      },
      {
        label: 'Delivery & Tracking',
        hint: 'Landing URLs, callbacks, or traffic guardrails are configured.',
        complete: this.landingPageUrlCount > 0
          || this.postbackRuleCount > 0
          || this.allowedReferrerCount > 0
          || this.allowedSourceCount > 0
      }
    ];
  }

  isFieldInvalid(fieldName: string): boolean {
    const field = this.form.get(fieldName);
    return !!(field && field.invalid && field.touched);
  }

  getFieldError(fieldName: string): string {
    const field = this.form.get(fieldName);
    if (!field || !field.errors) return '';

    if (field.errors['required']) return `${fieldName} is required`;
    if (field.errors['pattern']) return `Invalid format for ${fieldName}`;
    if (field.errors['min']) return `${fieldName} must be at least ${field.errors['min'].min}`;
    if (field.errors['maxlength']) return `${fieldName} is too long`;

    return 'Invalid value';
  }

  // === Postback Rules Visual Editor ===

  parsePostbackRulesToEntries(rules: any): void {
    this.postbackEntries = [];
    if (!rules || typeof rules !== 'object') return;
    for (const event of Object.keys(rules)) {
      const providers = rules[event];
      if (!providers || typeof providers !== 'object') continue;
      for (const provider of Object.keys(providers)) {
        const tmpl = providers[provider];
        this.postbackEntries.push({
          event,
          provider,
          method: tmpl?.method || 'GET',
          url: tmpl?.url || ''
        });
      }
    }
  }

  addPostbackEntry(): void {
    this.postbackEntries.push({ event: 'conversion', provider: '', method: 'GET', url: '' });
    this.syncPostbackEntriesToForm();
  }

  removePostbackEntry(index: number): void {
    this.postbackEntries.splice(index, 1);
    this.syncPostbackEntriesToForm();
  }

  syncPostbackEntriesToForm(): void {
    const rules: Record<string, Record<string, { url: string; method: string }>> = {};
    for (const entry of this.postbackEntries) {
      if (!entry.event || !entry.provider || !entry.url) continue;
      if (!rules[entry.event]) rules[entry.event] = {};
      rules[entry.event][entry.provider] = { url: entry.url, method: entry.method };
    }
    const nextRulesJson = JSON.stringify(rules, null, 2);
    const control = this.form.get('postback_rules');
    if (control && control.value !== nextRulesJson) {
      control.setValue(nextRulesJson);
    }
  }

  togglePostbackRawMode(): void {
    if (this.postbackRawMode) {
      // Switching from raw to visual — parse the JSON
      try {
        const parsed = JSON.parse(this.form.get('postback_rules')?.value || '{}');
        this.parsePostbackRulesToEntries(parsed);
      } catch {
        // Invalid JSON — stay in raw mode
        return;
      }
    } else {
      // Switching to raw — sync entries first
      this.syncPostbackEntriesToForm();
    }
    this.postbackRawMode = !this.postbackRawMode;
  }

  insertVariable(index: number, variable: string): void {
    const entry = this.postbackEntries[index];
    if (entry) {
      entry.url += variable;
      this.syncPostbackEntriesToForm();
    }
  }

  private countNonEmptyLines(value: string | null | undefined): number {
    return (value || '')
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean)
      .length;
  }

  private hasTextValue(controlName: string): boolean {
    return !!`${this.form.get(controlName)?.value ?? ''}`.trim();
  }

  private isFlowConfigurationComplete(): boolean {
    const flowType = this.form.get('flow_type')?.value as FlowType | undefined;

    if (!flowType) {
      return false;
    }

    if (flowType !== 'CLICK_TO_SMS') {
      return true;
    }

    return this.hasTextValue('short_code') && this.hasTextValue('sms_keyword');
  }
}

import { Component, OnDestroy, OnInit } from '@angular/core';
import { AbstractControl, FormBuilder, FormControl, ValidationErrors, Validators } from '@angular/forms';
import { Subject, debounceTime, distinctUntilChanged, finalize, takeUntil } from 'rxjs';
import { extractHttpErrorMessage } from '../../core/utils/http-error-message';
import { AdminProduct } from '../+state/models/product.model';
import { SMS_TEMPLATE_EVENT_TYPES, SMSTemplate } from '../+state/models/sms-template.model';
import { ProductService } from '../+state/services/product.service';
import { SMSTemplateApiService } from '../+state/services/sms-template-api.service';

@Component({ selector: 'app-sms-templates', templateUrl: './sms-templates.component.html', styleUrls: ['./sms-templates.component.scss'] })
export class SmsTemplatesComponent implements OnInit, OnDestroy {
  readonly maxCharacters = 480;
  readonly placeholders = [
    { token: '{{product_id}}', description: 'The product identifier.' },
    { token: '{{large_account}}', description: 'The configured large account.' },
    { token: '{{msisdn}}', description: 'Only the subscriber number’s last four digits (PII minimisation).' },
  ];
  readonly eventType = SMS_TEMPLATE_EVENT_TYPES.USER_OPTIN;
  readonly productSearch = new FormControl<string | AdminProduct>('');
  readonly form = this.fb.group({
    productId: [null as number | null, Validators.required],
    eventType: [this.eventType, Validators.required],
    enabled: [true],
    template: ['', [Validators.required, this.maxRunes(this.maxCharacters)]],
  });

  templates: SMSTemplate[] = [];
  products: AdminProduct[] = [];
  productNames = new Map<number, string>();
  selectedTemplate: SMSTemplate | null = null;
  loading = false;
  productsLoading = false;
  saving = false;
  togglingProductId: number | null = null;
  error: string | null = null;
  productError: string | null = null;
  success: string | null = null;
  private readonly destroy$ = new Subject<void>();

  constructor(private readonly fb: FormBuilder, private readonly api: SMSTemplateApiService, private readonly productService: ProductService) {}

  ngOnInit(): void {
    this.loadTemplates();
    this.searchProducts('');
    this.productSearch.valueChanges.pipe(debounceTime(300), distinctUntilChanged(), takeUntil(this.destroy$)).subscribe((value) => {
      if (typeof value === 'string') {
        this.form.controls.productId.setValue(null);
        this.searchProducts(value);
      }
    });
  }

  ngOnDestroy(): void { this.destroy$.next(); this.destroy$.complete(); }

  get characterCount(): number { return Array.from(this.form.controls.template.value ?? '').length; }
  get characterLimitExceeded(): boolean { return this.characterCount > this.maxCharacters; }

  displayProduct(product: AdminProduct | string | null): string { return typeof product === 'object' && product ? product.name : product ?? ''; }
  productLabel(productId: number): string { return this.productNames.get(productId) ?? `Product #${productId} (unavailable)`; }

  selectProduct(product: AdminProduct): void {
    this.form.controls.productId.setValue(product.id);
    this.productNames.set(product.id, product.name);
  }

  loadTemplates(): void {
    this.loading = true; this.error = null;
    this.api.list().pipe(finalize(() => this.loading = false)).subscribe({
      next: (templates) => { this.templates = templates; this.resolveProductNames(templates); },
      error: (error) => this.error = extractHttpErrorMessage(error, 'Could not load SMS templates. Please try again.'),
    });
  }

  startCreate(): void {
    this.selectedTemplate = null; this.success = null; this.productSearch.setValue('');
    this.form.reset({ productId: null, eventType: this.eventType, enabled: true, template: '' });
  }

  edit(template: SMSTemplate): void {
    this.selectedTemplate = template; this.success = null;
    const product = this.products.find((item) => item.id === template.productId);
    this.productSearch.setValue(product ?? this.productLabel(template.productId), { emitEvent: false });
    this.form.reset({ productId: template.productId, eventType: template.eventType, enabled: template.enabled, template: template.template });
  }

  save(): void {
    if (this.form.invalid) { this.form.markAllAsTouched(); return; }
    const productId = this.form.controls.productId.value;
    if (!productId) return;
    this.saving = true; this.error = null; this.success = null;
    this.api.upsert(productId, {
      eventType: this.eventType,
      enabled: Boolean(this.form.controls.enabled.value),
      template: this.form.controls.template.value ?? '',
    }).pipe(finalize(() => this.saving = false)).subscribe({
      next: (saved) => { this.success = `SMS template for ${this.productLabel(saved.productId)} saved.`; this.selectedTemplate = saved; this.loadTemplates(); },
      error: (error) => this.error = extractHttpErrorMessage(error, 'Could not save the SMS template. Please review the form and try again.'),
    });
  }

  toggle(template: SMSTemplate): void {
    this.togglingProductId = template.productId; this.error = null; this.success = null;
    this.api.setEnabled(template.productId, !template.enabled).pipe(finalize(() => this.togglingProductId = null)).subscribe({
      next: (updated) => { this.templates = this.templates.map((item) => item.productId === updated.productId && item.eventType === updated.eventType ? updated : item); this.success = `${this.productLabel(updated.productId)} template ${updated.enabled ? 'enabled' : 'disabled'}.`; },
      error: (error) => this.error = extractHttpErrorMessage(error, 'Could not update the template status. Please try again.'),
    });
  }

  private searchProducts(query: string): void {
    this.productsLoading = true; this.productError = null;
    this.productService.list({ q: query.trim() || undefined, page: 1, page_size: 20 }).pipe(finalize(() => this.productsLoading = false)).subscribe({
      next: (response) => { this.products = response.products ?? []; this.products.forEach((product) => this.productNames.set(product.id, product.name)); },
      error: (error) => this.productError = extractHttpErrorMessage(error, 'Could not search products. Please try again.'),
    });
  }

  private resolveProductNames(templates: SMSTemplate[]): void {
    if (!templates.length) return;
    this.loadProductNamesPage(1, new Set(templates.map((template) => template.productId)));
  }

  private loadProductNamesPage(page: number, unresolvedIds: Set<number>): void {
    const pageSize = 100;
    this.productService.list({ page, page_size: pageSize }).subscribe({
      next: (response) => {
        (response.products ?? []).forEach((product) => {
          this.productNames.set(product.id, product.name);
          unresolvedIds.delete(product.id);
        });
        if (unresolvedIds.size > 0 && page * pageSize < response.total_count) {
          this.loadProductNamesPage(page + 1, unresolvedIds);
        }
      },
      error: () => { /* Unresolved products use the required unavailable label. */ },
    });
  }

  private maxRunes(maximum: number): (control: AbstractControl) => ValidationErrors | null {
    return (control) => Array.from(String(control.value ?? '')).length > maximum ? { maxRunes: true } : null;
  }
}

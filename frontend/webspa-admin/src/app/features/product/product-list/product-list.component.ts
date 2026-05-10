import { Component, OnInit, ViewChild } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatPaginator, PageEvent } from '@angular/material/paginator';
import { MatTableDataSource } from '@angular/material/table';
import {
  AdminProduct,
  ProductDependencyCounts,
  ProductMutationPayload
} from '../../+state/models/product.model';
import { ProductService } from '../../+state/services/product.service';

@Component({
  selector: 'app-product-list',
  templateUrl: './product-list.component.html',
  styleUrls: ['./product-list.component.scss']
})
export class ProductListComponent implements OnInit {
  loading = false;
  saving = false;

  displayedColumns: string[] = [
    'id',
    'product_id',
    'name',
    'price_point_id',
    'price_point_value',
    'short_code',
    'created_at',
    'actions'
  ];

  dataSource = new MatTableDataSource<AdminProduct>([]);
  totalCount = 0;
  page = 1;
  pageSize = 20;
  pageSizes = [10, 20, 50, 100];

  filters = {
    q: '',
    short_code: ''
  };

  form: ProductMutationPayload = {
    product_id: '',
    name: '',
    price_point_id: 0,
    price_point_value: 0,
    short_code: ''
  };
  editingProductId: number | null = null;

  batchJson = `[
  {
    "product_id": "27188",
    "name": "Level 23 Daily",
    "price_point_id": 1,
    "price_point_value": 2.3,
    "short_code": "1234"
  }
]`;

  @ViewChild(MatPaginator) paginator!: MatPaginator;

  constructor(
    private productService: ProductService,
    private snackBar: MatSnackBar
  ) {}

  ngOnInit(): void {
    this.loadProducts();
  }

  loadProducts(): void {
    this.loading = true;
    this.productService.list({
      page: this.page,
      page_size: this.pageSize,
      q: this.filters.q || undefined,
      short_code: this.filters.short_code || undefined
    }).subscribe({
      next: (response) => {
        this.dataSource.data = response.products || [];
        this.totalCount = response.total_count || 0;
        this.loading = false;
      },
      error: () => {
        this.loading = false;
        this.toast('Failed to load products');
      }
    });
  }

  applyFilters(): void {
    this.page = 1;
    this.loadProducts();
  }

  clearFilters(): void {
    this.filters = { q: '', short_code: '' };
    this.page = 1;
    this.loadProducts();
  }

  onPageChange(event: PageEvent): void {
    this.page = event.pageIndex + 1;
    this.pageSize = event.pageSize;
    this.loadProducts();
  }

  editProduct(item: AdminProduct): void {
    this.editingProductId = item.id;
    this.form = {
      product_id: item.product_id,
      name: item.name,
      price_point_id: item.price_point_id,
      price_point_value: item.price_point_value,
      short_code: item.short_code
    };
  }

  resetForm(): void {
    this.editingProductId = null;
    this.form = {
      product_id: '',
      name: '',
      price_point_id: 0,
      price_point_value: 0,
      short_code: ''
    };
  }

  saveProduct(): void {
    if (!this.form.product_id || !this.form.name || !this.form.short_code || this.form.price_point_id <= 0) {
      this.toast('product_id, name, short_code, and price_point_id are required');
      return;
    }

    this.saving = true;
    const request$ = this.editingProductId
      ? this.productService.update(this.editingProductId, this.form)
      : this.productService.create(this.form);

    request$.subscribe({
      next: () => {
        this.saving = false;
        this.toast(this.editingProductId ? 'Product updated' : 'Product created');
        this.resetForm();
        this.loadProducts();
      },
      error: (err) => {
        this.saving = false;
        this.toast(this.extractErrorMessage(err, 'Failed to save product'));
      }
    });
  }

  deleteProduct(item: AdminProduct): void {
    const ok = confirm(`Delete product ${item.product_id}? This cannot be undone.`);
    if (!ok) {
      return;
    }

    this.productService.delete(item.id).subscribe({
      next: () => {
        this.toast('Product deleted');
        this.loadProducts();
      },
      error: (err) => {
        if (err?.status === 409 && err?.error?.dependency_counts) {
          const counts = err.error.dependency_counts as ProductDependencyCounts;
          this.toast(`Delete blocked. Campaigns: ${counts.campaign_count}, Subscriptions: ${counts.subscription_count}`);
          return;
        }
        this.toast(this.extractErrorMessage(err, 'Failed to delete product'));
      }
    });
  }

  runBatchUpsert(): void {
    let parsed: unknown;
    try {
      parsed = JSON.parse(this.batchJson);
    } catch {
      this.toast('Invalid batch JSON');
      return;
    }

    if (!Array.isArray(parsed) || parsed.length === 0) {
      this.toast('Batch JSON must be a non-empty array');
      return;
    }

    this.saving = true;
    this.productService.batchUpsert({ products: parsed as ProductMutationPayload[] }).subscribe({
      next: (res) => {
        this.saving = false;
        this.toast(`Batch upsert completed (${res.count})`);
        this.loadProducts();
      },
      error: (err) => {
        this.saving = false;
        this.toast(this.extractErrorMessage(err, 'Batch upsert failed'));
      }
    });
  }

  private extractErrorMessage(err: any, fallback: string): string {
    if (typeof err?.error === 'string' && err.error.trim()) {
      return err.error;
    }
    if (err?.error?.error) {
      return err.error.error;
    }
    if (err?.message) {
      return err.message;
    }
    return fallback;
  }

  private toast(message: string): void {
    this.snackBar.open(message, 'Close', { duration: 4000 });
  }
}

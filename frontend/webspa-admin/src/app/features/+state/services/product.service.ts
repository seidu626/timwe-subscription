import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from 'src/environments/environment';
import {
  AdminProduct,
  ProductBatchPayload,
  ProductFilters,
  ProductListResponse,
  ProductMutationPayload
} from '../models/product.model';

@Injectable({
  providedIn: 'root'
})
export class ProductService {
  private baseUrl = `${environment.acquisitionApiEndpoint}/v1/admin/products`;

  constructor(private http: HttpClient) {}

  list(filters?: ProductFilters): Observable<ProductListResponse> {
    let params = new HttpParams();
    if (filters) {
      if (filters.page) {
        params = params.set('page', filters.page.toString());
      }
      if (filters.page_size) {
        params = params.set('page_size', filters.page_size.toString());
      }
      if (filters.q) {
        params = params.set('q', filters.q);
      }
      if (filters.short_code) {
        params = params.set('short_code', filters.short_code);
      }
    }
    return this.http.get<ProductListResponse>(this.baseUrl, { params });
  }

  create(payload: ProductMutationPayload): Observable<AdminProduct> {
    return this.http.post<AdminProduct>(this.baseUrl, payload);
  }

  update(id: number, payload: ProductMutationPayload): Observable<AdminProduct> {
    return this.http.put<AdminProduct>(`${this.baseUrl}/${id}`, payload);
  }

  delete(id: number): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/${id}`);
  }

  batchUpsert(payload: ProductBatchPayload): Observable<{ message: string; count: number }> {
    return this.http.post<{ message: string; count: number }>(`${this.baseUrl}/batch`, payload);
  }
}

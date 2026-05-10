import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders, HttpParams } from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { map, catchError } from 'rxjs/operators';
import { LoaderService } from '../../shared/loader/loader.service';
import { Guid } from '../utils/guid';

@Injectable({
  providedIn: 'root',
})
export class DataService {
  constructor(private http: HttpClient, private loader: LoaderService) { }

  download(url: string, filename: string): Observable<void> {
    this.loader.show();
    return this.http.get(url, { responseType: 'blob', observe: 'response' }).pipe(
      map((response) => {
        const content = response.status === 204
          ? new Blob(['No content available.'], { type: 'text/plain' })
          : response.body!;
        const fileUrl = window.URL.createObjectURL(content);
        const a = document.createElement('a');
        document.body.appendChild(a);
        a.style.display = 'none';
        a.href = fileUrl;
        a.download = filename;
        a.click();
        window.URL.revokeObjectURL(fileUrl);
        a.remove();
      }),
      catchError(this.handleError),
      // Hide loader once the process completes or on error
      map(() => this.loader.hide())
    );
  }

  get(url: string, params?: HttpParams, observeHeaders: boolean = false): Observable<any> {
    this.loader.show();
    const options = this.createOptions(params, observeHeaders);

    return this.http.get(url, options).pipe(
      map((res: any) => {
        this.loader.hide();
        return res;
      }),
      catchError(this.handleError),
    );
  }

  post(url: string, data: any, params?: HttpParams): Observable<any> {
    return this.makeRequest('post', url, data, params);
  }

  postWithId(url: string, data: any, params?: HttpParams): Observable<any> {
    return this.makeRequest('post', url, data, params);
  }

  put(url: string, data: any, params?: HttpParams): Observable<any> {
    return this.makeRequest('put', url, data, params);
  }

  putWithId(url: string, data: any, params?: HttpParams): Observable<any> {
    return this.makeRequest('put', url, data, params);
  }

  patch(url: string, body: any, params?: HttpParams): Observable<any> {
    this.loader.show();
    const options = this.createOptions(params);

    return this.http.patch(url, body, options).pipe(
      map((res: any) => {
        this.loader.hide();
        return res;
      }),
      catchError(this.handleError),
    );
  }

  delete(url: string, params?: HttpParams): Observable<any> {
    this.loader.show();
    const options = this.createOptions(params);

    return this.http.delete(url, options).pipe(
      map((res: any) => {
        this.loader.hide();
        return res;
      }),
      catchError(this.handleError),
    );
  }

  private makeRequest(method: 'post' | 'put', url: string, data: any, params?: HttpParams): Observable<any> {
    this.loader.show();
    const options = this.createOptions(params);

    return this.http[method](url, data, options).pipe(
      map((res: any) => {
        this.loader.hide();
        return res;
      }),
      catchError(this.handleError)
    );
  }

  private handleError(error: any) {
    console.error(
      'Error:',
      `status: ${error?.status}, ` +
      `statusText: ${error?.statusText}, ` +
      `message: ${error?.message}`
    );
    return throwError(() => error);
  }

  private createOptions(params?: HttpParams, observeHeaders: boolean = false) {
    const headers = new HttpHeaders({
      'Content-Type': 'application/json',
      'x-requestid': Guid.newGuid(),
    });

    // Note: Authentication headers (Authorization Bearer) are added by HTTP interceptors
    const options: any = { headers, params };

    if (observeHeaders) {
      options['observe'] = 'response';
    }

    return options;
  }

}

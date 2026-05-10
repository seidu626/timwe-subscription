import { environment } from '@env/environment';
import { Injectable } from '@angular/core';
import {
  HttpClient,
  HttpHeaders,
  HttpErrorResponse
} from '@angular/common/http';
import {
  Observable,
  Subject,
  BehaviorSubject,
  Subscription,
  Subscriber,
  of
} from 'rxjs';
import { IConfiguration } from '@app/core/models/configuration.model';
import { StoreSettings } from '../../shared/models/store-settings.model';

@Injectable()
export class StoreSettingsService {
  serverSettings: StoreSettings = new Object() as StoreSettings;
  private settingsLoadedSource = new BehaviorSubject<StoreSettings>(null);
  settingsLoaded$ = this.settingsLoadedSource.asObservable();

  private checkOutSource = new BehaviorSubject<boolean>(false);
  isCheckedOut$ = of(false); // this.checkOutSource.asObservable();
  isReady = false;
  storeUrl: string;
  settingsUrl: string;

  constructor(
    private http: HttpClient
  ) {
    this.storeUrl = environment.baseApiEndpoint + '/api/store';
    this.settingsUrl = environment.baseApiEndpoint + '/api/settings';
    this.isReady = true;
  }

  load() {
    return this.http.get(this.settingsUrl + '/StoreSettings').subscribe((response) => {
      console.log('STORE settings loaded');
      this.serverSettings = response as StoreSettings;
      this.isReady = true;
      this.settingsLoadedSource.next(this.serverSettings);
    });
  }

  isCheckedOut() {
    return this.http.get(this.storeUrl + '/checkedout').subscribe((response: boolean) => {
      this.checkOutSource.next(response);
    });
  }
}

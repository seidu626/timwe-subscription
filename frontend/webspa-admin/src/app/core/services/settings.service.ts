import { Injectable } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { GeneralSettings } from '../models/general-settings';
import { SettingInfo } from '../models/settingInfo';
import { APP_SETTINGS } from '../models/app-settings';
import { DataService } from '.';
import { environment } from '../../../environments/environment';


@Injectable()
export class SettingsService {
  generalSettings: GeneralSettings = new Object() as GeneralSettings;
  settings: SettingInfo[] = [];

  private settingsLoadedSource = new BehaviorSubject<GeneralSettings>({} as GeneralSettings);
  // eslint-disable-next-line @typescript-eslint/member-ordering
  settingsLoaded$ = this.settingsLoadedSource.asObservable();

  constructor(private service: DataService) {  }

  public getJSON(): Observable<GeneralSettings> {
    // https://gist.github.com/keeguon/2310008
    return this.service
      .get(`${window.location.origin}/assets/appsettings.json`)
      .pipe(map((res: any) => res));
  }

  // https://github.com/IntertechInc/angular-app-initializer
  getSettings(): Promise<any> {
    // if (!APP_SETTINGS.generalSettings) {return  Promise.resolve(); }
    const promise = this.getJSON()
      .toPromise()
      .then(settings => {
        APP_SETTINGS.generalSettings = settings as GeneralSettings;
        return settings;
      });

    return promise;
  }

  // https://github.com/IntertechInc/angular-app-initializer
  getApiSettings(): Promise<any> {
    // if (!APP_SETTINGS.generalSettings) {return  Promise.resolve(); }
    const promise = this.getGeneralSettings()
      .toPromise()
      .then(settings => {
        APP_SETTINGS.generalSettings = settings as GeneralSettings;
        return settings;
      });

    return promise;
  }

  getAll(options?: any): Observable<SettingInfo[]> {
    const url = environment.baseApiEndpoint + '/settings';
    return this.service.get(url).pipe(
      map((payload: any) => {
        this.settings = payload;
        return payload;
      })
    );
  }

  get(id: number): Observable<SettingInfo>  {
    const url = environment.baseApiEndpoint + `/settings/${id}`;
    return this.service.get(url).pipe(
      map((payload: any) => payload)
    );
  }

  delete(id: number): Observable<any>  {
    const url = environment.baseApiEndpoint + `/settings/${id}`;
    return this.service.delete(url).pipe(
      map((payload) => payload)
    );
  }

  post(payload: any): Observable<Response> {
    return this.service.post('', payload);
  }

  put(id: any, payload: any): Observable<Response> {
    return this.service.put(id, payload);
  }

  getGeneralSettings(): Observable<GeneralSettings> {
    const url = environment.baseApiEndpoint + 'settings/GeneralSettings';
    return this.service.get(url).pipe(
      map((payload: any) => {
        this.settingsLoadedSource.next(payload);
        return payload;
      })
    );
  }

  postGeneralSettings(body: any, options?: any): Observable<Response> {
    const url = environment.baseApiEndpoint + '/settings/GeneralSettings';
    return this.service.post(url, body);
  }
}

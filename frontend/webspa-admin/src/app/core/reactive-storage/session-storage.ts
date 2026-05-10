import { Observable } from 'rxjs';
import { Injectable } from '@angular/core';
import { Store } from './store';

@Injectable()
export class SessionStorage {
  constructor(private store: Store) {}

  setItem<T>(key: string, payload: T) {
    this.store.setItem(key, payload);
  }

  getItem<T>(key: string): Observable<T | null> {
    return this.store.getItem<T>(key);
  }

  removeItem(key: string) {
    this.store.removeItem(key);
  }

}

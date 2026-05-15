import { Injectable } from '@angular/core';
import { catchError, map, Observable, throwError } from 'rxjs';
import { Notification, NotificationPagedResponse } from '../models/notification.model';
import { DataService } from '../../../core/services';
import { HttpParams, HttpClient } from '@angular/common/http';
import { environment } from '../../../../environments/environment';


@Injectable({
    providedIn: 'root'
})
export class NotificationService {
    private baseUrl = environment.notificationApiEndpoint + '/api/v1/notification';

    constructor(private http: HttpClient, private dataService: DataService) { }

    getNotifications(filters: any): Observable<NotificationPagedResponse> {
        let params = new HttpParams();
        Object.keys(filters).forEach(key => {
            if (filters[key] !== undefined && filters[key] !== null && filters[key] !== '') {
                params = params.set(key, filters[key]);
                if (key === 'entryChannel') {
                    params = params.set('entry_channel', filters[key]);
                }
            }
        });

        return this.dataService.get(`${this.baseUrl}/list`, params, true).pipe(
            map((response: any) => {
                if (!response || !response.headers) {
                    throw new Error("Missing headers in the response");
                }

                const result: NotificationPagedResponse = {
                    pageSize: +response.body.pageSize || 10,
                    page: +response.body.page || 1,
                    totalCount: +response.body.totalCount || 0,
                    data: response.body.data || [],
                    totalPages: response.body.totalPages || 1,
                };

                return result;
            }),
            catchError(error => {
                console.error('Error fetching notifications:', error);
                return throwError(() => error);
            })
        );
    }

    getNotificationById(id: number): Observable<Notification> {
        return this.dataService.get(`${this.baseUrl}/${id}`);
    }
}

/* eslint-disable @typescript-eslint/member-ordering */
import { Injectable } from '@angular/core';
import { DataService } from '../../../core/services/data.service';
import { BehaviorSubject, Observable } from 'rxjs';
import { map, catchError } from 'rxjs/operators';
import { HttpClient } from '@angular/common/http';
import { environment } from '../../../../environments/environment';
import { renewAfterTimer } from '../../../core/utils/refresh-subscription';
import { Redemption, VoucherRedemptionCount, SerialReport } from '../models/voucher.model';
const endpoint = environment.baseApiEndpoint;

@Injectable()
export class ReportService {
    private apiUrl = endpoint + '/api/reports';

    constructor(
        private http: HttpClient,
        private dataService: DataService
    ) { }

    getSerialReportByVoucherGroup(): Observable<SerialReport[]> {
        return this.dataService.get(`${this.apiUrl}/GetSerialReportByVoucherGroup`);
    }

    getRedeemedSerialReportByVoucherGroup(): Observable<SerialReport[]> {
        return this.dataService.get(`${this.apiUrl}/GetRedeemedSerialReportByVoucherGroup`);
    }

    // Function to get redemption details by staff within a period
    getRedemptionsByStaff(userName: string, startDate?: string, endDate?: string): Observable<Redemption[]> {
        return this.dataService.get(`${this.apiUrl}/redemptionsByStaff?userName=${userName}&startDate=${startDate}&endDate=${endDate}`);
    }

    // Function to get total redemptions within a period
    getTotalRedemptions(startDate?: string, endDate?: string): Observable<VoucherRedemptionCount> {
        return this.dataService.get(`${this.apiUrl}/totalRedemptions?startDate=${startDate}&endDate=${endDate}`, { params: { startDate, endDate } });
    }

    getRedemptionCountOverTime(startDate?: string, endDate?: string): Observable<{ date: string, value: number }[]> {
        return this.dataService.get(`${this.apiUrl}/redemptionCountOverTime?startDate=${startDate}&endDate=${endDate}`, { params: { startDate, endDate } })
            .pipe(map(response => {
                let redemptionList: any[] = [];
                let index = 0;
                Object.keys(response).forEach(key => {
                    redemptionList[index] = { date: key, value: response[key] };
                    index++;
                });
                return redemptionList;
            }));
    }

    getRedemptionValueOverTime(startDate?: string, endDate?: string): Observable<{ date: string, value: number }[]> {
        return this.dataService.get(`${this.apiUrl}/redemptionValueOverTime?startDate=${startDate}&endDate=${endDate}`, { params: { startDate, endDate } })
            .pipe(map(response => {
                let redemptionList: any[] = [];
                let index = 0;
                Object.keys(response).forEach(key => {
                    redemptionList[index] = { date: key, value: response[key] };
                    index++;
                });
                return redemptionList;
            }));
    }


    getRedemptionCountByVoucherGroup(startDate?: string, endDate?: string): Observable<{ group: string, value: number }[]> {
        const params: { [key: string]: string } = {};

        if (startDate) {
            params['startDate'] = startDate;
        }

        if (endDate) {
            params['endDate'] = endDate;
        }

        return this.http.get<{ [key: string]: number }>(`${this.apiUrl}/redemptionCountByVoucherGroup`, { params })
            .pipe(
                map((response: any) => {
                    const groupList: { group: string, value: number }[] = [];
                    Object.keys(response).forEach((key, index) => {
                        groupList[index] = { group: key, value: response[key] };
                    });
                    return groupList;
                })
            );
    }



    getStaffRedemptionActivity(startDate?: string, endDate?: string): Observable<{ staff: string, count: number }[]> {
        const params: { [key: string]: string | number | boolean } = {};

        if (startDate) {
            params['startDate'] = startDate;
        }

        if (endDate) {
            params['endDate'] = endDate;
        }

        return this.http.get<{ [key: string]: number }>(`${this.apiUrl}/staffRedemptionActivity`, { params })
            .pipe(
                map((response: any) => {
                    const activityList: { staff: string, count: number }[] = [];
                    Object.keys(response).forEach((key, index) => {
                        activityList[index] = { staff: key, count: response[key] };
                    });
                    return activityList;
                })
            );
    }


}


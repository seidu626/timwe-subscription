import { Injectable } from '@angular/core';
import { Observable, of } from 'rxjs';
import { map, catchError } from 'rxjs/operators';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { environment } from '../../../../environments/environment';
import { DataService } from '../../../core/services';
import { Redemption, RedemptionPagedResponse, Serial, createRedemption, createSerial } from '../models/voucher.model';
const endpoint = environment.baseApiEndpoint;

@Injectable({
    providedIn: 'root'
})
export class VoucherRedemptionService {
    private url = endpoint + '/api/voucherRedemption';

    constructor(
        private http: HttpClient,
        private dataService: DataService
    ) { }

    /**
     *
     * @param null
     * @returns {Observable<Observable<any[]>}
     */
    getAll(): Observable<Array<any>> {
        const url = this.url;
        return this.dataService.get(url);
    }


    getRedemptions(pageIndex: number, pageSize: number, search?: string, sortBy?: string): Observable<RedemptionPagedResponse> {
        let url = this.url;
        const headers = new HttpHeaders({ observe: 'response' });
        const options = { responseType: 'json', headers: headers }
        url = url + '?pageIndex=' + pageIndex + '&pageSize=' + pageSize + '&search=' +
            ((search) ? search.toString() : '') + '&sortBy=' + ((sortBy) ? sortBy.toString() : '');
        return this.http.get(url, { observe: 'response' }).pipe(
            map((response: any) => {
                const responseHeaders = JSON.parse(response.headers.get('x-pagination'));
                const result: RedemptionPagedResponse = { ...responseHeaders, data: this.parseRedemption(response) };
                return result;
            })
        );
    }
    
    parseRedemption(response: any): Redemption[] {
        return response.body.map((r: { id: any; idNumber: string, serialNo: any; voucherGroup: any; value: any; pin: any; msisdn: any;
            firstName: string; lastName: string; otherDetails: string; userName: string; createdAt: Date  }) =>
            createRedemption({
                id: r.id, serialNo: r.serialNo, msisdn: r.msisdn,
                idNumber: r.idNumber,
                voucherGroup: r.voucherGroup, value: r.value, 
                pin: r.pin, firstName: r.firstName, lastName: r.lastName, 
                otherDetails: r.otherDetails, userName: r.userName, createdAt: r.createdAt
            }));
    }


    get(param: any): Observable<any> {
        const path = this.url + '/' + param;
        return this.dataService.get(path);
    }

    put(param: any = null, body: object = {}): Observable<any> {
        const path = param ? this.url + '/' + param : this.url;
        return this.dataService.put(path, body);
    }

    patch(param: any, body: object = {}): Observable<any> {
        const path = this.url + '/' + param;
        return this.dataService.patch(path, body);
    }

    post({ body = {}, path = '' }: { body?: object; path?: string; } = {}): Observable<any> {
        let url = this.url;
        if (path) {
            url = url + path;
        }
        return this.dataService.post(url, body);
    }

    delete(param: any): Observable<any> {
        const path = this.url + '/' + param;
        return this.dataService.delete({ url: path });
    }
}


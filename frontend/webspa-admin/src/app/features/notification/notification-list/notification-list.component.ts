// notification-list.component.ts
import { Component, OnInit, ViewChild, AfterViewInit } from '@angular/core';
import { MatPaginator } from '@angular/material/paginator';
import { MatTableDataSource } from '@angular/material/table';
import { MatSort } from '@angular/material/sort';
import { MatSnackBar } from '@angular/material/snack-bar';
import { NotificationService } from '../../+state/services/notification.service';

@Component({
    selector: 'app-notification-list',
    templateUrl: './notification-list.component.html',
    styleUrls: ['./notification-list.component.scss']
})
export class NotificationListComponent implements OnInit, AfterViewInit {
    loading: boolean = false;
    displayedColumns: string[] = ['id', 'partnerRole', 'msisdn', 'entryChannel', 'type', 'createdAt'];
    dataSource = new MatTableDataSource<any>([]);
    totalRecords = 0;
    pageSizes = [5, 10, 20, 30];

    filters = {
        startDate: '',
        endDate: '',
        partnerRole: '',
        msisdn: '',
        type: '',
        entryChannel: ''
    };

    @ViewChild(MatPaginator) paginator!: MatPaginator;
    @ViewChild(MatSort) sort!: MatSort;

    constructor(
        private notificationService: NotificationService,
        private snackBar: MatSnackBar
    ) { }

    ngOnInit() {
        this.loadNotifications(1, 10, this.filters);
    }

    ngAfterViewInit(): void {
        this.dataSource.paginator = this.paginator;
        this.dataSource.sort = this.sort;
    }

    loadNotifications(page: number, pageSize: number, filters: any) {
        this.loading = true;
        const formattedFilters = {
            ...filters,
            page: page,
            pageSize: pageSize,
            startDate: this.toDateQuery(filters.startDate),
            endDate: this.toDateQuery(filters.endDate)
        };
        this.notificationService.getNotifications(formattedFilters).subscribe({
            next: (response) => {
                this.dataSource.data = response.data;
                this.totalRecords = response.totalCount;
                this.loading = false;
            },
            error: (err) => {
                this.loading = false;
                this.snackBar.open('Failed to load notifications', 'Close', {
                    duration: 5000,
                    panelClass: ['error-snackbar']
                });
            }
        });
    }

    applyFilters() {
        this.loadNotifications(1, this.paginator?.pageSize || 10, this.filters);
    }

    onPageChange(event: any) {
        this.loadNotifications(event.pageIndex + 1, event.pageSize, this.filters);
    }

    private toDateQuery(value: any): string {
        if (!value) return '';
        const date = value instanceof Date ? value : new Date(value);
        if (Number.isNaN(date.getTime())) return '';
        const year = date.getFullYear();
        const month = `${date.getMonth() + 1}`.padStart(2, '0');
        const day = `${date.getDate()}`.padStart(2, '0');
        return `${year}-${month}-${day}`;
    }
}

// notification-list.component.ts
import { Component, OnInit, ViewChild } from '@angular/core';
import { MatPaginator } from '@angular/material/paginator';
import { MatTableDataSource } from '@angular/material/table';
import { MatSnackBar } from '@angular/material/snack-bar';
import { Sort } from '@angular/material/sort';
import { NotificationService } from '../../+state/services/notification.service';

@Component({
    selector: 'app-notification-list',
    templateUrl: './notification-list.component.html',
    styleUrls: ['./notification-list.component.scss']
})
export class NotificationListComponent implements OnInit {
    loading: boolean = false;
    displayedColumns: string[] = ['id', 'partnerRole', 'msisdn', 'entryChannel', 'type', 'createdAt'];
    dataSource = new MatTableDataSource<any>([]);
    totalRecords = 0;
    pageIndex = 0;
    pageSize = 10;
    pageSizes = [5, 10, 20, 30];

    filters = {
        startDate: '',
        endDate: '',
        partnerRole: '',
        msisdn: '',
        type: '',
        entryChannel: ''
    };

    sortBy = 'createdAt';
    sortDir: 'asc' | 'desc' = 'desc';

    @ViewChild(MatPaginator) paginator!: MatPaginator;

    constructor(
        private notificationService: NotificationService,
        private snackBar: MatSnackBar
    ) { }

    ngOnInit() {
        this.loadNotifications(this.pageIndex + 1, this.pageSize, this.filters);
    }

    trackById = (_: number, row: any) => row?.id ?? _;

    loadNotifications(page: number, pageSize: number, filters: any) {
        this.loading = true;
        const formattedFilters = {
            ...filters,
            page: page,
            pageSize: pageSize,
            sort_by: this.sortBy,
            sort_dir: this.sortDir,
            startDate: this.toDateQuery(filters.startDate),
            endDate: this.toDateQuery(filters.endDate)
        };
        this.notificationService.getNotifications(formattedFilters).subscribe({
            next: (response) => {
                this.dataSource.data = response.data;
                this.totalRecords = response.totalCount;
                this.pageIndex = (response.page || page) - 1;
                this.pageSize = response.pageSize || pageSize;
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
        this.pageIndex = 0;
        this.loadNotifications(1, this.pageSize, this.filters);
    }

    onPageChange(event: any) {
        this.pageIndex = event.pageIndex;
        this.pageSize = event.pageSize;
        this.loadNotifications(event.pageIndex + 1, event.pageSize, this.filters);
    }

    onSortChange(event: Sort) {
        this.sortBy = event.active || 'createdAt';
        this.sortDir = (event.direction || 'desc') as 'asc' | 'desc';
        this.pageIndex = 0;
        this.loadNotifications(1, this.pageSize, this.filters);
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

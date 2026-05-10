import { Component } from '@angular/core';

@Component({
  selector: 'app-dashboard',
  templateUrl: './dashboard.component.html',
  styleUrls: ['./dashboard.component.scss']
})
export class DashboardComponent {
  activeFilters: any = {};

  filters = [
    { name: 'Start Date', type: 'date', key: 'startDate' },
    { name: 'End Date', type: 'date', key: 'endDate' },
    { name: 'Shortcode', type: 'text', key: 'shortcode' },
    { name: 'Product', type: 'text', key: 'product' }
  ];

  onFilterChange(filters: any) {
    this.activeFilters = filters;
  }
}

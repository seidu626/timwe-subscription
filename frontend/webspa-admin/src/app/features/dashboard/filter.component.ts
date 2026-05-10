import { Component, Input, Output, EventEmitter } from '@angular/core';

@Component({
  selector: 'app-filter',
  templateUrl: './filter.component.html',
  styleUrls: ['./filter.component.scss']
})
export class FilterComponent {
  @Input() filters: any[] = [];
  @Output() filterChange = new EventEmitter<any>();

  filterValues: any = {};

  onFilterChange() {
    this.filterChange.emit(this.filterValues);
  }
}

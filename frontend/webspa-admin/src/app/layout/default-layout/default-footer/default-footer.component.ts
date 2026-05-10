import { DatePipe } from '@angular/common';
import { Component } from '@angular/core';
import { FooterComponent } from '@coreui/angular';

@Component({
    selector: 'app-default-footer',
    templateUrl: './default-footer.component.html',
    styleUrls: ['./default-footer.component.scss'],
    standalone: true, 
    imports: [DatePipe],
})
export class DefaultFooterComponent extends FooterComponent {
  date = new Date();
  company = 'Nouveauriche Global';
  companyUrl = 'http://nouveauricheglobalgroup.com/';


  constructor() {
    super();    
  }
}

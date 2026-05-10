import { CompanyInformationSettings } from './company-information-settings';
import { SocialSettings } from './social-settings';
import { ContactDataSettings } from './contact-data-settings';

export interface GeneralSettings {
  underMaintenance: boolean;
  dateClosed: string;
  isClosed: boolean;
  companyInformationSettings: CompanyInformationSettings;
  contactDataSettings: ContactDataSettings;
  socialSettings: SocialSettings;
}

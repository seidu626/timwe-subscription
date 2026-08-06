export const SMS_TEMPLATE_EVENT_TYPES = {
  USER_OPTIN: 'USER_OPTIN',
} as const;

export type SMSTemplateEventType = typeof SMS_TEMPLATE_EVENT_TYPES[keyof typeof SMS_TEMPLATE_EVENT_TYPES];

export interface SMSTemplate {
  id: number;
  tenantId: string;
  productId: number;
  eventType: SMSTemplateEventType;
  enabled: boolean;
  template: string;
  createdAt: string;
  updatedAt: string;
}

export interface SMSTemplateUpsert {
  eventType: SMSTemplateEventType;
  enabled: boolean;
  template: string;
}

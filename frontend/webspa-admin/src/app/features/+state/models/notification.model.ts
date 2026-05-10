// models/notification.model.ts
export interface Notification {
    id: number;
    partnerRole: string;
    productId: number;
    pricepointId: number;
    mcc: string;
    mnc: string;
    msisdn: string;
    entryChannel: string;
    transactionUUID: string;    
    notificationType: string;
    tags: string[];
    timestamp: Date;
    status: string;
    type: string;
    createdAt: Date;
}

export interface NotificationPagedResponse {
    pageSize: number;
    page: number;
    data: Notification[];
    totalCount: number;
    totalPages: number;
}
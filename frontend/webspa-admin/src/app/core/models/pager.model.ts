export interface IPager {
    itemsPage: number;
    totalItems: number;
    actualPage: number;
    totalPages: number;
    items: number;
}

export class PageModel {
    pageIndex: number;
    pageSize: number;
    count: number;
}

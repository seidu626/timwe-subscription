export interface PagedDataResponse<T> {
    pageSize: number;
    page: number;
    data: Array<T>;
    totalCount: number;
    totalPages: number;
}

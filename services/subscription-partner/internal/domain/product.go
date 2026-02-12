package domain

import "time"

type ProductRequest struct {
	ProductId       string  `json:"productId"`
	Name            string  `json:"name"`
	PricePointId    int     `json:"pricePointId"`
	PricePointValue float64 `json:"pricePointValue"`
	ShortCode       string  `json:"shortCode"`
}

type Product struct {
	Id              int       `json:"id"`
	ProductId       string    `json:"productId"`
	Name            string    `json:"name"`
	PricePointId    int       `json:"pricePointId"`
	PricePointValue float64   `json:"pricePointValue"`
	ShortCode       string    `json:"shortCode"`
	CreatedAt       time.Time `json:"createdAt"`
}

type ListProductResponse struct {
	Data         []*Product `json:"products"`
	TotalRecords int        `json:"totalRecords"`
	PageSize     int        `json:"pageSize"`
	Page         int        `json:"page"`
}

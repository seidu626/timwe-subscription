package handler

import (
	"encoding/json"
	"github.com/seidu626/subscription-manager/subscription/internal/domain"
	"github.com/seidu626/subscription-manager/subscription/internal/service"
	"github.com/valyala/fasthttp"
	"net/http"
	"strconv"
)

type ProductHandler struct {
	service *service.ProductService
}

func NewProductHandler(service *service.ProductService) *ProductHandler {
	return &ProductHandler{service: service}
}

// BatchCreateProducts handles the creation of multiple products at once
func (h *ProductHandler) BatchCreateProducts(ctx *fasthttp.RequestCtx) {
	var products []domain.ProductRequest
	if err := json.Unmarshal(ctx.PostBody(), &products); err != nil {
		ctx.Error("Invalid request payload", fasthttp.StatusBadRequest)
		return
	}

	if err := h.service.BatchCreateProducts(products); err != nil {
		ctx.Error("Failed to create products", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetStatusCode(http.StatusCreated)
	ctx.SetBody([]byte(`{"message": "Products created successfully"}`))
}

func (h *ProductHandler) CreateProduct(ctx *fasthttp.RequestCtx) {
	var productReq domain.ProductRequest
	if err := json.Unmarshal(ctx.PostBody(), &productReq); err != nil {
		ctx.Error("Invalid request payload", fasthttp.StatusBadRequest)
		return
	}

	product, err := h.service.CreateProduct(&productReq)
	if err != nil {
		ctx.Error("Error creating product", fasthttp.StatusInternalServerError)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusCreated)
	if err := json.NewEncoder(ctx).Encode(product); err != nil {
		ctx.Error("Error formatting response", fasthttp.StatusInternalServerError)
	}
}

func (h *ProductHandler) ListProducts(ctx *fasthttp.RequestCtx) {
	page, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("page")))
	if page == 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("pageSize")))
	if pageSize == 0 {
		pageSize = 10
	}

	products, err := h.service.ListProducts(page, pageSize)
	if err != nil {
		ctx.Error("Error fetching products", fasthttp.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(ctx).Encode(products); err != nil {
		ctx.Error("Error formatting response", fasthttp.StatusInternalServerError)
	}
}

func (h *ProductHandler) GetProduct(ctx *fasthttp.RequestCtx) {
	idParam := string(ctx.QueryArgs().Peek("id"))
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.Error("Invalid product ID", fasthttp.StatusBadRequest)
		return
	}

	product, err := h.service.GetProductByID(id)
	if err != nil {
		ctx.Error("Error retrieving product", fasthttp.StatusInternalServerError)
		return
	}

	if product == nil {
		ctx.Error("Product not found", fasthttp.StatusNotFound)
		return
	}

	if err := json.NewEncoder(ctx).Encode(product); err != nil {
		ctx.Error("Error formatting response", fasthttp.StatusInternalServerError)
	}
}

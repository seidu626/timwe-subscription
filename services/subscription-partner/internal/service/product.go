package service

import (
	"fmt"
	"github.com/seidu626/subscription-manager/subscription/internal/domain"
	"github.com/seidu626/subscription-manager/subscription/internal/repository"
)

type ProductService struct {
	repo *repository.ProductRepository
}

func NewProductService(repo *repository.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) CreateProduct(productReq *domain.ProductRequest) (*domain.Product, error) {
	product := &domain.Product{
		ProductId:       productReq.ProductId,
		Name:            productReq.Name,
		PricePointId:    productReq.PricePointId,
		PricePointValue: productReq.PricePointValue,
		ShortCode:       productReq.ShortCode,
	}
	if err := s.repo.CreateProduct(product); err != nil {
		return nil, err
	}
	return product, nil
}

func (s *ProductService) GetProductByID(id int) (*domain.Product, error) {
	return s.repo.GetProductByID(id)
}

func (s *ProductService) ListProducts(page, pageSize int) (*domain.ListProductResponse, error) {
	return s.repo.ListProducts(page, pageSize)
}

// BatchCreateProducts adds multiple products to the database
func (s *ProductService) BatchCreateProducts(products []domain.ProductRequest) error {
	if len(products) == 0 {
		return nil
	}

	// Perform validation if needed
	for _, product := range products {
		if product.ProductId == "" || product.Name == "" {
			return fmt.Errorf("product ID and name are required for each product")
		}
	}

	return s.repo.BatchCreateProducts(products)
}

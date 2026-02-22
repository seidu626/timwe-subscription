package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	cached "github.com/seidu626/subscription-manager/common/cache"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/seidu626/subscription-manager/subscription/internal/domain"
)

type ProductRepository struct {
	db    *sql.DB
	redis cached.RedisClient
	ctx   context.Context
}

func NewProductRepository(db *sql.DB, client cached.RedisClient) *ProductRepository {
	return &ProductRepository{db: db,
		redis: client,
		ctx:   context.Background(),
	}
}

func (r *ProductRepository) CreateProduct(product *domain.Product) error {
	query := `
		INSERT INTO products (product_id, name, price_point_id, price_point_value, short_code)
		VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`
	err := r.db.QueryRow(query, product.ProductId, product.Name, product.PricePointId, product.PricePointValue, product.ShortCode).
		Scan(&product.Id, &product.CreatedAt)
	return err
}

func (r *ProductRepository) ListProducts(page, pageSize int) (*domain.ListProductResponse, error) {
	cacheKey := fmt.Sprintf("__ALL_%d_%d_PRODUCTS__", page, pageSize)
	cachedData, err := r.redis.Get(r.ctx, cacheKey)
	if err == nil {
		var listResponse *domain.ListProductResponse
		if err := json.Unmarshal([]byte(cachedData), &listResponse); err == nil {
			return listResponse, nil
		}
	} else if !errors.Is(err, redis.Nil) {
		// Only log actual Redis errors, not cache misses
		log.Printf("Failed to find cached data: %+v", err.Error())
	}
	// If err == redis.Nil, it's a cache miss - proceed to database lookup

	offset := (page - 1) * pageSize
	query := `SELECT id, product_id, name, price_point_id, price_point_value, short_code, created_at 
              FROM products ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*domain.Product
	for rows.Next() {
		product := &domain.Product{}
		if err := rows.Scan(&product.Id, &product.ProductId, &product.Name, &product.PricePointId, &product.PricePointValue, &product.ShortCode, &product.CreatedAt); err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	countQuery := `SELECT COUNT(*) FROM products`
	var totalRecords int
	err = r.db.QueryRow(countQuery).Scan(&totalRecords)
	if err != nil {
		return nil, err
	}

	listResponse := &domain.ListProductResponse{
		Data:         products,
		TotalRecords: totalRecords,
		Page:         page,
		PageSize:     pageSize,
	}

	// Cache the results for future use
	data, err := json.Marshal(listResponse)
	if err == nil {
		_ = r.redis.Set(r.ctx, cacheKey, data, 30*time.Minute)
	}

	return listResponse, nil
}

func (r *ProductRepository) GetProductByID(id int) (*domain.Product, error) {
	query := `SELECT id, product_id, name, price_point_id, price_point_value, short_code, created_at FROM products WHERE id = $1`
	product := &domain.Product{}
	err := r.db.QueryRow(query, id).Scan(
		&product.Id,
		&product.ProductId,
		&product.Name,
		&product.PricePointId,
		&product.PricePointValue,
		&product.ShortCode,
		&product.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return product, err
}

// BatchCreateProducts inserts multiple products into the database
func (r *ProductRepository) BatchCreateProducts(products []domain.ProductRequest) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	query := `INSERT INTO products (product_id, name, price_point_id, price_point_value, short_code) VALUES ($1, $2, $3, $4, $5)`
	for _, product := range products {
		_, err := tx.Exec(query, product.ProductId, product.Name, product.PricePointId, product.PricePointValue, product.ShortCode)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

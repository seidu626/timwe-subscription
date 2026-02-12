package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/seidu626/subscription-manager/subscription-external/internal/domain"
	"go.uber.org/zap"
)

type ProductRepository struct {
	db     *sql.DB
	logger *zap.Logger
	redis  *redis.Client
	ctx    context.Context
}

func NewProductRepository(db *sql.DB, logger *zap.Logger, client *redis.Client) *ProductRepository {
	return &ProductRepository{db: db,
		logger: logger,
		redis:  client,
		ctx:    context.Background(),
	}
}

func (r *ProductRepository) GetProducts() ([]*domain.Product, error) {
	cacheKey := "__ALL__PRODUCTS__"
	cachedData, err := r.redis.Get(r.ctx, cacheKey).Result()
	if err == nil {
		var listResponse []*domain.Product
		if err := json.Unmarshal([]byte(cachedData), &listResponse); err == nil {
			r.logger.Debug("Cache hit for all products", zap.String("cacheKey", cacheKey))
			return listResponse, nil
		}
		r.logger.Debug("Cache hit but failed to unmarshal data", zap.String("cacheKey", cacheKey))
	} else if !errors.Is(err, redis.Nil) {
		// Only log actual Redis errors, not cache misses
		r.logger.Error("Failed to find cached data: ", zap.Error(err))
	} else {
		r.logger.Debug("Cache miss for all products", zap.String("cacheKey", cacheKey))
	}
	// If err == redis.Nil, it's a cache miss - proceed to database lookup

	query := `SELECT id, product_id, name, price_point_id, price_point_value, short_code, created_at 
              FROM products ORDER BY created_at`
	rows, err := r.db.Query(query)
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

	// Cache the results for future use
	data, err := json.Marshal(products)
	if err == nil {
		if setErr := r.redis.Set(r.ctx, cacheKey, data, 30*time.Minute).Err(); setErr != nil {
			r.logger.Warn("Failed to cache products data", zap.Error(setErr))
		} else {
			r.logger.Debug("Successfully cached all products", zap.String("cacheKey", cacheKey), zap.Int("count", len(products)))
		}
	}

	return products, nil
}

// GetProductsByIds retrieves a list of products based on provided product IDs.
func (r *ProductRepository) GetProductsByIds(productIds []string) ([]*domain.Product, error) {
	if len(productIds) == 0 {
		return nil, fmt.Errorf("no product IDs provided")
	}

	// Create context with timeout for database operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check cache first
	cacheKey := fmt.Sprintf("products:ids:%s", strings.Join(productIds, ","))
	cachedData, err := r.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache hit - unmarshal and return
		var products []*domain.Product
		if unmarshalErr := json.Unmarshal([]byte(cachedData), &products); unmarshalErr == nil {
			r.logger.Debug("Cache hit for products by IDs", zap.String("cacheKey", cacheKey), zap.Strings("productIds", productIds), zap.Int("count", len(products)))
			return products, nil
		}
		r.logger.Warn("Failed to unmarshal cached products data", zap.Error(err))
	}
	// If err == redis.Nil, it's a cache miss - proceed to database lookup

	// Prepare placeholders for the SQL IN clause based on the number of IDs
	placeholders := make([]string, len(productIds))
	args := make([]interface{}, len(productIds))

	for i, id := range productIds {
		placeholders[i] = fmt.Sprintf("$%d", i+1) // Use PostgreSQL-style placeholders
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, product_id, name, price_point_id, price_point_value, short_code, created_at 
		FROM products 
		WHERE product_id IN (%s)
		ORDER BY created_at`, strings.Join(placeholders, ","))

	// Use context-aware query with timeout
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query products by IDs: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			r.logger.Warn("Failed to close rows", zap.Error(err))
		}
	}(rows)

	var products []*domain.Product
	for rows.Next() {
		product := &domain.Product{}
		if err := rows.Scan(
			&product.Id,
			&product.ProductId,
			&product.Name,
			&product.PricePointId,
			&product.PricePointValue,
			&product.ShortCode,
			&product.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating through products: %w", err)
	}

	// Cache the results for future use
	data, err := json.Marshal(products)
	if err == nil {
		if setErr := r.redis.Set(r.ctx, cacheKey, data, 30*time.Minute).Err(); setErr != nil {
			r.logger.Warn("Failed to cache products data", zap.Error(setErr))
		} else {
			r.logger.Debug("Successfully cached products by IDs", zap.String("cacheKey", cacheKey), zap.Strings("productIds", productIds), zap.Int("count", len(products)))
		}
	}

	return products, nil
}

func (r *ProductRepository) GetProductByID(id int) (*domain.Product, error) {
	query := `SELECT id, product_id, name, price_point_id, price_point_value, short_code, created_at FROM products WHERE product_id = $1`
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
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return product, err
}

// GetProduct retrieves a product by string product ID
func (r *ProductRepository) GetProduct(productID string) (*domain.Product, error) {
	query := `SELECT id, product_id, name, price_point_id, price_point_value, short_code, created_at FROM products WHERE product_id = $1`
	product := &domain.Product{}
	err := r.db.QueryRow(query, productID).Scan(
		&product.Id,
		&product.ProductId,
		&product.Name,
		&product.PricePointId,
		&product.PricePointValue,
		&product.ShortCode,
		&product.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return product, err
}

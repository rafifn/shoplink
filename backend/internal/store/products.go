package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Product is one of a seller's products. PriceSen is rupiah in sen.
type Product struct {
	ID          int64
	SellerID    int64
	Name        string
	Description string
	PriceSen    int64
	Stock       int
	ImageURL    *string // nil when the product has no image
}

// NewProduct holds what a seller enters for a product; validation happens in the API.
type NewProduct struct {
	Name        string
	Description string
	PriceSen    int64
	Stock       int
	ImageURL    *string
}

const productColumns = "id, seller_id, name, description, price_sen, stock, image_url"

func scanProduct(row pgx.Row) (Product, error) {
	var p Product
	err := row.Scan(&p.ID, &p.SellerID, &p.Name, &p.Description, &p.PriceSen, &p.Stock, &p.ImageURL)
	return p, err
}

// ListProducts returns the seller's products that aren't archived, newest first.
// It never returns another seller's products, and never nil.
func (s *Store) ListProducts(ctx context.Context, sellerID int64) ([]Product, error) {
	rows, err := s.pool.Query(ctx,
		"select "+productColumns+" from products where seller_id = $1 and archived_at is null order by id desc",
		sellerID)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	products := []Product{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, fmt.Errorf("list products: %w", err)
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	return products, nil
}

// CreateProduct adds a product for sellerID, which must come from the session.
func (s *Store) CreateProduct(ctx context.Context, sellerID int64, p NewProduct) (Product, error) {
	created, err := scanProduct(s.pool.QueryRow(ctx,
		"insert into products (seller_id, name, description, price_sen, stock, image_url) values ($1, $2, $3, $4, $5, $6) returning "+productColumns,
		sellerID, p.Name, p.Description, p.PriceSen, p.Stock, p.ImageURL))
	if err != nil {
		return Product{}, fmt.Errorf("create product: %w", err)
	}
	return created, nil
}

package store

import (
	"context"
	"errors"
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

// ProductByID returns one of the seller's live products. Another seller's
// product, an archived one, or a missing ID all give ErrNotFound, so callers
// can't tell them apart (and answer 404, never 403).
func (s *Store) ProductByID(ctx context.Context, sellerID, id int64) (Product, error) {
	p, err := scanProduct(s.pool.QueryRow(ctx,
		"select "+productColumns+" from products where id = $1 and seller_id = $2 and archived_at is null",
		id, sellerID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	if err != nil {
		return Product{}, fmt.Errorf("product by id: %w", err)
	}
	return p, nil
}

// UpdateProduct replaces the editable fields of one of the seller's live
// products. Same ErrNotFound rule as ProductByID.
func (s *Store) UpdateProduct(ctx context.Context, sellerID, id int64, p NewProduct) (Product, error) {
	updated, err := scanProduct(s.pool.QueryRow(ctx,
		`update products
		    set name = $3, description = $4, price_sen = $5, stock = $6, image_url = $7
		  where id = $1 and seller_id = $2 and archived_at is null
		returning `+productColumns,
		id, sellerID, p.Name, p.Description, p.PriceSen, p.Stock, p.ImageURL))
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	if err != nil {
		return Product{}, fmt.Errorf("update product: %w", err)
	}
	return updated, nil
}

package store

import (
	"testing"

	"github.com/rafifn/shoplink/backend/internal/testdb"
)

func ptr(s string) *string { return &s }

func TestCreateAndListProducts(t *testing.T) {
	pool := testdb.New(t)
	s := New(pool)
	sellerA := testdb.CreateUser(t, pool, "a@example.com", "Shop A")
	sellerB := testdb.CreateUser(t, pool, "b@example.com", "Shop B")

	mug, err := s.CreateProduct(t.Context(), sellerA, NewProduct{Name: "Mug", Description: "350 ml", PriceSen: 6_500_000, Stock: 12, ImageURL: ptr("https://img.example.com/mug.jpg")})
	if err != nil {
		t.Fatal(err)
	}
	if mug.ID == 0 || mug.SellerID != sellerA || mug.Name != "Mug" || mug.PriceSen != 6_500_000 || mug.Stock != 12 || mug.ImageURL == nil || *mug.ImageURL != "https://img.example.com/mug.jpg" {
		t.Errorf("CreateProduct = %+v", mug)
	}
	tote, err := s.CreateProduct(t.Context(), sellerA, NewProduct{Name: "Tote", PriceSen: 12_000_000})
	if err != nil {
		t.Fatal(err)
	}
	if tote.ImageURL != nil || tote.Description != "" {
		t.Errorf("optional fields: %+v, want no image and an empty description", tote)
	}
	if _, err := s.CreateProduct(t.Context(), sellerB, NewProduct{Name: "B's candle", PriceSen: 8_500_000}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), "insert into products (seller_id, name, price_sen, archived_at) values ($1, 'Old', 100, now())", sellerA); err != nil {
		t.Fatal(err)
	}

	got, err := s.ListProducts(t.Context(), sellerA)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != tote.ID || got[1].ID != mug.ID {
		t.Errorf("ListProducts(A) = %+v, want [Tote, Mug]: A's own, newest first, no archived, none of B's", got)
	}

	empty, err := s.ListProducts(t.Context(), testdb.CreateUser(t, pool, "c@example.com", "Shop C"))
	if err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("ListProducts for a new seller = %#v, %v; want an empty, non-nil list", empty, err)
	}
}

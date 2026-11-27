package api

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/rafifn/shoplink/backend/internal/store"
)

// Limits for product fields. maxPriceSen is JavaScript's Number.MAX_SAFE_INTEGER,
// matching the database check, so every price reaches the browser exactly.
const (
	maxProductName  = 120
	maxDescription  = 2000
	maxPriceSen     = 9_007_199_254_740_991
	maxStock        = 1_000_000
	maxImageURLSize = 2048
)

// productResponse is a product as the frontend sees it. seller_id and
// archived_at stay on the server.
type productResponse struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	PriceSen    int64   `json:"priceSen"`
	Stock       int     `json:"stock"`
	ImageURL    *string `json:"imageUrl"`
}

func toProduct(p store.Product) productResponse {
	return productResponse{
		ID: p.ID, Name: p.Name, Description: p.Description,
		PriceSen: p.PriceSen, Stock: p.Stock, ImageURL: p.ImageURL,
	}
}

// listProducts answers GET /api/products (behind requireSeller): the seller's
// products, newest first, as {"products": [...]}.
func listProducts(st *store.Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		products, err := st.ListProducts(r.Context(), sellerID(r.Context()))
		if err != nil {
			internalError(w, r, logger, err)
			return
		}
		out := make([]productResponse, 0, len(products))
		for _, p := range products {
			out = append(out, toProduct(p))
		}
		writeJSON(w, http.StatusOK, struct {
			Products []productResponse `json:"products"`
		}{out})
	}
}

// productInput is a product as a seller sends it. Pointers tell a missing
// field apart from a zero value. There is no seller field: the seller is
// always the one logged in (unknown fields are rejected).
type productInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	PriceSen    *int64  `json:"priceSen"`
	Stock       *int64  `json:"stock"`
	ImageURL    *string `json:"imageUrl"`
}

// createProduct answers POST /api/products (behind requireSeller).
func createProduct(st *store.Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in productInput
		if !decodeJSON(w, r, &in) {
			return
		}
		p, fields := checkProduct(in)
		if len(fields) > 0 {
			writeValidationError(w, fields)
			return
		}
		created, err := st.CreateProduct(r.Context(), sellerID(r.Context()), p)
		if err != nil {
			internalError(w, r, logger, err)
			return
		}
		writeJSON(w, http.StatusCreated, toProduct(created))
	}
}

// checkProduct validates a complete product. It returns the cleaned-up values
// (trimmed text, no image for an empty URL) and a message per invalid field.
func checkProduct(in productInput) (store.NewProduct, map[string]string) {
	fields := map[string]string{}
	var p store.NewProduct

	if in.Name != nil {
		p.Name = strings.TrimSpace(*in.Name)
	}
	switch n := utf8.RuneCountInString(p.Name); {
	case n == 0:
		fields["name"] = "Enter a product name."
	case n > maxProductName:
		fields["name"] = "Keep the name to 120 characters or fewer."
	}

	if in.Description != nil {
		p.Description = strings.TrimSpace(*in.Description)
	}
	if utf8.RuneCountInString(p.Description) > maxDescription {
		fields["description"] = "Keep the description to 2,000 characters or fewer."
	}

	switch {
	case in.PriceSen == nil:
		fields["priceSen"] = "Enter a price."
	case *in.PriceSen < 0:
		fields["priceSen"] = "The price can't be negative."
	case *in.PriceSen > maxPriceSen:
		fields["priceSen"] = "That price is too high."
	default:
		p.PriceSen = *in.PriceSen
	}

	switch {
	case in.Stock == nil:
		fields["stock"] = "Enter how many you have in stock."
	case *in.Stock < 0:
		fields["stock"] = "Stock can't be negative."
	case *in.Stock > maxStock:
		fields["stock"] = "Stock can be at most 1,000,000."
	default:
		p.Stock = int(*in.Stock)
	}

	if in.ImageURL != nil {
		if u := strings.TrimSpace(*in.ImageURL); u != "" {
			if msg := checkImageURL(u); msg != "" {
				fields["imageUrl"] = msg
			} else {
				p.ImageURL = &u
			}
		}
	}
	return p, fields
}

// checkImageURL accepts only absolute http(s) URLs, so a product image can
// never be a javascript: or data: URL.
func checkImageURL(raw string) string {
	if len(raw) > maxImageURLSize {
		return "Keep the image URL to 2,048 characters or fewer."
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "Enter an image URL that starts with http:// or https://."
	}
	return ""
}

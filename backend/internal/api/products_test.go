package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rafifn/shoplink/backend/internal/store"
	"github.com/rafifn/shoplink/backend/internal/testdb"
)

// sellerSession is a logged-in seller: their ID and session cookie value.
type sellerSession struct {
	id     int64
	cookie string
}

type productsTest struct {
	h    http.Handler
	pool *pgxpool.Pool
	a, b sellerSession
}

func newProductsTest(t *testing.T) productsTest {
	t.Helper()
	pool := testdb.New(t)
	st := store.New(pool)
	login := func(email string) sellerSession {
		id := testdb.CreateUser(t, pool, email, "Shop "+email)
		token, _, err := st.CreateSession(t.Context(), id)
		if err != nil {
			t.Fatal(err)
		}
		return sellerSession{id: id, cookie: token}
	}
	return productsTest{
		h:    New(Options{DB: pool, Store: st, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}),
		pool: pool,
		a:    login("a@example.com"),
		b:    login("b@example.com"),
	}
}

// call sends a request as the seller with cookie (none if ""), with body as JSON if non-nil.
func call(t *testing.T, h http.Handler, method, path, cookie string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = strings.NewReader(string(raw))
	}
	req := httptest.NewRequestWithContext(t.Context(), method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: cookie}) //nolint:gosec // G124: a cookie the test request sends, not one the server sets
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type productJSON struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	PriceSen    int64   `json:"priceSen"`
	Stock       int64   `json:"stock"`
	ImageURL    *string `json:"imageUrl"`
}

func decodeStrict[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	dec := json.NewDecoder(strings.NewReader(rec.Body.String()))
	dec.DisallowUnknownFields() // never leak seller_id, archived_at, or anything else
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("body %s: %v", rec.Body, err)
	}
	return v
}

func getProducts(t *testing.T, h http.Handler, cookie string) []productJSON {
	t.Helper()
	rec := call(t, h, http.MethodGet, "/api/products", cookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/products: status %d (body %s)", rec.Code, rec.Body)
	}
	body := decodeStrict[struct {
		Products []productJSON `json:"products"`
	}](t, rec)
	if body.Products == nil {
		t.Fatalf(`"products" must be a list, got %s`, rec.Body)
	}
	return body.Products
}

func validMug() map[string]any {
	return map[string]any{
		"name": "Stoneware mug", "description": "Wheel-thrown, 350 ml.", "priceSen": 6_500_000,
		"stock": 12, "imageUrl": "https://img.example.com/mug.jpg",
	}
}

func TestProductsNeedLogin(t *testing.T) {
	pt := newProductsTest(t)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		var body any
		if method == http.MethodPost {
			body = validMug()
		}
		rec := call(t, pt.h, method, "/api/products", "", body)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without a session: status %d, want 401", method, rec.Code)
		}
	}
}

func TestAddAndListProducts(t *testing.T) {
	pt := newProductsTest(t)

	if got := getProducts(t, pt.h, pt.a.cookie); len(got) != 0 {
		t.Fatalf("a new seller has products: %+v", got)
	}

	rec := call(t, pt.h, http.MethodPost, "/api/products", pt.a.cookie, validMug())
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST: status %d, want 201 (body %s)", rec.Code, rec.Body)
	}
	mug := decodeStrict[productJSON](t, rec)
	if mug.ID == 0 || mug.Name != "Stoneware mug" || mug.Description != "Wheel-thrown, 350 ml." || mug.PriceSen != 6_500_000 ||
		mug.Stock != 12 || mug.ImageURL == nil || *mug.ImageURL != "https://img.example.com/mug.jpg" {
		t.Errorf("created = %+v", mug)
	}

	tote := decodeStrict[productJSON](t, call(t, pt.h, http.MethodPost, "/api/products", pt.a.cookie,
		map[string]any{"name": "  Linen tote  ", "priceSen": 12_000_000, "stock": 0}))
	if tote.Name != "Linen tote" || tote.ImageURL != nil || tote.Description != "" {
		t.Errorf("optional fields and trimming: %+v", tote)
	}

	got := getProducts(t, pt.h, pt.a.cookie)
	if len(got) != 2 || got[0].ID != tote.ID || got[1].ID != mug.ID {
		t.Errorf("list = %+v, want [tote, mug], newest first", got)
	}
}

func TestSellersOnlySeeTheirOwnProducts(t *testing.T) {
	pt := newProductsTest(t)
	call(t, pt.h, http.MethodPost, "/api/products", pt.a.cookie, validMug())
	b := validMug()
	b["name"] = "B's candle"
	call(t, pt.h, http.MethodPost, "/api/products", pt.b.cookie, b)
	if _, err := pt.pool.Exec(t.Context(), "insert into products (seller_id, name, price_sen, archived_at) values ($1, 'Archived', 100, now())", pt.a.id); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		who  string
		s    sellerSession
		want string
	}{{"A", pt.a, "Stoneware mug"}, {"B", pt.b, "B's candle"}} {
		got := getProducts(t, pt.h, tc.s.cookie)
		if len(got) != 1 || got[0].Name != tc.want {
			t.Errorf("seller %s sees %+v, want only %q (own, not archived)", tc.who, got, tc.want)
		}
	}
}

func TestAddProductTakesTheSellerFromTheSession(t *testing.T) {
	pt := newProductsTest(t)
	body := validMug()
	body["sellerId"] = pt.b.id // trying to create a product for someone else

	rec := call(t, pt.h, http.MethodPost, "/api/products", pt.a.cookie, body)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400 (body %s)", rec.Code, rec.Body)
	}
	assertErrorBody(t, rec.Body.Bytes(), "invalid_json")
	if got := getProducts(t, pt.h, pt.b.cookie); len(got) != 0 {
		t.Errorf("seller B got a product: %+v", got)
	}
}

func TestAddProductKeepsTheLargestPriceExact(t *testing.T) {
	pt := newProductsTest(t)
	body := validMug()
	body["priceSen"] = int64(9_007_199_254_740_991)

	rec := call(t, pt.h, http.MethodPost, "/api/products", pt.a.cookie, body)

	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"priceSen":9007199254740991`) {
		t.Errorf("status %d, body %s; want 201 with the exact price", rec.Code, rec.Body)
	}
}

func TestAddProductValidation(t *testing.T) {
	with := func(key string, value any) map[string]any {
		b := validMug()
		if value == nil {
			delete(b, key)
		} else {
			b[key] = value
		}
		return b
	}
	tests := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"no name", with("name", nil), "name"},
		{"blank name", with("name", "   "), "name"},
		{"name over 120 characters", with("name", strings.Repeat("é", 121)), "name"},
		{"description over 2000 characters", with("description", strings.Repeat("a", 2001)), "description"},
		{"no price", with("priceSen", nil), "priceSen"},
		{"negative price", with("priceSen", -1), "priceSen"},
		{"price above the maximum", with("priceSen", int64(9_007_199_254_740_992)), "priceSen"},
		{"no stock", with("stock", nil), "stock"},
		{"negative stock", with("stock", -1), "stock"},
		{"stock over a million", with("stock", 1_000_001), "stock"},
		{"image URL that isn't a URL", with("imageUrl", "not a url"), "imageUrl"},
		{"image URL with another scheme", with("imageUrl", "javascript:alert(1)"), "imageUrl"},
		{"ftp image URL", with("imageUrl", "ftp://img.example.com/a.jpg"), "imageUrl"},
		{"image URL over 2048 characters", with("imageUrl", "https://img.example.com/"+strings.Repeat("a", 2030)), "imageUrl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt := newProductsTest(t)
			fields := validationFields(t, call(t, pt.h, http.MethodPost, "/api/products", pt.a.cookie, tt.body))
			if len(fields) != 1 || fields[tt.field] == "" {
				t.Errorf("fields = %v, want only a message for %s", fields, tt.field)
			}
		})
	}

	t.Run("a fractional price is a type error", func(t *testing.T) {
		pt := newProductsTest(t)
		rec := call(t, pt.h, http.MethodPost, "/api/products", pt.a.cookie, with("priceSen", 1.5))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d, want 400", rec.Code)
		}
		assertErrorBody(t, rec.Body.Bytes(), "invalid_json")
	})

	t.Run("limits are inclusive", func(t *testing.T) {
		pt := newProductsTest(t)
		body := map[string]any{
			"name": strings.Repeat("é", 120), "description": strings.Repeat("a", 2000), "priceSen": 0, "stock": 1_000_000,
			"imageUrl": "http://img.example.com/" + strings.Repeat("a", 2048-len("http://img.example.com/")),
		}
		if rec := call(t, pt.h, http.MethodPost, "/api/products", pt.a.cookie, body); rec.Code != http.StatusCreated {
			t.Errorf("status %d, want 201 (body %s)", rec.Code, rec.Body)
		}
	})

	t.Run("an empty image URL means no image", func(t *testing.T) {
		pt := newProductsTest(t)
		rec := call(t, pt.h, http.MethodPost, "/api/products", pt.a.cookie, with("imageUrl", "  "))
		if rec.Code != http.StatusCreated || decodeStrict[productJSON](t, rec).ImageURL != nil {
			t.Errorf("status %d, body %s; want 201 with imageUrl null", rec.Code, rec.Body)
		}
	})
}

package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// addProduct creates validMug() for the seller and returns it.
func addProduct(t *testing.T, h http.Handler, cookie string) productJSON {
	t.Helper()
	rec := call(t, h, http.MethodPost, "/api/products", cookie, validMug())
	if rec.Code != http.StatusCreated {
		t.Fatalf("add product: status %d (body %s)", rec.Code, rec.Body)
	}
	return decodeStrict[productJSON](t, rec)
}

func productPath(id int64) string { return fmt.Sprintf("/api/products/%d", id) }

// sameProduct compares by value, including the image URL behind the pointer.
func sameProduct(a, b productJSON) bool {
	sameImage := (a.ImageURL == nil && b.ImageURL == nil) ||
		(a.ImageURL != nil && b.ImageURL != nil && *a.ImageURL == *b.ImageURL)
	a.ImageURL, b.ImageURL = nil, nil
	return a == b && sameImage
}

func TestGetProduct(t *testing.T) {
	pt := newProductsTest(t)
	mug := addProduct(t, pt.h, pt.a.cookie)

	rec := call(t, pt.h, http.MethodGet, productPath(mug.ID), pt.a.cookie, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if got := decodeStrict[productJSON](t, rec); got.ID != mug.ID || got.Name != mug.Name || got.PriceSen != mug.PriceSen {
		t.Errorf("GET = %+v, want %+v", got, mug)
	}
}

// Anything that isn't the seller's own, live product is 404 — never 403 — so
// IDs can't be probed to learn what other sellers have (CLAUDE.md).
func TestProductNotFoundCases(t *testing.T) {
	pt := newProductsTest(t)
	bProduct := addProduct(t, pt.h, pt.b.cookie)
	var archived int64
	if err := pt.pool.QueryRow(t.Context(),
		"insert into products (seller_id, name, price_sen, archived_at) values ($1, 'Old', 100, now()) returning id", pt.a.id,
	).Scan(&archived); err != nil {
		t.Fatal(err)
	}

	paths := map[string]string{
		"another seller's product": productPath(bProduct.ID),
		"an archived product":      productPath(archived),
		"a missing product":        productPath(999_999),
		"a non-numeric id":         "/api/products/abc",
		"id zero":                  "/api/products/0",
		"a negative id":            "/api/products/-1",
		"an id too big for int64":  "/api/products/99999999999999999999",
	}
	for name, path := range paths {
		for _, method := range []string{http.MethodGet, http.MethodPatch} {
			var body any
			if method == http.MethodPatch {
				body = map[string]any{"name": "Hijacked"}
			}
			rec := call(t, pt.h, method, path, pt.a.cookie, body)
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s %s (%s): status %d, want 404", method, path, name, rec.Code)
				continue
			}
			assertErrorBody(t, rec.Body.Bytes(), "not_found")
		}
	}

	// And B's product really is untouched.
	if got := decodeStrict[productJSON](t, call(t, pt.h, http.MethodGet, productPath(bProduct.ID), pt.b.cookie, nil)); got.Name != bProduct.Name {
		t.Errorf("seller A's PATCH changed seller B's product: %+v", got)
	}
}

func TestEditProductNeedsLogin(t *testing.T) {
	pt := newProductsTest(t)
	mug := addProduct(t, pt.h, pt.a.cookie)
	for _, method := range []string{http.MethodGet, http.MethodPatch} {
		var body any
		if method == http.MethodPatch {
			body = map[string]any{"name": "x"}
		}
		if rec := call(t, pt.h, method, productPath(mug.ID), "", body); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without a session: status %d, want 401", method, rec.Code)
		}
	}
}

func TestPatchChangesOnlyWhatIsSent(t *testing.T) {
	pt := newProductsTest(t)
	mug := addProduct(t, pt.h, pt.a.cookie)

	rec := call(t, pt.h, http.MethodPatch, productPath(mug.ID), pt.a.cookie, map[string]any{"name": "  Big mug ", "stock": 0})

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	got := decodeStrict[productJSON](t, rec)
	if got.Name != "Big mug" || got.Stock != 0 {
		t.Errorf("changed fields: %+v", got)
	}
	if got.Description != mug.Description || got.PriceSen != mug.PriceSen || got.ImageURL == nil || *got.ImageURL != *mug.ImageURL {
		t.Errorf("fields that weren't sent changed: %+v, was %+v", got, mug)
	}
	// The change is stored, not just echoed.
	if again := decodeStrict[productJSON](t, call(t, pt.h, http.MethodGet, productPath(mug.ID), pt.a.cookie, nil)); !sameProduct(again, got) {
		t.Errorf("GET after PATCH = %+v, want %+v", again, got)
	}
}

func TestPatchImageURL(t *testing.T) {
	tests := []struct {
		name     string
		body     map[string]any
		wantNull bool
	}{
		{"absent keeps the image", map[string]any{"name": "Mug"}, false},
		{"null removes it", map[string]any{"imageUrl": nil}, true},
		{"blank removes it", map[string]any{"imageUrl": "  "}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt := newProductsTest(t)
			mug := addProduct(t, pt.h, pt.a.cookie)
			got := decodeStrict[productJSON](t, call(t, pt.h, http.MethodPatch, productPath(mug.ID), pt.a.cookie, tt.body))
			if (got.ImageURL == nil) != tt.wantNull {
				t.Errorf("imageUrl = %v, want null: %v", got.ImageURL, tt.wantNull)
			}
		})
	}

	t.Run("a new URL replaces it", func(t *testing.T) {
		pt := newProductsTest(t)
		mug := addProduct(t, pt.h, pt.a.cookie)
		got := decodeStrict[productJSON](t, call(t, pt.h, http.MethodPatch, productPath(mug.ID), pt.a.cookie,
			map[string]any{"imageUrl": "https://img.example.com/new.jpg"}))
		if got.ImageURL == nil || *got.ImageURL != "https://img.example.com/new.jpg" {
			t.Errorf("imageUrl = %v", got.ImageURL)
		}
	})

	t.Run("a number is a type error naming the field", func(t *testing.T) {
		pt := newProductsTest(t)
		mug := addProduct(t, pt.h, pt.a.cookie)
		rec := call(t, pt.h, http.MethodPatch, productPath(mug.ID), pt.a.cookie, map[string]any{"imageUrl": 123})
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "imageUrl") {
			t.Errorf("status %d, body %s; want 400 naming imageUrl", rec.Code, rec.Body)
		}
		assertErrorBody(t, rec.Body.Bytes(), "invalid_json")
	})
}

func TestPatchValidation(t *testing.T) {
	tests := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"blank name", map[string]any{"name": " "}, "name"},
		{"name over 120 characters", map[string]any{"name": strings.Repeat("é", 121)}, "name"},
		{"negative price", map[string]any{"priceSen": -1}, "priceSen"},
		{"price above the maximum", map[string]any{"priceSen": int64(9_007_199_254_740_992)}, "priceSen"},
		{"stock over a million", map[string]any{"stock": 1_000_001}, "stock"},
		{"bad image URL", map[string]any{"imageUrl": "javascript:alert(1)"}, "imageUrl"},
		// null is "not sent" for the required fields, so it can't blank them.
		{"null name", map[string]any{"name": nil}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt := newProductsTest(t)
			mug := addProduct(t, pt.h, pt.a.cookie)
			rec := call(t, pt.h, http.MethodPatch, productPath(mug.ID), pt.a.cookie, tt.body)
			if tt.field == "" {
				if rec.Code != http.StatusOK || decodeStrict[productJSON](t, rec).Name != mug.Name {
					t.Errorf("status %d, body %s; want 200 with the name unchanged", rec.Code, rec.Body)
				}
				return
			}
			fields := validationFields(t, rec)
			if len(fields) != 1 || fields[tt.field] == "" {
				t.Errorf("fields = %v, want only a message for %s", fields, tt.field)
			}
			// A rejected edit changes nothing.
			if got := decodeStrict[productJSON](t, call(t, pt.h, http.MethodGet, productPath(mug.ID), pt.a.cookie, nil)); !sameProduct(got, mug) {
				t.Errorf("rejected PATCH changed the product: %+v, was %+v", got, mug)
			}
		})
	}

	t.Run("a sellerId field is rejected", func(t *testing.T) {
		pt := newProductsTest(t)
		mug := addProduct(t, pt.h, pt.a.cookie)
		rec := call(t, pt.h, http.MethodPatch, productPath(mug.ID), pt.a.cookie, map[string]any{"sellerId": pt.b.id})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d, want 400", rec.Code)
		}
		assertErrorBody(t, rec.Body.Bytes(), "invalid_json")
	})
}

package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"

	"github.com/rafifn/shoplink/backend/internal/store"
)

// productID reads {id} from the path. Anything that isn't a positive int64
// can't be a product, so it gets the same 404 as a product that doesn't exist.
func productID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeProductNotFound(w)
		return 0, false
	}
	return id, true
}

// One message for every "not yours or not there" case, so responses can't
// be used to probe other sellers' IDs.
func writeProductNotFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "not_found", "That product doesn't exist.")
}

// getProduct answers GET /api/products/{id} (behind requireSeller).
func getProduct(st *store.Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := productID(w, r)
		if !ok {
			return
		}
		p, err := st.ProductByID(r.Context(), sellerID(r.Context()), id)
		if errors.Is(err, store.ErrNotFound) {
			writeProductNotFound(w)
			return
		}
		if err != nil {
			internalError(w, r, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, toProduct(p))
	}
}

// productPatch is a partial edit: only fields present in the body change.
// For the required fields, null counts as "not sent". imageUrl needs to tell
// "not sent" (keep the image) from null or "" (remove it).
type productPatch struct {
	Name        *string        `json:"name"`
	Description *string        `json:"description"`
	PriceSen    *int64         `json:"priceSen"`
	Stock       *int64         `json:"stock"`
	ImageURL    optionalString `json:"imageUrl"`
}

// optionalString remembers whether its field appeared in the JSON at all.
type optionalString struct {
	Set   bool
	Value *string // nil for JSON null
}

func (o *optionalString) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		// Keep decodeJSON's "Field "imageUrl" must be text." message.
		return &json.UnmarshalTypeError{Value: "non-string", Type: reflect.TypeFor[string](), Field: "imageUrl"}
	}
	o.Value = &s
	return nil
}

// patchProduct answers PATCH /api/products/{id} (behind requireSeller). The
// edit is applied to the stored product and the result is validated as a
// whole, with the same rules as creating one.
func patchProduct(st *store.Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := productID(w, r)
		if !ok {
			return
		}
		var patch productPatch
		if !decodeJSON(w, r, &patch) {
			return
		}
		seller := sellerID(r.Context())
		current, err := st.ProductByID(r.Context(), seller, id)
		if errors.Is(err, store.ErrNotFound) {
			writeProductNotFound(w)
			return
		}
		if err != nil {
			internalError(w, r, logger, err)
			return
		}

		stock := int64(current.Stock)
		in := productInput{
			Name: &current.Name, Description: &current.Description,
			PriceSen: &current.PriceSen, Stock: &stock, ImageURL: current.ImageURL,
		}
		if patch.Name != nil {
			in.Name = patch.Name
		}
		if patch.Description != nil {
			in.Description = patch.Description
		}
		if patch.PriceSen != nil {
			in.PriceSen = patch.PriceSen
		}
		if patch.Stock != nil {
			in.Stock = patch.Stock
		}
		if patch.ImageURL.Set {
			in.ImageURL = patch.ImageURL.Value // nil or blank removes the image
		}

		p, fields := checkProduct(in)
		if len(fields) > 0 {
			writeValidationError(w, fields)
			return
		}
		updated, err := st.UpdateProduct(r.Context(), seller, id, p)
		if errors.Is(err, store.ErrNotFound) {
			writeProductNotFound(w) // archived between the read and the write
			return
		}
		if err != nil {
			internalError(w, r, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, toProduct(updated))
	}
}

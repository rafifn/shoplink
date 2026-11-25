package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/rafifn/shoplink/backend/internal/store"
)

// meResponse is the logged-in seller as the frontend sees them. It is also the
// body of a successful signup or login.
type meResponse struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	ShopName string `json:"shopName"`
}

func toMe(u store.User) meResponse {
	return meResponse{ID: u.ID, Email: u.Email, ShopName: u.ShopName}
}

// me answers GET /api/me (behind requireSeller).
func me(st *store.Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := st.UserByID(r.Context(), sellerID(r.Context()))
		if errors.Is(err, store.ErrNotFound) {
			// Can't happen while sessions cascade with users, but never 500 on it.
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Please log in.")
			return
		}
		if err != nil {
			internalError(w, r, logger, err)
			return
		}
		writeJSON(w, http.StatusOK, toMe(u))
	}
}

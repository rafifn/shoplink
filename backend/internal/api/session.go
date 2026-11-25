package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/rafifn/shoplink/backend/internal/store"
)

// sessionCookie holds the session token. HttpOnly keeps it away from scripts;
// SameSite=Lax keeps other sites from sending it with their requests.
const sessionCookie = "shoplink_session"

func setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time, secure bool) {
	//nolint:gosec // G124: Secure comes from COOKIE_SECURE (true unless local http); HttpOnly and SameSite are always set
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	//nolint:gosec // G124: as in setSessionCookie, Secure follows COOKIE_SECURE
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

type sellerIDKey struct{}

// sellerID returns the logged-in seller's ID. Only call it inside requireSeller.
func sellerID(ctx context.Context) int64 {
	id, _ := ctx.Value(sellerIDKey{}).(int64)
	return id
}

// requireSeller lets the request through only with a valid session cookie,
// and puts the seller's ID in the request context for the handler (and for
// every query, which must filter by it). Otherwise: 401 unauthenticated.
func requireSeller(st *store.Store, logger *slog.Logger, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Please log in.")
			return
		}
		id, err := st.SessionUserID(r.Context(), c.Value)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Your session has ended. Please log in again.")
			return
		}
		if err != nil {
			internalError(w, r, logger, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), sellerIDKey{}, id)))
	}
}

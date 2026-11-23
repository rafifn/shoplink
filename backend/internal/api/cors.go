package api

import "net/http"

// cors lets the browser app at origin call the API with cookies. Locally that
// is the Vite dev server on another port. In production the app and the API
// share one domain, origin is empty, and this adds nothing.
//
// Only an exact match of the Origin header is allowed: never "*", and never
// a reflected origin, because the responses carry the seller's session cookie.
func cors(origin string, next http.Handler) http.Handler {
	if origin == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Add("Vary", "Origin")

		preflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
		if preflight {
			h.Add("Vary", "Access-Control-Request-Method")
			h.Add("Vary", "Access-Control-Request-Headers")
		}

		if r.Header.Get("Origin") == origin {
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			if preflight {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE")
				h.Set("Access-Control-Allow-Headers", "Content-Type")
				h.Set("Access-Control-Max-Age", "600")
			}
		}

		// Answer every preflight here. Without the allow headers above, the
		// browser blocks the real request from any other origin.
		if preflight {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

package api

import (
	"fmt"
	"mime"
	"net/http"
)

// jsonRouting wraps the router so that everything it answers is JSON:
//   - unknown paths get 404 not_found and wrong methods 405
//     method_not_allowed (with the Allow header), in the standard error shape
//     instead of Go's plain text
//   - POST, PUT and PATCH must send Content-Type: application/json, or get
//     415 unsupported_media_type. Besides keeping handlers simple, this blocks
//     cross-site form posts (CSRF): a browser only sends JSON across sites
//     after a CORS preflight, which the API refuses for other origins.
func jsonRouting(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, pattern := mux.Handler(r)
		if pattern == "" {
			// No route matched. Ask the mux's own handler what it would say
			// (404, or 405 plus Allow) without sending its plain text.
			rec := &recordHeader{header: http.Header{}}
			h.ServeHTTP(rec, r)
			switch rec.status {
			case http.StatusNotFound:
				writeError(w, http.StatusNotFound, "not_found", "Not found.")
			case http.StatusMethodNotAllowed:
				w.Header().Set("Allow", rec.header.Get("Allow"))
				writeError(w, http.StatusMethodNotAllowed, "method_not_allowed",
					fmt.Sprintf("%s is not allowed here.", r.Method))
			default:
				mux.ServeHTTP(w, r) // e.g. a redirect to the cleaned-up path
			}
			return
		}

		if sendsBody(r.Method) && !isJSON(r.Header.Get("Content-Type")) {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type",
				"Send the request body as JSON with Content-Type: application/json.")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func sendsBody(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch
}

// isJSON accepts "application/json" with any parameters (e.g. charset), in any case.
func isJSON(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == "application/json"
}

// recordHeader captures the status and headers a handler sets and drops its body.
type recordHeader struct {
	header http.Header
	status int
}

func (r *recordHeader) Header() http.Header { return r.header }

func (r *recordHeader) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}

func (r *recordHeader) Write(b []byte) (int, error) {
	r.WriteHeader(http.StatusOK)
	return len(b), nil
}

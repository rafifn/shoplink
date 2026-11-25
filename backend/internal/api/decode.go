package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
)

// maxBodyBytes caps every JSON request body. Nothing in ShopLink needs more;
// images are URLs, not uploads.
const maxBodyBytes = 1 << 20 // 1 MB

// decodeJSON reads the request body into dst, which must be a pointer. The
// body must be exactly one JSON value with only fields dst knows about, and at
// most 1 MB. If it isn't, decodeJSON writes the error response (400
// invalid_json or 413 body_too_large) and returns false, so a handler can do:
//
//	var in createProductInput
//	if !decodeJSON(w, r, &in) {
//		return
//	}
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		writeDecodeError(w, err)
		return false
	}
	// Anything after the first value, even another valid object, is an error.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeDecodeError(w, err)
		} else {
			writeError(w, http.StatusBadRequest, "invalid_json", "The request body must be one JSON object.")
		}
		return false
	}
	return true
}

// writeDecodeError turns an encoding/json error into a message a person can act on.
func writeDecodeError(w http.ResponseWriter, err error) {
	var (
		tooBig    *http.MaxBytesError
		syntax    *json.SyntaxError
		wrongType *json.UnmarshalTypeError
	)
	switch {
	case errors.As(err, &tooBig):
		writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "The request body is larger than 1 MB.")
	case errors.Is(err, io.EOF):
		writeError(w, http.StatusBadRequest, "invalid_json", "The request body is empty; send a JSON object.")
	case errors.Is(err, io.ErrUnexpectedEOF):
		writeError(w, http.StatusBadRequest, "invalid_json", "The request body is not valid JSON (it ends too early).")
	case errors.As(err, &syntax):
		writeError(w, http.StatusBadRequest, "invalid_json", fmt.Sprintf("The request body is not valid JSON (at byte %d).", syntax.Offset))
	case errors.As(err, &wrongType) && wrongType.Field != "":
		writeError(w, http.StatusBadRequest, "invalid_json", fmt.Sprintf("Field %q must be %s.", wrongType.Field, jsonKind(wrongType.Type)))
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		// encoding/json has no error type for this; the field name comes quoted.
		field := strings.TrimPrefix(err.Error(), "json: unknown field ")
		writeError(w, http.StatusBadRequest, "invalid_json", fmt.Sprintf("Unknown field %s.", field))
	default:
		writeError(w, http.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
}

// jsonKind names the JSON type a Go type expects, e.g. "a number" for int.
func jsonKind(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "text"
	case reflect.Bool:
		return "true or false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "a whole number"
	case reflect.Float32, reflect.Float64:
		return "a number"
	case reflect.Slice, reflect.Array:
		return "a list"
	default:
		return "an object"
	}
}

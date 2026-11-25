package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type echoInput struct {
	Name  string `json:"name"`
	Stock int    `json:"stock"`
}

// echoHandler decodes the body with decodeJSON and sends it back.
func echoHandler(w http.ResponseWriter, r *http.Request) {
	var in echoInput
	if !decodeJSON(w, r, &in) {
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func postEcho(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/echo", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	echoHandler(rec, req)
	return rec
}

func TestDecodeJSONValid(t *testing.T) {
	rec := postEcho(t, `{"name":"Mug","stock":3}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	var got echoInput
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got != (echoInput{"Mug", 3}) {
		t.Fatalf("decoded %+v (%v), want {Mug 3}", got, err)
	}
}

func TestDecodeJSONRejects(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantStatus  int
		wantCode    string
		wantMessage string // substring of the message
	}{
		{"empty body", ``, 400, "invalid_json", "empty"},
		{"not JSON", `name=Mug`, 400, "invalid_json", "not valid JSON"},
		{"cut off", `{"name":"Mug"`, 400, "invalid_json", "not valid JSON"},
		{"unknown field", `{"name":"Mug","price":1}`, 400, "invalid_json", `"price"`},
		{"wrong type", `{"name":"Mug","stock":"three"}`, 400, "invalid_json", `"stock"`},
		{"two JSON values", `{"name":"Mug"} {"name":"Cup"}`, 400, "invalid_json", "one JSON object"},
		{"too big", `{"name":"` + strings.Repeat("a", maxBodyBytes) + `"}`, 413, "body_too_large", "1 MB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := postEcho(t, tt.body)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			assertErrorBody(t, rec.Body.Bytes(), tt.wantCode)
			var body struct {
				Error struct{ Message string } `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(body.Error.Message, tt.wantMessage) {
				t.Errorf("message %q should mention %s", body.Error.Message, tt.wantMessage)
			}
		})
	}
}

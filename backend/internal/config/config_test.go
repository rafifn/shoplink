package config

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	const dbURL = "postgres://shoplink:secret@localhost:5432/shoplink?sslmode=disable" //nolint:gosec // fake password; the test checks errors never echo it

	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr string // substring of the error; empty means no error
	}{
		{
			name: "defaults",
			env:  map[string]string{"DATABASE_URL": dbURL},
			want: Config{DatabaseURL: dbURL, Port: "8080"},
		},
		{
			name: "all set",
			env:  map[string]string{"DATABASE_URL": dbURL, "PORT": "9000", "FRONTEND_ORIGIN": "http://localhost:5173"},
			want: Config{DatabaseURL: dbURL, Port: "9000", FrontendOrigin: "http://localhost:5173"},
		},
		{
			name: "https origin with port",
			env:  map[string]string{"DATABASE_URL": dbURL, "FRONTEND_ORIGIN": "https://shop.example.com:8443"},
			want: Config{DatabaseURL: dbURL, Port: "8080", FrontendOrigin: "https://shop.example.com:8443"},
		},
		{name: "missing DATABASE_URL", env: map[string]string{}, wantErr: "DATABASE_URL"},
		{name: "port not a number", env: map[string]string{"DATABASE_URL": dbURL, "PORT": "http"}, wantErr: "PORT"},
		{name: "port zero", env: map[string]string{"DATABASE_URL": dbURL, "PORT": "0"}, wantErr: "PORT"},
		{name: "port too big", env: map[string]string{"DATABASE_URL": dbURL, "PORT": "70000"}, wantErr: "PORT"},
		{name: "origin with trailing slash", env: map[string]string{"DATABASE_URL": dbURL, "FRONTEND_ORIGIN": "http://localhost:5173/"}, wantErr: "FRONTEND_ORIGIN"},
		{name: "origin with path", env: map[string]string{"DATABASE_URL": dbURL, "FRONTEND_ORIGIN": "http://localhost:5173/app"}, wantErr: "FRONTEND_ORIGIN"},
		{name: "origin without scheme", env: map[string]string{"DATABASE_URL": dbURL, "FRONTEND_ORIGIN": "localhost:5173"}, wantErr: "FRONTEND_ORIGIN"},
		{name: "origin wildcard", env: map[string]string{"DATABASE_URL": dbURL, "FRONTEND_ORIGIN": "*"}, wantErr: "FRONTEND_ORIGIN"},
		{name: "origin other scheme", env: map[string]string{"DATABASE_URL": dbURL, "FRONTEND_ORIGIN": "ftp://localhost"}, wantErr: "FRONTEND_ORIGIN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(func(key string) string { return tt.env[key] })
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Load() = %+v, want an error mentioning %s", got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Load() error = %q, want it to mention %s", err, tt.wantErr)
				}
				if strings.Contains(err.Error(), "secret") {
					t.Fatalf("Load() error leaks the database password: %q", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

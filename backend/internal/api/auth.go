package api

import (
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/rafifn/shoplink/backend/internal/store"
)

// bcryptCost matches the seed (cmd/seed), so every stored hash is alike.
const bcryptCost = bcrypt.DefaultCost

type signupInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	ShopName string `json:"shopName"`
}

// signup answers POST /api/auth/signup: it creates the seller, logs them in
// (session cookie), and returns them like GET /api/me does, with 201.
func signup(st *store.Store, logger *slog.Logger, cookieSecure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in signupInput
		if !decodeJSON(w, r, &in) {
			return
		}
		email := normalizeEmail(in.Email)
		shopName := strings.TrimSpace(in.ShopName)

		fields := map[string]string{}
		if msg := checkEmail(email); msg != "" {
			fields["email"] = msg
		}
		if msg := checkPassword(in.Password); msg != "" {
			fields["password"] = msg
		}
		switch n := utf8.RuneCountInString(shopName); {
		case n == 0:
			fields["shopName"] = "Enter your shop's name."
		case n > 80:
			fields["shopName"] = "Keep the shop name to 80 characters or fewer."
		}
		if len(fields) > 0 {
			writeValidationError(w, fields)
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcryptCost)
		if err != nil {
			internalError(w, r, logger, err)
			return
		}
		u, err := st.CreateUser(r.Context(), email, string(hash), shopName)
		if errors.Is(err, store.ErrEmailTaken) {
			writeError(w, http.StatusConflict, "email_taken", "That email already has an account. Log in instead?")
			return
		}
		if err != nil {
			internalError(w, r, logger, err)
			return
		}

		token, expiresAt, err := st.CreateSession(r.Context(), u.ID)
		if err != nil {
			// The account exists; they can log in. Don't hide that behind a 500 retry loop.
			logger.ErrorContext(r.Context(), "signup: session not created", "user_id", u.ID, "err", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Your account was created, but logging you in failed. Please log in.")
			return
		}
		setSessionCookie(w, token, expiresAt, cookieSecure)
		writeJSON(w, http.StatusCreated, toMe(u))
	}
}

// normalizeEmail trims and lowercases, so "A@x.com " and "a@x.com" are one account.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// checkEmail returns what's wrong with a normalized email, or "".
func checkEmail(email string) string {
	if email == "" {
		return "Enter your email address."
	}
	addr, err := mail.ParseAddress(email)
	// A bare address only: "Name <a@b.c>" parses too, but isn't what we store.
	if err != nil || addr.Address != email || len(email) > 254 || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") {
		return "Enter a valid email address, like name@example.com."
	}
	return ""
}

// checkPassword returns what's wrong with a password, or "". bcrypt only
// reads the first 72 bytes, so longer passwords are refused rather than cut.
func checkPassword(password string) string {
	switch {
	case len(password) < 8:
		return "Use at least 8 characters."
	case len(password) > 72:
		return "Use at most 72 bytes (fewer if you use emoji or accented letters)."
	}
	return ""
}

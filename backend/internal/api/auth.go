package api

import (
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/rafifn/shoplink/backend/internal/store"
)

// bcryptCost matches the seed (cmd/seed), so every stored hash is alike.
const bcryptCost = bcrypt.DefaultCost

// compareHash checks a password against a bcrypt hash. A variable so tests
// can count the comparisons.
var compareHash = bcrypt.CompareHashAndPassword

// dummyHash is compared against when an email has no account, so a failed
// login takes about as long either way and timing can't reveal accounts.
var dummyHash = sync.OnceValue(func() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("no account has this password"), bcryptCost)
	if err != nil {
		panic(err) // only fails for passwords over 72 bytes or a bad cost
	}
	return h
})

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

type loginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// login answers POST /api/auth/login. A wrong email and a wrong password get
// the same 401 in about the same time, so nobody can probe which emails have
// accounts. Success sets a new session cookie and returns the /api/me body.
func login(st *store.Store, logger *slog.Logger, cookieSecure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in loginInput
		if !decodeJSON(w, r, &in) {
			return
		}
		email := normalizeEmail(in.Email)
		fields := map[string]string{}
		if email == "" {
			fields["email"] = "Enter your email address."
		}
		if in.Password == "" {
			fields["password"] = "Enter your password."
		}
		if len(fields) > 0 {
			writeValidationError(w, fields)
			return
		}

		u, hash, err := st.UserCredentials(r.Context(), email)
		if errors.Is(err, store.ErrNotFound) {
			_ = compareHash(dummyHash(), []byte(in.Password)) // same work as a real check
			writeInvalidCredentials(w)
			return
		}
		if err != nil {
			internalError(w, r, logger, err)
			return
		}
		if compareHash([]byte(hash), []byte(in.Password)) != nil {
			writeInvalidCredentials(w)
			return
		}

		token, expiresAt, err := st.CreateSession(r.Context(), u.ID)
		if err != nil {
			internalError(w, r, logger, err)
			return
		}
		setSessionCookie(w, token, expiresAt, cookieSecure)
		writeJSON(w, http.StatusOK, toMe(u))
	}
}

func writeInvalidCredentials(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "invalid_credentials", "That email and password don't match an account.")
}

// logout answers POST /api/auth/logout: it deletes the session (if any) and
// clears the cookie. Logging out while logged out is fine too: 204 either way.
func logout(st *store.Store, logger *slog.Logger, cookieSecure bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
			if err := st.DeleteSession(r.Context(), c.Value); err != nil {
				internalError(w, r, logger, err)
				return
			}
		}
		clearSessionCookie(w, cookieSecure)
		w.WriteHeader(http.StatusNoContent)
	}
}

package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrEmailTaken means another account already uses that email.
var ErrEmailTaken = errors.New("email already has an account")

// User is a seller account, without the password hash.
type User struct {
	ID       int64
	Email    string
	ShopName string
}

// UserByID returns the seller with id, or ErrNotFound.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, "select id, email, shop_name from users where id = $1", id).
		Scan(&u.ID, &u.Email, &u.ShopName)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("user by id: %w", err)
	}
	return u, nil
}

// CreateUser adds a seller. email must already be normalized (trimmed and
// lowercased; the database checks the latter) and passwordHash a bcrypt hash.
// Returns ErrEmailTaken if the email is in use.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash, shopName string) (User, error) {
	u := User{Email: email, ShopName: shopName}
	err := s.pool.QueryRow(ctx,
		"insert into users (email, password_hash, shop_name) values ($1, $2, $3) returning id",
		email, passwordHash, shopName,
	).Scan(&u.ID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation on users.email
		return User{}, ErrEmailTaken
	}
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

// UserCredentials returns the seller with a normalized email and their
// password hash, for checking a login. Returns ErrNotFound for an unknown email.
func (s *Store) UserCredentials(ctx context.Context, email string) (User, string, error) {
	var u User
	var hash string
	err := s.pool.QueryRow(ctx, "select id, email, shop_name, password_hash from users where email = $1", email).
		Scan(&u.ID, &u.Email, &u.ShopName, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	if err != nil {
		return User{}, "", fmt.Errorf("user credentials: %w", err)
	}
	return u, hash, nil
}

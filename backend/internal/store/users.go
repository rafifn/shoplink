package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

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

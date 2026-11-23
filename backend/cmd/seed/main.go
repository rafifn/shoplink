// Command seed loads the demo seller and sample products into the database at
// DATABASE_URL. Run it with `make seed`.
//
// It is safe to run more than once: the seller is upserted by email (which also
// applies a changed DEMO_SELLER_PASSWORD), and a sample product is only added
// if the seller doesn't already have a product with that name.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

const demoShopName = "Demo Shop"

type sampleProduct struct {
	name        string
	description string
	priceCents  int
	stock       int
}

// One product has a single item left, which is handy for trying the
// "two buyers, last item" case by hand.
var sampleProducts = []sampleProduct{
	{"Hand-poured soy candle", "Sandalwood and vanilla, 200 g, about 40 hours of burn time.", 2400, 25},
	{"Stoneware mug", "Wheel-thrown, 350 ml, dishwasher safe.", 1800, 12},
	{"Linen tote bag", "Natural linen with an inside pocket, 38 x 42 cm.", 3200, 30},
	{"Single-origin coffee beans", "Medium roast, 250 g, roasted to order.", 1600, 1},
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	dbURL := os.Getenv("DATABASE_URL")
	email := strings.ToLower(strings.TrimSpace(os.Getenv("DEMO_SELLER_EMAIL")))
	password := os.Getenv("DEMO_SELLER_PASSWORD")
	if dbURL == "" || email == "" || password == "" {
		return errors.New("DATABASE_URL, DEMO_SELLER_EMAIL and DEMO_SELLER_PASSWORD must be set (see .env.example)")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	var sellerID int64
	err = tx.QueryRow(ctx, `
		insert into users (email, password_hash, shop_name)
		values ($1, $2, $3)
		on conflict (email) do update
			set password_hash = excluded.password_hash, shop_name = excluded.shop_name
		returning id`,
		email, string(hash), demoShopName,
	).Scan(&sellerID)
	if err != nil {
		return fmt.Errorf("upsert demo seller: %w", err)
	}

	added := 0
	for _, p := range sampleProducts {
		tag, err := tx.Exec(ctx, `
			insert into products (seller_id, name, description, price_cents, stock)
			select $1::bigint, $2::text, $3::text, $4::integer, $5::integer
			where not exists (
				select 1 from products where seller_id = $1::bigint and name = $2::text
			)`,
			sellerID, p.name, p.description, p.priceCents, p.stock,
		)
		if err != nil {
			return fmt.Errorf("insert product %q: %w", p.name, err)
		}
		added += int(tag.RowsAffected())
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	fmt.Printf("seed: demo seller %s (id %d), %d new of %d sample products\n",
		email, sellerID, added, len(sampleProducts))
	return nil
}

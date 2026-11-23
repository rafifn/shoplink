-- Prices are rupiah in sen (1/100 rupiah), stored as bigint: integer topped out
-- at Rp 21.474.836,47, too little for a laptop or most order totals.
-- The upper bound is JavaScript's Number.MAX_SAFE_INTEGER (about Rp 90 trillion),
-- so every price reaches the browser exactly as a JSON number.

-- +goose Up
alter table products rename column price_cents to price_sen;
alter table products alter column price_sen type bigint;
alter table products drop constraint products_price_cents_check;
alter table products add constraint products_price_sen_check
    check (price_sen between 0 and 9007199254740991);

-- +goose Down
-- Fails with "integer out of range" (and changes nothing) if a price no longer
-- fits in integer, instead of silently truncating it.
alter table products drop constraint products_price_sen_check;
alter table products alter column price_sen type integer;
alter table products rename column price_sen to price_cents;
alter table products add constraint products_price_cents_check check (price_cents >= 0);

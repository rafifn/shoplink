-- Tables for Phase 1 (seller accounts and products). See docs/design.md, Data model.

-- +goose Up
create table users (
    id            bigint generated always as identity primary key,
    -- The app lowercases emails before saving; the check makes the database enforce it,
    -- so "A@x.com" and "a@x.com" can't become two accounts.
    email         text not null unique check (email = lower(email)),
    password_hash text not null,
    shop_name     text not null,
    created_at    timestamptz not null default now()
);

create table sessions (
    -- Random token stored in the session cookie.
    id         text primary key,
    user_id    bigint not null references users (id) on delete cascade,
    expires_at timestamptz not null
);

create index sessions_user_id_idx on sessions (user_id);

create table products (
    id          bigint generated always as identity primary key,
    -- No cascade: old orders will point at products, so sellers and products are never hard-deleted.
    seller_id   bigint not null references users (id),
    name        text not null,
    description text not null default '',
    -- Money is whole cents, never a decimal.
    price_cents integer not null check (price_cents >= 0),
    -- Second safety net behind the stock transaction: stock can never go below zero.
    stock       integer not null default 0 check (stock >= 0),
    image_url   text,
    -- Products are archived, not deleted.
    archived_at timestamptz
);

create index products_seller_id_idx on products (seller_id);

-- +goose Down
drop table products;
drop table sessions;
drop table users;

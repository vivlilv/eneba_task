CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash BYTEA PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON sessions(user_id);

CREATE TABLE IF NOT EXISTS products (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    edition TEXT NOT NULL,
    platform TEXT NOT NULL,
    region TEXT NOT NULL,
    category TEXT NOT NULL,
    description TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sellers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    rating NUMERIC(3,2) NOT NULL DEFAULT 5.00 CHECK (rating >= 0 AND rating <= 5)
);

CREATE TABLE IF NOT EXISTS offers (
    id TEXT PRIMARY KEY,
    product_id TEXT NOT NULL REFERENCES products(id),
    seller_id TEXT NOT NULL REFERENCES sellers(id),
    price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
    currency CHAR(3) NOT NULL DEFAULT 'EUR' CHECK (currency = 'EUR'),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (seller_id, product_id)
);
CREATE INDEX IF NOT EXISTS offers_product_active_idx ON offers(product_id, active);

CREATE TABLE IF NOT EXISTS digital_keys (
    id BIGSERIAL PRIMARY KEY,
    offer_id TEXT NOT NULL REFERENCES offers(id),
    code TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS digital_keys_offer_id_idx ON digital_keys(offer_id, id);

CREATE TABLE IF NOT EXISTS homepage_blocks (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    kind TEXT NOT NULL,
    eyebrow TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    cta_label TEXT NOT NULL DEFAULT '',
    cta_url TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS homepage_block_products (
    block_id TEXT NOT NULL REFERENCES homepage_blocks(id) ON DELETE CASCADE,
    product_id TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    PRIMARY KEY (block_id, product_id),
    UNIQUE (block_id, position)
);

CREATE TABLE IF NOT EXISTS cart_items (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    offer_id TEXT NOT NULL REFERENCES offers(id),
    price_seen_cents BIGINT NOT NULL CHECK (price_seen_cents >= 0),
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, offer_id)
);

CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id BIGINT NOT NULL REFERENCES users(id),
    status TEXT NOT NULL CHECK (status IN ('pending_payment', 'paid')),
    currency CHAR(3) NOT NULL DEFAULT 'EUR' CHECK (currency = 'EUR'),
    total_cents BIGINT NOT NULL CHECK (total_cents >= 0),
    checkout_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, checkout_key)
);
CREATE INDEX IF NOT EXISTS orders_user_created_idx ON orders(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS order_items (
    id BIGSERIAL PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id TEXT NOT NULL REFERENCES products(id),
    offer_id TEXT NOT NULL REFERENCES offers(id),
    product_title_snapshot TEXT NOT NULL,
    seller_name_snapshot TEXT NOT NULL,
    unit_price_cents BIGINT NOT NULL CHECK (unit_price_cents >= 0),
    digital_key_id BIGINT UNIQUE REFERENCES digital_keys(id),
    UNIQUE (order_id, offer_id)
);

CREATE TABLE IF NOT EXISTS payment_attempts (
    id BIGSERIAL PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('succeeded', 'declined')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (order_id, idempotency_key)
);

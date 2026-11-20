ALTER TABLE products ADD COLUMN IF NOT EXISTS listed BOOLEAN NOT NULL DEFAULT TRUE;

UPDATE products
SET listed = FALSE
WHERE id IN ('product-demo-adventure-pc-eu', 'product-demo-racer-console-eu');

DELETE FROM homepage_block_products
WHERE product_id IN ('product-demo-adventure-pc-eu', 'product-demo-racer-console-eu');

DELETE FROM cart_items
WHERE offer_id IN (
    SELECT id FROM offers
    WHERE product_id IN ('product-demo-adventure-pc-eu', 'product-demo-racer-console-eu')
);

UPDATE offers
SET active = FALSE
WHERE product_id IN ('product-demo-adventure-pc-eu', 'product-demo-racer-console-eu');

INSERT INTO sellers (id, name, rating)
VALUES
    ('seller-cobaltfox', 'Cobalt Fox Digital', 4.88),
    ('seller-neonforge', 'Neon Forge', 4.93)
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name, rating = EXCLUDED.rating;

INSERT INTO products (id, slug, title, edition, platform, region, category, description)
VALUES
    ('product-grand-theft-auto-vi-ps5', 'grand-theft-auto-vi-ps5', 'Grand Theft Auto VI', 'Standard Edition', 'PlayStation 5', 'Global', 'Action', 'Standard Edition for PlayStation 5. Demo offers use synthetic, non-redeemable keys.'),
    ('product-sekiro-shadows-die-twice-pc', 'sekiro-shadows-die-twice-pc', 'Sekiro: Shadows Die Twice', 'Game of the Year Edition', 'PC (Steam)', 'Global', 'Action', 'Game of the Year Edition for PC via Steam. Demo offers use synthetic, non-redeemable keys.')
ON CONFLICT (id) DO UPDATE
SET slug = EXCLUDED.slug,
    title = EXCLUDED.title,
    edition = EXCLUDED.edition,
    platform = EXCLUDED.platform,
    region = EXCLUDED.region,
    category = EXCLUDED.category,
    description = EXCLUDED.description,
    listed = TRUE;

INSERT INTO homepage_block_products (block_id, product_id, position)
VALUES
    ('block-more-to-discover', 'product-grand-theft-auto-vi-ps5', 5),
    ('block-more-to-discover', 'product-sekiro-shadows-die-twice-pc', 6)
ON CONFLICT (block_id, product_id) DO NOTHING;

INSERT INTO offers (id, product_id, seller_id, price_cents, currency, active)
VALUES
    ('offer-hollow-knight', 'product-hollow-knight-pc', 'seller-northstar', 699, 'EUR', TRUE),
    ('offer-hollow-knight-pixelharbor', 'product-hollow-knight-pc', 'seller-pixelharbor', 735, 'EUR', TRUE),
    ('offer-hollow-knight-cobaltfox', 'product-hollow-knight-pc', 'seller-cobaltfox', 769, 'EUR', TRUE),
    ('offer-hollow-knight-neonforge', 'product-hollow-knight-pc', 'seller-neonforge', 804, 'EUR', TRUE),
    ('offer-silksong', 'product-silksong-pc', 'seller-pixelharbor', 1499, 'EUR', TRUE),
    ('offer-silksong-northstar', 'product-silksong-pc', 'seller-northstar', 1574, 'EUR', TRUE),
    ('offer-silksong-cobaltfox', 'product-silksong-pc', 'seller-cobaltfox', 1649, 'EUR', TRUE),
    ('offer-silksong-neonforge', 'product-silksong-pc', 'seller-neonforge', 1724, 'EUR', TRUE),
    ('offer-minecraft', 'product-minecraft-pc', 'seller-northstar', 2499, 'EUR', TRUE),
    ('offer-minecraft-pixelharbor', 'product-minecraft-pc', 'seller-pixelharbor', 2624, 'EUR', TRUE),
    ('offer-minecraft-cobaltfox', 'product-minecraft-pc', 'seller-cobaltfox', 2749, 'EUR', TRUE),
    ('offer-minecraft-neonforge', 'product-minecraft-pc', 'seller-neonforge', 2874, 'EUR', TRUE),
    ('offer-terraria', 'product-terraria-pc', 'seller-pixelharbor', 799, 'EUR', TRUE),
    ('offer-terraria-northstar', 'product-terraria-pc', 'seller-northstar', 839, 'EUR', TRUE),
    ('offer-terraria-cobaltfox', 'product-terraria-pc', 'seller-cobaltfox', 879, 'EUR', TRUE),
    ('offer-terraria-neonforge', 'product-terraria-pc', 'seller-neonforge', 919, 'EUR', TRUE),
    ('offer-gta-vi-northstar', 'product-grand-theft-auto-vi-ps5', 'seller-northstar', 6999, 'EUR', TRUE),
    ('offer-gta-vi-pixelharbor', 'product-grand-theft-auto-vi-ps5', 'seller-pixelharbor', 7349, 'EUR', TRUE),
    ('offer-gta-vi-cobaltfox', 'product-grand-theft-auto-vi-ps5', 'seller-cobaltfox', 7699, 'EUR', TRUE),
    ('offer-gta-vi-neonforge', 'product-grand-theft-auto-vi-ps5', 'seller-neonforge', 8049, 'EUR', TRUE),
    ('offer-sekiro-northstar', 'product-sekiro-shadows-die-twice-pc', 'seller-northstar', 4999, 'EUR', TRUE),
    ('offer-sekiro-pixelharbor', 'product-sekiro-shadows-die-twice-pc', 'seller-pixelharbor', 5249, 'EUR', TRUE),
    ('offer-sekiro-cobaltfox', 'product-sekiro-shadows-die-twice-pc', 'seller-cobaltfox', 5499, 'EUR', TRUE),
    ('offer-sekiro-neonforge', 'product-sekiro-shadows-die-twice-pc', 'seller-neonforge', 5749, 'EUR', TRUE)
ON CONFLICT (id) DO UPDATE
SET price_cents = EXCLUDED.price_cents,
    currency = EXCLUDED.currency,
    active = EXCLUDED.active;

DELETE FROM digital_keys dk
USING offers o
WHERE dk.offer_id = o.id
  AND o.product_id IN (
      'product-hollow-knight-pc',
      'product-silksong-pc',
      'product-minecraft-pc',
      'product-terraria-pc',
      'product-grand-theft-auto-vi-ps5',
      'product-sekiro-shadows-die-twice-pc'
  )
  AND NOT EXISTS (
      SELECT 1 FROM order_items oi WHERE oi.digital_key_id = dk.id
  );

INSERT INTO digital_keys (offer_id, code)
SELECT inventory.offer_id,
       'DEMO-NOT-REDEEMABLE-' || upper(replace(inventory.offer_id, 'offer-', '')) || '-M004-' || lpad(key_number::text, 3, '0')
FROM (VALUES
    ('offer-hollow-knight', 3),
    ('offer-hollow-knight-pixelharbor', 5),
    ('offer-hollow-knight-cobaltfox', 7),
    ('offer-hollow-knight-neonforge', 4),
    ('offer-silksong', 5),
    ('offer-silksong-northstar', 3),
    ('offer-silksong-cobaltfox', 6),
    ('offer-silksong-neonforge', 8),
    ('offer-minecraft', 8),
    ('offer-minecraft-pixelharbor', 6),
    ('offer-minecraft-cobaltfox', 5),
    ('offer-minecraft-neonforge', 4),
    ('offer-terraria', 4),
    ('offer-terraria-northstar', 8),
    ('offer-terraria-cobaltfox', 5),
    ('offer-terraria-neonforge', 7),
    ('offer-gta-vi-northstar', 5),
    ('offer-gta-vi-pixelharbor', 4),
    ('offer-gta-vi-cobaltfox', 8),
    ('offer-gta-vi-neonforge', 3),
    ('offer-sekiro-northstar', 8),
    ('offer-sekiro-pixelharbor', 5),
    ('offer-sekiro-cobaltfox', 4),
    ('offer-sekiro-neonforge', 6)
) AS inventory(offer_id, key_count)
CROSS JOIN LATERAL generate_series(1, inventory.key_count) AS generated(key_number)
ON CONFLICT (code) DO NOTHING;

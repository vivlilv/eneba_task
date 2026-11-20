CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS products_title_trgm_idx ON products USING GIN (lower(title) gin_trgm_ops);

INSERT INTO products (id, slug, title, edition, platform, region, category, description)
VALUES
    ('product-hollow-knight-pc', 'hollow-knight-pc', 'Hollow Knight', 'Standard Edition', 'PC', 'Global', 'Metroidvania', 'A fictional demo listing for a hand-drawn action-adventure game.'),
    ('product-silksong-pc', 'hollow-knight-silksong-pc', 'Hollow Knight: Silksong', 'Standard Edition', 'PC', 'Global', 'Metroidvania', 'A fictional demo listing for a fast-paced adventure.'),
    ('product-minecraft-pc', 'minecraft-pc', 'Minecraft', 'Java Edition', 'PC', 'Global', 'Sandbox', 'A fictional demo listing for a creative sandbox game.'),
    ('product-terraria-pc', 'terraria-pc', 'Terraria', 'Standard Edition', 'PC', 'Global', 'Sandbox', 'A fictional demo listing for a 2D sandbox adventure.')
ON CONFLICT (id) DO NOTHING;

INSERT INTO offers (id, product_id, seller_id, price_cents, currency, active)
VALUES
    ('offer-hollow-knight', 'product-hollow-knight-pc', 'seller-northstar', 699, 'EUR', TRUE),
    ('offer-silksong', 'product-silksong-pc', 'seller-pixelharbor', 1499, 'EUR', TRUE),
    ('offer-minecraft', 'product-minecraft-pc', 'seller-northstar', 2499, 'EUR', TRUE),
    ('offer-terraria', 'product-terraria-pc', 'seller-pixelharbor', 799, 'EUR', TRUE)
ON CONFLICT (id) DO NOTHING;

INSERT INTO digital_keys (offer_id, code)
SELECT inventory.offer_id,
       'DEMO-NOT-REDEEMABLE-' || upper(replace(inventory.offer_id, 'offer-', '')) || '-' || lpad(key_number::text, 3, '0')
FROM (VALUES
    ('offer-hollow-knight', 12),
    ('offer-silksong', 7),
    ('offer-minecraft', 19),
    ('offer-terraria', 5)
) AS inventory(offer_id, key_count)
CROSS JOIN LATERAL generate_series(1, inventory.key_count) AS generated(key_number)
ON CONFLICT (code) DO NOTHING;

INSERT INTO homepage_blocks (id, title, kind, eyebrow, body, cta_label, cta_url, position)
VALUES ('block-more-to-discover', 'More to discover', 'product_rail', 'EXPLORE MORE', 'A few more worlds to explore, from underground kingdoms to open-ended sandboxes.', '', '', 4)
ON CONFLICT (id) DO NOTHING;

INSERT INTO homepage_block_products (block_id, product_id, position)
VALUES
    ('block-more-to-discover', 'product-hollow-knight-pc', 1),
    ('block-more-to-discover', 'product-silksong-pc', 2),
    ('block-more-to-discover', 'product-minecraft-pc', 3),
    ('block-more-to-discover', 'product-terraria-pc', 4)
ON CONFLICT (block_id, product_id) DO NOTHING;

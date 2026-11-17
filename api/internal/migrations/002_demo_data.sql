INSERT INTO products (id, slug, title, edition, platform, region, category, description)
VALUES
    ('product-demo-adventure-pc-eu', 'demo-adventure-pc-eu', 'Demo Adventure', 'Standard Edition', 'PC', 'Europe', 'Adventure', 'A fictional demo product used to exercise the marketplace purchase flow.'),
    ('product-demo-racer-console-eu', 'demo-racer-console-eu', 'Demo Racer', 'Standard Edition', 'Console', 'Europe', 'Racing', 'A fictional demo product used to exercise homepage product blocks.')
ON CONFLICT (id) DO NOTHING;

INSERT INTO sellers (id, name, rating)
VALUES
    ('seller-northstar', 'Northstar Digital', 4.92),
    ('seller-pixelharbor', 'Pixel Harbor', 4.81)
ON CONFLICT (id) DO NOTHING;

INSERT INTO offers (id, product_id, seller_id, price_cents, currency, active)
VALUES
    ('offer-adventure-northstar', 'product-demo-adventure-pc-eu', 'seller-northstar', 1999, 'EUR', TRUE),
    ('offer-adventure-pixelharbor', 'product-demo-adventure-pc-eu', 'seller-pixelharbor', 2149, 'EUR', TRUE),
    ('offer-racer-northstar', 'product-demo-racer-console-eu', 'seller-northstar', 1299, 'EUR', TRUE)
ON CONFLICT (id) DO NOTHING;

INSERT INTO digital_keys (offer_id, code)
VALUES
    ('offer-adventure-northstar', 'DEMO-NOT-REDEEMABLE-ADVENTURE-001'),
    ('offer-adventure-northstar', 'DEMO-NOT-REDEEMABLE-ADVENTURE-002'),
    ('offer-adventure-pixelharbor', 'DEMO-NOT-REDEEMABLE-PIXEL-001'),
    ('offer-adventure-pixelharbor', 'DEMO-NOT-REDEEMABLE-PIXEL-002'),
    ('offer-racer-northstar', 'DEMO-NOT-REDEEMABLE-RACER-001')
ON CONFLICT (code) DO NOTHING;

INSERT INTO homepage_blocks (id, title, kind, eyebrow, body, cta_label, cta_url, position)
VALUES
    ('block-hero', 'Big games. Better prices.', 'hero', 'LEVEL UP FOR LESS', 'Find your next favorite and compare offers from trusted sellers.', 'Explore picks', '#block-featured', 1),
    ('block-featured', 'Picked for you', 'featured', 'CURATED FOR YOU', 'Compare available offers on hand-picked digital games.', '', '', 2),
    ('block-trending', 'Popular right now', 'product_rail', 'TRENDING', 'Popular products with offers ready to buy.', '', '', 3)
ON CONFLICT (id) DO NOTHING;

INSERT INTO homepage_block_products (block_id, product_id, position)
VALUES
    ('block-featured', 'product-demo-adventure-pc-eu', 1),
    ('block-trending', 'product-demo-racer-console-eu', 1),
    ('block-trending', 'product-demo-adventure-pc-eu', 2)
ON CONFLICT (block_id, product_id) DO NOTHING;

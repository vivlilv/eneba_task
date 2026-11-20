UPDATE homepage_blocks
SET cta_url = '#block-cheap-games'
WHERE id = 'block-hero';

DELETE FROM homepage_blocks
WHERE id IN ('block-featured', 'block-trending');

UPDATE homepage_blocks
SET position = 3
WHERE id = 'block-more-to-discover';

INSERT INTO homepage_blocks (id, title, kind, eyebrow, body, cta_label, cta_url, position)
VALUES (
    'block-cheap-games',
    'Cheap games',
    'cheap_games',
    'UNDER €10',
    'Good games at low prices. Every pick has an available seller offer under €10.',
    '',
    '',
    2
)
ON CONFLICT (id) DO UPDATE
SET title = EXCLUDED.title,
    kind = EXCLUDED.kind,
    eyebrow = EXCLUDED.eyebrow,
    body = EXCLUDED.body,
    cta_label = EXCLUDED.cta_label,
    cta_url = EXCLUDED.cta_url,
    position = EXCLUDED.position;

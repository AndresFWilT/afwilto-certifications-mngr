ALTER TABLE categories ADD COLUMN color text;

-- Backfill: deterministic assignment from the same 8-color palette the
-- frontend already uses, cycling by path order, so nothing visually jumps
-- for existing data once the frontend switches from hash-derived color to
-- this persisted one.
WITH ordered AS (
  SELECT id, row_number() OVER (ORDER BY path) - 1 AS rn FROM categories
),
palette(idx, hex) AS (
  VALUES (0,'#22D3EE'),(1,'#A78BFA'),(2,'#FBBF24'),(3,'#34D399'),
         (4,'#FB7185'),(5,'#38BDF8'),(6,'#A3E635'),(7,'#E879F9')
)
UPDATE categories c
SET color = p.hex
FROM ordered o JOIN palette p ON p.idx = (o.rn % 8)
WHERE c.id = o.id;

ALTER TABLE categories
    ALTER COLUMN color SET NOT NULL,
    ADD CONSTRAINT categories_color_hex CHECK (color ~ '^#[0-9A-F]{6}$');

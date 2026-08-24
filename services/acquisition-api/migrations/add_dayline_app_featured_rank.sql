-- Dayline app catalog featuring. Nullable and additive: NULL means not
-- featured (existing rows unchanged); lower rank sorts earlier in the
-- app's featured hero row.
ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS app_featured_rank INTEGER;

-- Dayline app catalog enrichment columns.
--
-- The Dayline mobile app catalog (GET /v1/app/catalog) prefers these
-- app-specific fields over the landing-page lp_copy fields, since lp_copy is
-- shaped for web LP rendering (heroTitle/heDescription/etc.) rather than a
-- product catalog card (name/tagline/description/category/artwork/sample).
-- All columns are nullable; when app_* is null the catalog handler falls back
-- to a pragmatic lp_copy mapping. Additive only, no backfill required.
ALTER TABLE campaigns
    ADD COLUMN IF NOT EXISTS app_name TEXT,
    ADD COLUMN IF NOT EXISTS app_tagline TEXT,
    ADD COLUMN IF NOT EXISTS app_description TEXT,
    ADD COLUMN IF NOT EXISTS app_category VARCHAR(80),
    ADD COLUMN IF NOT EXISTS app_artwork_url TEXT,
    ADD COLUMN IF NOT EXISTS app_sample_content TEXT;

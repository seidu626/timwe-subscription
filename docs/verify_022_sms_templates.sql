-- slice-harness: allow-new-canonical-path: requested scratch-database migration assertions.
-- Run after applying migrations through 022: psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f docs/verify_022_sms_templates.sql
-- This script leaves no fixture data behind.

BEGIN;

INSERT INTO tenants (id, tenant_key, name, status, default_country)
VALUES ('02200000-0000-0000-0000-000000000022', 'verify-022', 'Verify 022', 'ACTIVE', 'GH');

INSERT INTO subscriptions (
    tenant_id, partner_role_id, user_identifier, user_identifier_type,
    product_id, status
)
VALUES (
    '02200000-0000-0000-0000-000000000022', 2117, '233240000022',
    'MSISDN', 32535, 'active'
);

INSERT INTO tenant_product_sms_templates (tenant_id, product_id, event_type, enabled, template)
VALUES (
    '02200000-0000-0000-0000-000000000022', 32535, 'USER_OPTIN', TRUE,
    'Welcome to product {{product_id}}. Subscriber ending {{msisdn}} is active.'
);

-- Direct confirmation enqueue: no synthetic cadence records are required.
INSERT INTO message_outbox (
    job_id, idempotency_key, subscription_id, series_id, content_item_id,
    message_text, planned_send_at, tenant_id
)
SELECT
    '02200000-0000-0000-0000-000000000001', 'verify-022-direct', s.id,
    NULL, NULL, 'Welcome to product 32535. Subscriber ending 0022 is active.',
    NOW(), s.tenant_id
FROM subscriptions s
WHERE s.tenant_id = '02200000-0000-0000-0000-000000000022'
  AND s.product_id = 32535
  AND s.user_identifier = '233240000022';

-- Existing cadence enqueue: series/content references remain the text source.
INSERT INTO product_message_series (tenant_id, partner_role_id, product_id, name)
VALUES ('02200000-0000-0000-0000-000000000022', 2117, 32535, 'verify-022')
RETURNING id \gset cadence_

INSERT INTO message_content_items (tenant_id, series_id, content_version, seq_no, message_text)
VALUES ('02200000-0000-0000-0000-000000000022', :cadence_id, 1, 1, 'Existing cadence text')
RETURNING id \gset content_

INSERT INTO message_outbox (
    job_id, idempotency_key, subscription_id, series_id, content_item_id,
    message_text, planned_send_at, tenant_id
)
SELECT
    '02200000-0000-0000-0000-000000000002', 'verify-022-cadence', s.id,
    :cadence_id, :content_id, NULL, NOW(), s.tenant_id
FROM subscriptions s
WHERE s.tenant_id = '02200000-0000-0000-0000-000000000022'
  AND s.product_id = 32535
  AND s.user_identifier = '233240000022';

DO $$
BEGIN
    IF (SELECT count(*) FROM message_outbox WHERE idempotency_key IN ('verify-022-direct', 'verify-022-cadence')) <> 2 THEN
        RAISE EXCEPTION 'expected both direct and cadence outbox rows';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM message_outbox
        WHERE idempotency_key = 'verify-022-direct'
          AND series_id IS NULL AND content_item_id IS NULL AND message_text IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'direct confirmation row does not satisfy direct-text shape';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM message_outbox
        WHERE idempotency_key = 'verify-022-cadence'
          AND series_id IS NOT NULL AND content_item_id IS NOT NULL AND message_text IS NULL
    ) THEN
        RAISE EXCEPTION 'cadence row does not satisfy cadence-content shape';
    END IF;
END $$;

SELECT idempotency_key, series_id IS NULL AS direct_text, message_text
FROM message_outbox
WHERE idempotency_key IN ('verify-022-direct', 'verify-022-cadence')
ORDER BY idempotency_key;

ROLLBACK;

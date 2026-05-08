# CORRECTED: Strategy Using Notifications Table for Charging Status

## Key Discovery: Notifications Table Contains Charging Status

The `notifications` table receives webhook notifications from TIMWE with types:
- **CHARGE**: Successful charging event
- **USER_RENEWED**: Successful renewal/recurring charge
- **USER_OPTOUT**: User opted out
- **RENEWAL**: Renewal attempt (may or may not be successful)
- **USER_OPTIN**: User opted in

## Understanding the Charging Flow

1. User subscribes → `USER_OPTIN` notification
2. TIMWE attempts charging → Asynchronous process
3. If successful → `CHARGE` notification
4. For recurring → `USER_RENEWED` notification
5. If user cancels → `USER_OPTOUT` notification

## Identifying Failed Charging Subscriptions

### Query to Find Subscriptions WITHOUT Successful Charging

```sql
-- Find active subscriptions that never received a CHARGE notification
WITH active_subscriptions AS (
    SELECT DISTINCT
        s.id,
        s.user_identifier as msisdn,
        s.product_id,
        s.entry_channel,
        s.created_at as subscription_date,
        s.status
    FROM subscriptions s
    WHERE s.status = 'active' 
       OR s.status IS NULL
),
successful_charges AS (
    SELECT DISTINCT
        n.msisdn,
        n.product_id,
        MAX(n.created_at) as last_charge_date,
        COUNT(*) as charge_count
    FROM notifications n
    WHERE n.type IN ('CHARGE', 'USER_RENEWED')
    GROUP BY n.msisdn, n.product_id
),
optin_notifications AS (
    SELECT DISTINCT
        n.msisdn,
        n.product_id,
        n.created_at as optin_date
    FROM notifications n
    WHERE n.type = 'USER_OPTIN'
)
SELECT 
    a.id,
    a.msisdn,
    a.product_id,
    a.entry_channel,
    a.subscription_date,
    o.optin_date,
    c.last_charge_date,
    c.charge_count,
    CASE 
        WHEN c.msisdn IS NULL THEN 'NEVER_CHARGED'
        WHEN c.last_charge_date < NOW() - INTERVAL '30 days' THEN 'NOT_RECENTLY_CHARGED'
        ELSE 'RECENTLY_CHARGED'
    END as charging_status,
    EXTRACT(DAY FROM NOW() - a.subscription_date) as days_since_subscription
FROM active_subscriptions a
LEFT JOIN optin_notifications o ON a.msisdn = o.msisdn AND a.product_id = o.product_id
LEFT JOIN successful_charges c ON a.msisdn = c.msisdn AND a.product_id = c.product_id
WHERE 
    -- Never charged OR not charged in last 30 days
    (c.msisdn IS NULL OR c.last_charge_date < NOW() - INTERVAL '30 days')
    -- Subscription is at least 1 day old (time for charging to process)
    AND a.subscription_date < NOW() - INTERVAL '1 day'
ORDER BY a.subscription_date;
```

### Analyze Charging Failure Patterns

```sql
-- Get statistics on charging failures
WITH charging_analysis AS (
    SELECT 
        s.user_identifier as msisdn,
        s.product_id,
        s.created_at as subscription_date,
        s.status,
        s.entry_channel,
        -- Check for OPTIN notification
        EXISTS(
            SELECT 1 FROM notifications n 
            WHERE n.msisdn = s.user_identifier 
            AND n.product_id = s.product_id 
            AND n.type = 'USER_OPTIN'
        ) as has_optin,
        -- Check for CHARGE notification
        EXISTS(
            SELECT 1 FROM notifications n 
            WHERE n.msisdn = s.user_identifier 
            AND n.product_id = s.product_id 
            AND n.type IN ('CHARGE', 'USER_RENEWED')
        ) as has_charge,
        -- Get last charge date
        (
            SELECT MAX(n.created_at) 
            FROM notifications n 
            WHERE n.msisdn = s.user_identifier 
            AND n.product_id = s.product_id 
            AND n.type IN ('CHARGE', 'USER_RENEWED')
        ) as last_charge_date
    FROM subscriptions s
    WHERE s.status = 'active' OR s.status IS NULL
)
SELECT 
    COUNT(*) as total_active,
    SUM(CASE WHEN has_optin THEN 1 ELSE 0 END) as with_optin,
    SUM(CASE WHEN has_charge THEN 1 ELSE 0 END) as with_charge,
    SUM(CASE WHEN has_optin AND NOT has_charge THEN 1 ELSE 0 END) as optin_no_charge,
    SUM(CASE WHEN NOT has_optin AND NOT has_charge THEN 1 ELSE 0 END) as no_optin_no_charge,
    SUM(CASE WHEN last_charge_date < NOW() - INTERVAL '30 days' THEN 1 ELSE 0 END) as stale_charges
FROM charging_analysis;
```

## Updated Implementation Strategy

### Step 1: Create View for Failed Charging Subscriptions

```sql
CREATE OR REPLACE VIEW charging_failed_subscriptions AS
WITH active_subs AS (
    SELECT 
        s.id,
        s.user_identifier as msisdn,
        s.product_id,
        s.entry_channel,
        s.created_at,
        s.status
    FROM subscriptions s
    WHERE (s.status = 'active' OR s.status IS NULL)
    AND s.created_at < NOW() - INTERVAL '1 day' -- Allow time for initial charging
),
charge_history AS (
    SELECT 
        n.msisdn,
        n.product_id,
        MAX(n.created_at) as last_charge,
        COUNT(*) as total_charges
    FROM notifications n
    WHERE n.type IN ('CHARGE', 'USER_RENEWED')
    GROUP BY n.msisdn, n.product_id
)
SELECT 
    a.id as subscription_id,
    a.msisdn,
    a.product_id,
    a.entry_channel,
    a.created_at as subscription_date,
    ch.last_charge,
    ch.total_charges,
    CASE
        WHEN ch.msisdn IS NULL THEN 'NEVER_CHARGED'
        WHEN ch.last_charge < NOW() - INTERVAL '30 days' THEN 'CHARGING_STALE'
        WHEN ch.last_charge < NOW() - INTERVAL '7 days' THEN 'CHARGING_DELAYED'
        ELSE 'CHARGING_RECENT'
    END as charging_status,
    EXTRACT(DAY FROM NOW() - a.created_at) as days_since_subscription,
    EXTRACT(DAY FROM NOW() - ch.last_charge) as days_since_last_charge
FROM active_subs a
LEFT JOIN charge_history ch ON a.msisdn = ch.msisdn AND a.product_id = ch.product_id
WHERE ch.msisdn IS NULL  -- Never charged
   OR ch.last_charge < NOW() - INTERVAL '30 days';  -- Or not charged recently
```

### Step 2: Count Actual Failed Subscriptions

```sql
-- Get the actual count of subscriptions with charging issues
SELECT 
    charging_status,
    COUNT(*) as count,
    COUNT(DISTINCT msisdn) as unique_msisdns
FROM charging_failed_subscriptions
GROUP BY charging_status;

-- Get total
SELECT COUNT(*) as total_failed_charging
FROM charging_failed_subscriptions;
```

### Step 3: Updated Migration Script

```sql
-- Add columns to track charging status from notifications
ALTER TABLE subscriptions 
ADD COLUMN IF NOT EXISTS last_charge_notification_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS total_charge_notifications INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS charging_health_status VARCHAR(50);

-- Create index for performance
CREATE INDEX IF NOT EXISTS idx_subscriptions_charge_notification 
    ON subscriptions(last_charge_notification_at);

-- Update existing records with charging data
UPDATE subscriptions s
SET 
    last_charge_notification_at = ch.last_charge,
    total_charge_notifications = ch.total_charges,
    charging_health_status = CASE
        WHEN ch.last_charge IS NULL THEN 'NEVER_CHARGED'
        WHEN ch.last_charge < NOW() - INTERVAL '30 days' THEN 'STALE'
        WHEN ch.last_charge < NOW() - INTERVAL '7 days' THEN 'DELAYED'
        ELSE 'HEALTHY'
    END
FROM (
    SELECT 
        msisdn,
        product_id,
        MAX(created_at) as last_charge,
        COUNT(*) as total_charges
    FROM notifications
    WHERE type IN ('CHARGE', 'USER_RENEWED')
    GROUP BY msisdn, product_id
) ch
WHERE s.user_identifier = ch.msisdn 
  AND s.product_id = ch.product_id;
```

### Step 4: Updated Query for Charging Failures

```sql
-- Optimized query to identify charging failures
CREATE OR REPLACE FUNCTION identify_charging_failures(
    p_limit INTEGER DEFAULT NULL,
    p_offset INTEGER DEFAULT 0,
    p_days_threshold INTEGER DEFAULT 30
)
RETURNS TABLE (
    subscription_id INTEGER,
    msisdn VARCHAR,
    product_id INTEGER,
    entry_channel VARCHAR,
    subscription_date TIMESTAMP,
    last_charge_date TIMESTAMP,
    days_without_charge INTEGER,
    charging_status VARCHAR
) AS $$
BEGIN
    RETURN QUERY
    WITH charge_data AS (
        SELECT 
            n.msisdn,
            n.product_id,
            MAX(n.created_at) as last_charge
        FROM notifications n
        WHERE n.type IN ('CHARGE', 'USER_RENEWED')
        GROUP BY n.msisdn, n.product_id
    )
    SELECT 
        s.id as subscription_id,
        s.user_identifier as msisdn,
        s.product_id,
        s.entry_channel,
        s.created_at as subscription_date,
        cd.last_charge as last_charge_date,
        CASE 
            WHEN cd.last_charge IS NULL THEN 
                EXTRACT(DAY FROM NOW() - s.created_at)::INTEGER
            ELSE 
                EXTRACT(DAY FROM NOW() - cd.last_charge)::INTEGER
        END as days_without_charge,
        CASE
            WHEN cd.last_charge IS NULL THEN 'NEVER_CHARGED'
            WHEN cd.last_charge < NOW() - INTERVAL '30 days' THEN 'STALE_CHARGE'
            ELSE 'RECENT_CHARGE'
        END as charging_status
    FROM subscriptions s
    LEFT JOIN charge_data cd ON s.user_identifier = cd.msisdn 
                              AND s.product_id = cd.product_id
    WHERE (s.status = 'active' OR s.status IS NULL)
      AND s.created_at < NOW() - INTERVAL '1 day'
      AND (cd.last_charge IS NULL 
           OR cd.last_charge < NOW() - (p_days_threshold || ' days')::INTERVAL)
    ORDER BY days_without_charge DESC
    LIMIT p_limit
    OFFSET p_offset;
END;
$$ LANGUAGE plpgsql;
```

### Step 5: Analysis Script Update

```bash
#!/bin/bash
# Analyze charging failures using notifications table

echo "Analyzing Charging Failures from Notifications..."

# Get counts by charging status
psql -d subscription_manager -c "
WITH charge_analysis AS (
    SELECT 
        s.user_identifier as msisdn,
        s.product_id,
        s.created_at as sub_date,
        (SELECT MAX(created_at) FROM notifications n 
         WHERE n.msisdn = s.user_identifier 
         AND n.product_id = s.product_id 
         AND n.type IN ('CHARGE', 'USER_RENEWED')) as last_charge
    FROM subscriptions s
    WHERE s.status = 'active' OR s.status IS NULL
)
SELECT 
    CASE
        WHEN last_charge IS NULL THEN 'Never Charged'
        WHEN last_charge < NOW() - INTERVAL '60 days' THEN '> 60 days ago'
        WHEN last_charge < NOW() - INTERVAL '30 days' THEN '30-60 days ago'
        WHEN last_charge < NOW() - INTERVAL '7 days' THEN '7-30 days ago'
        ELSE '< 7 days ago'
    END as last_charge_category,
    COUNT(*) as subscription_count,
    ROUND(COUNT(*)::numeric / (SELECT COUNT(*) FROM charge_analysis) * 100, 2) as percentage
FROM charge_analysis
GROUP BY last_charge_category
ORDER BY 
    CASE last_charge_category
        WHEN 'Never Charged' THEN 1
        WHEN '> 60 days ago' THEN 2
        WHEN '30-60 days ago' THEN 3
        WHEN '7-30 days ago' THEN 4
        ELSE 5
    END;
"

# Get total count of failed charging
FAILED_COUNT=$(psql -t -d subscription_manager -c "
    SELECT COUNT(DISTINCT s.id)
    FROM subscriptions s
    LEFT JOIN LATERAL (
        SELECT MAX(created_at) as last_charge
        FROM notifications n
        WHERE n.msisdn = s.user_identifier 
        AND n.product_id = s.product_id
        AND n.type IN ('CHARGE', 'USER_RENEWED')
    ) ch ON true
    WHERE (s.status = 'active' OR s.status IS NULL)
    AND s.created_at < NOW() - INTERVAL '1 day'
    AND (ch.last_charge IS NULL OR ch.last_charge < NOW() - INTERVAL '30 days')
")

echo "Total Subscriptions with Charging Issues: $FAILED_COUNT"
```

## Processing Strategy for Identified Failures

Now that we can identify charging failures, here's the processing approach:

```go
// Process subscriptions with charging failures
func (s *SubscriptionService) ProcessChargingFailures(batchSize int) error {
    // Query for failed charging subscriptions
    query := `
        SELECT 
            s.id,
            s.user_identifier as msisdn,
            s.product_id,
            s.entry_channel
        FROM subscriptions s
        LEFT JOIN LATERAL (
            SELECT MAX(created_at) as last_charge
            FROM notifications n
            WHERE n.msisdn = s.user_identifier 
            AND n.product_id = s.product_id
            AND n.type IN ('CHARGE', 'USER_RENEWED')
        ) ch ON true
        WHERE (s.status = 'active' OR s.status IS NULL)
        AND s.created_at < NOW() - INTERVAL '1 day'
        AND (ch.last_charge IS NULL OR ch.last_charge < NOW() - INTERVAL '30 days')
        AND (s.resubscribe_status IS NULL OR s.resubscribe_status != 'completed')
        ORDER BY s.created_at
        LIMIT $1
    `
    
    rows, err := s.db.Query(query, batchSize)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    var failures []ChargingFailure
    for rows.Next() {
        var f ChargingFailure
        if err := rows.Scan(&f.ID, &f.MSISDN, &f.ProductID, &f.EntryChannel); err != nil {
            continue
        }
        failures = append(failures, f)
    }
    
    // Process each failure
    for _, failure := range failures {
        // Unsubscribe and resubscribe
        if err := s.ResubscribeUser(failure.MSISDN, failure.EntryChannel, []string{fmt.Sprintf("%d", failure.ProductID)}); err != nil {
            s.logger.Error("Failed to resubscribe", 
                zap.String("msisdn", failure.MSISDN),
                zap.Int("productId", failure.ProductID),
                zap.Error(err))
            continue
        }
        
        // Mark as processed
        s.markAsProcessed(failure.ID)
    }
    
    return nil
}
```

## Verification Query

To verify this matches the reported 25M number:

```sql
-- Final count of subscriptions with charging issues
SELECT 
    'Total Active Subscriptions' as metric,
    COUNT(*) as count
FROM subscriptions
WHERE status = 'active' OR status IS NULL

UNION ALL

SELECT 
    'Never Received Charge Notification' as metric,
    COUNT(DISTINCT s.id) as count
FROM subscriptions s
LEFT JOIN notifications n ON s.user_identifier = n.msisdn 
    AND s.product_id = n.product_id
    AND n.type IN ('CHARGE', 'USER_RENEWED')
WHERE (s.status = 'active' OR s.status IS NULL)
AND n.id IS NULL

UNION ALL

SELECT 
    'Not Charged in 30+ Days' as metric,
    COUNT(DISTINCT s.id) as count
FROM subscriptions s
LEFT JOIN LATERAL (
    SELECT MAX(created_at) as last_charge
    FROM notifications n
    WHERE n.msisdn = s.user_identifier 
    AND n.product_id = s.product_id
    AND n.type IN ('CHARGE', 'USER_RENEWED')
) ch ON true
WHERE (s.status = 'active' OR s.status IS NULL)
AND ch.last_charge < NOW() - INTERVAL '30 days'

UNION ALL

SELECT 
    'TOTAL CHARGING FAILURES' as metric,
    COUNT(DISTINCT s.id) as count
FROM subscriptions s
LEFT JOIN LATERAL (
    SELECT MAX(created_at) as last_charge
    FROM notifications n
    WHERE n.msisdn = s.user_identifier 
    AND n.product_id = s.product_id
    AND n.type IN ('CHARGE', 'USER_RENEWED')
) ch ON true
WHERE (s.status = 'active' OR s.status IS NULL)
AND s.created_at < NOW() - INTERVAL '1 day'
AND (ch.last_charge IS NULL OR ch.last_charge < NOW() - INTERVAL '30 days');
```

This approach should identify the actual charging failures and likely match or be close to the reported 25,169,944 number!

---https://medium.com/coding-blocks/creating-user-database-and-adding-access-on-postgresql-8bfcd2f4a91e
--- sudo -u postgres psql
create database subscription_manager;
-- Replace the password below with a locally generated secret or a secret-manager reference.
create user sm_admin with encrypted password 'REPLACE_WITH_LOCAL_DB_PASSWORD';
grant all privileges on database subscription_manager to sm_admin;
GRANT ALL ON SCHEMA public TO sm_admin;
ALTER DATABASE subscription_manager OWNER TO sm_admin;

        CREATE TABLE userbase (
                                  Id SERIAL PRIMARY KEY,
                                  Msisdn VARCHAR(15) NOT NULL UNIQUE,
                                  Type VARCHAR(20) NOT NULL
        );

        -- Indexes to improve lookup performance
        CREATE INDEX idx_userbase_msisdn ON userbase (Msisdn);
        CREATE INDEX idx_userbase_type ON userbase (Type);


-- products table creation
CREATE TABLE IF NOT EXISTS products (
                                        id SERIAL PRIMARY KEY,
                                        product_id VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    price_point_id INT NOT NULL,
    price_point_value DECIMAL(10, 2) NOT NULL,
    short_code VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
ALTER TABLE products ALTER COLUMN price_point_value TYPE DECIMAL(10, 2) USING price_point_value::DECIMAL;




CREATE TABLE listResponse (
                               id SERIAL PRIMARY KEY,
                               partner_role INTEGER,
                               external_tx_id VARCHAR(255),
                               product_id INTEGER,
                                pricepoint_id INTEGER,
                               mcc VARCHAR(10),
                               mnc VARCHAR(10),
                               request VARCHAR(20),
                               type VARCHAR(20),
                               large_account VARCHAR(50),
                               transaction_uuid VARCHAR(255),
                               mno_delivery_code VARCHAR(50),
                               entry_channel VARCHAR(50),
                               message_type VARCHAR(50),
                               message TEXT,
                               created_at TIMESTAMPTZ DEFAULT NOW(),
                               tags TEXT[]
);

ALTER TABLE listResponse
    ADD COLUMN type VARCHAR(20) NULL;

CREATE TABLE listResponse (
                               id SERIAL PRIMARY KEY,
                               partner_role_id INTEGER NOT NULL,
                               user_identifier VARCHAR(50) NOT NULL,
                               user_identifier_type VARCHAR(20) NOT NULL,
                               product_id INTEGER NOT NULL,
                               mcc VARCHAR(5),
                               mnc VARCHAR(5),
                               entry_channel VARCHAR(20),
                               large_account VARCHAR(50),
                               sub_keyword VARCHAR(50),
                               tracking_id VARCHAR(50),
                               client_ip VARCHAR(50),
                               campaign_url VARCHAR(255),
                               transaction_auth_code VARCHAR(100),
                               status VARCHAR(20) DEFAULT 'active',
                               cancel_reason INTEGER,
                               cancel_source INTEGER,
                               start_date TIMESTAMP DEFAULT NOW(),
                               end_date TIMESTAMP
                               created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Invalid MSISDN tracking table
CREATE TABLE IF NOT EXISTS invalid_msisdn_logs (
    id SERIAL PRIMARY KEY,
    msisdn VARCHAR(15) NOT NULL,
    product_id INTEGER,
    pricepoint_id INTEGER,
    partner_role_id INTEGER,
    entry_channel VARCHAR(50),
    request_id VARCHAR(100),
    response_code VARCHAR(50),
    response_message TEXT,
    subscription_result VARCHAR(100),
    subscription_error TEXT,
    external_tx_id VARCHAR(255),
    transaction_id VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Subscriptions table for tracking user subscriptions
CREATE TABLE IF NOT EXISTS subscriptions (
    id SERIAL PRIMARY KEY,
    partner_role_id INTEGER NOT NULL,
    user_identifier VARCHAR(50) NOT NULL,
    user_identifier_type VARCHAR(20) NOT NULL DEFAULT 'MSISDN',
    product_id INTEGER NOT NULL,
    mcc VARCHAR(5),
    mnc VARCHAR(5),
    entry_channel VARCHAR(20),
    large_account VARCHAR(50),
    sub_keyword VARCHAR(50),
    tracking_id VARCHAR(50),
    client_ip VARCHAR(50),
    campaign_url VARCHAR(255),
    transaction_auth_code VARCHAR(100),
    status VARCHAR(20) DEFAULT 'active',
    cancel_reason INTEGER,
    cancel_source INTEGER,
    start_date TIMESTAMP DEFAULT NOW(),
    end_date TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Notifications table for tracking system notifications
CREATE TABLE IF NOT EXISTS notifications (
    id SERIAL PRIMARY KEY,
    type VARCHAR(50) NOT NULL,
    msisdn VARCHAR(15),
    product_id INTEGER,
    message TEXT,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_msisdn ON invalid_msisdn_logs (msisdn);
CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_product_id ON invalid_msisdn_logs (product_id);
CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_created_at ON invalid_msisdn_logs (created_at);
CREATE INDEX IF NOT EXISTS idx_invalid_msisdn_logs_response_code ON invalid_msisdn_logs (response_code);

-- Notification scan indexes for monitor
CREATE INDEX IF NOT EXISTS idx_notifications_type_created_at ON notifications (type, created_at);
CREATE INDEX IF NOT EXISTS idx_notifications_type_id ON notifications (type, id);
CREATE INDEX IF NOT EXISTS idx_notifications_msisdn_product_created ON notifications (msisdn, product_id, created_at);

-- Subscription lookup/upsert helper indexes
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_product_status ON subscriptions (user_identifier, product_id, status);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_product ON subscriptions (user_identifier, product_id);

-- Insert some test data for debugging
-- Test products
INSERT INTO products (product_id, name, price_point_id, price_point_value, short_code) VALUES
('8509', 'Test Product 1', 1, 1.00, 'TEST1'),
('14392', 'Test Product 2', 2, 2.00, 'TEST2'),
('14396', 'Test Product 3', 3, 3.00, 'TEST3'),
('14397', 'Test Product 4', 4, 4.00, 'TEST4'),
('14398', 'Test Product 5', 5, 5.00, 'TEST5'),
('27188', 'Test Product 6', 6, 6.00, 'TEST6'),
('14439', 'Test Product 7', 7, 7.00, 'TEST7'),
('28366', 'Test Product 8', 8, 8.00, 'TEST8')
ON CONFLICT (product_id) DO NOTHING;

-- Test userbase entries
INSERT INTO userbase (msisdn, type) VALUES
('233241234567', 'Regular'),
('233241234568', 'Regular'),
('233241234569', 'Regular'),
('233241234570', 'Regular'),
('233241234571', 'Regular')
ON CONFLICT (msisdn) DO NOTHING;

-- Test subscriptions (some users with products, some without)
INSERT INTO subscriptions (partner_role_id, user_identifier, user_identifier_type, product_id, status) VALUES
(1, '233241234567', 'MSISDN', 8509, 'active'),
(1, '233241234568', 'MSISDN', 14392, 'active'),
(1, '233241234569', 'MSISDN', 14396, 'active')
ON CONFLICT (partner_role_id, user_identifier, product_id) DO NOTHING;

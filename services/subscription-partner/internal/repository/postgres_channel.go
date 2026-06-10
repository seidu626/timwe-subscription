package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrTenantChannelNotFound is returned by ChannelIDByKeys when no active row matches.
var ErrTenantChannelNotFound = errors.New("tenant channel not found")

// ChannelIDByKeys resolves a channel_key to its UUID for a given tenant.
// Returns ("", ErrTenantChannelNotFound) when no active row matches.
func (r *SubscriptionRepository) ChannelIDByKeys(tenantID, channelKey string) (string, error) {
	tenantID = strings.TrimSpace(tenantID)
	channelKey = strings.TrimSpace(channelKey)
	if tenantID == "" || channelKey == "" {
		return "", fmt.Errorf("tenantID and channelKey are required")
	}

	var channelUUID string
	err := r.db.QueryRowContext(r.ctx, `
		SELECT id::text FROM tenant_channels
		WHERE tenant_id = $1 AND channel_key = $2 AND status = 'ACTIVE'
		LIMIT 1
	`, tenantID, channelKey).Scan(&channelUUID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrTenantChannelNotFound
	}
	if err != nil {
		return "", fmt.Errorf("resolve channel by key: %w", err)
	}
	return channelUUID, nil
}

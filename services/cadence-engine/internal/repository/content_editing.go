package repository

import (
	"context"
	"database/sql"

	"github.com/seidu626/subscription-manager/cadence-engine/internal/domain"
)

// GetContentItemForSeries loads one content item by id, scoped to the series
// so a cross-series item id can never be edited through another series' route.
func (r *CadenceRepository) GetContentItemForSeries(ctx context.Context, seriesID, itemID int64) (*domain.ContentItem, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id::text, channel_id::text, series_id, content_version, COALESCE(seq_no, 0), message_text,
		       content_kind, link_url, cta_label, is_active, created_at, updated_at
		FROM message_content_items
		WHERE id = $1 AND series_id = $2
	`, itemID, seriesID)

	var c domain.ContentItem
	var tenantID, channelID, linkURL, ctaLabel sql.NullString
	if err := row.Scan(&c.ID, &tenantID, &channelID, &c.SeriesID, &c.ContentVersion, &c.SeqNo, &c.MessageText, &c.ContentKind, &linkURL, &ctaLabel, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	c.TenantID = nullStringPtr(tenantID)
	c.ChannelID = nullStringPtr(channelID)
	c.LinkURL = nullStringPtr(linkURL)
	c.CTALabel = nullStringPtr(ctaLabel)
	return &c, nil
}

// SetTxActor records the acting admin on the transaction so the
// message_content_revisions trigger can attribute the change.
func (r *CadenceRepository) SetTxActor(ctx context.Context, tx *sql.Tx, actor string) error {
	_, err := tx.ExecContext(ctx, `SELECT set_config('app.actor', $1, true)`, actor)
	return err
}

// UpdateContentItemTx edits a content item in place by id. Version and seq_no
// are deliberately immutable: renumbering a published version would skip or
// repeat messages for subscribers holding a cursor into it.
func (r *CadenceRepository) UpdateContentItemTx(ctx context.Context, tx *sql.Tx, itemID int64, messageText string, contentKind string, linkURL *string, ctaLabel *string, isActive bool) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE message_content_items
		SET message_text = $2,
		    content_kind = $3,
		    link_url = $4,
		    cta_label = $5,
		    is_active = $6
		WHERE id = $1
	`, itemID, messageText, contentKind, nullStringPtrValue(linkURL), nullStringPtrValue(ctaLabel), isActive)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CountOtherActiveContentItemsTx counts active items in a version excluding
// one item; used to refuse deactivating the last active item of the live
// version, which would stop every subscriber with "no_content".
func (r *CadenceRepository) CountOtherActiveContentItemsTx(ctx context.Context, tx *sql.Tx, seriesID int64, contentVersion int, excludeItemID int64) (int64, error) {
	var n int64
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM message_content_items
		WHERE series_id = $1 AND content_version = $2 AND is_active = TRUE AND id <> $3
	`, seriesID, contentVersion, excludeItemID).Scan(&n)
	return n, err
}

// ContentImpact reports the blast radius of editing a content version: how
// many subscriber states are actively receiving the series and how many
// planned-but-unsent outbox jobs reference items of that version.
func (r *CadenceRepository) ContentImpact(ctx context.Context, seriesID int64, contentVersion int) (activeStates int64, pendingJobs int64, err error) {
	err = r.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM subscription_message_state
			 WHERE series_id = $1 AND status = 'ACTIVE'),
			(SELECT COUNT(*) FROM message_outbox mo
			 JOIN message_content_items ci ON ci.id = mo.content_item_id
			 WHERE mo.series_id = $1 AND ci.content_version = $2
			   AND mo.status IN ('PENDING', 'RETRYING'))
	`, seriesID, contentVersion).Scan(&activeStates, &pendingJobs)
	return activeStates, pendingJobs, err
}

// CountContentItems counts all items (active or not) in a version.
func (r *CadenceRepository) CountContentItems(ctx context.Context, seriesID int64, contentVersion int) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM message_content_items
		WHERE series_id = $1 AND content_version = $2
	`, seriesID, contentVersion).Scan(&n)
	return n, err
}

// MaxContentVersion returns the highest content version authored for a series
// (0 when the series has no content yet).
func (r *CadenceRepository) MaxContentVersion(ctx context.Context, seriesID int64) (int, error) {
	var v int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(content_version), 0) FROM message_content_items
		WHERE series_id = $1
	`, seriesID).Scan(&v)
	return v, err
}

// CloneContentVersionTx copies every item of fromVersion into toVersion,
// preserving seq numbers, kinds, links, and active flags, so edits can be
// staged on a draft version and published atomically.
func (r *CadenceRepository) CloneContentVersionTx(ctx context.Context, tx *sql.Tx, seriesID int64, fromVersion, toVersion int) (int64, error) {
	res, err := tx.ExecContext(ctx, `
		INSERT INTO message_content_items (tenant_id, channel_id, series_id, content_version, seq_no, message_text, content_kind, link_url, cta_label, is_active)
		SELECT tenant_id, channel_id, series_id, $3, seq_no, message_text, content_kind, link_url, cta_label, is_active
		FROM message_content_items
		WHERE series_id = $1 AND content_version = $2
	`, seriesID, fromVersion, toVersion)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

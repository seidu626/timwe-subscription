package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/notification/internal/domain"
)

type tenantEventPayload struct {
	EventID        string   `json:"eventId"`
	Type           string   `json:"type"`
	MSISDN         string   `json:"msisdn"`
	Operator       string   `json:"operator"`
	SubscriptionID string   `json:"subscriptionId,omitempty"`
	Keyword        string   `json:"keyword,omitempty"`
	Channel        string   `json:"channel"`
	IntentRef      string   `json:"intentRef,omitempty"`
	OccurredAt     string   `json:"occurredAt"`
	Amount         *float64 `json:"amount,omitempty"`
	Currency       string   `json:"currency,omitempty"`
	TransactionRef string   `json:"transactionRef,omitempty"`
}

func (r *NotificationRepository) enqueueTenantEvent(ctx context.Context, tx *sql.Tx, n *domain.NotificationRequest, notificationID int64, receivedAt time.Time) error {
	if n.TenantID == nil {
		return nil
	}
	kind := strings.ToUpper(strings.TrimSpace(n.Type))
	if kind != "USER_OPTIN" && kind != "CHARGE" && kind != "USER_RENEWED" && kind != "USER_OPTOUT" {
		return nil
	}
	// Same binding the dispatcher delivers with; nothing is queued for a binding it would refuse.
	var bindingChannelID, bindingRef string
	err := tx.QueryRowContext(ctx, activeTenantEventBindingSQL, *n.TenantID).Scan(&bindingChannelID, &bindingRef)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lookup tenant event credential: %w", err)
	}
	// TIMWE posts bare callbacks, so the tenant is usually inferred from the product (SkipOptinSMS).
	// Those callbacks are authenticated at the edge: nginx admits the callback routes only from TIMWE's
	// egress address. What the event path adds is the product check the SMS path gets from its
	// (tenant, product) template key: a tenant only hears about a product it alone owns, whichever way
	// the tenant was resolved.
	var ownsProduct bool
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(bool_and(p.tenant_id = $1::uuid), false) AND count(DISTINCT p.tenant_id) = 1
  FROM products p JOIN tenants t ON t.id = p.tenant_id AND t.status = 'ACTIVE'
  WHERE p.product_id = $2`, *n.TenantID, n.ProductID).Scan(&ownsProduct)
	if err != nil {
		return fmt.Errorf("lookup tenant event product owner: %w", err)
	}
	// An explicit callback channel must be an ACTIVE channel of this tenant; otherwise the binding's channel.
	channelArg := nullStringPtrValue(n.ChannelID)
	if !channelArg.Valid {
		channelArg = sql.NullString{String: bindingChannelID, Valid: true}
	}
	var channelKey, channelOperator string
	var channelID sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id::text, channel_key, COALESCE(operator,'') FROM tenant_channels
  WHERE tenant_id = $1::uuid AND status = 'ACTIVE' AND id = $2::uuid`, *n.TenantID, channelArg).Scan(&channelID, &channelKey, &channelOperator)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("lookup tenant event channel: %w", err)
	}

	ref := strings.TrimSpace(n.TransactionUUID)
	if ref == "" {
		ref = strings.TrimSpace(n.ExternalTxID)
	}
	identity := ref
	if identity == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d:%s", kind, n.MSISDN, n.ProductID, n.Message)))
		identity = hex.EncodeToString(sum[:])
	}
	eventID := "timwe:" + kind + ":" + identity
	if len(eventID) > 128 {
		sum := sha256.Sum256([]byte(eventID))
		eventID = "timwe:" + kind + ":" + hex.EncodeToString(sum[:])
	}
	status, reason := "pending", ""
	if !ownsProduct {
		status, reason = "skipped", "PRODUCT_NOT_OWNED"
		log.Printf("tenant event skipped: notification_id=%d product_id=%d reason=%s", notificationID, n.ProductID, reason)
	} else if (kind == "CHARGE" || kind == "USER_RENEWED") && strings.TrimSpace(n.TransactionUUID) == "" {
		status, reason = "skipped", "NO_TRANSACTION_UUID"
		log.Printf("tenant charge event skipped: notification_id=%d reason=%s", notificationID, reason)
	}
	operator := notificationOperator(n.MCC, n.MNC, channelKey, channelOperator)
	if operator == "" && status == "pending" {
		status, reason = "skipped", "UNKNOWN_OPERATOR"
	}
	msisdn := normalizeEventMSISDN(n.MSISDN)
	if msisdn == "" && status == "pending" {
		status, reason = "skipped", "INVALID_MSISDN"
	}
	channel := "SMS"
	entry := strings.ToUpper(strings.TrimSpace(n.EntryChannel))
	for _, marker := range []string{"WEB", "WAP", "HE", "HEADER", "LANDING"} {
		if strings.Contains(entry, marker) {
			channel = "APP"
			break
		}
	}
	occurred := receivedAt
	for _, raw := range []string{n.OccurredAt, n.Timestamp} {
		if parsed, parseErr := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw)); parseErr == nil {
			occurred = parsed
			break
		}
	}
	p := tenantEventPayload{EventID: eventID, MSISDN: msisdn, Operator: operator, Channel: channel, OccurredAt: occurred.UTC().Format(time.RFC3339Nano)}
	switch kind {
	case "USER_OPTIN":
		p.Type, p.Channel = "SUBSCRIBE", "SMS"
		var clickID string
		queryErr := tx.QueryRowContext(ctx, `SELECT click_id FROM acquisition_transactions
   CROSS JOIN LATERAL (SELECT regexp_replace(msisdn, '[^0-9]', '', 'g') AS digits) normalized
   WHERE tenant_id = $1::uuid
    AND (CASE WHEN length(digits) = 10 AND left(digits,1) = '0' THEN '233' || substr(digits,2)
              WHEN length(digits) = 9 THEN '233' || digits ELSE digits END) = $2
    AND created_at BETWEEN $3::timestamptz - interval '60 minutes' AND $3::timestamptz
    AND click_id IS NOT NULL AND btrim(click_id) <> ''
   ORDER BY created_at DESC, id DESC LIMIT 1`, *n.TenantID, msisdn, receivedAt).Scan(&clickID)
		if queryErr != nil && !errors.Is(queryErr, sql.ErrNoRows) {
			return fmt.Errorf("lookup acquisition click id: %w", queryErr)
		}
		if queryErr == nil {
			p.IntentRef, p.Channel = clickID, "APP"
		}
	case "CHARGE", "USER_RENEWED":
		p.Type, p.TransactionRef = "CHARGE_SUCCESS", strings.TrimSpace(n.TransactionUUID)
		if parsed, ok := notificationAmount(n.Amount); ok {
			p.Amount = &parsed
		}
		p.Currency = strings.ToUpper(strings.TrimSpace(n.Currency))
	case "USER_OPTOUT":
		p.Type = "UNSUBSCRIBE"
	}
	var subscriptionID sql.NullInt64
	queryErr := tx.QueryRowContext(ctx, `SELECT id FROM subscriptions
		WHERE tenant_id = $1::uuid AND user_identifier = $2 AND product_id = $3
		ORDER BY created_at DESC, id DESC LIMIT 1`, *n.TenantID, n.MSISDN, n.ProductID).Scan(&subscriptionID)
	if queryErr != nil && !errors.Is(queryErr, sql.ErrNoRows) {
		return fmt.Errorf("lookup subscription id: %w", queryErr)
	}
	if subscriptionID.Valid {
		p.SubscriptionID = strconv.FormatInt(subscriptionID.Int64, 10)
	}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO tenant_event_outbox
		(event_id, tenant_id, channel_id, source_notification_id, source_type, transaction_ref, payload, status, last_error)
		VALUES ($1,$2::uuid,$3::uuid,$4,$5,$6,$7::jsonb,$8,$9)
		ON CONFLICT DO NOTHING`, eventID, *n.TenantID, channelID, notificationID,
		kind, sql.NullString{String: ref, Valid: ref != "" && (kind == "CHARGE" || kind == "USER_RENEWED")}, body, status, reason)
	if err != nil {
		return fmt.Errorf("enqueue tenant event: %w", err)
	}
	return nil
}

func notificationOperator(mcc, mnc, channelKey, channelOperator string) string {
	if strings.TrimSpace(mcc) == "620" {
		switch strings.TrimLeft(strings.TrimSpace(mnc), "0") {
		case "1":
			return "MTN"
		case "2":
			return "TELECEL"
		case "3", "6":
			return "AT"
		}
	}
	if strings.TrimSpace(mcc) != "" && strings.TrimSpace(mnc) != "" {
		return ""
	}
	switch {
	case strings.HasSuffix(channelKey, "-mtn"), strings.EqualFold(channelOperator, "MTN"):
		return "MTN"
	case strings.HasSuffix(channelKey, "-telecel"), strings.EqualFold(channelOperator, "TELECEL"):
		return "TELECEL"
	case channelKey == "timwe-gh-airteltigo-ghana", strings.Contains(strings.ToLower(channelOperator), "airteltigo"), strings.HasSuffix(channelKey, "-at"), strings.EqualFold(channelOperator, "AT"):
		return "AT"
	}
	return ""
}

func normalizeEventMSISDN(raw string) string {
	var b strings.Builder
	for _, ch := range raw {
		if ch >= '0' && ch <= '9' {
			b.WriteRune(ch)
		}
	}
	v := b.String()
	if len(v) == 10 && v[0] == '0' {
		v = "233" + v[1:]
	}
	if len(v) == 9 {
		v = "233" + v
	}
	if len(v) < 11 || len(v) > 15 {
		return ""
	}
	return v
}

func notificationAmount(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var number float64
	if err := json.Unmarshal(raw, &number); err != nil {
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return 0, false
		}
		number, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
	}
	return number, number > 0
}

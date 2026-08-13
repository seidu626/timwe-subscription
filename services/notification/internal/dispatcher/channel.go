package dispatcher

const (
	channelPush = "PUSH"
	channelSMS  = "SMS"
)

// channelDecisionInput captures everything decideChannel needs to route a
// single outbox job. It is deliberately a plain struct (not the domain job
// type) so the decision stays a pure function, independent of any lookup.
type channelDecisionInput struct {
	// preferredChannel is the subscriber's stored preference for this
	// product ("PUSH", "SMS", "BOTH", or "" when unset).
	preferredChannel string
	// hasDevice is true when the subscriber has a registered FCM device
	// token.
	hasDevice bool
	// pushConfigured is true when an FCM sender is wired in (the
	// FCM_CREDENTIALS_JSON_PATH env var was set at startup).
	pushConfigured bool
}

// decideChannel implements the contract's routing rule: "jobs for msisdns
// with channel pref PUSH and a registered device go to the FCM sender, else
// SMS (existing path)". A stored preference of "BOTH" is intentionally not
// treated as push-eligible here: the contract names PUSH explicitly, and
// broadening that reading is a product decision left for a future change,
// not one made unilaterally by this delivery-routing code.
func decideChannel(in channelDecisionInput) string {
	if in.pushConfigured && in.hasDevice && in.preferredChannel == channelPush {
		return channelPush
	}
	return channelSMS
}

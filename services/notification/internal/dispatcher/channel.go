package dispatcher

const (
	channelPush     = "PUSH"
	channelSMS      = "SMS"
	channelUserPref = "USER_PREF"
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
	// deliveryChannel is the message series policy: USER_PREF, SMS, or PUSH.
	deliveryChannel string
}

// decideChannel keeps USER_PREF compatible with existing subscriber-pref
// routing, while SMS and PUSH are admin-controlled series overrides.
func decideChannel(in channelDecisionInput) string {
	switch in.deliveryChannel {
	case channelSMS:
		return channelSMS
	case channelPush:
		if in.pushConfigured && in.hasDevice {
			return channelPush
		}
		return channelSMS
	}
	if in.pushConfigured && in.hasDevice && in.preferredChannel == channelPush {
		return channelPush
	}
	return channelSMS
}

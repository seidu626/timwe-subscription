package domain

import "strings"

// IsFailedChargeCode reports whether an mnoDeliveryCode records a charge the
// carrier attempted and did not collect (NO_BALANCE and the like). An empty code
// proves neither outcome, so it is not classified as failed.
func IsFailedChargeCode(code string) bool {
	code = strings.TrimSpace(code)
	return code != "" && !strings.EqualFold(code, DeliveredMnoCode)
}

// IsFailedChargeAttempt reports whether n is a CHARGE or USER_RENEWED callback
// the carrier did not collect. Such a callback changes no subscription and is
// not a CHARGE_SUCCESS for the tenant; storing the row is all it needs.
func (n *NotificationRequest) IsFailedChargeAttempt() bool {
	if n == nil {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(n.Type)) {
	case ChargeEvent, UserRenewedEvent:
		return IsFailedChargeCode(n.MnoDeliveryCode)
	}
	return false
}

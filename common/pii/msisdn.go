// Package pii provides helpers for redacting personally-identifiable information
// before it reaches log sinks.
package pii

// MaskMSISDN returns a masked representation of an MSISDN safe for logging.
// It keeps the first 5 characters (country prefix + start of subscriber number)
// and the last 2 characters, replacing the middle with '*'.
//
// Examples:
//
//	"233241234567" → "23324****67"
//	"12345"        → "***"   (too short)
//	""             → "***"
// MaskMSISDNs returns a slice of masked MSISDNs for safe logging of lists.
// Each element is passed through MaskMSISDN.
func MaskMSISDNs(msisdns []string) []string {
	out := make([]string, len(msisdns))
	for i, m := range msisdns {
		out[i] = MaskMSISDN(m)
	}
	return out
}

func MaskMSISDN(msisdn string) string {
	const keepPrefix = 5
	const keepSuffix = 2
	n := len(msisdn)
	if n <= keepPrefix+keepSuffix {
		return "***"
	}
	buf := make([]byte, 0, n)
	buf = append(buf, msisdn[:keepPrefix]...)
	for i := keepPrefix; i < n-keepSuffix; i++ {
		buf = append(buf, '*')
	}
	buf = append(buf, msisdn[n-keepSuffix:]...)
	return string(buf)
}

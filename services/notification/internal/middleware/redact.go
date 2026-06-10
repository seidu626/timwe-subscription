package middleware

// RedactSensitiveHeader returns a redacted version of a sensitive header value
// (Authorization, Proxy-Authorization, X-He-Msisdn, X-MSISDN, etc.).
// All but the last 4 characters are replaced with "***..."; values shorter
// than 4 characters are fully redacted to "****".
// An empty string is returned as-is.
//
// Use this whenever sensitive header values must appear in log output.
func RedactSensitiveHeader(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return "****"
	}
	return "***" + value[len(value)-4:]
}

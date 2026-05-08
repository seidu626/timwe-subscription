package service

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	nonDigitRegex = regexp.MustCompile(`\D`)
	ghanaPrefixes = []string{
		"23324", "23354", "23355", "23353",
		"23320", "23350",
		"23326", "23327", "23356", "23357",
	}
)

// normalizeMSISDNForCountry normalizes an MSISDN into a canonical numeric form.
// For Ghana (GH), the canonical format is 233XXXXXXXXX.
func normalizeMSISDNForCountry(rawMSISDN, country string) (string, error) {
	msisdn := strings.TrimSpace(rawMSISDN)
	if msisdn == "" {
		return "", fmt.Errorf("msisdn is required")
	}

	msisdn = strings.ReplaceAll(msisdn, " ", "")
	msisdn = strings.ReplaceAll(msisdn, "\t", "")
	msisdn = strings.ReplaceAll(msisdn, "-", "")
	msisdn = strings.ReplaceAll(msisdn, "(", "")
	msisdn = strings.ReplaceAll(msisdn, ")", "")
	msisdn = strings.TrimPrefix(msisdn, "+")

	if nonDigitRegex.MatchString(msisdn) {
		return "", fmt.Errorf("msisdn must contain only digits")
	}

	if strings.EqualFold(country, "GH") {
		return normalizeGhanaMSISDN(msisdn)
	}

	// Non-GH fallback: keep current behavior broad (9-15 digits),
	// while still ensuring a clean numeric value.
	if len(msisdn) < 9 || len(msisdn) > 15 {
		return "", fmt.Errorf("invalid msisdn format")
	}

	return msisdn, nil
}

func normalizeGhanaMSISDN(msisdn string) (string, error) {
	var normalized string

	switch {
	case len(msisdn) == 12 && strings.HasPrefix(msisdn, "233"):
		normalized = msisdn
	case len(msisdn) == 10 && strings.HasPrefix(msisdn, "0"):
		normalized = "233" + msisdn[1:]
	case len(msisdn) == 9:
		// LP UI displays a fixed "233" prefix, so users may enter only local 9 digits.
		normalized = "233" + msisdn
	default:
		return "", fmt.Errorf("invalid ghana msisdn format")
	}

	for _, prefix := range ghanaPrefixes {
		if strings.HasPrefix(normalized, prefix) {
			return normalized, nil
		}
	}

	return "", fmt.Errorf("invalid ghana mobile prefix")
}

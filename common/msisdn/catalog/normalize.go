package catalog

import (
	"fmt"
	"strings"
	"unicode"
)

var ghMTNPrefixes = []string{"23324", "23325", "23353", "23354", "23355", "23359"}
var ngMTNPrefixes = []string{"234703", "234706", "234803", "234806", "234810", "234813", "234814", "234816", "234903", "234906", "234913", "234916"}

// Normalize converts supported local and international formats to digits-only
// E.164 form. It intentionally supports only the two catalog regions.
func Normalize(raw, requestedRegion string) (string, string, error) {
	digits := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		switch r {
		case '+', ' ', '-', '(', ')', '.':
			return -1
		default:
			return r
		}
	}, strings.TrimSpace(raw))
	if digits == "" || strings.IndexFunc(digits, func(r rune) bool { return !unicode.IsDigit(r) }) >= 0 {
		return "", "", fmt.Errorf("msisdn must contain digits and common separators only")
	}

	region := strings.ToUpper(strings.TrimSpace(requestedRegion))
	if region == "" {
		switch {
		case strings.HasPrefix(digits, "233") || (strings.HasPrefix(digits, "0") && len(digits) == 10):
			region = RegionGhana
		case strings.HasPrefix(digits, "234") || (strings.HasPrefix(digits, "0") && len(digits) == 11):
			region = RegionNigeria
		default:
			return "", "", fmt.Errorf("cannot infer region; choose GH or NG")
		}
	}

	switch region {
	case RegionGhana:
		if strings.HasPrefix(digits, "0") && len(digits) == 10 {
			digits = "233" + digits[1:]
		}
		if len(digits) != 12 || !strings.HasPrefix(digits, "233") {
			return "", "", fmt.Errorf("Ghana MSISDN must be 10 local digits or 12 digits beginning 233")
		}
	case RegionNigeria:
		if strings.HasPrefix(digits, "0") && len(digits) == 11 {
			digits = "234" + digits[1:]
		}
		if len(digits) != 13 || !strings.HasPrefix(digits, "234") {
			return "", "", fmt.Errorf("Nigeria MSISDN must be 11 local digits or 13 digits beginning 234")
		}
	default:
		return "", "", fmt.Errorf("unsupported region %q; use GH or NG", region)
	}

	return digits, region, nil
}

func NormalizeTelco(raw, region, msisdn string) string {
	if telco := strings.ToUpper(strings.TrimSpace(raw)); telco != "" {
		switch telco {
		case "MTN GHANA", "MTN NIGERIA":
			return "MTN"
		case "VODAFONE", "TELECEL GHANA":
			return "TELECEL"
		case "AIRTELTIGO", "AIRTEL TIGO":
			return "AIRTELTIGO"
		default:
			return telco
		}
	}
	prefixes := ghMTNPrefixes
	if region == RegionNigeria {
		prefixes = ngMTNPrefixes
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(msisdn, prefix) {
			return "MTN"
		}
	}
	return "UNKNOWN"
}

func SupportsMADAPI(region, telco string) bool {
	return strings.EqualFold(region, RegionGhana) && strings.EqualFold(telco, "MTN")
}

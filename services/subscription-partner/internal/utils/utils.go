package utils

import (
	"fmt"
	"github.com/seidu626/subscription-manager/common/config"
	"math/rand"
	"time"
)

// GenerateRandomMSISDN Generates a random MSISDN for a given telco
func GenerateRandomMSISDN(telco string, config config.Config) (string, error) {
	prefixes, exists := config.Application.TelcoPrefixes[telco]
	if !exists {
		return "", fmt.Errorf("invalid telco: %s", telco)
	}
	rand.Seed(time.Now().UnixNano())
	prefix := prefixes[rand.Intn(len(prefixes))]

	// Generate a 6-digit random number
	suffix := rand.Intn(900000) + 100000 // Ensures a 6-digit number
	return fmt.Sprintf("%s%d", prefix, suffix), nil
}

// GenerateBatchMSISDNS Generates multiple MSISDNS based on telco and count
func GenerateBatchMSISDNS(telco string, count int, config config.Config) ([]string, error) {
	var msisdns []string
	for i := 0; i < count; i++ {
		msisdn, err := GenerateRandomMSISDN(telco, config)
		if err != nil {
			return nil, err
		}
		msisdns = append(msisdns, msisdn)
	}
	return msisdns, nil
}

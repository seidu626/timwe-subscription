package utils

import (
	"fmt"
	"github.com/valyala/fasthttp"
	"github.com/xuri/excelize/v2"
	"strings"
)

// GetClientIP Helper function to get the client IP
func GetClientIP(ctx *fasthttp.RequestCtx) string {
	// Directly get the IP from the remote address
	ip := ctx.RemoteIP().String()

	// Check for 'X-Forwarded-For' header for clients behind proxies
	forwardedIP := ctx.Request.Header.Peek("X-Forwarded-For")
	if len(forwardedIP) > 0 {
		ip = string(forwardedIP)
	} else {
		// Check 'X-Real-IP' header as a fallback
		realIP := ctx.Request.Header.Peek("X-Real-IP")
		if len(realIP) > 0 {
			ip = string(realIP)
		}
	}

	return ip
}

// ExtractUniquePrefixes extracts and returns a unique list of the first 3 digits of MSISDNs in an Excel file.
func ExtractUniquePrefixes(filePath string) ([]string, error) {
	// Open the Excel file
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %v", err)
	}
	defer f.Close()

	// Assuming the UserIdentifier data is in the first sheet
	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read rows from sheet: %v", err)
	}

	// Set to store unique prefixes
	prefixSet := make(map[string]bool)

	// Loop through the rows, starting from the second row if the first row is a header
	for _, row := range rows[1:] {
		if len(row) == 0 {
			continue // Skip empty rows
		}

		msisdn := strings.TrimSpace(row[0]) // Assume UserIdentifier is in the first column
		if len(msisdn) >= 3 {
			prefix := msisdn[:3]
			prefixSet[prefix] = true
		}
	}

	// Collect unique prefixes into a slice
	uniquePrefixes := make([]string, 0, len(prefixSet))
	for prefix := range prefixSet {
		uniquePrefixes = append(uniquePrefixes, prefix)
	}

	return uniquePrefixes, nil
}

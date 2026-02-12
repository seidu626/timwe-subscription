package utils

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

// MSISDNDataLoader handles loading MSISDN data from various sources
type MSISDNDataLoader struct {
	samples []string
}

// NewMSISDNDataLoader creates a new data loader instance
func NewMSISDNDataLoader() *MSISDNDataLoader {
	return &MSISDNDataLoader{
		samples: make([]string, 0),
	}
}

// LoadFromCSV loads MSISDN numbers from a CSV file
// Expects the CSV to have MSISDNs in the first column
func (loader *MSISDNDataLoader) LoadFromCSV(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	
	// Read header if exists
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %v", err)
	}

	// Determine if first row is header or data
	isHeader := false
	if len(header) > 0 {
		// Check if first value looks like a header (non-numeric)
		firstVal := strings.TrimSpace(header[0])
		if firstVal != "" {
			// Try to parse as number
			for _, char := range firstVal {
				if char < '0' || char > '9' {
					isHeader = true
					break
				}
			}
		}
	}

	// If first row is not header, add it to samples
	if !isHeader && len(header) > 0 {
		msisdn := strings.TrimSpace(header[0])
		if isValidMSISDNFormat(msisdn) {
			loader.samples = append(loader.samples, msisdn)
		}
	}

	// Read the rest of the file
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading CSV record: %v", err)
		}

		if len(record) > 0 {
			msisdn := strings.TrimSpace(record[0])
			if isValidMSISDNFormat(msisdn) {
				loader.samples = append(loader.samples, msisdn)
			}
		}
	}

	return nil
}

// LoadFromTextFile loads MSISDN numbers from a text file (one per line)
func (loader *MSISDNDataLoader) LoadFromTextFile(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open text file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		msisdn := strings.TrimSpace(scanner.Text())
		if isValidMSISDNFormat(msisdn) {
			loader.samples = append(loader.samples, msisdn)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading text file: %v", err)
	}

	return nil
}

// GetSamples returns the loaded MSISDN samples
func (loader *MSISDNDataLoader) GetSamples() []string {
	return loader.samples
}

// GetSamplesByPrefix returns samples filtered by prefix
func (loader *MSISDNDataLoader) GetSamplesByPrefix(prefix string) []string {
	filtered := make([]string, 0)
	for _, msisdn := range loader.samples {
		if strings.HasPrefix(msisdn, prefix) {
			filtered = append(filtered, msisdn)
		}
	}
	return filtered
}

// GetUniquePrefixes returns all unique prefixes (first 6 digits) from the samples
func (loader *MSISDNDataLoader) GetUniquePrefixes() []string {
	prefixMap := make(map[string]bool)
	for _, msisdn := range loader.samples {
		if len(msisdn) >= 6 {
			prefix := msisdn[:6]
			prefixMap[prefix] = true
		}
	}

	prefixes := make([]string, 0, len(prefixMap))
	for prefix := range prefixMap {
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

// GetPrefixDistribution returns a map of prefixes to their counts
func (loader *MSISDNDataLoader) GetPrefixDistribution() map[string]int {
	distribution := make(map[string]int)
	for _, msisdn := range loader.samples {
		if len(msisdn) >= 6 {
			prefix := msisdn[:6]
			distribution[prefix]++
		}
	}
	return distribution
}

// isValidMSISDNFormat checks if a string looks like a valid MSISDN
func isValidMSISDNFormat(msisdn string) bool {
	// Ghana MSISDNs should be 12 digits starting with 233
	if len(msisdn) != 12 {
		return false
	}

	if !strings.HasPrefix(msisdn, "233") {
		return false
	}

	// Check if all characters are digits
	for _, char := range msisdn {
		if char < '0' || char > '9' {
			return false
		}
	}

	return true
}

// LoadTigoUserbaseData is a convenience function to load Tigo userbase data
// and populate the global MSISDN pool
func LoadTigoUserbaseData(filepath string) error {
	loader := NewMSISDNDataLoader()
	
	// Determine file type by extension
	if strings.HasSuffix(strings.ToLower(filepath), ".csv") {
		if err := loader.LoadFromCSV(filepath); err != nil {
			return err
		}
	} else if strings.HasSuffix(strings.ToLower(filepath), ".txt") {
		if err := loader.LoadFromTextFile(filepath); err != nil {
			return err
		}
	} else {
		// Try CSV first, then text
		if err := loader.LoadFromCSV(filepath); err != nil {
			if err := loader.LoadFromTextFile(filepath); err != nil {
				return fmt.Errorf("failed to load data as CSV or text file: %v", err)
			}
		}
	}

	samples := loader.GetSamples()
	if len(samples) == 0 {
		return fmt.Errorf("no valid MSISDN samples found in file")
	}

	// Load samples into the global pool
	LoadMSISDNSamples(samples)

	return nil
}

// AnalyzeMSISDNPatterns analyzes patterns in loaded MSISDN data
// Returns statistics about the data that can be used to improve generation
func AnalyzeMSISDNPatterns(filepath string) (*MSISDNAnalysis, error) {
	loader := NewMSISDNDataLoader()
	
	if strings.HasSuffix(strings.ToLower(filepath), ".csv") {
		if err := loader.LoadFromCSV(filepath); err != nil {
			return nil, err
		}
	} else {
		if err := loader.LoadFromTextFile(filepath); err != nil {
			return nil, err
		}
	}

	analysis := &MSISDNAnalysis{
		TotalSamples:       len(loader.samples),
		UniquePrefixes:     loader.GetUniquePrefixes(),
		PrefixDistribution: loader.GetPrefixDistribution(),
		SuffixPatterns:     make(map[string]int),
	}

	// Analyze suffix patterns
	for _, msisdn := range loader.samples {
		if len(msisdn) >= 12 {
			suffix := msisdn[6:] // Get last 6 digits
			
			// Check for patterns
			if isSequential(suffix) {
				analysis.SuffixPatterns["sequential"]++
			}
			if isRepeating(suffix) {
				analysis.SuffixPatterns["repeating"]++
			}
			if isBlockPattern(suffix) {
				analysis.SuffixPatterns["block"]++
			}
		}
	}

	return analysis, nil
}

// MSISDNAnalysis holds analysis results
type MSISDNAnalysis struct {
	TotalSamples       int
	UniquePrefixes     []string
	PrefixDistribution map[string]int
	SuffixPatterns     map[string]int
}

// Helper functions for pattern detection
func isSequential(suffix string) bool {
	if len(suffix) < 3 {
		return false
	}
	
	for i := 0; i < len(suffix)-2; i++ {
		if suffix[i+1] == suffix[i]+1 && suffix[i+2] == suffix[i]+2 {
			return true
		}
	}
	return false
}

func isRepeating(suffix string) bool {
	if len(suffix) < 3 {
		return false
	}
	
	for i := 0; i < len(suffix)-2; i++ {
		if suffix[i] == suffix[i+1] && suffix[i] == suffix[i+2] {
			return true
		}
	}
	return false
}

func isBlockPattern(suffix string) bool {
	if len(suffix) != 6 {
		return false
	}
	
	// Check if first 3 digits are similar to last 3
	first := suffix[:3]
	second := suffix[3:]
	
	similarity := 0
	for i := 0; i < 3; i++ {
		if first[i] == second[i] {
			similarity++
		}
	}
	
	return similarity >= 2
}

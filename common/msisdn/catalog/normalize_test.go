package catalog

import "testing"

func TestNormalizeSupportedRegions(t *testing.T) {
	tests := []struct {
		name, input, region, want, wantRegion string
	}{
		{"ghana local", "024 123 4567", RegionGhana, "233241234567", RegionGhana},
		{"ghana international", "+233-55-123-4567", "", "233551234567", RegionGhana},
		{"nigeria local", "0803 123 4567", RegionNigeria, "2348031234567", RegionNigeria},
		{"nigeria international", "+234 806 123 4567", "", "2348061234567", RegionNigeria},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, region, err := Normalize(test.input, test.region)
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}
			if got != test.want || region != test.wantRegion {
				t.Fatalf("Normalize() = %q/%q, want %q/%q", got, region, test.want, test.wantRegion)
			}
		})
	}
}

func TestNormalizeRejectsUnsupportedOrMalformed(t *testing.T) {
	for _, input := range []string{"", "23324abc4567", "12345", "+1 202 555 0100"} {
		if _, _, err := Normalize(input, ""); err == nil {
			t.Fatalf("Normalize(%q) expected error", input)
		}
	}
}

func TestNormalizeTelco(t *testing.T) {
	if got := NormalizeTelco("", RegionGhana, "233551234567"); got != "MTN" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeTelco("MTN Ghana", RegionGhana, "233551234567"); got != "MTN" {
		t.Fatalf("got %q", got)
	}
	if SupportsMADAPI(RegionNigeria, "MTN") {
		t.Fatal("MADAPI must remain scoped to MTN Ghana")
	}
}

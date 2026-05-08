package service

import "testing"

func TestNormalizeMSISDNForCountry_GH(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		shouldErr bool
	}{
		{
			name:  "local with leading zero",
			input: "0561914461",
			want:  "233561914461",
		},
		{
			name:  "already canonical",
			input: "233561914461",
			want:  "233561914461",
		},
		{
			name:  "canonical with plus and spaces",
			input: "+233 56 191 4461",
			want:  "233561914461",
		},
		{
			name:  "local 9 digits",
			input: "561914461",
			want:  "233561914461",
		},
		{
			name:      "invalid short",
			input:     "123456",
			shouldErr: true,
		},
		{
			name:      "invalid prefix",
			input:     "0591914461",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeMSISDNForCountry(tt.input, "GH")
			if tt.shouldErr {
				if err == nil {
					t.Fatalf("expected error, got value %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}

func TestNormalizeMSISDNForCountry_NonGH(t *testing.T) {
	got, err := normalizeMSISDNForCountry("+254 712-345-678", "KE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "254712345678" {
		t.Fatalf("expected normalized non-GH number, got %s", got)
	}
}

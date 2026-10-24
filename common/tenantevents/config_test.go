package tenantevents

import "testing"

func TestParse_HonorOptoutActionDefaultsOff(t *testing.T) {
	t.Setenv(AllowPrivateTargetsEnv, "")
	base := `"url":"https://talentrivia.com/api/airtime/events","signing_secret":"0123456789abcdef","default_amount":0.8,"default_currency":"GHS"`
	cases := map[string]struct {
		blob string
		want bool
	}{
		"absent":         {`{` + base + `}`, false},
		"explicit false": {`{` + base + `,"honor_optout_action":false}`, false},
		"explicit true":  {`{` + base + `,"honor_optout_action":true}`, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, err := Parse([]byte(tc.blob))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if cfg.HonorOptoutAction != tc.want {
				t.Fatalf("HonorOptoutAction = %v; want %v", cfg.HonorOptoutAction, tc.want)
			}
		})
	}
	if _, err := Parse([]byte(`{` + base + `,"honor_optout_action":"yes"}`)); err == nil {
		t.Fatal("Parse accepted a non-boolean honor_optout_action")
	}
}

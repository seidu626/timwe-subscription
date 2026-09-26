package worker

import (
	"strings"
	"testing"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
)

const reportFixture = `textbox13,textbox12
01 September 2026,30 September 2026

Textbox1,Textbox2,Textbox4,Textbox5,Textbox6,Textbox7,Textbox8,Textbox9,Textbox10,Textbox11,REPORT_DATE,PARTNER_ID,PARTNER_NAME,SHORT_CODE,PRODUCT_ID,PRODUCT_NAME,PRICEPOINT_ID,PRICEPOINT_VALUE,QTY_CHARGES,TOTAL_INCOME,TOTAL_DESIRED
Day,Provider ID,Provider Name,Shortcode,Product ID,Product Name,Pricepoint ID,Pricepoint Value,Success Billings,Revenue,03/09/2026,1919,Nouveau Riche Global LTD,6555,32535,Careerify,70946,0.40,2,0.80,8000
`

func TestParseTIMWEReportCSV(t *testing.T) {
	source := repository.TIMWEReportSource{Key: "careerify", Shortcode: "6555", ProductID: 32535, PartnerID: 1919}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	rows, err := ParseTIMWEReportCSV(strings.NewReader(reportFixture), source, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].SuccessBillings != 2 || rows[0].Revenue != "0.80" || rows[0].PricepointID != 70946 {
		t.Fatalf("unexpected imported rows: %+v", rows)
	}
	for name, mutated := range map[string]string{
		"partner mismatch":  strings.Replace(reportFixture, ",1919,Nouveau", ",2222,Nouveau", 1),
		"product mismatch":  strings.Replace(reportFixture, ",32535,Careerify", ",88888,Careerify", 1),
		"unexpected date":   strings.Replace(reportFixture, "03/09/2026", "03/10/2026", 1),
		"negative quantity": strings.Replace(reportFixture, ",2,0.80,", ",-2,0.80,", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseTIMWEReportCSV(strings.NewReader(mutated), source, start, end); err == nil {
				t.Fatal("malformed report was accepted")
			}
		})
	}
}

func TestValidateTIMWEReportURL(t *testing.T) {
	good := "https://reports.timwetech.com/Reportserver/Pages/ReportViewer.aspx?/Partner%20Reports/MEP/Ghana/Charges%20Consolidated&rs:Command=Render&Partner=opaque"
	if err := validateTIMWEReportURL(good); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.Replace(good, "https://", "http://", 1),
		strings.Replace(good, "reports.timwetech.com", "evil.example", 1),
		strings.Replace(good, "reports.timwetech.com", "user:password@reports.timwetech.com", 1),
	} {
		if err := validateTIMWEReportURL(bad); err == nil {
			t.Fatalf("unsafe URL accepted: %q", bad)
		}
	}
}

package worker

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/seidu626/subscription-manager/acquisition-api/internal/repository"
	"go.uber.org/zap"
)

const maxTIMWECSVBytes = 5 << 20

var reportKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var reportShortcodePattern = regexp.MustCompile(`^[0-9]{1,20}$`)
var reportMoneyPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]{1,4})?$`)

type timweReportStore interface {
	ResolveTIMWETenant(context.Context, string) (string, error)
	ReplaceTIMWEReport(context.Context, repository.TIMWEReportSource, time.Time, time.Time, []repository.TIMWEReportRow, time.Time) error
	RecordTIMWEReportFailure(context.Context, repository.TIMWEReportSource, string, time.Time) error
}

// TIMWEReportSync is an opt-in collector for the authenticated Charges
// Consolidated CSV. It never accepts a report URL from a browser request.
type TIMWEReportSync struct {
	store    timweReportStore
	logger   *zap.Logger
	client   *http.Client
	baseURL  string
	username string
	password string
	sources  []repository.TIMWEReportSource
	interval time.Duration
	location *time.Location
}

func (s *TIMWEReportSync) PollInterval() time.Duration { return s.interval }

func NewTIMWEReportSyncFromEnv(store timweReportStore, logger *zap.Logger) (*TIMWEReportSync, error) {
	rawSources := strings.TrimSpace(os.Getenv("TIMWE_REPORT_SOURCES"))
	if rawSources == "" {
		return nil, nil
	}
	var sources []repository.TIMWEReportSource
	if err := json.Unmarshal([]byte(rawSources), &sources); err != nil || len(sources) == 0 {
		return nil, errors.New("TIMWE_REPORT_SOURCES must be a nonempty JSON array")
	}
	baseURL := strings.TrimSpace(os.Getenv("TIMWE_REPORT_URL"))
	if err := validateTIMWEReportURL(baseURL); err != nil {
		return nil, err
	}
	username, password := os.Getenv("TIMWE_REPORT_USERNAME"), os.Getenv("TIMWE_REPORT_PASSWORD")
	if username == "" || password == "" {
		return nil, errors.New("TIMWE report credentials are required when sources are configured")
	}
	seen := map[string]bool{}
	seenFilters := map[string]bool{}
	for i := range sources {
		source := &sources[i]
		source.TenantKey = strings.TrimSpace(source.TenantKey)
		source.Shortcode = strings.TrimSpace(source.Shortcode)
		filterKey := fmt.Sprintf("%s/%s/%d/%d", source.TenantKey, source.Shortcode, source.ProductID, source.PartnerID)
		if !reportKeyPattern.MatchString(source.Key) || source.TenantKey == "" || !reportShortcodePattern.MatchString(source.Shortcode) || source.ProductID <= 0 || source.PartnerID <= 0 || seen[source.Key] || seenFilters[filterKey] {
			return nil, errors.New("invalid or duplicate TIMWE report source")
		}
		seen[source.Key] = true
		seenFilters[filterKey] = true
	}
	interval := 15 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("TIMWE_REPORT_POLL_INTERVAL")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed < 5*time.Minute || parsed > 24*time.Hour {
			return nil, errors.New("TIMWE_REPORT_POLL_INTERVAL must be between 5m and 24h")
		}
		interval = parsed
	}
	location, err := time.LoadLocation("Africa/Accra")
	if err != nil {
		return nil, fmt.Errorf("load report timezone: %w", err)
	}
	return &TIMWEReportSync{
		store: store, logger: logger, baseURL: baseURL, username: username, password: password,
		sources: sources, interval: interval, location: location,
		client: &http.Client{
			Timeout:       25 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

func validateTIMWEReportURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Hostname(), "reports.timwetech.com") || u.Port() != "" || u.User != nil || u.Fragment != "" || !strings.EqualFold(u.Path, "/Reportserver/Pages/ReportViewer.aspx") || !strings.Contains(strings.ToLower(u.RawQuery), "charges%20consolidated") || u.Query().Get("Partner") == "" || u.Query().Get("rs:Command") != "Render" {
		return errors.New("TIMWE_REPORT_URL must be the credential-free Charges Consolidated HTTPS URL")
	}
	for _, key := range []string{"StartDate", "EndDate", "Shortcode", "Product", "rs:Format"} {
		if u.Query().Has(key) {
			return errors.New("TIMWE_REPORT_URL must not contain report filters or an export format")
		}
	}
	return nil
}

func (s *TIMWEReportSync) Run(ctx context.Context) {
	s.syncOnce(ctx, time.Now())
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.syncOnce(ctx, now)
		}
	}
}

func (s *TIMWEReportSync) syncOnce(ctx context.Context, now time.Time) {
	localNow := now.In(s.location)
	end := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, s.location)
	start := end.AddDate(0, 0, -30)
	for _, configured := range s.sources {
		if ctx.Err() != nil {
			return
		}
		source := configured
		tenantID, err := s.store.ResolveTIMWETenant(ctx, source.TenantKey)
		if err != nil {
			s.logger.Error("TIMWE report tenant resolution failed", zap.String("source_key", source.Key))
			continue
		}
		source.TenantID = tenantID
		rows, code, err := s.fetchAndParse(ctx, source, start, end)
		if err == nil {
			err = s.store.ReplaceTIMWEReport(ctx, source, start, end, rows, now.UTC())
			if err != nil {
				code = "persistence"
			}
		}
		if err != nil {
			if recordErr := s.store.RecordTIMWEReportFailure(ctx, source, code, now.UTC()); recordErr != nil {
				s.logger.Error("TIMWE report failure state could not be recorded", zap.String("source_key", source.Key))
			}
			s.logger.Warn("TIMWE report import failed", zap.String("source_key", source.Key), zap.String("error_code", code))
			continue
		}
		s.logger.Info("TIMWE report import succeeded", zap.String("source_key", source.Key), zap.Int("rows", len(rows)), zap.Time("imported_at", now.UTC()))
	}
}

func (s *TIMWEReportSync) fetchAndParse(ctx context.Context, source repository.TIMWEReportSource, start, end time.Time) ([]repository.TIMWEReportRow, string, error) {
	query := s.baseURL + "&StartDate=" + url.QueryEscape(start.Format("02/01/2006")) + "&EndDate=" + url.QueryEscape(end.Format("02/01/2006")) + "&Shortcode=" + url.QueryEscape(source.Shortcode) + "&Product=" + strconv.Itoa(source.ProductID) + "&rs:Format=CSV"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, query, nil)
	if err != nil {
		return nil, "request", err
	}
	req.SetBasicAuth(s.username, s.password)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "transport", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "http_" + strconv.Itoa(resp.StatusCode), errors.New("report HTTP status was not 200")
	}
	if !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/csv") {
		return nil, "content_type", errors.New("report response was not CSV")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTIMWECSVBytes+1))
	if err != nil {
		return nil, "read", err
	}
	if len(body) > maxTIMWECSVBytes {
		return nil, "size", errors.New("report CSV exceeded size limit")
	}
	rows, err := ParseTIMWEReportCSV(strings.NewReader(strings.TrimPrefix(string(body), "\ufeff")), source, start, end)
	if err != nil {
		return nil, "parse", err
	}
	return rows, "", nil
}

// ParseTIMWEReportCSV handles Power BI Report Server's layout columns before
// the actual REPORT_DATE..TOTAL_INCOME columns. It rejects unexpected rows.
func ParseTIMWEReportCSV(input io.Reader, source repository.TIMWEReportSource, start, end time.Time) ([]repository.TIMWEReportRow, error) {
	reader := csv.NewReader(input)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid report CSV: %w", err)
	}
	if len(records) < 4 || len(records[1]) < 2 {
		return nil, errors.New("report CSV lacks a date range and header")
	}
	for index, expected := range []time.Time{start, end} {
		parsed, err := time.Parse("02 January 2006", strings.TrimSpace(records[1][index]))
		if err != nil || parsed.Format("2006-01-02") != expected.Format("2006-01-02") {
			return nil, errors.New("report CSV date range does not match request")
		}
	}
	headerIndex := -1
	column := map[string]int{}
	for i, record := range records {
		for j, name := range record {
			column[strings.ToUpper(strings.TrimSpace(name))] = j
		}
		if _, ok := column["REPORT_DATE"]; ok {
			headerIndex = i
			break
		}
		clear(column)
	}
	required := []string{"REPORT_DATE", "PARTNER_ID", "SHORT_CODE", "PRODUCT_ID", "PRODUCT_NAME", "PRICEPOINT_ID", "PRICEPOINT_VALUE", "QTY_CHARGES", "TOTAL_INCOME"}
	if headerIndex < 0 {
		return nil, errors.New("report CSV lacks data header")
	}
	for _, name := range required {
		if _, ok := column[name]; !ok {
			return nil, errors.New("report CSV lacks required column")
		}
	}
	rows := make([]repository.TIMWEReportRow, 0)
	seen := map[string]bool{}
	for _, record := range records[headerIndex+1:] {
		if len(record) == 0 {
			continue
		}
		for _, name := range required {
			if column[name] >= len(record) {
				return nil, errors.New("report CSV has a truncated row")
			}
		}
		value := func(name string) string { return strings.TrimSpace(record[column[name]]) }
		day, err := time.Parse("02/01/2006", value("REPORT_DATE"))
		if err != nil || day.Before(start) || day.After(end) {
			return nil, errors.New("report CSV has an invalid day")
		}
		partnerID, err := strconv.Atoi(value("PARTNER_ID"))
		if err != nil || partnerID != source.PartnerID || value("SHORT_CODE") != source.Shortcode {
			return nil, errors.New("report CSV partner or shortcode mismatch")
		}
		productID, err := strconv.Atoi(value("PRODUCT_ID"))
		if err != nil || productID != source.ProductID {
			return nil, errors.New("report CSV product mismatch")
		}
		pricepointID, err := strconv.Atoi(value("PRICEPOINT_ID"))
		if err != nil || pricepointID <= 0 {
			return nil, errors.New("report CSV pricepoint is invalid")
		}
		billings, err := strconv.ParseInt(value("QTY_CHARGES"), 10, 64)
		if err != nil || billings < 0 || !reportMoneyPattern.MatchString(value("PRICEPOINT_VALUE")) || !reportMoneyPattern.MatchString(value("TOTAL_INCOME")) {
			return nil, errors.New("report CSV quantity or money is invalid")
		}
		key := day.Format("2006-01-02") + ":" + strconv.Itoa(pricepointID)
		if seen[key] {
			return nil, errors.New("report CSV repeats a day and pricepoint")
		}
		seen[key] = true
		rows = append(rows, repository.TIMWEReportRow{
			Day: day, PartnerID: partnerID, Shortcode: source.Shortcode, ProductID: productID,
			ProductName: value("PRODUCT_NAME"), PricepointID: pricepointID,
			PricepointValue: value("PRICEPOINT_VALUE"), SuccessBillings: billings, Revenue: value("TOTAL_INCOME"),
		})
	}
	return rows, nil
}

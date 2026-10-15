package config

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/caarlos0/env/v11"

	"bugun/internal/analytics"
	"bugun/internal/insight"
	"bugun/internal/jobs"
	"bugun/internal/market"
	"bugun/internal/media"
	"bugun/internal/rfq"
)

// required are the settings without a default.
var required = map[string]string{"DATABASE_URL": "postgres://localhost/test", "DATA_ENCRYPTION_KEY": "key"}

func load(t *testing.T, vars map[string]string) (Config, error) {
	t.Helper()
	m := map[string]string{}
	for k, v := range required {
		m[k] = v
	}
	for k, v := range vars {
		m[k] = v
	}
	return parse(env.Options{Environment: m})
}

func TestDefaults(t *testing.T) {
	c, err := load(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !c.MigrateOnStart || c.RequestTimeout != 15*time.Second || c.RateLimitPerMinute != 120 ||
		c.HTTPWriteTimeout != time.Minute || c.HTTPIdleTimeout != 2*time.Minute || c.BotMessagesPerMinute != 20 ||
		c.HeartbeatURL != "" || c.HeartbeatInterval != 5*time.Minute || c.JobHeartbeat != time.Minute || c.JobLeaseTimeout != 5*time.Minute {
		t.Errorf("server defaults changed: %+v", c)
	}
	want := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")}
	if len(c.TrustedProxies) != 2 || c.TrustedProxies[0] != want[0] || c.TrustedProxies[1] != want[1] {
		t.Errorf("TrustedProxies = %v, want %v", c.TrustedProxies, want)
	}

	// The schedule the API always had.
	clocks := func(cs []jobs.Clock) string {
		var s []string
		for _, c := range cs {
			s = append(s, c.String())
		}
		return strings.Join(s, ",")
	}
	for name, got := range map[string]string{
		"07:30,12:00,16:30": clocks(c.ScheduleAutoPricing), "03:30": clocks(c.ScheduleAnomalies),
		"04:00": clocks(c.ScheduleSellerScores), "04:30": clocks(c.ScheduleSessionsCleanup),
		"10:15": clocks(c.ScheduleFX), "06:30": clocks(c.ScheduleWeather), "19:00": clocks(c.ScheduleSellerDigest),
		"08:00,17:00": clocks(c.ScheduleWatchAlerts), "11:00": clocks(c.ScheduleRatingPrompts),
		"Mon 09:00": c.ScheduleWeeklyReminder.String(),
	} {
		if got != name {
			t.Errorf("schedule = %q, want %q", got, name)
		}
	}

	// Defaults match what the packages use without configuration.
	if got := c.AnomalyRules(); got != insight.DefaultAnomalyRules {
		t.Errorf("AnomalyRules = %+v, want %+v", got, insight.DefaultAnomalyRules)
	}
	ap := c.AutoPriceRules()
	if ap.ManualPause != market.DefaultAutoPriceRules.ManualPause || ap.MinMarketViews != market.DefaultAutoPriceRules.MinMarketViews ||
		ap.DailyStep != analytics.DefaultRepriceRules.DailyStep {
		t.Errorf("AutoPriceRules = %+v, want %+v", ap, market.DefaultAutoPriceRules)
	}
	for goal, f := range analytics.DefaultRepriceRules.Goals {
		if ap.Goals[goal] != f {
			t.Errorf("goal %s = %v, want %v", goal, ap.Goals[goal], f)
		}
	}
	if got := c.RFQLimits(); got != rfq.DefaultLimits {
		t.Errorf("RFQLimits = %+v, want %+v", got, rfq.DefaultLimits)
	}
	if got := c.MediaLimits(); got != media.DefaultLimits {
		t.Errorf("MediaLimits = %+v, want %+v", got, media.DefaultLimits)
	}
	if c.MaxPhotosPerListing != market.DefaultMaxPhotos {
		t.Errorf("MaxPhotosPerListing = %d, want %d", c.MaxPhotosPerListing, market.DefaultMaxPhotos)
	}
}

func TestOverrides(t *testing.T) {
	c, err := load(t, map[string]string{
		"SCHEDULE_AUTO_PRICING":    "06:00, 18:45",
		"SCHEDULE_WEEKLY_REMINDER": "fri 17:30",
		"ANOMALY_Z":                "4",
		"ANOMALY_MIN_PROVINCES":    "7",
		"REPRICE_GOAL_FAST":        "0.95",
		"REPRICE_DAILY_STEP":       "0.1",
		"AUTO_PRICE_MANUAL_PAUSE":  "12h",
		"RFQ_MAX_OPEN":             "5",
		"RFQ_MAX_DAYS":             "14",
		"MAX_PHOTOS_PER_LISTING":   "8",
		"RATE_LIMIT_PER_MINUTE":    "300",
		"TRUSTED_PROXIES":          "10.0.0.0/8",
		"MIGRATE_ON_START":         "false",
		"HEARTBEAT_URL":            "https://hc-ping.com/abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.ScheduleAutoPricing) != 2 || c.ScheduleAutoPricing[1] != (jobs.Clock{Hour: 18, Minute: 45}) {
		t.Errorf("ScheduleAutoPricing = %v", c.ScheduleAutoPricing)
	}
	if c.ScheduleWeeklyReminder != (jobs.WeekClock{Day: time.Friday, Clock: jobs.Clock{Hour: 17, Minute: 30}}) {
		t.Errorf("ScheduleWeeklyReminder = %v", c.ScheduleWeeklyReminder)
	}
	if r := c.AnomalyRules(); r.Z != 4 || r.MinProvinces != 7 || r.MinMove != 0.25 {
		t.Errorf("AnomalyRules = %+v", r)
	}
	if r := c.AutoPriceRules(); r.Goals["fast"] != 0.95 || r.Goals["profit"] != 1.03 || r.DailyStep != 0.1 || r.ManualPause != 12*time.Hour {
		t.Errorf("AutoPriceRules = %+v", r)
	}
	if l := c.RFQLimits(); l.MaxOpen != 5 || l.MaxDays != 14 || l.DefaultDays != 3 {
		t.Errorf("RFQLimits = %+v", l)
	}
	if c.MaxPhotosPerListing != 8 || c.RateLimitPerMinute != 300 || c.MigrateOnStart || c.HeartbeatURL == "" {
		t.Errorf("overrides not applied: %+v", c)
	}
	if len(c.TrustedProxies) != 1 || c.TrustedProxies[0] != netip.MustParsePrefix("10.0.0.0/8") {
		t.Errorf("TrustedProxies = %v", c.TrustedProxies)
	}
}

func TestInvalidValuesAreRejected(t *testing.T) {
	for name, vars := range map[string]map[string]string{
		"hour 25":               {"SCHEDULE_FX": "25:00"},
		"minute 60":             {"SCHEDULE_WEATHER": "06:60"},
		"no colon":              {"SCHEDULE_AUTO_PRICING": "07:30,1200"},
		"one-digit minute":      {"SCHEDULE_ANOMALIES": "3:3"},
		"words":                 {"SCHEDULE_SELLER_DIGEST": "evening"},
		"weekday":               {"SCHEDULE_WEEKLY_REMINDER": "Funday 09:00"},
		"weekly without time":   {"SCHEDULE_WEEKLY_REMINDER": "Mon"},
		"bad proxy":             {"TRUSTED_PROXIES": "localhost"},
		"proxy without mask":    {"TRUSTED_PROXIES": "127.0.0.1"},
		"not a number":          {"RFQ_MAX_OPEN": "three"},
		"zero rate limit":       {"RATE_LIMIT_PER_MINUTE": "0"},
		"write before timeout":  {"HTTP_WRITE_TIMEOUT": "10s"},
		"days out of order":     {"RFQ_DEFAULT_DAYS": "9"},
		"min days zero":         {"RFQ_MIN_DAYS": "0"},
		"daily step too big":    {"REPRICE_DAILY_STEP": "1.5"},
		"goal too far":          {"REPRICE_GOAL_PROFIT": "3"},
		"one province":          {"ANOMALY_MIN_PROVINCES": "1"},
		"negative z":            {"ANOMALY_Z": "-1"},
		"lease shorter than 2x": {"JOB_LEASE_TIMEOUT": "90s"},
		"no photos":             {"MAX_PHOTOS_PER_LISTING": "0"},
		"tiny uploads":          {"MEDIA_MAX_UPLOAD_BYTES": "1000"},
		"zero heartbeat":        {"HEARTBEAT_INTERVAL": "0s"},
	} {
		if _, err := load(t, vars); err == nil {
			t.Errorf("%s (%v): accepted, want an error", name, vars)
		}
	}
}

func TestClockParsing(t *testing.T) {
	for in, want := range map[string]jobs.Clock{"00:00": {}, "07:30": {Hour: 7, Minute: 30}, "23:59": {Hour: 23, Minute: 59}, " 9:05 ": {Hour: 9, Minute: 5}} {
		var c jobs.Clock
		if err := c.UnmarshalText([]byte(in)); err != nil || c != want {
			t.Errorf("%q = %v, %v; want %v", in, c, err, want)
		}
	}
	for _, in := range []string{"24:00", "-1:00", "12:5", "12:345", "12", "", "ab:cd"} {
		var c jobs.Clock
		if err := c.UnmarshalText([]byte(in)); err == nil {
			t.Errorf("%q accepted as %v", in, c)
		}
	}
}

func TestSettingsDefaults(t *testing.T) {
	c, err := load(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.ForecastRules(); got != insight.DefaultForecastRules {
		t.Errorf("ForecastRules = %+v, want %+v", got, insight.DefaultForecastRules)
	}
	if got := c.ClusterRules(); got != insight.DefaultClusterRules {
		t.Errorf("ClusterRules = %+v, want %+v", got, insight.DefaultClusterRules)
	}
	// The intervals the API always had.
	if c.SchedulePriceDailyEvery != time.Hour || c.ScheduleDealPromptsEvery != 15*time.Minute || c.ScheduleMonitoringEvery != time.Hour {
		t.Errorf("intervals = %s %s %s", c.SchedulePriceDailyEvery, c.ScheduleDealPromptsEvery, c.ScheduleMonitoringEvery)
	}
	// Every integration is off.
	if c.MediaStorage != "disk" || c.SentryDSN != "" || c.UzExEnabled || c.TelegramWebAppMaxAge != time.Hour {
		t.Errorf("integration defaults changed: %+v", c)
	}
	if c.SentryEnv() != "development" {
		t.Errorf("SentryEnv = %q, want APP_ENV", c.SentryEnv())
	}
}

func TestSettingsOverrides(t *testing.T) {
	c, err := load(t, map[string]string{
		"FORECAST_WEEKS":              "6",
		"FORECAST_TREND_DAMPING":      "0.7",
		"CLUSTER_MAX_GROUPS":          "4",
		"CLUSTER_NOTABLE_GAP":         "0.05",
		"SCHEDULE_PRICE_DAILY_EVERY":  "30m",
		"SCHEDULE_DEAL_PROMPTS_EVERY": "5m",
		"MEDIA_STORAGE":               "s3",
		"S3_ENDPOINT":                 "http://minio:9000",
		"S3_BUCKET":                   "photos",
		"S3_ACCESS_KEY":               "a",
		"S3_SECRET_KEY":               "b",
		"S3_USE_PATH_STYLE":           "true",
		"S3_PUBLIC_BASE_URL":          "https://cdn.example.com/photos/",
		"SENTRY_DSN":                  "https://key@o0.ingest.example.com/1",
		"SENTRY_ENVIRONMENT":          "staging",
		"UZEX_ENABLED":                "true",
		"SCHEDULE_UZEX":               "17:00,19:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r := c.ForecastRules(); r.Weeks != 6 || r.TrendDamping != 0.7 || r.MinBacktests != 20 {
		t.Errorf("ForecastRules = %+v", r)
	}
	if r := c.ClusterRules(); r.MaxClusters != 4 || r.NotableGap != 0.05 || r.Top != 3 {
		t.Errorf("ClusterRules = %+v", r)
	}
	if c.SchedulePriceDailyEvery != 30*time.Minute || c.ScheduleDealPromptsEvery != 5*time.Minute {
		t.Errorf("intervals = %s %s", c.SchedulePriceDailyEvery, c.ScheduleDealPromptsEvery)
	}
	if s := c.S3(); s.Bucket != "photos" || !s.PathStyle || s.PublicBaseURL != "https://cdn.example.com/photos" {
		t.Errorf("S3 = %+v", s)
	}
	if c.SentryEnv() != "staging" || !c.UzExEnabled || len(c.ScheduleUzEx) != 2 {
		t.Errorf("overrides not applied: %+v", c)
	}
}

func TestInvalidSettingsAreRejected(t *testing.T) {
	for name, vars := range map[string]map[string]string{
		"no forecast weeks":           {"FORECAST_WEEKS": "0"},
		"forecast too far":            {"FORECAST_WEEKS": "20"},
		"damping above 1":             {"FORECAST_TREND_DAMPING": "1.5"},
		"no trend cap":                {"FORECAST_MAX_TREND_EFFECT": "0"},
		"too few backtests":           {"FORECAST_MIN_BACKTESTS": "2"},
		"more backtests than a year":  {"FORECAST_MIN_BACKTESTS": "50"},
		"interval of 100%":            {"FORECAST_INTERVAL_QUANTILE": "1"},
		"coverage above 1":            {"CLUSTER_PROFILE_COVERAGE": "1.2"},
		"zero province coverage":      {"CLUSTER_PROVINCE_COVERAGE": "0"},
		"two provinces":               {"CLUSTER_MIN_PROVINCES": "2"},
		"one group":                   {"CLUSTER_MAX_GROUPS": "1"},
		"no top":                      {"CLUSTER_TOP": "0"},
		"negative gap":                {"CLUSTER_NOTABLE_GAP": "-0.1"},
		"interval not dividing a day": {"SCHEDULE_PRICE_DAILY_EVERY": "7h"},
		"interval too short":          {"SCHEDULE_DEAL_PROMPTS_EVERY": "10s"},
		"interval too long":           {"SCHEDULE_MONITORING_EVERY": "48h"},
		"unknown storage":             {"MEDIA_STORAGE": "ftp"},
		"s3 without bucket":           {"MEDIA_STORAGE": "s3", "S3_ENDPOINT": "s3.example.com", "S3_ACCESS_KEY": "a", "S3_SECRET_KEY": "b"},
		"public url without scheme":   {"S3_PUBLIC_BASE_URL": "cdn.example.com"},
		"webapp max age too short":    {"TELEGRAM_WEBAPP_MAX_AGE": "10s"},
		"sample rate above 1":         {"SENTRY_TRACES_SAMPLE_RATE": "2"},
		"uzex bad url":                {"UZEX_ENABLED": "true", "UZEX_URL": "uzex.uz"},
	} {
		if _, err := load(t, vars); err == nil {
			t.Errorf("%s (%v): accepted, want an error", name, vars)
		}
	}
}

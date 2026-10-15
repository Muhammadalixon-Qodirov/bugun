package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"bugun/internal/insight"
	"bugun/internal/media"
)

// validateSettings checks the analytics parameters, the interval jobs and
// the optional integrations.
func (c Config) validateSettings() []error {
	var errs []error
	check := func(ok bool, format string, args ...any) {
		if !ok {
			errs = append(errs, fmt.Errorf(format, args...))
		}
	}
	between := func(name string, x, lo, hi float64) {
		check(x > lo && x <= hi, "%s must be above %v and at most %v, got %v", name, lo, hi, x)
	}

	check(c.ForecastWeeks >= 1 && c.ForecastWeeks <= 12, "FORECAST_WEEKS must be 1–12, got %d", c.ForecastWeeks)
	between("FORECAST_TREND_DAMPING", c.ForecastTrendDamping, 0, 1)
	between("FORECAST_MAX_TREND_EFFECT", c.ForecastMaxTrendEffect, 0, 1)
	// A year holds about 52 weekly origins, fewer the further ahead the forecast looks.
	check(c.ForecastMinBacktests >= 5 && c.ForecastMinBacktests <= 52-c.ForecastWeeks,
		"FORECAST_MIN_BACKTESTS must be 5–%d, got %d", 52-c.ForecastWeeks, c.ForecastMinBacktests)
	check(c.ForecastIntervalQuantile >= 0.5 && c.ForecastIntervalQuantile < 1,
		"FORECAST_INTERVAL_QUANTILE must be at least 0.5 and below 1, got %v", c.ForecastIntervalQuantile)

	between("CLUSTER_PROFILE_COVERAGE", c.ClusterProfileCoverage, 0, 1)
	between("CLUSTER_PROVINCE_COVERAGE", c.ClusterProvinceCoverage, 0, 1)
	check(c.ClusterMinProvinces >= 3, "CLUSTER_MIN_PROVINCES must be at least 3, got %d", c.ClusterMinProvinces)
	check(c.ClusterMinProducts >= 2, "CLUSTER_MIN_PRODUCTS must be at least 2, got %d", c.ClusterMinProducts)
	check(c.ClusterMaxGroups >= 2, "CLUSTER_MAX_GROUPS must be at least 2, got %d", c.ClusterMaxGroups)
	check(c.ClusterTop >= 1, "CLUSTER_TOP must be positive, got %d", c.ClusterTop)
	check(c.ClusterNotableGap >= 0 && c.ClusterNotableGap < 1, "CLUSTER_NOTABLE_GAP must be at least 0 and below 1, got %v", c.ClusterNotableGap)

	for name, d := range map[string]time.Duration{
		"SCHEDULE_PRICE_DAILY_EVERY": c.SchedulePriceDailyEvery, "SCHEDULE_DEAL_PROMPTS_EVERY": c.ScheduleDealPromptsEvery,
		"SCHEDULE_MONITORING_EVERY": c.ScheduleMonitoringEvery,
	} {
		// Slots are the time truncated to the interval, so it must fit a day
		// a whole number of times to land on the same clock times every day.
		check(d >= time.Minute && d <= 24*time.Hour && (24*time.Hour)%d == 0,
			"%s must be between 1m and 24h and divide a day evenly, got %s", name, d)
	}

	switch c.MediaStorage {
	case "disk":
	case "s3":
		check(c.S3Endpoint != "" && c.S3Bucket != "" && c.S3AccessKey != "" && c.S3SecretKey != "",
			"MEDIA_STORAGE=s3 needs S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY and S3_SECRET_KEY")
	default:
		check(false, "MEDIA_STORAGE must be disk or s3, got %q", c.MediaStorage)
	}
	if c.S3PublicBaseURL != "" {
		u, err := url.Parse(c.S3PublicBaseURL)
		check(err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "", "S3_PUBLIC_BASE_URL must be an http(s) URL")
	}

	check(c.TelegramWebAppMaxAge >= time.Minute && c.TelegramWebAppMaxAge <= 7*24*time.Hour,
		"TELEGRAM_WEBAPP_MAX_AGE must be between 1m and 168h, got %s", c.TelegramWebAppMaxAge)

	check(c.SentryTracesSampleRate >= 0 && c.SentryTracesSampleRate <= 1,
		"SENTRY_TRACES_SAMPLE_RATE must be 0–1, got %v", c.SentryTracesSampleRate)

	if c.UzExEnabled {
		u, err := url.Parse(c.UzExURL)
		check(err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "", "UZEX_URL must be an http(s) URL")
		check(len(c.ScheduleUzEx) > 0, "SCHEDULE_UZEX needs at least one time")
	}
	return errs
}

// ForecastRules are the forecast's parameters.
func (c Config) ForecastRules() insight.ForecastRules {
	return insight.ForecastRules{Weeks: c.ForecastWeeks, TrendDamping: c.ForecastTrendDamping, MaxTrendEffect: c.ForecastMaxTrendEffect,
		MinBacktests: c.ForecastMinBacktests, IntervalQuantile: c.ForecastIntervalQuantile}
}

// ClusterRules are the parameters of the similar viloyat groups.
func (c Config) ClusterRules() insight.ClusterRules {
	return insight.ClusterRules{ProfileCoverage: c.ClusterProfileCoverage, ProvinceCoverage: c.ClusterProvinceCoverage,
		MinProfiles: c.ClusterMinProvinces, MinProfileItems: c.ClusterMinProducts, MaxClusters: c.ClusterMaxGroups,
		Top: c.ClusterTop, NotableGap: c.ClusterNotableGap}
}

// S3 is the object storage for photos when MEDIA_STORAGE=s3.
func (c Config) S3() media.S3Config {
	return media.S3Config{
		Endpoint: c.S3Endpoint, Region: c.S3Region, Bucket: c.S3Bucket,
		AccessKey: c.S3AccessKey, SecretKey: c.S3SecretKey, PathStyle: c.S3UsePathStyle,
		PublicBaseURL: strings.TrimRight(c.S3PublicBaseURL, "/"),
	}
}

// SentryEnv is the environment name reported with errors: SENTRY_ENVIRONMENT,
// else APP_ENV.
func (c Config) SentryEnv() string {
	if c.SentryEnvironment != "" {
		return c.SentryEnvironment
	}
	return c.Env
}

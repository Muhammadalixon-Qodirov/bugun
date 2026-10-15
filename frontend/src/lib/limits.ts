import type { Limits } from "@/lib/api/gen/types.gen";

export type { Limits };

/**
 * The API's business limits (GET /v1/meta/limits) as they are by default.
 * Used only while the real values are loading or when the request failed, so a
 * form still works; the API checks every value again when it is sent.
 */
export const FALLBACK_LIMITS: Limits = {
  rfq: { max_open: 3, default_days: 3, min_days: 1, max_days: 7 },
  auto_price: { max_ratio: 3, daily_step: 0.05, manual_pause_hours: 24 },
  photos: { max_per_listing: 5, max_bytes: 10 * 1024 * 1024, max_pixels: 40_000_000 },
  watches_max: 20,
  compare_max_lines: 4,
  admin_deals_limit: 100,
  admin_anomalies_limit: 300,
};

/** The limits change only with an API restart, which also says they may be cached this long. */
export const LIMITS_MAX_AGE_SECONDS = 300;

/** Whole megabytes or megapixels for messages ("10 MB", "40 MP"). */
export const mega = (n: number, unit: number) => Math.round((n / unit) * 10) / 10;
export const MB = 1024 * 1024;

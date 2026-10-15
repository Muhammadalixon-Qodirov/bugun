// Optional error tracking with Sentry. Everything here is a no-op unless
// NEXT_PUBLIC_SENTRY_DSN is set; the SDK itself is loaded only then
// (src/instrumentation.ts, src/instrumentation-client.ts).
//
// Imported by next.config.ts and src/proxy.ts, so no "@/" imports and no SDK import.

import type { ErrorEvent } from "@sentry/nextjs";

/** The project's DSN, or undefined when error tracking is off. Inlined at build time. */
export const sentryDsn = process.env.NEXT_PUBLIC_SENTRY_DSN?.trim() || undefined;

/** Where the browser sends events (for the Content-Security-Policy), or null when off. */
export function sentryOrigin() {
  if (!sentryDsn) return null;
  try {
    return new URL(sentryDsn).origin;
  } catch {
    return null;
  }
}

/** Shared SDK options for the browser, the server and the edge runtime. */
export function sentryOptions() {
  return {
    dsn: sentryDsn,
    environment: process.env.NEXT_PUBLIC_SENTRY_ENVIRONMENT || process.env.NODE_ENV,
    // Performance tracing is off unless asked for: errors are what we are after.
    tracesSampleRate: Number(process.env.NEXT_PUBLIC_SENTRY_TRACES_SAMPLE_RATE) || 0,
    // No IP addresses, cookies or request bodies.
    sendDefaultPii: false,
    beforeSend: scrubEvent,
    beforeBreadcrumb: <B>(breadcrumb: B) => scrub(breadcrumb) as B,
  };
}

// Header, cookie and field names whose values are never sent.
const SECRET_KEY = /^(cookie|cookies|set-cookie|authorization|x-internal-secret|x-client-ip|x-forwarded-for|x-real-ip|phone|phone_number|contact|initdata|tgwebappdata|hash)$/i;
// Uzbek numbers (+998 90 000 00 00, 998900000000, (90) 000-00-00) and other long international ones.
const PHONE = /(?:\+|\b)(?:998[\s-]?)?\(?\d{2}\)?[\s-]?\d{3}[\s-]?\d{2}[\s-]?\d{2}\b|\+\d[\d\s-]{9,}\d/g;
// Telegram Mini App launch data (user name, id, signature) in page addresses.
const TG_DATA = /(tgWebAppData|initData)=[^&#\s]*/gi;

function scrubString(s: string) {
  return s.replace(TG_DATA, "$1=[filtered]").replace(PHONE, "[phone]");
}

/** A copy of any value with secrets dropped and phone numbers masked. */
export function scrub(value: unknown, depth = 0): unknown {
  if (typeof value === "string") return scrubString(value);
  if (!value || typeof value !== "object" || depth > 8) return value;
  if (Array.isArray(value)) return value.map((v) => scrub(v, depth + 1));
  return Object.fromEntries(
    Object.entries(value).map(([k, v]) => [k, SECRET_KEY.test(k) ? "[filtered]" : scrub(v, depth + 1)]),
  );
}

/** beforeSend: no cookies or user details, and no phone numbers anywhere in the event. */
export function scrubEvent(event: ErrorEvent): ErrorEvent {
  const e = scrub(event) as ErrorEvent;
  if (e.request) {
    delete e.request.cookies;
    delete e.request.data;
  }
  if (e.user) e.user = e.user.id != null ? { id: e.user.id } : undefined;
  return e;
}

/** Reports an error an error boundary caught (those never reach the global handlers). */
export function captureBoundaryError(error: unknown) {
  if (!sentryDsn) return;
  import("@sentry/nextjs")
    .then((Sentry) => Sentry.captureException(error))
    .catch(() => {});
}

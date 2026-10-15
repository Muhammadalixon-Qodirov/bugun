import type { Instrumentation } from "next";

import { sentryDsn, sentryOptions } from "@/lib/sentry";

// Server and edge error tracking, only when NEXT_PUBLIC_SENTRY_DSN is set.
export async function register() {
  if (!sentryDsn) return;
  if (process.env.NEXT_RUNTIME === "nodejs" || process.env.NEXT_RUNTIME === "edge") {
    const Sentry = await import("@sentry/nextjs");
    Sentry.init(sentryOptions());
  }
}

/** Errors while rendering pages or running server actions and route handlers. */
export const onRequestError: Instrumentation.onRequestError = async (...args) => {
  if (!sentryDsn) return;
  const Sentry = await import("@sentry/nextjs");
  Sentry.captureRequestError(...args);
};

// Browser error tracking: loads Sentry only when NEXT_PUBLIC_SENTRY_DSN is set
// at build time, so without it no SDK code reaches the browser.
import { sentryDsn, sentryOptions } from "@/lib/sentry";

type Sdk = typeof import("@sentry/nextjs");
let sdk: Sdk | undefined;

if (sentryDsn) {
  import("@sentry/nextjs")
    .then((Sentry) => {
      Sentry.init(sentryOptions());
      sdk = Sentry;
    })
    .catch(() => {
      // Error tracking must never break the site.
    });
}

/** Names page navigations in Sentry's performance view (when tracing is on). */
export function onRouterTransitionStart(href: string, navigationType: string) {
  sdk?.captureRouterTransitionStart(href, navigationType);
}

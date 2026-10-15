import { withSentryConfig } from "@sentry/nextjs/config";
import type { NextConfig } from "next";
import createNextIntlPlugin from "next-intl/plugin";

import { apiInternalUrl, mediaBaseUrl, publicApiOrigin, publicApiUrl, siteUrl } from "./src/lib/env";
import { sentryDsn } from "./src/lib/sentry";

// Checked when the config loads: a production build or server without these
// stops here rather than calling localhost.
const apiUrl = apiInternalUrl();
const browserApiUrl = publicApiUrl();
const apiOrigin = publicApiOrigin();
const mediaBase = mediaBaseUrl();
siteUrl();

const production = process.env.NODE_ENV === "production";

// The Content-Security-Policy needs a per-request nonce, so it is set in src/proxy.ts.
const securityHeaders = [
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(self), geolocation=(), payment=(), usb=(), browsing-topics=()" },
  ...(production ? [{ key: "Strict-Transport-Security", value: "max-age=63072000; includeSubDomains" }] : []),
];

const nextConfig: NextConfig = {
  // The Go API is served under the site's own origin, so the session cookie is
  // first-party and Telegram login returns to the same domain.
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${apiUrl}/:path*` }];
  },
  async headers() {
    return [{ source: "/:path*", headers: securityHeaders }];
  },
  images: {
    // Listing photos (/media/{key}): through this site's /api proxy, or from the
    // API's own host, which may redirect to public photo storage (NEXT_PUBLIC_MEDIA_BASE_URL).
    localPatterns: [{ pathname: "/api/media/**", search: "" }],
    remotePatterns: [
      ...(apiOrigin ? [new URL(`${browserApiUrl}/media/**`)] : []),
      ...(mediaBase ? [new URL(`${mediaBase}/**`)] : []),
    ],
    // Stored photos never change under the same key.
    minimumCacheTTL: 60 * 60 * 24 * 30,
  },
  poweredByHeader: false,
  // Allow the dev tunnel host (cloudflared) to load dev resources.
  allowedDevOrigins: ["*.trycloudflare.com"],
};

const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");

// Sentry's build step only when error tracking is on. Source maps are uploaded
// (and then removed from the public build) only with SENTRY_AUTH_TOKEN, so a
// build without the token never fails or calls Sentry.
const sentryAuthToken = process.env.SENTRY_AUTH_TOKEN || undefined;
const withSentry = (config: NextConfig) =>
  sentryDsn
    ? withSentryConfig(config, {
        org: process.env.SENTRY_ORG,
        project: process.env.SENTRY_PROJECT,
        authToken: sentryAuthToken,
        silent: !process.env.CI,
        telemetry: false,
        sourcemaps: { disable: !sentryAuthToken, deleteSourcemapsAfterUpload: true },
        release: { create: Boolean(sentryAuthToken) },
      })
    : config;

export default withSentry(withNextIntl(nextConfig));

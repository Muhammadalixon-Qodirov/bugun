import { createHash } from "node:crypto";

import { mediaBaseUrl, publicApiOrigin } from "@/lib/env";
import { sentryOrigin } from "@/lib/sentry";
import { telegramFlagScript } from "@/lib/telegram";

const development = process.env.NODE_ENV === "development";

// The root layout's inline Telegram flag script is allowed by its hash.
const flagScriptHash = `'sha256-${createHash("sha256").update(telegramFlagScript).digest("base64")}'`;

/**
 * The page's Content-Security-Policy. Scripts need this request's nonce (Next.js
 * adds it to its own tags) or must be loaded by such a script ('strict-dynamic'),
 * which covers telegram-web-app.js. Inline style attributes stay allowed: the
 * animation and chart code sets them.
 */
export function contentSecurityPolicy(nonce: string) {
  const api = publicApiOrigin();
  // Error reports go straight to Sentry's ingest host, only when it is configured.
  const sentry = sentryOrigin();
  // Photos the API redirects to public storage (S3 or a CDN), when configured.
  const mediaBase = mediaBaseUrl();
  const media = mediaBase && new URL(mediaBase).origin;
  const directives: Record<string, (string | null | false)[] | false> = {
    "default-src": ["'self'"],
    // 'self' and telegram.org only matter to old browsers without 'strict-dynamic'.
    "script-src": ["'self'", `'nonce-${nonce}'`, flagScriptHash, "'strict-dynamic'", "https://telegram.org", development && "'unsafe-eval'"],
    "style-src": ["'self'", "'unsafe-inline'"],
    "img-src": ["'self'", "data:", "blob:", api, media],
    "font-src": ["'self'", "data:"],
    // Voice recordings are held as blobs before they are sent.
    "media-src": ["'self'", "blob:"],
    "connect-src": ["'self'", api, sentry, development && "ws:", development && "wss:"],
    "object-src": ["'none'"],
    "base-uri": ["'self'"],
    "form-action": ["'self'"],
    // The Mini App runs inside Telegram's web clients.
    "frame-ancestors": ["'self'", "https://web.telegram.org", "https://*.telegram.org"],
    "upgrade-insecure-requests": development ? false : [],
  };
  return Object.entries(directives)
    .flatMap(([name, sources]) => (sources ? [[name, ...sources.filter(Boolean)].join(" ")] : []))
    .join("; ");
}

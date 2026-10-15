// Addresses the site depends on. In development they fall back to the local
// API and dev server; a production build or server without them stops with an
// error instead of quietly calling localhost.
//
// Imported by next.config.ts, so it must not use the "@/" path alias.

const production = process.env.NODE_ENV === "production";

function setting(name: string, value: string | undefined, developmentDefault: string) {
  const v = value?.trim().replace(/\/+$/, "");
  if (v) return v;
  if (production) throw new Error(`${name} is not set. Production needs it (see frontend/.env.example).`);
  return developmentDefault;
}

/** The Go API as the Next.js server reaches it (the /api proxy and server components). Server only. */
export function apiInternalUrl() {
  return setting("API_INTERNAL_URL", process.env.API_INTERNAL_URL, "http://localhost:8080");
}

/**
 * The API as the browser reaches it: "/api" (the default) goes through this
 * site's own proxy, an absolute URL is the API on a sibling subdomain. The
 * value is inlined at build time.
 */
export function publicApiUrl() {
  return process.env.NEXT_PUBLIC_API_URL?.trim().replace(/\/+$/, "") || "/api";
}

/** The public site address, for canonical links, the sitemap and link previews. Server only. */
export function siteUrl() {
  const vercel = process.env.VERCEL_PROJECT_PRODUCTION_URL;
  return setting("NEXT_PUBLIC_SITE_URL", process.env.NEXT_PUBLIC_SITE_URL || (vercel && `https://${vercel}`), "http://localhost:3000");
}

/** The API's origin when the browser calls it on another host (for CSP and image optimisation), else null. */
export function publicApiOrigin() {
  const url = publicApiUrl();
  return /^https?:\/\//.test(url) ? new URL(url).origin : null;
}

/**
 * Public address of the photo storage (e.g. an S3 bucket or CDN) that the API's
 * /media/{key} redirects to, or null when the API serves the photos itself.
 * Optional; its origin is allowed for next/image and the Content-Security-Policy.
 */
export function mediaBaseUrl() {
  const url = process.env.NEXT_PUBLIC_MEDIA_BASE_URL?.trim().replace(/\/+$/, "");
  return url && /^https?:\/\//.test(url) ? url : null;
}

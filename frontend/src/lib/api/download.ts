import { publicApiUrl } from "@/lib/api/client-config";

/**
 * A browser link to a file the API serves (CSV, PDF). It goes to the public API
 * origin, which the session cookie also reaches, so a plain <a download> works
 * in production and through the dev proxy alike.
 */
export function apiFileUrl(path: `/v1/${string}`, query: Record<string, string | number | undefined> = {}) {
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) if (v !== undefined && v !== "") params.set(k, String(v));
  const qs = params.toString();
  return `${publicApiUrl}${path}${qs ? `?${qs}` : ""}`;
}

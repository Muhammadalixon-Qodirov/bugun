import { apiInternalUrl, publicApiUrl as browserApiUrl } from "@/lib/env";

import type { CreateClientConfig } from "./gen/client.gen";

/**
 * Where the browser reaches the API. In production the site and the API live on
 * sibling subdomains (CORS + a shared cookie domain), or "/api" for this site's
 * own proxy; in development the Next.js /api rewrite proxies to the local Go server.
 */
export const publicApiUrl = browserApiUrl();

export const createClientConfig: CreateClientConfig = (config) => ({
  ...config,
  baseUrl: typeof window === "undefined" ? apiInternalUrl() : publicApiUrl,
  credentials: "include",
});

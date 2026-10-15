import "server-only";

import { unstable_cache } from "next/cache";
import { cookies, headers } from "next/headers";
import { cache } from "react";

import {
  browseProducts,
  getCatalog,
  getListing,
  getMe,
  getMyListing,
  getProduct,
  listListings,
  listMyListings,
  metaLimits,
} from "@/lib/api/gen/sdk.gen";
import type { BrowseProductsData, ListListingsData } from "@/lib/api/gen/types.gen";
import { FALLBACK_LIMITS, type Limits, LIMITS_MAX_AGE_SECONDS } from "@/lib/limits";
import { REGION_COOKIE } from "@/lib/region";
import { SESSION_COOKIE } from "@/lib/session";

// Forwards the visitor's session cookie when a server component calls the API,
// plus their IP so the API rate-limits per visitor rather than per site server.
export async function withCookies() {
  const [store, h] = await Promise.all([cookies(), headers()]);
  const forwarded: Record<string, string> = { cookie: store.toString() };
  const secret = process.env.INTERNAL_API_SECRET;
  const ip = h.get("x-forwarded-for")?.split(",")[0]?.trim() || h.get("x-real-ip");
  if (secret && ip) {
    forwarded["x-internal-secret"] = secret;
    forwarded["x-client-ip"] = ip;
  }
  return { headers: forwarded, cache: "no-store" as const };
}

/**
 * The signed-in user, or null for visitors. Deduplicated per request. Visitors
 * without a session cookie are answered here, sparing every page an API call.
 */
export const getCurrentUser = cache(async () => {
  if (!(await cookies()).has(SESSION_COOKIE)) return null;
  const { data, response } = await getMe(await withCookies());
  if (data) return data;
  // Only an unknown or expired session means "signed out". A busy or failing API
  // (429, 5xx) must not bounce a signed-in visitor to the login page.
  if (response?.status === 401) return null;
  throw new Error(`Could not load the current user: ${response?.status ?? "no response"}`);
});

// Regions, markets and products change only with a deploy: cache for an hour
// across requests instead of fetching them on every page.
const cachedCatalog = unstable_cache(
  async () => {
    const { data, error } = await getCatalog();
    if (!data) throw new Error(`Could not load the catalog: ${JSON.stringify(error)}`);
    return data;
  },
  ["catalog-v1"],
  { revalidate: 3600 },
);
export const getCatalogData = cache(() => cachedCatalog());

// The API's business limits change only with its restart. A failed request is
// not cached (it throws inside), so the next page asks again.
class LimitsUnavailable extends Error {
  constructor(readonly status?: number) {
    super(`Could not load the limits: ${status ?? "no response"}`);
  }
}
const cachedLimits = unstable_cache(
  async () => {
    const { data, response } = await metaLimits({ signal: AbortSignal.timeout(3000) });
    if (!data) throw new LimitsUnavailable(response?.status);
    return data;
  },
  ["meta-limits-v1"],
  { revalidate: LIMITS_MAX_AGE_SECONDS },
);

/**
 * The API's business limits. Null when the API could not be reached or failed
 * (5xx): the browser then asks itself (useLimits). An API that answers but has
 * no limits (an older version) gets the defaults, without asking again.
 */
export const getLimitsOrNull = cache(async (): Promise<Limits | null> =>
  cachedLimits().catch((err: unknown) => {
    const status = err instanceof LimitsUnavailable ? err.status : undefined;
    return status && status < 500 ? FALLBACK_LIMITS : null;
  }),
);

/** The API's business limits, the defaults when it could not be asked. */
export const getLimits = cache(async (): Promise<Limits> => (await getLimitsOrNull()) ?? FALLBACK_LIMITS);

export async function getMyListings() {
  const { data } = await listMyListings(await withCookies());
  return data?.listings ?? [];
}

/**
 * Region used for prices: the visitor's saved choice, else the province of
 * their profile, else the whole country (0).
 */
export const getRegionId = cache(async () => {
  const store = await cookies();
  const saved = Number(store.get(REGION_COOKIE)?.value);
  if (Number.isInteger(saved) && saved >= 0 && store.has(REGION_COOKIE)) return saved;
  const [me, catalog] = await Promise.all([getCurrentUser(), getCatalogData()]);
  const district = catalog.regions.find((r) => r.id === me?.region_id);
  return district?.parent_id ?? district?.id ?? 0;
});

export async function browse(query: NonNullable<BrowseProductsData["query"]>) {
  const { data, error } = await browseProducts({ ...(await withCookies()), query });
  if (!data) throw new Error(`Could not load the products: ${JSON.stringify(error)}`);
  return data.products;
}

/** Product page data, or null when the product does not exist. */
export const getProductPage = cache(async function getProductPage(slug: string, region: number, trade: "ulgurji" | "chakana") {
  const { data, response } = await getProduct({ ...(await withCookies()), path: { slug }, query: { region, trade } });
  if (response?.status === 404) return null;
  if (!data) throw new Error("Could not load the product");
  return data;
});

export async function getOwnListing(id: number) {
  const { data } = await getMyListing({ ...(await withCookies()), path: { id } });
  return data?.listing ?? null;
}

export async function feed(query: NonNullable<ListListingsData["query"]>) {
  const { data, error } = await listListings({ ...(await withCookies()), query });
  if (!data) throw new Error(`Could not load the listings: ${JSON.stringify(error)}`);
  return data;
}

/** Public listing page, or null when missing or expired. */
export const getListingPage = cache(async function getListingPage(id: number, region: number) {
  const { data, response } = await getListing({ ...(await withCookies()), path: { id }, query: { region } });
  if (response?.status === 404) return null;
  if (!data) throw new Error("Could not load the listing");
  return data;
});

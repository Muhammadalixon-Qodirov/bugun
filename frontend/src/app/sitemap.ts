import type { MetadataRoute } from "next";

import { listListings } from "@/lib/api/gen/sdk.gen";
import { siteUrl } from "@/lib/env";
import { getCatalogData } from "@/lib/server-api";

// Rebuilt at most hourly; a failed API call leaves the pages that need no data.
export const revalidate = 3600;

const PER_PAGE = 60; // the API's maximum
const MAX_PAGES = 10;

const staticPaths = ["/", "/katalog", "/statistika", "/xarita", "/solishtirish", "/tahlilchi", "/narx"];

async function activeListings() {
  const page = (n: number) => listListings({ query: { page: n, per_page: PER_PAGE } }).then((r) => r.data);
  const first = await page(1);
  if (!first) return [];
  const pages = Math.min(Math.ceil(first.total / PER_PAGE), MAX_PAGES);
  const rest = await Promise.all(Array.from({ length: Math.max(0, pages - 1) }, (_, i) => page(i + 2)));
  return [first, ...rest].flatMap((p) => p?.listings ?? []);
}

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const base = siteUrl();
  const entry = (path: string, extra: Partial<MetadataRoute.Sitemap[number]> = {}) => ({ url: `${base}${path}`, ...extra });
  const pages: MetadataRoute.Sitemap = staticPaths.map((p) => entry(p, { changeFrequency: "daily", priority: p === "/" ? 1 : 0.7 }));

  const [catalog, listings] = await Promise.all([getCatalogData().catch(() => null), activeListings().catch(() => [])]);
  if (catalog) {
    for (const p of catalog.products) {
      pages.push(entry(`/mahsulot/${p.slug}`, { changeFrequency: "daily", priority: 0.8 }));
      pages.push(entry(`/mahsulot/${p.slug}/tahlil`, { changeFrequency: "weekly", priority: 0.5 }));
    }
    for (const r of catalog.regions.filter((r) => !r.parent_id)) pages.push(entry(`/hudud/${r.id}`, { changeFrequency: "weekly", priority: 0.5 }));
  }
  for (const l of listings) pages.push(entry(`/elon/${l.id}`, { lastModified: l.updated_at, changeFrequency: "weekly", priority: 0.6 }));
  for (const seller of new Set(listings.map((l) => l.seller_id))) pages.push(entry(`/sotuvchi/${seller}`, { changeFrequency: "weekly", priority: 0.4 }));
  return pages;
}

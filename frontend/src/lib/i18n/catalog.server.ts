import "server-only";

import { getLocale } from "next-intl/server";
import { cache } from "react";

import { defaultLocale } from "@/i18n/config";
import { catalogNames, type CatalogNameMaps, emptyCatalogNames } from "@/lib/i18n/catalog";
import { getCatalogData } from "@/lib/server-api";

type Dict = { categories: Record<string, string>; products: Record<string, string>; regions: Record<string, string>; markets: Record<string, string> };

/** Name maps for the visitor's language, built once per request. */
export const getCatalogNameMaps = cache(async (): Promise<CatalogNameMaps> => {
  const [locale, catalog] = await Promise.all([getLocale(), getCatalogData()]);
  const provinceSlugs = Object.fromEntries(catalog.regions.filter((r) => !r.parent_id).map((r) => [r.id, r.slug]));
  if (locale === defaultLocale) return { ...emptyCatalogNames, provinceSlugs };
  const dict = (await import(`@/messages/catalog.${locale}.json`)).default as Dict;
  const byUzName = (items: { slug: string; name: string }[], names: Record<string, string>) =>
    Object.fromEntries(items.filter((i) => names[i.slug]).map((i) => [i.name, names[i.slug]]));
  return {
    categories: dict.categories,
    products: dict.products,
    productsByName: byUzName(catalog.products, dict.products),
    regions: byUzName(catalog.regions, dict.regions),
    markets: byUzName(catalog.markets, dict.markets),
    provinceSlugs,
  };
});

/** Catalog name helpers for server components. */
export async function getCatalogNames() {
  return catalogNames(await getCatalogNameMaps());
}

/**
 * Catalog names (categories, products, regions, markets) come from the API in
 * Uzbek. Their English and Russian names live in messages/catalog.{en,ru}.json,
 * keyed by slug. Listing payloads carry region and market names rather than
 * slugs, so those are looked up by their (unique) Uzbek name. A missing
 * translation shows the Uzbek name, never an empty string.
 */
export type CatalogNameMaps = {
  categories: Record<string, string>;
  products: Record<string, string>;
  productsByName: Record<string, string>;
  regions: Record<string, string>;
  markets: Record<string, string>;
  /** Province (top-level region) id -> slug, in every language, for short names (stats.provinceShort). */
  provinceSlugs: Record<number, string>;
};

export const emptyCatalogNames: CatalogNameMaps = { categories: {}, products: {}, productsByName: {}, regions: {}, markets: {}, provinceSlugs: {} };

export type CatalogNames = {
  category: (slug: string, fallback: string) => string;
  product: (slug: string | undefined, fallback: string) => string;
  region: (uzName: string) => string;
  market: (uzName: string) => string;
};

export function catalogNames(maps: CatalogNameMaps): CatalogNames {
  return {
    category: (slug, fallback) => maps.categories[slug] ?? fallback,
    product: (slug, fallback) => (slug && maps.products[slug]) || maps.productsByName[fallback] || fallback,
    region: (name) => maps.regions[name] ?? name,
    market: (name) => maps.markets[name] ?? name,
  };
}

import { BadgePercent, LayoutGrid, Package } from "lucide-react";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";

import { CatalogToolbar, FilterPanel } from "@/components/catalog-filters";
import { categoryIcons } from "@/components/category-icon";
import { ListingFeed } from "@/components/listing-feed";
import { CategoryTabs, MarketNavProvider, MarketPending } from "@/components/market-nav";
import type { ListListingsData } from "@/lib/api/gen/types.gen";
import { getCatalogNames } from "@/lib/i18n/catalog.server";
import { feed, getCatalogData, getRegionId } from "@/lib/server-api";

type Query = NonNullable<ListListingsData["query"]>;
export type SearchParams = Record<string, string | string[] | undefined>;

const one = (v: string | string[] | undefined) => (typeof v === "string" && v ? v : undefined);
const num = (v: string | string[] | undefined) => {
  const n = Number(one(v));
  return Number.isInteger(n) && n > 0 ? n : undefined;
};

/** The marketplace: category row, filters and an endlessly loading listing grid. */
export async function MarketView({ searchParams, basePath }: { searchParams: SearchParams; basePath: string }) {
  const [catalog, region, t, tc, names] = await Promise.all([getCatalogData(), getRegionId(), getTranslations("market"), getTranslations("common"), getCatalogNames()]);
  const category = one(searchParams.category);
  const product = one(searchParams.product);
  const query: Query = {
    region,
    q: one(searchParams.q)?.slice(0, 60),
    category: catalog.categories.some((c) => c.slug === category) ? (category as Query["category"]) : undefined,
    product: catalog.products.some((p) => p.slug === product) ? product : undefined,
    trade: one(searchParams.trade) === "chakana" ? "chakana" : one(searchParams.trade) === "ulgurji" ? "ulgurji" : undefined,
    sort: (["deal", "price_asc", "price_desc", "popular"] as const).find((s) => s === one(searchParams.sort)) ?? "new",
    min_price: num(searchParams.min_price),
    max_price: num(searchParams.max_price),
  };
  const first = await feed({ ...query, per_page: 24 });
  // Names in the visitor's language; the Uzbek one stays for search.
  const productOptions = catalog.products.map((p) => ({ slug: p.slug, name: names.product(p.slug, p.name), uzName: p.name, category: p.category }));
  const categories = catalog.categories.map((c) => ({ slug: c.slug, name: names.category(c.slug, c.name) }));
  const regionUz = catalog.regions.find((r) => r.id === region)?.name;

  return (
    <MarketNavProvider>
      <nav aria-label={t("categories")} className="-mx-4 border-b sm:mx-0">
        <Suspense fallback={<div className="h-[4.5rem]" />}>
          <CategoryTabs
            basePath={basePath}
            tabs={[
              { key: "all", label: tc("all"), icon: <LayoutGrid strokeWidth={1.5} className="size-6" /> },
              ...categories.map((c) => {
                const Icon = categoryIcons[c.slug] ?? Package;
                return { key: c.slug, label: c.name, icon: <Icon strokeWidth={1.5} className="size-6" /> };
              }),
              { key: "deal", label: t("deals"), icon: <BadgePercent strokeWidth={1.5} className="size-6" />, accent: true },
            ]}
          />
        </Suspense>
      </nav>

      <div className="mt-8 grid gap-8 lg:grid-cols-[15rem_1fr]">
        <aside className="hidden lg:block">
          <div className="sticky top-24 max-h-[calc(var(--app-height)-7rem)] overflow-y-auto rounded-3xl scrollbar-none" data-lenis-prevent>
            <Suspense>
              <FilterPanel basePath={basePath} products={productOptions} />
            </Suspense>
          </div>
        </aside>
        <div className="min-w-0 space-y-6">
          <Suspense>
            <CatalogToolbar
              basePath={basePath}
              total={first.total}
              regionName={regionUz ? names.region(regionUz) : tc("wholeCountry")}
              products={productOptions}
              categories={categories}
            />
          </Suspense>
          <MarketPending>
            <ListingFeed query={query} initial={first} />
          </MarketPending>
        </div>
      </div>
    </MarketNavProvider>
  );
}

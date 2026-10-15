import { TableProperties } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Page } from "@/components/page";
import { PageHeader } from "@/components/page-header";
import { CompareView } from "@/components/stats/compare-view";
import { buttonVariants } from "@/components/ui/button";
import { getCatalogData, getLimits } from "@/lib/server-api";
import { parseList, parsePeriod, parseTrade } from "@/lib/stats";

export async function generateMetadata(): Promise<Metadata> {
  const [t, limits] = await Promise.all([getTranslations("stats"), getLimits()]);
  return { title: t("compare.metaTitle"), description: t("compare.description", { max: limits.compare_max_lines }) };
}

// Shown when the link carries no products: two staples most people know the price of.
const DEFAULT_PRODUCTS = ["pomidor", "kartoshka"];

export default async function ComparePage({ searchParams }: PageProps<"/solishtirish">) {
  const sp = await searchParams;
  const [catalog, t, limits] = await Promise.all([getCatalogData(), getTranslations("stats"), getLimits()]);
  const known = new Set(catalog.products.map((p) => p.slug));
  const listed = parseList(sp.products).filter((s) => known.has(s));
  // A district id stands for its province, as the API reads it.
  const byId = new Map(catalog.regions.map((r) => [r.id, r]));
  const regions = [
    ...new Set(
      parseList(sp.regions).flatMap((s) => {
        const r = byId.get(Number(s));
        return r ? [r.parent_id ?? r.id] : [];
      }),
    ),
  ];
  const fallback = DEFAULT_PRODUCTS.filter((s) => known.has(s)).slice(0, Math.max(1, Math.floor(limits.compare_max_lines / Math.max(1, regions.length))));
  const products = [...new Set(sp.products === undefined ? fallback : listed)];

  return (
    <Page className="py-10 sm:py-14">
      <PageHeader
        eyebrow={
          <Link href="/statistika" className="hover:text-foreground">
            {t("eyebrow")}
          </Link>
        }
        title={t("compare.title")}
        description={t("compare.description", { max: limits.compare_max_lines })}
        action={
          <Link href="/statistika" className={buttonVariants({ variant: "outline", size: "pill", className: "h-11" })}>
            <TableProperties /> {t("links.heat")}
          </Link>
        }
      />
      <div className="mt-8">
        <CompareView
          products={catalog.products.map(({ slug, name, category, base_unit }) => ({ slug, name, category, base_unit }))}
          provinces={catalog.regions.filter((r) => !r.parent_id).map(({ id, name }) => ({ id, name }))}
          categories={catalog.categories}
          initial={{ products, regions, trade: parseTrade(sp.trade), period: parsePeriod(sp.period) }}
        />
      </div>
    </Page>
  );
}

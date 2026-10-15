import { TableProperties } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Page } from "@/components/page";
import { PageHeader } from "@/components/page-header";
import { MapView } from "@/components/stats/map-view";
import { buttonVariants } from "@/components/ui/button";
import { getCatalogData } from "@/lib/server-api";
import { parseList, parseTrade } from "@/lib/stats";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("stats");
  return { title: t("map.metaTitle"), description: t("map.description") };
}

export default async function MapPage({ searchParams }: PageProps<"/xarita">) {
  const sp = await searchParams;
  const [catalog, t] = await Promise.all([getCatalogData(), getTranslations("stats")]);
  const asked = parseList(sp.product)[0];
  const product = catalog.products.find((p) => p.slug === asked)?.slug ?? catalog.products.find((p) => p.slug === "pomidor")?.slug ?? catalog.products[0].slug;

  return (
    <Page className="py-10 sm:py-14">
      <PageHeader
        eyebrow={
          <Link href="/statistika" className="hover:text-foreground">
            {t("eyebrow")}
          </Link>
        }
        title={t("map.title")}
        description={t("map.description")}
        action={
          <Link href="/statistika" className={buttonVariants({ variant: "outline", size: "pill", className: "h-11" })}>
            <TableProperties /> {t("links.heat")}
          </Link>
        }
      />
      <div className="mt-8">
        <MapView
          products={catalog.products.map(({ slug, name, category, base_unit }) => ({ slug, name, category, base_unit }))}
          regions={catalog.regions.filter((r) => !r.parent_id).map(({ id, name }) => ({ id, name }))}
          categories={catalog.categories}
          initial={{ product, trade: parseTrade(sp.trade) }}
        />
      </div>
    </Page>
  );
}

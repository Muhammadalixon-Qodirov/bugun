import { ArrowDownRight, ArrowUpRight, ChartLine, Map as MapIcon, MessageCircleQuestion, TableProperties } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { DemoBadge } from "@/components/demo-badge";
import { EmptyState } from "@/components/empty-state";
import { Reveal } from "@/components/motion";
import { Page } from "@/components/page";
import { PageHeader } from "@/components/page-header";
import { ProvinceClusters } from "@/components/stats/clusters";
import { CategoryTabs, TradeToggle } from "@/components/stats/filters";
import { DiffLegend, HeatTable } from "@/components/stats/heat-table";
import { Card, DemandList, MoverList } from "@/components/stats/lists";
import { PendingArea, QueryNav } from "@/components/stats/query-nav";
import { buttonVariants } from "@/components/ui/button";
import { provinceClusters } from "@/lib/api/gen/sdk.gen";
import { getHeatmap, getTopDemand } from "@/lib/server/stats";
import { getCatalogData, withCookies } from "@/lib/server-api";
import { parseCategory, parseTrade, type Trade } from "@/lib/stats";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("stats");
  return { title: t("heat.metaTitle"), description: t("heat.description") };
}

export default async function StatisticsPage({ searchParams }: PageProps<"/statistika">) {
  const sp = await searchParams;
  const trade = parseTrade(sp.trade);
  const category = parseCategory(sp.category);
  const [heat, demand, clusters, catalog, t, tn] = await Promise.all([
    getHeatmap(trade, category),
    getTopDemand(),
    getClusters(trade),
    getCatalogData(),
    getTranslations("stats"),
    getTranslations("nav"),
  ]);

  return (
    <QueryNav>
      <Page className="py-10 sm:py-14">
        <PageHeader
          eyebrow={
            <span className="flex flex-wrap items-center gap-2">
              {t("eyebrow")}
              <DemoBadge share={heat.demo_share} />
            </span>
          }
          title={t("heat.title")}
          description={t("heat.description")}
          action={
            <div className="flex flex-wrap gap-2">
              <Link href="/solishtirish" className={buttonVariants({ variant: "outline", size: "pill", className: "h-11" })}>
                <ChartLine /> {t("links.compare")}
              </Link>
              <Link href="/xarita" className={buttonVariants({ variant: "outline", size: "pill", className: "h-11" })}>
                <MapIcon /> {t("links.map")}
              </Link>
              <Link href="/tahlilchi" className={buttonVariants({ variant: "outline", size: "pill", className: "h-11" })}>
                <MessageCircleQuestion /> {tn("analyst")}
              </Link>
            </div>
          }
        />

        <Reveal delay={0.05} className="mt-8 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <CategoryTabs categories={catalog.categories} />
          <TradeToggle className="shrink-0" />
        </Reveal>

        <PendingArea className="mt-6 space-y-6">
          <Reveal delay={0.1}>
            {heat.rows.length === 0 ? (
              <EmptyState icon={<TableProperties strokeWidth={1.5} />} title={t("heat.emptyTitle")} description={t("heat.emptyText")} />
            ) : (
              <div className="space-y-3">
                <div className="flex flex-wrap items-end justify-between gap-3">
                  <DiffLegend spots={heat.rows.some((r) => r.cells.some((c) => c?.spot))} />
                  <p className="text-xs text-muted-foreground">{t("heat.count", { count: heat.rows.length })}</p>
                </div>
                <HeatTable data={heat} />
                <p className="text-xs text-muted-foreground">
                  {t("heat.note")} {t("freshness")}
                </p>
              </div>
            )}
          </Reveal>

          {clusters && (
            <Reveal delay={0.1}>
              <ProvinceClusters data={clusters} />
            </Reveal>
          )}

          <div className="grid gap-4 md:grid-cols-2">
            <Card title={t("movers.up")} hint={<Arrow up label={t("movers.hint")} />}>
              <MoverList items={heat.up} dir="up" />
            </Card>
            <Card title={t("movers.down")} hint={<Arrow label={t("movers.hint")} />}>
              <MoverList items={heat.down} dir="down" />
            </Card>
            <Card title={t("demand.top")} hint={t("demand.topHint")}>
              <DemandList items={demand.top} kind="top" />
            </Card>
            <Card title={t("demand.rising")} hint={t("demand.risingHint")}>
              <DemandList items={demand.rising} kind="rising" />
            </Card>
          </div>
        </PendingArea>
      </Page>
    </QueryNav>
  );
}

/** Province groups by price profile; the section is left out when the API cannot answer. */
async function getClusters(trade: Trade) {
  const { data } = await provinceClusters({ ...(await withCookies()), query: { trade } });
  return data ?? null;
}

function Arrow({ up, label }: { up?: boolean; label: string }) {
  const Icon = up ? ArrowUpRight : ArrowDownRight;
  return (
    <span className="flex items-center gap-1.5">
      <Icon className="size-3.5" />
      {label}
    </span>
  );
}

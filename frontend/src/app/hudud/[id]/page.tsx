import { ArrowDownRight, ArrowRight, ArrowUpRight, Minus, Route, Truck } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { BarList } from "@/components/charts/bar-list";
import { DemoBadge } from "@/components/demo-badge";
import { Reveal } from "@/components/motion";
import { Page } from "@/components/page";
import { PageHeader } from "@/components/page-header";
import { SimilarProvinces } from "@/components/stats/clusters";
import { TradeToggle } from "@/components/stats/filters";
import { Card, Collecting, DemandList } from "@/components/stats/lists";
import { PendingArea, QueryNav } from "@/components/stats/query-nav";
import { IndexChart, ProvincePicker, RegionProducts, Signed } from "@/components/stats/region";
import { buttonVariants } from "@/components/ui/button";
import { ZoneBadge } from "@/components/zone-badge";
import type { Count, RegionProduct } from "@/lib/api/gen/types.gen";
import { getCatalogNames } from "@/lib/i18n/catalog.server";
import { getFormat } from "@/lib/i18n/format.server";
import { formatSom } from "@/lib/labels";
import { getRegionOverview, getTopDemand } from "@/lib/server/stats";
import { getCatalogData } from "@/lib/server-api";
import { parseTrade } from "@/lib/stats";
import { cn } from "@/lib/utils";

const regionId = (id: string) => (/^\d{1,9}$/.test(id) && Number(id) > 0 ? Number(id) : null);

export async function generateMetadata({ params }: PageProps<"/hudud/[id]">): Promise<Metadata> {
  const id = regionId((await params).id);
  const [t, names] = await Promise.all([getTranslations("stats"), getCatalogNames()]);
  const data = id ? await getRegionOverview(id, "ulgurji") : null;
  if (!data) return { title: t("region.notFoundTitle") };
  const name = names.region(data.province_name);
  return { title: t("region.metaTitle", { region: name }), description: t("region.description", { count: data.products.length }) };
}

export default async function RegionPage({ params, searchParams }: PageProps<"/hudud/[id]">) {
  const [{ id: raw }, sp] = await Promise.all([params, searchParams]);
  const id = regionId(raw);
  if (!id) notFound();
  const trade = parseTrade(sp.trade);
  const data = await getRegionOverview(id, trade);
  if (!data) notFound();

  const [demand, catalog, t, tc, fmt, names] = await Promise.all([
    getTopDemand(data.province_id),
    getCatalogData(),
    getTranslations("stats"),
    getTranslations("common"),
    getFormat(),
    getCatalogNames(),
  ]);
  const province = names.region(data.province_name);
  const provinces = catalog.regions.filter((r) => !r.parent_id).map(({ id, name }) => ({ id, name }));
  const slugs = new Map(catalog.products.map((p) => [p.id, p.slug]));
  const perUnit = (unit: string) => tc("perUnit", { unit: fmt.unit(unit) });
  const { passport } = data;

  return (
    <QueryNav>
      <Page className="py-10 sm:py-14">
        <PageHeader
          eyebrow={
            <span className="flex flex-wrap items-center gap-2">
              <Link href="/statistika" className="hover:text-foreground">
                {t("eyebrow")}
              </Link>
              <DemoBadge share={data.demo_share} />
            </span>
          }
          title={province}
          description={t("region.description", { count: data.products.length })}
          action={
            <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:items-center">
              <ProvincePicker provinces={provinces} current={data.province_id} />
              <TradeToggle />
            </div>
          }
        />

        <PendingArea className="mt-8 space-y-6">
          <Reveal delay={0.05} className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6">
            <Tile label={t("passport.sellers")} value={formatSom(passport.sellers)} />
            <Tile label={t("passport.listings")} value={formatSom(passport.listings)} />
            <Tile label={t("passport.markets")} value={formatSom(passport.markets)} />
            <CountTile label={t("passport.rfqs")} count={passport.rfqs} prevLabel={t("passport.prev")} fmt={fmt} />
            <CountTile label={t("passport.deals")} count={passport.deals} prevLabel={t("passport.prev")} fmt={fmt} />
            <CountTile label={t("passport.demand")} count={passport.demand} prevLabel={t("passport.prev")} fmt={fmt} />
          </Reveal>
          <p className="-mt-3 text-xs text-muted-foreground">{t("passport.hint")}</p>

          <Reveal delay={0.1} className={cn("grid gap-4", data.index.length > 1 && "lg:grid-cols-[minmax(0,1fr)_22rem]")}>
            {data.index.length > 1 && (
              <Card title={t("region.index")} hint={t("region.indexHint", { count: data.index_basket })}>
                <IndexChart points={data.index} province={province} />
              </Card>
            )}
            <SimilarProvinces items={data.similar} />
          </Reveal>

          <Card title={t("region.products")} hint={t("region.productsHint")}>
            {data.products.length === 0 ? <Collecting text={t("region.noProducts")} /> : <RegionProducts products={data.products} categories={catalog.categories} provinceId={data.province_id} />}
          </Card>

          <div className="grid gap-4 md:grid-cols-2">
            <Card title={t("region.shortages")} hint={t("region.shortagesHint")}>
              <PressureList items={data.shortages} provinceId={data.province_id} perUnit={perUnit} name={names.product} />
            </Card>
            <Card title={t("region.surpluses")} hint={t("region.surplusesHint")}>
              <PressureList items={data.surpluses} provinceId={data.province_id} perUnit={perUnit} name={names.product} />
            </Card>
          </div>

          <section>
            <h2 className="display text-2xl">{t("region.opportunities")}</h2>
            <p className="mt-1 max-w-2xl text-sm text-muted-foreground">{t("region.opportunitiesHint")}</p>
            {data.opportunities.length === 0 ? (
              <div className="mt-4">
                <Collecting text={t("region.noOpportunities")} />
              </div>
            ) : (
              <ul className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {data.opportunities.map((o) => {
                  const slug = slugs.get(o.product_id);
                  const product = names.product(slug, o.name);
                  const there = names.region(o.sell_in);
                  const body = (
                    <>
                      <div className="flex items-start justify-between gap-3">
                        <p className="font-semibold tracking-tight">{product}</p>
                        <span className="num shrink-0 rounded-full bg-primary/10 px-2.5 py-0.5 text-sm font-semibold text-primary">
                          {fmt.percent(o.net_pct, { signed: true })}
                        </span>
                      </div>
                      <div className="mt-4 grid grid-cols-[1fr_auto_1fr] items-end gap-2 text-sm">
                        <div className="min-w-0">
                          <p className="truncate text-xs text-muted-foreground">{t("region.here")}</p>
                          <p className="num font-medium">{formatSom(o.here)}</p>
                        </div>
                        <ArrowRight className="mb-1 size-4 text-muted-foreground" />
                        <div className="min-w-0 text-right">
                          <p className="truncate text-xs text-muted-foreground">{there}</p>
                          <p className="num font-medium">{formatSom(o.there)}</p>
                        </div>
                      </div>
                      <p className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                        <span>{perUnit(o.unit)}</span>
                        <span className="flex items-center gap-1">
                          <Route className="size-3.5" />
                          {t("region.distance", { km: formatSom(o.distance_km) })}
                        </span>
                        <span className="flex items-center gap-1">
                          <Truck className="size-3.5" />
                          {t("region.netHint")}
                        </span>
                      </p>
                    </>
                  );
                  return (
                    <li key={`${o.product_id}-${o.sell_in_id}`}>
                      {slug ? (
                        <Link
                          href={`/mahsulot/${slug}/tahlil?region=${o.sell_in_id}`}
                          className="block h-full rounded-3xl border bg-card p-5 transition-[box-shadow,transform] duration-150 hover:shadow-lg hover:shadow-black/[0.04] pointer-coarse:active:scale-[0.99]"
                        >
                          {body}
                        </Link>
                      ) : (
                        <div className="h-full rounded-3xl border bg-card p-5">{body}</div>
                      )}
                    </li>
                  );
                })}
              </ul>
            )}
          </section>

          <div className="grid gap-4 md:grid-cols-2">
            <Card title={t("region.districts")} hint={t("region.activityHint")}>
              {data.districts.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t("region.noActivity")}</p>
              ) : (
                <BarList
                  rows={data.districts.map((d) => ({
                    key: String(d.id),
                    label: names.region(d.name),
                    value: d.listings,
                    display: t("region.listings", { count: d.listings }),
                    note: t("region.sellers", { count: d.sellers }),
                    noteTone: "muted",
                  }))}
                />
              )}
            </Card>
            <Card title={t("region.markets")} hint={t("region.activityHint")}>
              {data.markets.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t("region.noActivity")}</p>
              ) : (
                <BarList
                  rows={data.markets.map((m) => ({
                    key: String(m.id),
                    label: names.market(m.name),
                    value: m.listings,
                    display: t("region.listings", { count: m.listings }),
                    note: t("region.sellers", { count: m.sellers }),
                    noteTone: "muted",
                  }))}
                />
              )}
            </Card>
            <Card title={t("demand.top")} hint={t("demand.topHintRegion")}>
              <DemandList items={demand.top} kind="top" region={data.province_id} />
            </Card>
            <Card title={t("demand.rising")} hint={t("demand.risingHint")}>
              <DemandList items={demand.rising} kind="rising" region={data.province_id} />
            </Card>
          </div>

          <div className="flex flex-wrap items-center justify-between gap-3">
            <p className="text-xs text-muted-foreground">{t("freshness")}</p>
            <div className="flex flex-wrap gap-2">
              <Link href="/statistika" className={buttonVariants({ variant: "outline", size: "pill" })}>
                {t("links.heat")}
              </Link>
              <Link href={`/solishtirish?regions=${data.province_id}`} className={buttonVariants({ variant: "outline", size: "pill" })}>
                {t("links.compare")}
              </Link>
            </div>
          </div>
        </PendingArea>
      </Page>
    </QueryNav>
  );
}

function Tile({ label, value, children }: { label: string; value: string; children?: React.ReactNode }) {
  return (
    <div className="min-w-0 rounded-2xl border bg-card px-4 py-4 sm:px-5">
      <p className="truncate text-sm text-muted-foreground">{label}</p>
      <p className="num mt-1 text-2xl font-semibold tracking-tight">{value}</p>
      {children}
    </div>
  );
}

/** A weekly count with its change against the week before (the arrow and sign carry the direction). */
function CountTile({ label, count, prevLabel, fmt }: { label: string; count: Count; prevLabel: string; fmt: Awaited<ReturnType<typeof getFormat>> }) {
  const d = count.diff;
  const Icon = d == null || Math.abs(d) < 0.005 ? Minus : d > 0 ? ArrowUpRight : ArrowDownRight;
  return (
    <Tile label={label} value={formatSom(count.now)}>
      <p className="mt-0.5 flex flex-wrap items-center gap-x-1.5 text-xs text-muted-foreground">
        {d != null && (
          <span className="num inline-flex items-center font-medium text-foreground">
            <Icon className="size-3.5" />
            {fmt.percent(d, { signed: true })}
          </span>
        )}
        <span className="num">
          {prevLabel} {formatSom(count.prev ?? 0)}
        </span>
      </p>
    </Tile>
  );
}

function PressureList({
  items,
  provinceId,
  perUnit,
  name,
}: {
  items: RegionProduct[];
  provinceId: number;
  perUnit: (unit: string) => string;
  name: (slug: string, fallback: string) => string;
}) {
  if (items.length === 0) return <Collecting />;
  return (
    <ul className="-mx-2">
      {items.map((p) => (
        <li key={p.id}>
          <Link
            href={`/mahsulot/${p.slug}/tahlil?region=${provinceId}`}
            className={cn("flex items-center justify-between gap-3 rounded-xl px-2 py-2 text-sm transition-colors hover:bg-muted/60")}
          >
            <span className="min-w-0">
              <span className="block truncate font-medium">{name(p.slug, p.name)}</span>
              <span className="num text-xs text-muted-foreground">
                {formatSom(p.median)} {perUnit(p.unit)}
              </span>
            </span>
            <span className="flex shrink-0 items-center gap-2">
              <Signed value={p.week} className="text-xs" />
              <ZoneBadge zone={p.zone} />
            </span>
          </Link>
        </li>
      ))}
    </ul>
  );
}

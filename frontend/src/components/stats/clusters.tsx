"use client";

import { useQuery } from "@tanstack/react-query";
import type { GeoPermissibleObjects } from "d3-geo";
import { CircleCheck, CircleDashed, CircleSlash } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";

import { useCatalogNames } from "@/components/catalog-names";
import { Choropleth, type MapFeature } from "@/components/charts/choropleth";
import { DemoBadge } from "@/components/demo-badge";
import { Card, Collecting } from "@/components/stats/lists";
import { Signed } from "@/components/stats/region";
import { Skeleton } from "@/components/ui/skeleton";
import { provinceMapOptions } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { Clusters, ProductDiff, SimilarProvince } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { cn } from "@/lib/utils";

const qualityIcon = { strong: CircleCheck, weak: CircleDashed, none: CircleSlash };

/** A group's colour follows its price level: cheaper green, dearer orange, grey near the country; full strength at ±10%. */
function levelFill(level: number) {
  const strength = Math.min(1, Math.abs(level) / 0.1);
  if (strength < 0.05) return "var(--viz-mid)";
  return `color-mix(in oklab, var(${level < 0 ? "--viz-cheap" : "--viz-dear"}) ${Math.round(25 + 65 * strength)}%, var(--viz-mid))`;
}

/**
 * Groups of provinces with a similar price profile (K-means). The separation
 * score is shown as it is, and below the threshold the page says there are
 * no clear groups instead of drawing weak ones.
 */
export function ProvinceClusters({ data }: { data: Clusters }) {
  const t = useTranslations("stats");
  const names = useCatalogNames();
  const fmt = useFormat();
  const Icon = qualityIcon[data.quality];

  return (
    <Card title={t("clusters.title")} hint={t("clusters.hint", { products: data.products })}>
      {data.k === 0 ? (
        <Collecting text={t("clusters.noData")} />
      ) : (
        <>
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
            <span
              className={cn(
                "inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium",
                data.quality === "strong" ? "bg-primary/10 text-primary" : "bg-muted text-muted-foreground",
              )}
            >
              <Icon className="size-3.5" />
              {t(`clusters.quality.${data.quality}`)}
            </span>
            <span className="num text-xs font-medium">{t("clusters.silhouette", { value: fmt.decimal(data.silhouette, 2) })}</span>
            <span className="text-xs text-muted-foreground">{t("clusters.silhouetteHint")}</span>
            <DemoBadge share={data.demo_share} />
          </div>

          {data.groups.length === 0 ? (
            <p className="mt-4 rounded-2xl bg-muted/50 px-4 py-3 text-sm text-muted-foreground">{t("clusters.none")}</p>
          ) : (
            <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] lg:items-start">
              <ClustersMap data={data} />
              <ul className="grid gap-3 sm:grid-cols-2">
              {data.groups.map((g, i) => (
                <li key={g.id} className="flex flex-col gap-3 rounded-2xl bg-muted/50 p-4">
                  <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
                    <p className="font-semibold tracking-tight">
                      <span aria-hidden className="mr-2 inline-block size-2.5 rounded-full ring-1 ring-foreground/10" style={{ background: levelFill(g.level) }} />
                      {t("clusters.group", { n: i + 1 })}
                      <span className="ml-2 text-xs font-normal text-muted-foreground">{t("clusters.provinces", { count: g.provinces.length })}</span>
                    </p>
                    <span className={cn("num text-xs font-medium", Math.abs(g.level) < 0.005 ? "text-muted-foreground" : g.level < 0 ? "text-primary" : "text-warm-foreground")}>
                      {t("clusters.level", { value: fmt.percent(g.level, { signed: true }) })}
                    </span>
                  </div>
                  <ul className="flex flex-wrap gap-1.5">
                    {g.provinces.map((p) => (
                      <li key={p.id}>
                        <Link
                          href={`/hudud/${p.id}`}
                          className="inline-flex min-h-8 items-center rounded-full border bg-card px-2.5 text-xs transition-colors hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/40 focus-visible:outline-none"
                        >
                          {names.region(p.name)}
                        </Link>
                      </li>
                    ))}
                  </ul>
                  {(g.cheaper.length > 0 || g.dearer.length > 0) && (
                    <div className="grid grid-cols-2 gap-3 border-t border-foreground/10 pt-3 text-xs">
                      <Products title={t("clusters.cheaper")} items={g.cheaper} />
                      <Products title={t("clusters.dearer")} items={g.dearer} />
                    </div>
                  )}
                </li>
              ))}
              </ul>
            </div>
          )}

          {data.excluded.length > 0 && (
            <p className="mt-3 text-xs text-muted-foreground">{t("clusters.excluded", { list: data.excluded.map((p) => names.region(p.name)).join(", ") })}</p>
          )}
        </>
      )}
    </Card>
  );
}

/** The groups on the map, each province in its group's colour; a click opens the province. */
function ClustersMap({ data }: { data: Clusters }) {
  const t = useTranslations("stats");
  const names = useCatalogNames();
  const fmt = useFormat();
  const router = useRouter();
  const map = useQuery(provinceMapOptions({ query: {} }));

  const groupOf = new Map(data.groups.flatMap((g, i) => g.provinces.map((p) => [p.id, { n: i + 1, level: g.level }] as const)));
  const features: MapFeature[] = (map.data?.features ?? []).flatMap((f) => {
    const id = Number(f.properties.region_id);
    if (!id) return [];
    const g = groupOf.get(id);
    const rows = g ? [{ key: "group", label: t("clusters.group", { n: g.n }), value: t("clusters.level", { value: fmt.percent(g.level, { signed: true }) }), color: levelFill(g.level) }] : [];
    return [{ id: String(id), geometry: f.geometry as GeoPermissibleObjects, name: names.region(String(f.properties.name ?? "")), fill: g ? levelFill(g.level) : "var(--muted)", rows, notes: g ? [] : [t("clusters.notGrouped")] }];
  });

  if (map.isError) return null;
  if (!map.data) return <Skeleton className="aspect-[16/10] w-full rounded-2xl" />;
  return (
    <div className="rounded-2xl border p-2">
      <Choropleth features={features} onSelect={(id) => router.push(`/hudud/${id}`)} ariaLabel={t("clusters.mapAria")} aspect={0.62} />
    </div>
  );
}

function Products({ title, items }: { title: string; items: ProductDiff[] }) {
  const names = useCatalogNames();
  if (items.length === 0) return <div />;
  return (
    <div className="min-w-0">
      <p className="text-muted-foreground">{title}</p>
      <ul className="mt-1 space-y-1">
        {items.map((d) => (
          <li key={d.product.id} className="flex items-baseline justify-between gap-2">
            <Link href={`/mahsulot/${d.product.slug}/tahlil`} className="min-w-0 truncate hover:underline">
              {names.product(d.product.slug, d.product.name)}
            </Link>
            <Signed value={d.diff} className="text-xs" />
          </li>
        ))}
      </ul>
    </div>
  );
}

/** The provinces with the nearest price profile, linked, with a badge when K-means put them in the same group. */
export function SimilarProvinces({ items }: { items: SimilarProvince[] }) {
  const t = useTranslations("stats");
  const names = useCatalogNames();
  const fmt = useFormat();
  return (
    <Card title={t("similar.title")} hint={t("similar.hint")}>
      {items.length === 0 ? (
        <Collecting text={t("similar.empty")} />
      ) : (
        <ul className="-mx-2">
          {items.map((s) => (
            <li key={s.province.id}>
              <Link
                href={`/hudud/${s.province.id}`}
                className="flex min-h-11 items-center justify-between gap-3 rounded-xl px-2 py-2 text-sm transition-colors hover:bg-muted/60"
              >
                <span className="min-w-0">
                  <span className="block font-medium">{names.region(s.province.name)}</span>
                  <span className="num text-xs text-muted-foreground">{t("similar.gap", { value: fmt.percent(s.gap) })}</span>
                </span>
                {s.same_group && (
                  <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-primary/10 px-2.5 py-1 text-xs font-medium text-primary">
                    <CircleCheck className="size-3.5" />
                    {t("similar.sameGroup")}
                  </span>
                )}
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

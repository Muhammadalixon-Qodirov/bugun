"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { GeoPermissibleObjects } from "d3-geo";
import { ArrowRight, ChevronDown, MapPinned, Package } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useEffect, useMemo, useState } from "react";

import { useCatalogNames } from "@/components/catalog-names";
import { Choropleth, type MapFeature } from "@/components/charts/choropleth";
import type { TooltipRow } from "@/components/charts/tooltip";
import { EmptyState } from "@/components/empty-state";
import { Segmented } from "@/components/segmented";
import { Picker } from "@/components/stats/picker";
import { Skeleton } from "@/components/ui/skeleton";
import { ZoneBadge } from "@/components/zone-badge";
import { productAnalyticsOptions, provinceMapOptions } from "@/lib/api/gen/@tanstack/react-query.gen";
import { useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import type { Trade } from "@/lib/stats";
import { cn } from "@/lib/utils";

type ProductOption = { slug: string; name: string; category: string; base_unit: string };
type Zone = "surplus" | "balanced" | "shortage";
type Props = { region_id: number; name: string; offers?: number; median?: number; pressure?: number; zone?: Zone };

const ZONE_FILL: Record<Zone, string> = { surplus: "var(--viz-cheap)", balanced: "var(--viz-mid)", shortage: "var(--viz-dear)" };
// Sequential steps of one hue for provinces with a price but no pressure zone yet.
const PRICE_STEPS = [22, 38, 54, 70, 86];
const priceFill = (step: number) => `color-mix(in oklab, var(--primary) ${PRICE_STEPS[step]}%, transparent)`;
const NO_DATA = "var(--muted)";


/**
 * Demand and supply by province for one product: coloured by pressure zone
 * where there is enough history, otherwise by price. The same numbers are
 * listed under the map.
 */
export function MapView({
  products,
  regions,
  categories,
  initial,
}: {
  products: ProductOption[];
  regions: { id: number; name: string }[];
  categories: { slug: string; name: string }[];
  initial: { product: string; trade: Trade };
}) {
  const t = useTranslations("stats");
  const tc = useTranslations("common");
  const tz = useTranslations("codes");
  const names = useCatalogNames();
  const fmt = useFormat();
  const errors = useApiErrors();
  const [product, setProduct] = useState(initial.product);
  const [trade, setTrade] = useState<Trade>(initial.trade);
  const [selected, setSelected] = useState<string | null>(null);

  useEffect(() => {
    const qs = new URLSearchParams({ product });
    if (trade !== "ulgurji") qs.set("trade", trade);
    window.history.replaceState(null, "", `${window.location.pathname}?${qs}`);
  }, [product, trade]);

  const query = useQuery({ ...provinceMapOptions({ query: { product, trade } }), placeholderData: keepPreviousData });
  // Live prices are thin in most provinces; the last 30 days fill the gaps (marked as such).
  const recent = useQuery({
    ...productAnalyticsOptions({ path: { slug: product }, query: { trade, period: "30d" } }),
    placeholderData: keepPreviousData,
  });

  const option = products.find((p) => p.slug === product);
  const productName = option ? names.product(option.slug, option.name) : product;
  const perUnit = tc("perUnit", { unit: fmt.unit(option?.base_unit ?? "kg") });
  const regionNames = useMemo(() => new Map(regions.map((r) => [r.id, r.name])), [regions]);
  const categoryName = new Map(categories.map((c) => [c.slug, names.category(c.slug, c.name)]));

  const provinces = useMemo(() => {
    const features = query.data?.features ?? [];
    const history = new Map((recent.data?.provinces ?? []).map((a) => [a.id, a.median]));
    const priceOf = (p: Props) => p.median ?? history.get(p.region_id);
    const medians = features.flatMap((f) => {
      const m = priceOf(f.properties as Props);
      return m != null ? [m] : [];
    }).sort((a, b) => a - b);
    // Quantile step of a price among the provinces that have one.
    const step = (m: number) => (medians.length < 2 ? 2 : Math.min(4, Math.floor((medians.indexOf(m) / (medians.length - 1)) * 4.999)));
    return features.map((f) => {
      const p = f.properties as Props;
      // The map file spells names its own way; the catalog name translates reliably.
      const name = names.region(regionNames.get(p.region_id) ?? p.name);
      const zone = p.zone && p.zone in ZONE_FILL ? p.zone : undefined;
      const median = priceOf(p);
      const fill = zone ? ZONE_FILL[zone] : median != null ? priceFill(step(median)) : NO_DATA;
      return { ...p, median, fromHistory: p.median == null && median != null, id: f.id, geometry: f.geometry as GeoPermissibleObjects, name, fill, zone };
    });
  }, [query.data, recent.data, names, regionNames]);

  const features: MapFeature[] = provinces.map((p) => {
    const rows: TooltipRow[] = [
      { key: "median", label: p.fromHistory ? `${perUnit} · ${t("map.recent")}` : perUnit, value: p.median != null ? formatSom(p.median) : "—" },
      { key: "offers", label: t("map.offersLabel"), value: formatSom(p.offers ?? 0) },
    ];
    const notes = [p.zone ? `${tz(`zone.${p.zone}.label`)} · ${tz(`zone.${p.zone}.hint`)}` : tz("zone.none")];
    if (p.fromHistory) notes.unshift(t("map.recentNote"));
    return { id: p.id, geometry: p.geometry, name: p.name, fill: p.fill, rows, notes };
  });
  const sorted = [...provinces].sort((a, b) => (a.median ?? Infinity) - (b.median ?? Infinity) || a.name.localeCompare(b.name));
  const current = provinces.find((p) => p.id === selected);
  const zones = (["surplus", "balanced", "shortage"] as const).filter((z) => provinces.some((p) => p.zone === z));
  const priced = provinces.some((p) => !p.zone && p.median != null);
  const empty = provinces.some((p) => !p.zone && p.median == null);

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <Picker
          items={products.map((p) => ({ value: p.slug, label: names.product(p.slug, p.name), alias: p.name, hint: categoryName.get(p.category) }))}
          selected={[product]}
          onPick={(v) => {
            setProduct(v);
            setSelected(null);
          }}
          triggerLabel={t("map.chooseProduct")}
          placeholder={t("compare.searchProduct")}
          triggerClassName="flex h-11 w-full items-center gap-2 rounded-full border bg-card px-4 text-sm transition-colors outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50 data-[popup-open]:bg-muted sm:w-72"
          trigger={
            <>
              <Package className="size-4 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1 truncate text-left font-medium">{productName}</span>
              <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
            </>
          }
        />
        <Segmented
          label={t("trade")}
          className="w-full sm:w-64"
          value={trade}
          onChange={setTrade}
          options={[
            { value: "ulgurji", label: tc("wholesale") },
            { value: "chakana", label: tc("retail") },
          ]}
        />
      </div>

      {query.isError && !query.data ? (
        <EmptyState icon={<MapPinned strokeWidth={1.5} />} title={errors.message(query.error)} />
      ) : (
        <div className="grid gap-4 lg:grid-cols-[1fr_20rem]">
          <section className={cn("min-w-0 rounded-3xl border bg-card p-4 transition-opacity sm:p-6", query.isFetching && "opacity-60")} aria-busy={query.isFetching}>
            <h2 className="font-semibold tracking-tight">{t("map.heading", { product: productName })}</h2>
            <div className="mt-4">
              {provinces.length === 0 ? (
                <Skeleton className="aspect-[16/9] w-full rounded-2xl" />
              ) : (
                <Choropleth features={features} selected={selected} onSelect={setSelected} ariaLabel={t("map.aria", { product: productName })} />
              )}
            </div>
            <ul className="mt-4 flex flex-wrap gap-x-4 gap-y-2 text-xs text-muted-foreground" aria-label={t("map.legend")}>
              {zones.map((z) => (
                <li key={z} className="flex items-center gap-1.5">
                  <Swatch fill={ZONE_FILL[z]} />
                  {tz(`zone.${z}.label`)}
                </li>
              ))}
              {priced && (
                <li className="flex items-center gap-1.5">
                  <span className="flex overflow-hidden rounded-sm border">
                    {PRICE_STEPS.map((_, i) => (
                      <span key={i} className="h-3 w-3" style={{ backgroundColor: priceFill(i) }} />
                    ))}
                  </span>
                  {t("map.byPrice")}
                </li>
              )}
              {empty && (
                <li className="flex items-center gap-1.5">
                  <Swatch fill={NO_DATA} />
                  {t("map.noData")}
                </li>
              )}
            </ul>
          </section>

          <aside className="rounded-3xl border bg-card p-5 sm:p-6" aria-live="polite">
            {current ? (
              <div>
                <p className="eyebrow">{productName}</p>
                <h2 className="display mt-2 text-2xl">{current.name}</h2>
                {current.median != null ? (
                  <p className="num mt-4 text-3xl font-semibold tracking-tight">
                    {formatSom(current.median)}
                    <span className="ml-1.5 font-sans text-sm font-normal text-muted-foreground">{perUnit}</span>
                  </p>
                ) : (
                  <p className="mt-4 text-sm text-muted-foreground">{t("map.fewPrices")}</p>
                )}
                {current.fromHistory && <p className="mt-1 text-xs text-muted-foreground">{t("map.recentNote")}</p>}
                <p className="mt-1 text-sm text-muted-foreground">{t("map.offers", { count: current.offers ?? 0 })}</p>
                <div className="mt-4">
                  <ZoneBadge zone={current.zone} withHint />
                </div>
                {current.pressure != null && (
                  <p className="num mt-2 text-xs text-muted-foreground">{t("map.pressure", { value: fmt.decimal(current.pressure, 2) })}</p>
                )}
                <div className="mt-6 space-y-1">
                  <PanelLink href={`/hudud/${current.region_id}`}>{t("map.toRegion")}</PanelLink>
                  <PanelLink href={`/mahsulot/${product}/tahlil?region=${current.region_id}`}>{t("map.toProduct")}</PanelLink>
                </div>
              </div>
            ) : (
              <div className="flex h-full flex-col justify-center py-6 text-center lg:py-0">
                <MapPinned className="mx-auto size-6 text-muted-foreground" strokeWidth={1.5} />
                <p className="mt-3 font-medium">{t("map.pickTitle")}</p>
                <p className="mt-1 text-sm text-muted-foreground">{t("map.pickText")}</p>
              </div>
            )}
          </aside>
        </div>
      )}

      {sorted.length > 0 && (
        <section className="rounded-3xl border bg-card p-5 sm:p-6">
          <h2 className="font-semibold tracking-tight">{t("map.tableTitle")}</h2>
          <p className="mt-1 text-sm text-muted-foreground">{t("map.tableHint")}</p>
          <div className="relative mt-4 overflow-x-auto overscroll-x-contain rounded-2xl border">
            <table className="w-full border-separate border-spacing-0 text-sm">
              <thead>
                <tr className="text-xs text-muted-foreground">
                  <th scope="col" className="border-b px-4 py-2.5 text-left font-medium">{t("map.col.region")}</th>
                  <th scope="col" className="border-b px-3 py-2.5 text-right font-medium">{t("map.col.median")}</th>
                  <th scope="col" className="border-b px-3 py-2.5 text-right font-medium">{t("map.col.offers")}</th>
                  <th scope="col" className="border-b px-3 py-2.5 text-left font-medium">{t("map.col.zone")}</th>
                  <th scope="col" className="border-b px-4 py-2.5 text-right font-medium">
                    <span className="sr-only">{t("map.col.links")}</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((p) => (
                  <tr key={p.id} className={cn(p.id === selected && "bg-muted/60")}>
                    <th scope="row" className="border-b px-4 py-2.5 text-left font-medium whitespace-nowrap">
                      <button type="button" onClick={() => setSelected(p.id)} className="flex items-center gap-2 rounded-sm text-left outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50">
                        <Swatch fill={p.fill} />
                        {p.name}
                      </button>
                    </th>
                    <td className="num border-b px-3 py-2.5 text-right whitespace-nowrap">{p.median != null ? (
                        <>
                          {formatSom(p.median)}
                          {p.fromHistory && <span className="ml-1 text-[11px] text-muted-foreground">({t("map.recentMark")})</span>}
                        </>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}</td>
                    <td className="num border-b px-3 py-2.5 text-right">{p.offers ?? 0}</td>
                    <td className="border-b px-3 py-2">
                      <ZoneBadge zone={p.zone} className="whitespace-nowrap" />
                    </td>
                    <td className="border-b px-4 py-2.5 text-right whitespace-nowrap">
                      <Link href={`/hudud/${p.region_id}`} className="text-sm font-medium underline-offset-4 hover:underline">
                        {t("map.statsShort")}
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="mt-3 text-xs text-muted-foreground">{t("map.note")}</p>
        </section>
      )}
    </div>
  );
}

function Swatch({ fill }: { fill: string }) {
  return <span aria-hidden className="size-3 shrink-0 rounded-sm border border-foreground/10" style={{ backgroundColor: fill }} />;
}

function PanelLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <Link href={href} className="-mx-2 flex items-center justify-between gap-2 rounded-xl px-2 py-2.5 text-sm font-medium transition-colors hover:bg-muted">
      {children}
      <ArrowRight className="size-4 text-muted-foreground" />
    </Link>
  );
}

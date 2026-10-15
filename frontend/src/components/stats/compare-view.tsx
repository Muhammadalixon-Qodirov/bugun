"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { ChartLine, Plus, X } from "lucide-react";
import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";

import { useCatalogNames, useShortRegion } from "@/components/catalog-names";
import { LineChart } from "@/components/charts/line-chart";
import { DemoBadge } from "@/components/demo-badge";
import { EmptyState } from "@/components/empty-state";
import { useLimits } from "@/components/limits";
import { Segmented } from "@/components/segmented";
import { Picker } from "@/components/stats/picker";
import { Signed } from "@/components/stats/region";
import { Skeleton } from "@/components/ui/skeleton";
import { compareProductsOptions } from "@/lib/api/gen/@tanstack/react-query.gen";
import { useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { PERIODS, type Period, type Trade } from "@/lib/stats";
import { cn } from "@/lib/utils";

type ProductOption = { slug: string; name: string; category: string; base_unit: string };
type State = { products: string[]; regions: number[]; trade: Trade; period: Period };

const COLORS = ["var(--viz-1)", "var(--viz-2)", "var(--viz-3)", "var(--viz-4)"];
const pillClass =
  "flex h-10 items-center gap-1.5 rounded-full border border-dashed px-4 text-sm font-medium transition-colors outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-40 data-[popup-open]:bg-muted";

/**
 * Up to four price lines (products × provinces) on one chart, in so'm or as an
 * index from 100. The choice lives in the URL so a comparison can be shared.
 */
export function CompareView({
  products,
  provinces,
  categories,
  initial,
}: {
  products: ProductOption[];
  provinces: { id: number; name: string }[];
  categories: { slug: string; name: string }[];
  initial: State;
}) {
  const t = useTranslations("stats");
  const tc = useTranslations("common");
  const tcodes = useTranslations("codes");
  const names = useCatalogNames();
  const shortRegion = useShortRegion();
  const fmt = useFormat();
  const errors = useApiErrors();
  const [state, setState] = useState(initial);
  const [mode, setMode] = useState<"som" | "index">("som");
  // Lines on one chart (products × provinces), as the API allows.
  const maxLines = useLimits().compare_max_lines;

  // Mirror the choice in the address bar without a server round trip.
  useEffect(() => {
    // Built by hand so the lists keep plain commas: ?products=olma,nok&regions=1,110
    const qs = [
      state.products.length && `products=${state.products.map(encodeURIComponent).join(",")}`,
      state.regions.length && `regions=${state.regions.join(",")}`,
      state.trade !== "ulgurji" && `trade=${state.trade}`,
      state.period !== "90d" && `period=${state.period}`,
    ].filter(Boolean);
    const next = `${window.location.pathname}${qs.length ? `?${qs.join("&")}` : ""}`;
    if (next !== `${window.location.pathname}${window.location.search}`) window.history.replaceState(null, "", next);
  }, [state]);

  const lines = state.products.length * Math.max(1, state.regions.length);
  const query = useQuery({
    ...compareProductsOptions({
      query: {
        products: state.products.join(","),
        ...(state.regions.length ? { regions: state.regions.join(",") } : {}),
        trade: state.trade,
        period: state.period,
      },
    }),
    enabled: state.products.length > 0 && lines <= maxLines,
    placeholderData: keepPreviousData,
  });

  const bySlug = new Map(products.map((p) => [p.slug, p]));
  const categoryName = new Map(categories.map((c) => [c.slug, names.category(c.slug, c.name)]));
  const productItems = products.map((p) => ({ value: p.slug, label: names.product(p.slug, p.name), alias: p.name, hint: categoryName.get(p.category) }));
  const provinceItems = provinces.map((p) => ({ value: String(p.id), label: names.region(p.name), alias: p.name }));
  const provinceName = (id: number) => {
    const p = provinces.find((r) => r.id === id);
    return p ? names.region(p.name) : String(id);
  };
  const canAddProduct = (state.products.length + 1) * Math.max(1, state.regions.length) <= maxLines;
  const canAddRegion = state.products.length * (state.regions.length + 1) <= maxLines && state.products.length > 0;
  const update = (patch: Partial<State>) => setState((s) => ({ ...s, ...patch }));

  const data = query.data;
  const weekly = data?.bucket === "week";
  const at = (d: string) => Date.parse(`${d}T12:00:00Z`);
  const series = (data?.lines ?? []).map((l, i) => {
    const product = names.product(l.product.slug, l.product.name);
    const where = l.province_name ? shortRegion(l.province_id, names.region(l.province_name)) : tc("wholeCountry");
    return {
      id: `${l.product.slug}-${l.province_id ?? 0}`,
      label: `${product} · ${where}`,
      color: COLORS[i % COLORS.length],
      unit: l.product.unit,
      line: l,
      points: l.points.map((p) => ({ x: at(p.date), y: mode === "index" ? p.index : p.median })),
    };
  });
  const xFormat = (ms: number) => {
    if (!weekly) return fmt.dayMonth(new Date(ms).toISOString());
    const d = new Date(ms);
    return d.getUTCMonth() === 0 ? String(d.getUTCFullYear()) : fmt.monthShort(d.getUTCMonth() + 1);
  };
  const titleFormat = (ms: number) => {
    const iso = new Date(ms).toISOString();
    return weekly ? t("region.weekOf", { date: fmt.date(iso) }) : fmt.date(iso);
  };

  return (
    <div className="space-y-6">
      <section className="space-y-5 rounded-3xl border bg-card p-5 sm:p-6">
        <div>
          <p className="text-sm font-medium">{t("compare.products")}</p>
          <div className="mt-2 flex flex-wrap gap-2">
            {state.products.map((slug) => (
              <Chip
                key={slug}
                label={names.product(slug, bySlug.get(slug)?.name ?? slug)}
                removeLabel={t("compare.remove", { name: names.product(slug, bySlug.get(slug)?.name ?? slug) })}
                onRemove={() => update({ products: state.products.filter((s) => s !== slug) })}
              />
            ))}
            <Picker
              items={productItems}
              selected={state.products}
              onPick={(v) => !state.products.includes(v) && canAddProduct && update({ products: [...state.products, v] })}
              triggerLabel={t("compare.addProduct")}
              placeholder={t("compare.searchProduct")}
              triggerClassName={pillClass}
              triggerDisabled={!canAddProduct}
              trigger={
                <>
                  <Plus className="size-4" /> {t("compare.addProduct")}
                </>
              }
            />
          </div>
        </div>
        <div>
          <p className="text-sm font-medium">{t("compare.regions")}</p>
          <div className="mt-2 flex flex-wrap gap-2">
            {state.regions.length === 0 && <span className="flex h-10 items-center rounded-full bg-muted px-4 text-sm">{tc("wholeCountry")}</span>}
            {state.regions.map((id) => (
              <Chip
                key={id}
                label={provinceName(id)}
                removeLabel={t("compare.remove", { name: provinceName(id) })}
                onRemove={() => update({ regions: state.regions.filter((r) => r !== id) })}
              />
            ))}
            <Picker
              items={provinceItems}
              selected={state.regions.map(String)}
              onPick={(v) => !state.regions.includes(Number(v)) && canAddRegion && update({ regions: [...state.regions, Number(v)] })}
              triggerLabel={t("compare.addRegion")}
              placeholder={t("compare.searchRegion")}
              triggerClassName={pillClass}
              triggerDisabled={!canAddRegion}
              trigger={
                <>
                  <Plus className="size-4" /> {t("compare.addRegion")}
                </>
              }
            />
          </div>
        </div>
        <p className={cn("text-xs", lines > maxLines ? "text-destructive" : "text-muted-foreground")}>
          {t("compare.limit", { count: lines, max: maxLines })}
        </p>
        <div className="grid gap-2 sm:grid-cols-3">
          <Segmented
            label={t("trade")}
            value={state.trade}
            onChange={(v) => update({ trade: v })}
            options={[
              { value: "ulgurji", label: tc("wholesale") },
              { value: "chakana", label: tc("retail") },
            ]}
          />
          <Segmented label={t("compare.period")} value={state.period} onChange={(v) => update({ period: v })} options={PERIODS.map((p) => ({ value: p, label: tcodes(`period.${p}`) }))} />
          <Segmented
            label={t("compare.mode")}
            value={mode}
            onChange={setMode}
            options={[
              { value: "som", label: t("compare.som") },
              { value: "index", label: t("compare.index") },
            ]}
          />
        </div>
      </section>

      {state.products.length === 0 ? (
        <EmptyState icon={<ChartLine strokeWidth={1.5} />} title={t("compare.emptyTitle")} description={t("compare.emptyText")} />
      ) : lines > maxLines ? (
        <EmptyState icon={<ChartLine strokeWidth={1.5} />} title={t("compare.tooMany", { max: maxLines })} description={t("compare.tooManyText", { max: maxLines })} />
      ) : query.isError && !data ? (
        <EmptyState icon={<ChartLine strokeWidth={1.5} />} title={errors.message(query.error)} />
      ) : !data ? (
        <div className="rounded-3xl border bg-card p-5 sm:p-6">
          <Skeleton className="h-72 w-full rounded-2xl" />
        </div>
      ) : (
        <section className={cn("rounded-3xl border bg-card p-5 transition-opacity sm:p-6", query.isFetching && "opacity-60")} aria-busy={query.isFetching}>
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <h2 className="flex flex-wrap items-center gap-2 font-semibold tracking-tight">
              {mode === "index" ? t("compare.indexTitle") : t("compare.somTitle")}
              <DemoBadge share={data.demo_share} />
            </h2>
            <p className="text-xs text-muted-foreground">{weekly ? t("compare.weekly") : t("compare.daily")}</p>
          </div>
          <ul className="mt-3 flex flex-wrap gap-x-5 gap-y-1.5 text-sm" aria-label={t("compare.legend")}>
            {series.map((s) => (
              <li key={s.id} className="flex items-center gap-2">
                <span aria-hidden className="h-0.5 w-4 rounded-full" style={{ backgroundColor: s.color }} />
                <span className="text-muted-foreground">{s.label}</span>
              </li>
            ))}
          </ul>
          <div className="mt-4">
            {series.every((s) => s.points.length === 0) ? (
              <p className="py-16 text-center text-sm text-muted-foreground">{t("compare.noData")}</p>
            ) : (
              <LineChart
                series={series}
                height={300}
                yFormat={(v) => (mode === "index" ? fmt.decimal(v, 0) : formatSom(v))}
                xFormat={xFormat}
                titleFormat={titleFormat}
                endLabel={false}
                ariaLabel={t("compare.aria", { lines: series.map((s) => s.label).join(", ") })}
              />
            )}
          </div>
          <ul className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {series.map((s) => (
              <li key={s.id} className="rounded-2xl bg-muted/50 px-4 py-3">
                <p className="flex items-center gap-2 text-sm">
                  <span aria-hidden className="h-0.5 w-4 shrink-0 rounded-full" style={{ backgroundColor: s.color }} />
                  <span className="truncate text-muted-foreground">{s.label}</span>
                  <DemoBadge compact share={s.line.demo_share} className="ml-auto" />
                </p>
                {s.line.now != null ? (
                  <>
                    <p className="num mt-1.5 text-xl font-semibold tracking-tight">
                      {formatSom(s.line.now)}
                      <span className="ml-1 font-sans text-xs font-normal text-muted-foreground">{tc("perUnit", { unit: fmt.unit(s.unit) })}</span>
                    </p>
                    <p className="mt-0.5 flex items-center gap-1.5 text-xs text-muted-foreground">
                      <Signed value={s.line.change} />
                      {t("compare.overPeriod", { period: tcodes(`period.${state.period}`) })}
                    </p>
                  </>
                ) : (
                  <p className="mt-1.5 text-sm text-muted-foreground">{t("compare.noLine")}</p>
                )}
              </li>
            ))}
          </ul>
        </section>
      )}
      <p className="text-xs text-muted-foreground">{t("compare.note")}</p>
    </div>
  );
}

function Chip({ label, removeLabel, onRemove }: { label: string; removeLabel: string; onRemove: () => void }) {
  return (
    <span className="flex h-10 max-w-full items-center gap-1 rounded-full border bg-card pr-1 pl-4 text-sm font-medium">
      <span className="truncate">{label}</span>
      <button
        type="button"
        onClick={onRemove}
        aria-label={removeLabel}
        className="grid size-8 shrink-0 place-items-center rounded-full text-muted-foreground transition-colors outline-none hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <X className="size-4" />
      </button>
    </span>
  );
}

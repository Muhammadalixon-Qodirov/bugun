"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { memo, useCallback, useMemo, useRef, useState } from "react";

import { useCatalogNames, useShortRegion } from "@/components/catalog-names";
import { ChartTooltip } from "@/components/charts/tooltip";
import type { HeatCell, Heatmap } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { DIFF_BOUNDS, DIFF_STEPS, diffFill, diffStep } from "@/lib/stats";

type Spot = NonNullable<HeatCell["spot"]>;

/** Local Moran's I spots: a filled triangle for a cluster, an outline one for an outlier; up = dear, down = cheap. */
const SPOT_GLYPH: Record<Spot, string> = { hot: "▲", cold: "▼", high_outlier: "△", low_outlier: "▽" };

type Active = { row: number; col: number; x: number; y: number; width: number };
type Province = { id: number; name: string };

/**
 * Products down, provinces across. Each cell is the 14-day median coloured by
 * how far it is from the country's, and links to the product's analytics for
 * that province. The first column stays put while the table scrolls sideways.
 */
export function HeatTable({ data }: { data: Heatmap }) {
  const t = useTranslations("stats");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const shortRegion = useShortRegion();
  const fmt = useFormat();
  const wrap = useRef<HTMLDivElement>(null);
  const scrolledAt = useRef(0);
  const [active, setActive] = useState<Active | null>(null);

  const provinces = useMemo(() => data.provinces.map((p) => ({ id: p.id, name: names.region(p.name) })), [data.provinces, names]);
  // Cells slide under a still cursor while the table scrolls; those are not hovers.
  const show = useCallback((row: number, col: number, el: HTMLElement, pointer: boolean) => {
    if (pointer && performance.now() - scrolledAt.current < 150) return;
    const box = wrap.current?.getBoundingClientRect();
    const cell = el.getBoundingClientRect();
    if (!box) return;
    setActive({ row, col, x: cell.left - box.left + cell.width / 2, y: cell.bottom - box.top + 4, width: box.width });
  }, []);
  const hide = useCallback(() => setActive(null), []);

  const current = active ? data.rows[active.row] : null;
  const cell = current && active ? (current.cells[active.col] as HeatCell | null) : null;
  const tooltip =
    current && cell && active
      ? {
          title: `${provinces[active.col].name} · ${names.product(current.product.slug, current.product.name)}`,
          rows: [
            { key: "median", label: tc("perUnit", { unit: fmt.unit(current.product.unit) }), value: formatSom(cell.median) },
            { key: "offers", label: t("heat.offersLabel"), value: formatSom(cell.offers) },
            ...(cell.diff != null ? [{ key: "diff", label: t("heat.vsCountry"), value: fmt.percent(cell.diff, { signed: true }) }] : []),
          ],
          notes: cell.spot ? [`${SPOT_GLYPH[cell.spot]} ${t(`spot.${cell.spot}`)} (${t("spot.legend")})`] : [],
        }
      : null;

  return (
    <div ref={wrap} className="relative">
      <div
        className="relative max-h-[min(44rem,75vh)] overflow-auto overscroll-x-contain rounded-2xl border bg-card"
        data-lenis-prevent
        onScroll={() => {
          scrolledAt.current = performance.now();
          setActive(null);
        }}
      >
        <table className="w-full border-separate border-spacing-0 text-sm">
          <caption className="sr-only">{t("heat.caption")}</caption>
          <thead>
            <tr>
              <th scope="col" className="sticky top-0 left-0 z-30 w-28 border-b bg-card px-3 py-2.5 text-left text-xs font-medium text-muted-foreground sm:w-44">
                {t("heat.product")}
              </th>
              <th scope="col" className="sticky top-0 z-20 border-b bg-card px-2 py-2.5 text-right text-xs font-medium text-muted-foreground">
                {t("heat.country")}
              </th>
              {provinces.map((p) => (
                <th key={p.id} scope="col" title={p.name} className="sticky top-0 z-20 border-b bg-card px-1 py-2.5 text-center text-xs font-medium text-muted-foreground">
                  <Link href={`/hudud/${p.id}`} className="block rounded-md px-1 leading-tight whitespace-nowrap hover:text-foreground">
                    {shortRegion(p.id, p.name)}
                  </Link>
                </th>
              ))}
            </tr>
          </thead>
          <HeatRows data={data} provinces={provinces} onShow={show} onHide={hide} />
        </table>
      </div>
      {tooltip && active && (
        <ChartTooltip x={active.x} y={active.y} width={active.width} title={tooltip.title} rows={tooltip.rows} notes={tooltip.notes} />
      )}
    </div>
  );
}

/**
 * The table body, apart so that moving the tooltip from cell to cell does not
 * re-render some fifteen hundred cells.
 */
const HeatRows = memo(function HeatRows({
  data,
  provinces,
  onShow,
  onHide,
}: {
  data: Heatmap;
  provinces: Province[];
  onShow: (row: number, col: number, el: HTMLElement, pointer: boolean) => void;
  onHide: () => void;
}) {
  const t = useTranslations("stats");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const fmt = useFormat();
  return (
    <tbody>
      {data.rows.map((r, ri) => {
        const product = names.product(r.product.slug, r.product.name);
        return (
          <tr key={r.product.id}>
            <th scope="row" className="sticky left-0 z-10 max-w-28 border-b bg-card px-3 py-1.5 text-left font-normal sm:max-w-44">
              <Link href={`/mahsulot/${r.product.slug}/tahlil`} className="block truncate font-medium hover:underline">
                {product}
              </Link>
              <span className="block truncate text-xs text-muted-foreground">{tc("perUnit", { unit: fmt.unit(r.product.unit) })}</span>
            </th>
            <td className="num border-b px-2 py-1.5 text-right font-medium whitespace-nowrap">{formatSom(r.country)}</td>
            {provinces.map((p, ci) => {
              const c = r.cells[ci] as HeatCell | null;
              if (!c) {
                return (
                  <td key={p.id} className="border-b p-0.5">
                    <span className="grid h-10 min-w-16 place-items-center rounded-lg bg-muted/50 text-muted-foreground" aria-label={t("heat.noData", { region: p.name })}>
                      —
                    </span>
                  </td>
                );
              }
              const label = [
                `${p.name}, ${product}: ${formatSom(c.median)} ${tc("perUnit", { unit: fmt.unit(r.product.unit) })}`,
                t("heat.offers", { count: c.offers }),
                c.diff != null ? `${t("heat.vsCountry")} ${fmt.percent(c.diff, { signed: true })}` : "",
                c.spot ? t(`spot.${c.spot}`) : "",
              ]
                .filter(Boolean)
                .join(", ");
              return (
                <td key={p.id} className="border-b p-0.5">
                  <Link
                    href={`/mahsulot/${r.product.slug}/tahlil?region=${p.id}`}
                    aria-label={label}
                    onPointerEnter={(e) => onShow(ri, ci, e.currentTarget, true)}
                    onPointerLeave={(e) => e.pointerType === "mouse" && onHide()}
                    onFocus={(e) => onShow(ri, ci, e.currentTarget, false)}
                    onBlur={onHide}
                    style={{ backgroundColor: c.diff != null ? diffFill(diffStep(c.diff)) : "var(--viz-mid)" }}
                    className="num relative grid h-10 min-w-16 place-items-center rounded-lg px-1.5 text-[13px] text-foreground outline-none hover:ring-2 hover:ring-foreground focus-visible:ring-2 focus-visible:ring-foreground"
                  >
                    {formatSom(c.median)}
                    {c.spot && (
                      <span aria-hidden className="absolute top-0.5 right-1 text-[10px] leading-none">
                        {SPOT_GLYPH[c.spot]}
                      </span>
                    )}
                  </Link>
                </td>
              );
            })}
          </tr>
        );
      })}
    </tbody>
  );
});

/** The colour key: seven steps from much cheaper to much dearer, plus the "too few prices" cell. */
export function DiffLegend({ spots = false }: { spots?: boolean }) {
  const t = useTranslations("stats");
  const fmt = useFormat();
  const pct = (v: number) => fmt.percent(v, { digits: 0 });
  const range = (s: number) => {
    const a = Math.abs(s);
    if (a === 0) return `±${pct(DIFF_BOUNDS[0])}`;
    const sign = s < 0 ? "−" : "+";
    return a === 3 ? `${sign}${pct(DIFF_BOUNDS[2])}+` : `${sign}${pct(DIFF_BOUNDS[a - 1])}…${pct(DIFF_BOUNDS[a])}`;
  };
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-muted-foreground">
      <span className="flex items-center gap-2">
        <span>{t("legend.cheaper")}</span>
        <span className="flex overflow-hidden rounded-md">
          {DIFF_STEPS.map((s) => (
            <span key={s} title={range(s)} className="h-3 w-6 sm:w-8" style={{ backgroundColor: diffFill(s) }} />
          ))}
        </span>
        <span>{t("legend.dearer")}</span>
      </span>
      <span className="num">
        {range(-3)} · {range(0)} · {range(3)}
      </span>
      <span className="flex items-center gap-1.5">
        <span className="grid h-3 w-6 place-items-center rounded-sm bg-muted/50 text-[9px] leading-none">—</span>
        {t("legend.few")}
      </span>
      {spots && (
        <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <span className="flex items-center gap-1">
            <span aria-hidden className="text-foreground">{SPOT_GLYPH.hot}</span>
            {t("spot.short.hot")}
          </span>
          <span className="flex items-center gap-1">
            <span aria-hidden className="text-foreground">{SPOT_GLYPH.cold}</span>
            {t("spot.short.cold")}
          </span>
          <span className="flex items-center gap-1">
            <span aria-hidden className="text-foreground">
              {SPOT_GLYPH.high_outlier}
              {SPOT_GLYPH.low_outlier}
            </span>
            {t("spot.short.outlier")}
          </span>
          <span>({t("spot.legend")})</span>
        </span>
      )}
    </div>
  );
}

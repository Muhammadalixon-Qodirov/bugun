"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";

import { useCatalogNames } from "@/components/catalog-names";
import { type ChartDot, type ChartMark, LineChart, type LineSeries } from "@/components/charts/line-chart";
import { DemoBadge, demoPercent } from "@/components/demo-badge";
import type { AnalyticsOutputBody, AnomalyRef } from "@/lib/api/gen/types.gen";
import { holidayKey } from "@/lib/i18n/holidays";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

const day = (d: string) => Date.parse(`${d}T00:00:00Z`);

type Data = Pick<AnalyticsOutputBody, "series" | "forecast" | "events" | "jumps" | "scope" | "product" | "anomalies">;

type Unusual = {
  date: string;
  province: string;
  value: number;
  reference: number;
  diff: number;
  excluded: boolean;
  layers: AnomalyRef["layer"][];
  /** Share of the day's prices that are demo data. */
  demoShare: number;
};

/** One entry per viloyat-day: the two layers (own past, other viloyatlar) of the same day read as one. */
function unusualDays(anomalies: AnomalyRef[]): Unusual[] {
  const byKey = new Map<string, Unusual>();
  for (const a of anomalies) {
    const key = `${a.date}-${a.province_id}`;
    const u = byKey.get(key);
    if (u) {
      u.layers.push(a.layer);
      u.excluded ||= a.excluded;
      u.demoShare = Math.max(u.demoShare, a.demo_share);
      if (a.layer === "temporal") Object.assign(u, { reference: a.reference, diff: a.diff });
    } else {
      byKey.set(key, { date: a.date, province: a.province_name, value: a.value, reference: a.reference, diff: a.diff, excluded: a.excluded, layers: [a.layer], demoShare: a.demo_share });
    }
  }
  return [...byKey.values()].sort((a, b) => b.date.localeCompare(a.date));
}

/** Price history with its middle half, the forecast after it and the events along it; or the same as a table. */
export function PriceHistory({ data, productName }: { data: Data; productName: string }) {
  const t = useTranslations("analytics");
  const tc = useTranslations("codes");
  const fmt = useFormat();
  const names = useCatalogNames();
  const [table, setTable] = useState(false);

  const weekly = data.scope.bucket === "week";
  const span = data.series.length ? day(data.series[data.series.length - 1].date) - day(data.series[0].date) : 0;
  const long = span > 120 * 86_400_000;
  const multiYear = span > 400 * 86_400_000;

  const eventLabel = (e: { title: string; kind: string }) => {
    const key = e.kind === "holiday" ? holidayKey(e.title) : undefined;
    const title = key ? t(`holidays.${key}` as Parameters<typeof t>[0]) : e.title;
    return `${tc(`eventKind.${e.kind}` as Parameters<typeof tc>[0])}: ${title}`;
  };

  // Older API builds send no anomalies.
  const unusual = unusualDays(data.anomalies ?? []);
  // A day left out of a daily series is a gap in the line, not a bridge over it.
  const first = data.series[0]?.date;
  const lastDate = data.series[data.series.length - 1]?.date;
  const gaps = weekly || !first ? [] : [...new Set(unusual.filter((u) => u.excluded && u.date > first && u.date < lastDate).map((u) => u.date))];
  const rows = [
    ...data.series.map((p) => ({ x: day(p.date), y: p.median as number | null, lo: p.p25 as number | null, hi: p.p75 as number | null })),
    ...gaps.filter((d) => !data.series.some((p) => p.date === d)).map((d) => ({ x: day(d), y: null, lo: null, hi: null })),
  ].sort((a, b) => a.x - b.x);

  const series: LineSeries[] = [];
  const main: LineSeries = {
    id: "median",
    label: t("chart.median"),
    color: "var(--primary)",
    points: rows.map((r) => ({ x: r.x, y: r.y })),
    band: rows.map((r) => ({ x: r.x, lo: r.lo, hi: r.hi })),
  };
  series.push(main);
  const last = data.series[data.series.length - 1];
  if (data.forecast && last) {
    const start = { x: day(last.date), y: last.median };
    series.push({
      id: "forecast",
      label: t("chart.forecast"),
      color: "var(--primary)",
      dashed: true,
      points: [start, ...data.forecast.weeks.map((w) => ({ x: day(w.week), y: w.median }))],
      band: [{ x: start.x, lo: last.median, hi: last.median }, ...data.forecast.weeks.map((w) => ({ x: day(w.week), lo: w.low, hi: w.high }))],
    });
  }

  const unusualText = (u: Unusual) =>
    [
      t("anomalies.mark", { region: names.region(u.province), value: formatSom(u.value), reference: formatSom(u.reference) }),
      ...u.layers.map((l) => t(`anomalies.layer.${l}`)),
      u.excluded ? t("anomalies.excluded") : t("anomalies.checking"),
      u.demoShare > 0 && tc("demoShareLabel", { share: demoPercent(u.demoShare) }),
    ]
      .filter(Boolean)
      .join(" · ");
  // On weekly charts a day sits on its week's point, where the tooltip can reach it.
  const bucketOf = (d: string) => {
    if (!weekly) return day(d);
    const at = [...data.series].reverse().find((p) => p.date <= d);
    return day(at?.date ?? d);
  };

  const marks: ChartMark[] = [
    ...data.events.map((e) => ({ id: `e${e.id}-${e.starts_on}`, from: day(e.starts_on), to: e.ends_on ? day(e.ends_on) : undefined, label: eventLabel(e) })),
    ...data.jumps.map((j) => ({ id: `j${j.week}`, from: day(j.week), label: `${fmt.percent(j.change, { signed: true })}` })),
    ...unusual.map((u) => ({ id: `a${u.date}-${u.province}`, from: bucketOf(u.date), label: unusualText(u) })),
  ];
  const eventMarks = data.events.length + data.jumps.length > 0;
  // One glyph per date: ✕ wins when any viloyat's day was left out.
  const dotAt = new Map<number, ChartDot>();
  for (const u of unusual) {
    const x = bucketOf(u.date);
    if (!dotAt.get(x) || u.excluded) dotAt.set(x, { id: `a${x}`, x, kind: u.excluded ? "excluded" : "checking" });
  }
  const dots = [...dotAt.values()];
  const leftOut = unusual.filter((u) => u.excluded).length;

  const xFormat = (x: number) => {
    const d = new Date(x);
    if (!long) return fmt.dayMonth(d.toISOString());
    const m = fmt.monthShort(d.getUTCMonth() + 1);
    return multiYear ? `${m} ${String(d.getUTCFullYear()).slice(2)}` : m;
  };
  const titleFormat = (x: number) => {
    const iso = new Date(x).toISOString();
    return weekly ? t("jumps.week", { date: fmt.dayMonth(iso) }) : fmt.date(iso);
  };

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
          <span className="flex items-center gap-1.5">
            <span aria-hidden className="h-0.5 w-4 rounded-full bg-primary" />
            {t("chart.median")}
          </span>
          <span className="flex items-center gap-1.5">
            <span aria-hidden className="h-2.5 w-4 rounded-sm bg-primary/15" />
            {t("chart.band")}
          </span>
          {data.forecast && (
            <span className="flex items-center gap-1.5">
              <span aria-hidden className="h-0.5 w-4 rounded-full [background:repeating-linear-gradient(90deg,var(--primary)_0_4px,transparent_4px_7px)]" />
              {t("chart.forecast")}
            </span>
          )}
          {eventMarks && (
            <span className="flex items-center gap-1.5">
              <span aria-hidden className="h-2.5 w-4 rounded-sm bg-foreground/10" />
              {t("chart.events")}
            </span>
          )}
          {leftOut > 0 && (
            <span className="flex items-center gap-1.5">
              <span aria-hidden className="font-semibold text-muted-foreground">✕</span>
              {t("anomalies.legendExcluded")}
            </span>
          )}
          {unusual.length > leftOut && (
            <span className="flex items-center gap-1.5">
              <span aria-hidden className="grid size-3.5 place-items-center rounded-full border-2 border-warm text-[8px] font-bold text-warm-foreground">?</span>
              {t("anomalies.legendChecking")}
            </span>
          )}
        </div>
        <div className="flex rounded-full bg-muted p-0.5 text-xs" role="group">
          {[false, true].map((v) => (
            <button
              key={String(v)}
              type="button"
              aria-pressed={table === v}
              onClick={() => setTable(v)}
              className={cn("rounded-full px-3 py-1 transition-colors", table === v ? "bg-card font-medium shadow-sm dark:bg-foreground/15" : "text-muted-foreground hover:text-foreground")}
            >
              {v ? t("chart.tableToggle") : t("chart.chartToggle")}
            </button>
          ))}
        </div>
      </div>

      {table ? (
        <div className="mt-4 max-h-80 overflow-auto rounded-2xl border">
          <table className="w-full text-sm">
            <thead className="sticky top-0 bg-card text-left text-xs text-muted-foreground">
              <tr>
                <th className="px-3 py-2 font-medium">{t("chart.date")}</th>
                <th className="px-3 py-2 text-right font-medium">{t("chart.median")}</th>
                <th className="px-3 py-2 text-right font-medium">p25–p75</th>
                <th className="px-3 py-2 text-right font-medium">{t("chart.offers")}</th>
              </tr>
            </thead>
            <tbody className="num">
              {[...data.series].reverse().map((p) => (
                <tr key={p.date} className="border-t">
                  <td className="px-3 py-1.5">{titleFormat(day(p.date))}</td>
                  <td className="px-3 py-1.5 text-right font-medium">{formatSom(p.median)}</td>
                  <td className="px-3 py-1.5 text-right text-muted-foreground">
                    {formatSom(p.p25)}–{formatSom(p.p75)}
                  </td>
                  <td className="px-3 py-1.5 text-right text-muted-foreground">{p.offers}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="mt-4">
          <LineChart
            series={series}
            height={300}
            yFormat={formatSom}
            xFormat={xFormat}
            titleFormat={titleFormat}
            marks={marks}
            dots={dots}
            ariaLabel={t("chart.aria", { name: productName })}
          />
        </div>
      )}
      {leftOut > 0 && <p className="mt-2 text-xs text-muted-foreground">{t("anomalies.leftOut", { count: leftOut })}</p>}

      {unusual.length > 0 && (
        <div className="mt-4 rounded-2xl bg-muted/50 px-4 py-3 text-xs">
          <p className="font-medium">{t("anomalies.title", { count: unusual.length })}</p>
          <ul className="mt-2 space-y-1.5">
            {unusual.map((u) => (
              <li key={`${u.date}-${u.province}`} className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                <time dateTime={u.date} className="num font-medium">
                  {fmt.dayMonth(u.date)}
                </time>
                <span>{names.region(u.province)}</span>
                <span className="num">{t("anomalies.value", { value: formatSom(u.value), reference: formatSom(u.reference) })}</span>
                <span className={cn("num font-medium", u.diff < 0 ? "text-primary" : "text-warm-foreground")}>{fmt.percent(u.diff, { signed: true })}</span>
                <span className="text-muted-foreground">
                  {[...u.layers.map((l) => t(`anomalies.layer.${l}`)), u.excluded ? t("anomalies.excluded") : t("anomalies.checking")].join(" · ")}
                </span>
                <DemoBadge compact share={u.demoShare} className="self-center" />
              </li>
            ))}
          </ul>
          <p className="mt-2 text-muted-foreground">{t("anomalies.note")}</p>
        </div>
      )}
    </div>
  );
}

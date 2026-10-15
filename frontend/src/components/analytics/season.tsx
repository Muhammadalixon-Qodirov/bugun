"use client";

import { useTranslations } from "next-intl";

import { ColumnChart } from "@/components/charts/column-chart";
import type { Seasonality, Weekdays } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";

const tone = (v: number) => (v > 1.005 ? "var(--viz-dear)" : v < 0.995 ? "var(--viz-cheap)" : "var(--viz-mid)");

/**
 * The usual price in each month against the year (1 = the yearly level), as
 * columns up (dearer) or down (cheaper) from that level, this month marked.
 */
export function SeasonChart({ season }: { season: Seasonality }) {
  const t = useTranslations("analytics");
  const fmt = useFormat();
  const thisMonth = Number(new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Tashkent", month: "numeric" }).format(new Date()));
  return (
    <div>
      <ColumnChart
        height={200}
        baseline={1}
        ariaLabel={t("season.aria")}
        series={[{ id: "index", label: t("season.title"), color: "var(--viz-mid)" }]}
        colorFor={(v) => tone(v)}
        yFormat={(v) => fmt.percent(v - 1, { signed: true, digits: 0 })}
        valueFormat={(v) => t("season.vsUsual", { share: fmt.percent(v - 1, { signed: true }) })}
        data={season.months.map((v, i) => ({
          key: String(i + 1),
          label: fmt.monthShort(i + 1),
          title: fmt.month(i + 1),
          values: [v],
          highlight: i + 1 === thisMonth,
        }))}
      />
      <p className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-sm text-muted-foreground">
        <span>{t("season.cheapest", { month: fmt.month(season.cheapest_month) })}</span>
        <span>{t("season.dearest", { month: fmt.month(season.dearest_month) })}</span>
        <span>{t("season.samples", { count: season.samples })}</span>
      </p>
    </div>
  );
}

/** Price by weekday; a flat week says so instead of drawing noise. */
export function WeekdayChart({ weekdays }: { weekdays: Weekdays }) {
  const t = useTranslations("analytics");
  const fmt = useFormat();
  if (weekdays.spread < 0.01) return <p className="text-sm text-muted-foreground">{t("season.noWeekday")}</p>;
  return (
    <div>
      <ColumnChart
        height={170}
        baseline={1}
        ariaLabel={t("season.weekdayAria")}
        series={[{ id: "index", label: t("season.weekdays"), color: "var(--viz-mid)" }]}
        colorFor={(v) => tone(v)}
        yFormat={(v) => fmt.percent(v - 1, { signed: true, digits: 1 })}
        valueFormat={(v) => t("season.vsUsual", { share: fmt.percent(v - 1, { signed: true }) })}
        data={weekdays.days.map((v, i) => ({
          key: String(i),
          label: fmt.weekdayShort(i),
          title: fmt.weekday(i),
          values: [v],
          highlight: i + 1 === weekdays.cheapest_day,
        }))}
      />
      <p className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-sm text-muted-foreground">
        <span>{t("season.cheapestDay", { day: fmt.weekday(weekdays.cheapest_day - 1) })}</span>
        <span>{t("season.dearestDay", { day: fmt.weekday(weekdays.dearest_day - 1) })}</span>
      </p>
    </div>
  );
}

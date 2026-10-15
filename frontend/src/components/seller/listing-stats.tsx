"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Lightbulb } from "lucide-react";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { ColumnChart } from "@/components/charts/column-chart";
import { Segmented } from "@/components/segmented";
import { Stat } from "@/components/stat";
import { Skeleton } from "@/components/ui/skeleton";
import { listingStatsOptions } from "@/lib/api/gen/@tanstack/react-query.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { formatAmount } from "@/lib/listings";

const series = [
  { id: "views", color: "var(--viz-1)" },
  { id: "contacts", color: "var(--viz-2)" },
] as const;

/** One listing's unique views, contacts and deals over 7 or 30 days, next to the province's average. */
export function ListingStats({ id }: { id: number }) {
  const t = useTranslations("sellerStats");
  const fmt = useFormat();
  const [days, setDays] = useState<"7" | "30">("7");
  const { data: s, isError } = useQuery({
    ...listingStatsOptions({ path: { id }, query: { days: days === "7" ? 7 : 30 } }),
    placeholderData: keepPreviousData,
  });

  const chartSeries = series.map((x) => ({ ...x, label: t(`series.${x.id}`) }));
  // Sellers whose rate is under half the market's are probably priced above what buyers expect.
  const lowResponse = s?.market && s.views > 0 && s.contact_rate < s.market.contact_rate / 2;

  return (
    <section aria-labelledby="stats-title" className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 id="stats-title" className="display text-2xl">
            {t("title")}
          </h2>
          <p className="mt-1 text-sm text-muted-foreground">{t("hint")}</p>
        </div>
        <Segmented
          label={t("period")}
          value={days}
          onChange={setDays}
          options={[
            { value: "7", label: t("days", { count: 7 }) },
            { value: "30", label: t("days", { count: 30 }) },
          ]}
        />
      </div>

      {isError ? (
        <p className="rounded-3xl border bg-card px-5 py-10 text-center text-sm text-muted-foreground">{t("failed")}</p>
      ) : !s ? (
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-[6.5rem] rounded-2xl" />
            ))}
          </div>
          <Skeleton className="h-72 rounded-3xl" />
        </div>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <Stat label={t("views")} value={formatSom(s.views)} hint={t("viewsHint")} />
            <Stat label={t("contacts")} value={formatSom(s.contacts)} hint={t("contactsHint")} />
            <Stat label={t("rate")} value={fmt.percent(s.contact_rate)} hint={t("rateHint")} />
            <Stat
              label={t("deals")}
              value={formatSom(s.deals_confirmed)}
              hint={[
                t("dealsReported", { count: s.deals_reported }),
                s.sold_quantity > 0 ? t("sold", { amount: formatAmount(s.sold_quantity, fmt.decimal), unit: fmt.unit(s.unit) }) : null,
              ]
                .filter(Boolean)
                .join(" · ")}
            />
          </div>

          <div className="rounded-3xl border bg-card p-5 sm:p-6">
            <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
              <h3 className="font-semibold tracking-tight">{t("chartTitle")}</h3>
              <ul className="flex gap-4 text-xs text-muted-foreground">
                {chartSeries.map((x) => (
                  <li key={x.id} className="flex items-center gap-1.5">
                    <span className="size-2.5 rounded-[3px]" style={{ background: x.color }} aria-hidden />
                    {x.label}
                  </li>
                ))}
              </ul>
            </div>
            <ColumnChart
              height={220}
              ariaLabel={t("chartLabel", { days: s.days })}
              series={chartSeries}
              // Counts are whole numbers: small maxima give fractional ticks, which stay unlabelled.
              integer
              yFormat={formatSom}
              valueFormat={formatSom}
              data={s.series.map((d) => ({
                key: d.date,
                label: fmt.dayMonth(d.date),
                title: fmt.date(d.date),
                values: [d.views, d.contacts],
              }))}
            />
          </div>

          <div className="rounded-3xl border bg-card p-5 sm:p-6">
            <h3 className="font-semibold tracking-tight">{t("marketTitle")}</h3>
            {s.market ? (
              <>
                <p className="mt-2 text-sm leading-relaxed">
                  {t.rich("marketCompare", {
                    mine: fmt.percent(s.contact_rate),
                    market: fmt.percent(s.market.contact_rate),
                    b: (c) => <span className="num font-semibold">{c}</span>,
                  })}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {t("marketBasis", {
                    count: s.market.listings,
                    views: fmt.decimal(s.market.views_per_listing, 1),
                    contacts: fmt.decimal(s.market.contacts_per_listing, 1),
                  })}
                </p>
                {lowResponse && (
                  <p className="mt-4 flex gap-2 rounded-2xl bg-warm-soft px-4 py-3 text-sm text-warm-foreground">
                    <Lightbulb className="mt-0.5 size-4 shrink-0" aria-hidden />
                    {t("lowResponse")}
                  </p>
                )}
              </>
            ) : (
              <p className="mt-2 text-sm text-muted-foreground">{t("noMarket")}</p>
            )}
          </div>
        </>
      )}
    </section>
  );
}

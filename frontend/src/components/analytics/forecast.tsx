"use client";

import { useTranslations } from "next-intl";

import { BarList } from "@/components/charts/bar-list";
import { DemoBadge } from "@/components/demo-badge";
import type { Forecast } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

const day = (d: string) => `${d}T00:00:00Z`;

/**
 * Why the forecast looks the way it does: which method won the past-year test
 * and by how much, then each forecast week's change split into its drivers'
 * exact Shapley shares (they add up to the week's change from last week).
 */
export function ForecastExplain({ forecast }: { forecast: Forecast }) {
  const t = useTranslations("analytics");
  const fmt = useFormat();
  const pct = (v: number) => fmt.percent(v);
  const methodName = (m: Forecast["method"]) => t(`forecast.method.${m}`);
  const naive = forecast.methods.find((m) => m.method === "naive");
  const chosen = forecast.methods.find((m) => m.method === forecast.method);
  const b = (c: React.ReactNode) => <b className="font-medium text-foreground">{c}</b>;

  // One scale for every week's bars so the weeks compare at a glance.
  const widest = Math.max(0.01, ...forecast.weeks.flatMap((w) => w.drivers.map((d) => Math.abs(d.share))));

  return (
    <div className="mt-4 space-y-5 border-t pt-4">
      <div>
        <div className="flex items-start justify-between gap-3">
          <h3 className="flex items-center gap-2 text-sm font-semibold tracking-tight">
            {t("forecast.title")}
            {forecast.method === "ml" && (
              <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[11px] font-semibold text-primary" data-testid="forecast-ml">
                {t("forecast.mlBadge")}
              </span>
            )}
          </h3>
          <DemoBadge compact share={forecast.demo_share} />
        </div>
        <p className="mt-1 text-sm text-muted-foreground">
          {forecast.method === "naive" || !naive || !chosen
            ? t.rich("forecast.naiveWon", { error: pct(naive?.mape ?? chosen?.mape ?? 0), b })
            : t.rich("forecast.won", { method: methodName(forecast.method), error: pct(chosen.mape), naive: pct(naive.mape), skill: pct(forecast.skill), b })}
        </p>
        {forecast.method === "ml" && <p className="mt-2 text-xs leading-relaxed text-muted-foreground">{t("forecast.mlNote")}</p>}
      </div>

      <div className="grid gap-5 md:grid-cols-[minmax(0,16rem)_1fr]">
        <div>
          <p className="text-xs font-medium">{t("forecast.errors")}</p>
          <p className="mt-0.5 text-xs text-muted-foreground">{t("forecast.errorsHint", { weeks: forecast.tested })}</p>
          <BarList
            className="mt-3"
            max={Math.max(...forecast.methods.map((m) => m.mape), 0.001)}
            rows={forecast.methods.map((m) => ({
              key: m.method,
              label: methodName(m.method),
              value: m.mape,
              display: pct(m.mape),
              note: m.method === forecast.method ? t("forecast.chosen") : undefined,
              noteTone: "muted",
              highlight: m.method === forecast.method,
            }))}
          />
        </div>

        <div className="min-w-0">
          <p className="text-xs font-medium">{t("forecast.weeks")}</p>
          <p className="mt-0.5 text-xs text-muted-foreground">{t("forecast.weeksHint", { price: formatSom(forecast.base) })}</p>
          <ul className="mt-3 grid gap-3 sm:grid-cols-2">
            {forecast.weeks.map((w) => {
              const total = w.drivers.reduce((s, d) => s + d.share, 0);
              const title = t("jumps.week", { date: fmt.dayMonth(day(w.week)) });
              return (
                <li key={w.week} className="rounded-2xl bg-muted/50 px-3.5 py-3">
                  <div className="flex flex-wrap items-baseline justify-between gap-x-3">
                    <p className="text-xs text-muted-foreground">{title}</p>
                    <p className="num text-sm font-semibold">
                      {formatSom(w.median)}
                      <span className="ml-1.5 text-xs font-normal text-muted-foreground">
                        {formatSom(w.low)}–{formatSom(w.high)}
                      </span>
                    </p>
                  </div>
                  {w.drivers.length === 0 ? (
                    <p className="mt-2 text-xs text-muted-foreground">{t("forecast.flat")}</p>
                  ) : (
                    <ul className="mt-2 space-y-1.5" aria-label={t("forecast.aria", { week: title })}>
                      {w.drivers.map((d) => (
                        <SignedBar key={d.code} label={t(`forecast.driver.${d.code}`)} share={d.share} max={widest} value={fmt.percent(d.share, { signed: true })} />
                      ))}
                      {w.drivers.length > 1 && (
                        <li className="flex items-baseline justify-between gap-3 border-t border-foreground/10 pt-1.5 text-xs">
                          <span className="font-medium">{t("forecast.total")}</span>
                          <span className="num font-semibold">{fmt.percent(total, { signed: true })}</span>
                        </li>
                      )}
                    </ul>
                  )}
                </li>
              );
            })}
          </ul>
        </div>
      </div>

      <div className="space-y-1 text-xs text-muted-foreground">
        <p>{t("chart.forecastNote")}</p>
        <p>{t("chart.forecastError", { share: pct(forecast.mape[0] ?? 0), weeks: forecast.tested })}</p>
        {forecast.method === "season_trend" && <p>{t("forecast.seasonNote")}</p>}
      </div>
    </div>
  );
}

/**
 * One signed part of a change: the label and value in text, and a bar growing
 * left (cheaper, green) or right (dearer, warm) from a centre line. `stack`
 * puts a long label on its own line on phones.
 */
export function SignedBar({
  label,
  share,
  max,
  value,
  note,
  muted,
  stack,
}: {
  label: string;
  share: number;
  max: number;
  value: string;
  note?: string;
  muted?: boolean;
  stack?: boolean;
}) {
  const w = `${Math.max(3, Math.min(100, (100 * Math.abs(share)) / max))}%`;
  const color = muted ? "var(--muted-foreground)" : share < 0 ? "var(--viz-cheap)" : "var(--viz-dear)";
  return (
    <li
      className={cn(
        "grid items-center gap-x-2 gap-y-1 text-xs",
        stack ? "grid-cols-[minmax(0,1fr)_auto] sm:grid-cols-[minmax(0,1fr)_6rem_auto]" : "grid-cols-[minmax(0,1fr)_4.5rem_auto] sm:grid-cols-[minmax(0,1fr)_6rem_auto]",
      )}
    >
      <span className={cn("min-w-0", stack ? "col-span-2 sm:col-span-1" : "truncate")} title={label}>
        {label}
      </span>
      <span aria-hidden className="relative h-1.5 rounded-full bg-muted">
        <span className="absolute -inset-y-0.75 left-1/2 w-px bg-foreground/25" />
        <span className={cn("absolute inset-y-0 rounded-full", share < 0 ? "right-1/2" : "left-1/2")} style={{ width: `calc(${w} / 2)`, backgroundColor: color }} />
      </span>
      <span className={cn("num text-right whitespace-nowrap", note ? "min-w-26" : "min-w-12")}>
        <span className="font-medium">{value}</span>
        {note && <span className="ml-1.5 text-muted-foreground">{note}</span>}
      </span>
    </li>
  );
}

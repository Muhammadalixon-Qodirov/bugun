"use client";

import { Flame, Snowflake } from "lucide-react";
import { useTranslations } from "next-intl";

import { useCatalogNames } from "@/components/catalog-names";
import { LineChart } from "@/components/charts/line-chart";
import type { Fx, Weather } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

const day = (d: string) => Date.parse(`${d}T00:00:00Z`);

function Legend({ items }: { items: { color: string; label: string }[] }) {
  return (
    <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
      {items.map((i) => (
        <span key={i.label} className="flex items-center gap-1.5">
          <span aria-hidden className="h-0.5 w-4 rounded-full" style={{ backgroundColor: i.color }} />
          {i.label}
        </span>
      ))}
    </div>
  );
}

/** Daily high and low temperature on one °C axis, plus the period's frosts and heat waves. */
export function WeatherBlock({ weather, national }: { weather: Weather; national: boolean }) {
  const t = useTranslations("analytics");
  const fmt = useFormat();
  const names = useCatalogNames();
  const temp = (v: number) => `${fmt.decimal(v, 0)}°`;
  const long = weather.points.length > 120;
  return (
    <div>
      <p className="text-sm text-muted-foreground">
        {t("weather.subtitle", { scope: national ? t("weather.countryAverage") : names.region(weather.scope) })}
      </p>
      <div className="mt-3">
        <Legend
          items={[
            { color: "var(--viz-dear)", label: t("weather.tmax") },
            { color: "var(--viz-1)", label: t("weather.tmin") },
          ]}
        />
      </div>
      <div className="mt-2">
        <LineChart
          height={180}
          endLabel={false}
          ariaLabel={t("weather.aria")}
          yFormat={temp}
          xFormat={(x) => {
            const iso = new Date(x).toISOString();
            return long ? fmt.monthShort(new Date(x).getUTCMonth() + 1) : fmt.dayMonth(iso);
          }}
          titleFormat={(x) => fmt.date(new Date(x).toISOString())}
          series={[
            { id: "tmax", label: t("weather.tmax"), color: "var(--viz-dear)", points: weather.points.map((p) => ({ x: day(p.date), y: p.tmax })) },
            { id: "tmin", label: t("weather.tmin"), color: "var(--viz-1)", points: weather.points.map((p) => ({ x: day(p.date), y: p.tmin })) },
          ]}
        />
      </div>
      <p className="num mt-3 text-sm text-muted-foreground">
        {t("weather.precipTotal", {
          mm: formatSom(weather.points.reduce((sum, p) => sum + p.precip, 0)),
          days: weather.points.filter((p) => p.precip >= 1).length,
        })}
      </p>
      {weather.extremes.length > 0 && (
        <div className="mt-4">
          <p className="text-xs font-medium text-muted-foreground">{t("weather.extremes")}</p>
          <ul className="mt-2 flex flex-wrap gap-1.5">
            {weather.extremes.slice(0, 8).map((e) => {
              const Icon = e.kind === "frost" ? Snowflake : Flame;
              return (
                <li
                  key={`${e.date}-${e.province}`}
                  className={cn(
                    "inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs",
                    e.kind === "frost" ? "bg-secondary text-secondary-foreground" : "bg-warm-soft text-warm-foreground",
                  )}
                >
                  <Icon className="size-3.5" />
                  <span className="num font-medium">{temp(e.temp)}</span>
                  <span>
                    {fmt.dayMonth(e.date)} {new Date(day(e.date)).getUTCFullYear()} · {names.region(e.province).replace(/ (viloyati|Region)$/, "")}
                  </span>
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </div>
  );
}

/** Central Bank rates with their change over the period, and the dollar line when there is one. */
export function FxBlock({ fx }: { fx: Fx }) {
  const t = useTranslations("analytics");
  const fmt = useFormat();
  const usd = fx.rates.find((r) => r.ccy === "USD");
  const rate = (v: number) => (v >= 1000 ? formatSom(v) : fmt.decimal(v, 1));
  return (
    <div>
      <ul className="grid grid-cols-3 gap-2">
        {fx.rates.map((r) => (
          <li key={r.ccy} className="rounded-2xl bg-muted/60 px-3 py-2.5">
            <p className="text-xs text-muted-foreground">{r.ccy}</p>
            <p className="num mt-0.5 font-semibold">{rate(r.now)}</p>
            {r.change != null && (
              <p className={cn("num text-xs", r.change > 0 ? "text-warm-foreground" : r.change < 0 ? "text-primary" : "text-muted-foreground")}>
                {fmt.percent(r.change, { signed: true })}
              </p>
            )}
          </li>
        ))}
      </ul>
      {usd?.points && usd.points.length > 1 && (
        <div className="mt-4">
          <LineChart
            height={140}
            ariaLabel={t("fx.aria")}
            yFormat={formatSom}
            xFormat={(x) => fmt.monthShort(new Date(x).getUTCMonth() + 1)}
            titleFormat={(x) => fmt.date(new Date(x).toISOString())}
            series={[{ id: "usd", label: "USD", color: "var(--viz-1)", points: usd.points.map((p) => ({ x: day(p.date), y: p.rate })) }]}
          />
        </div>
      )}
      {fx.correlation_usd != null && fx.correlation_usd >= 0.5 && (
        <p className="mt-3 text-sm text-muted-foreground">{t("fx.corr", { r: fmt.decimal(fx.correlation_usd, 2) })}</p>
      )}
    </div>
  );
}

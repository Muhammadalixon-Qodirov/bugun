import { ArrowDownRight, ArrowRight, ArrowUpRight, ChevronRight, Clock3, Download, GitCompareArrows, Grid3x3, MapPinned, MessageCircleQuestion, Minus, ShoppingBasket, Store } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { getLocale, getTranslations } from "next-intl/server";
import { Suspense } from "react";

import { AnalyticsFilters } from "@/components/analytics/filters";
import { ForecastExplain } from "@/components/analytics/forecast";
import { PriceHistory } from "@/components/analytics/price-history";
import { SeasonChart, WeekdayChart } from "@/components/analytics/season";
import { TransportRate } from "@/components/analytics/transport-rate";
import { FxBlock, WeatherBlock } from "@/components/analytics/weather-fx";
import { BarList } from "@/components/charts/bar-list";
import { DemoBadge } from "@/components/demo-badge";
import { MarketNavProvider, MarketPending } from "@/components/market-nav";
import { Reveal } from "@/components/motion";
import { Page } from "@/components/page";
import { ReferencePrices } from "@/components/reference-prices";
import { buttonVariants } from "@/components/ui/button";
import { ZoneBadge } from "@/components/zone-badge";
import { type Period, periods } from "@/lib/analytics";
import { apiFileUrl } from "@/lib/api/download";
import type { AdviceItem, AnalyticsOutputBody, EventRef } from "@/lib/api/gen/types.gen";
import { getCatalogNames } from "@/lib/i18n/catalog.server";
import { getFormat } from "@/lib/i18n/format.server";
import { holidayKey } from "@/lib/i18n/holidays";
import { formatSom } from "@/lib/labels";
import { type AnalyticsQuery, getProductAdviceText, getProductAnalytics } from "@/lib/server/analytics";
import { getCatalogData } from "@/lib/server-api";
import { cn } from "@/lib/utils";

type Search = Record<string, string | string[] | undefined>;

function queryOf(sp: Search): AnalyticsQuery {
  const one = (k: string) => (typeof sp[k] === "string" ? (sp[k] as string) : undefined);
  const region = Number(one("region"));
  const transport = Number(one("transport"));
  const period = one("period");
  return {
    region: Number.isInteger(region) && region > 0 ? region : 0,
    trade: one("trade") === "chakana" ? "chakana" : "ulgurji",
    period: (periods as readonly string[]).includes(period ?? "") ? (period as Period) : "1y",
    transport: Number.isInteger(transport) && transport >= 0 && transport <= 5000 ? transport : undefined,
  };
}

export async function generateMetadata({ params }: PageProps<"/mahsulot/[slug]/tahlil">): Promise<Metadata> {
  const { slug } = await params;
  const [t, catalog, names] = await Promise.all([getTranslations("analytics"), getCatalogData(), getCatalogNames()]);
  const product = catalog.products.find((p) => p.slug === slug);
  const name = product ? names.product(product.slug, product.name) : slug;
  return { title: t("metaTitle", { name }), description: t("metaDescription", { name }) };
}

export default async function ProductAnalyticsPage({ params, searchParams }: PageProps<"/mahsulot/[slug]/tahlil">) {
  const [{ slug }, sp] = await Promise.all([params, searchParams]);
  const query = queryOf(sp);
  const [data, catalog, t, tc, tcodes, fmt, names, locale] = await Promise.all([
    getProductAnalytics(slug, query),
    getCatalogData(),
    getTranslations("analytics"),
    getTranslations("common"),
    getTranslations("codes"),
    getFormat(),
    getCatalogNames(),
    getLocale(),
  ]);
  if (!data) notFound();

  const { product } = data;
  const basePath = `/mahsulot/${slug}/tahlil`;
  const productName = names.product(product.slug, product.name);
  const categoryUz = catalog.categories.find((c) => c.slug === product.category)?.name;
  const category = categoryUz ? names.category(product.category, categoryUz) : "";
  const unit = fmt.unit(product.base_unit);
  const perUnit = tc("perUnit", { unit });
  const provinceId = data.scope.province_id ?? 0;
  const scopeName = data.scope.province_name ? names.region(data.scope.province_name) : tc("wholeCountry");
  const provinces = catalog.regions.filter((r) => r.parent_id == null).map((r) => ({ id: r.id, name: r.name }));
  const offersHref = `/mahsulot/${slug}${query.trade === "chakana" ? "?trade=chakana" : ""}`;
  const csv = apiFileUrl(`/v1/products/${slug}/analytics.csv`, { region: provinceId || undefined, trade: query.trade, period: query.period, transport: query.transport });

  return (
    <Page className="py-5 sm:py-8">
      <nav className="flex flex-wrap items-center gap-1 text-[13px] text-muted-foreground" aria-label={t("filters.label")}>
        <Link href="/" className="transition-colors hover:text-foreground">
          {tc("market")}
        </Link>
        <ChevronRight className="size-3.5 opacity-60" />
        <Link href={`/?category=${product.category}`} className="transition-colors hover:text-foreground">
          {category}
        </Link>
        <ChevronRight className="size-3.5 opacity-60" />
        <Link href={offersHref} className="transition-colors hover:text-foreground">
          {productName}
        </Link>
        <ChevronRight className="size-3.5 opacity-60" />
        <span className="text-foreground">{t("tab")}</span>
      </nav>

      <Reveal className="mt-6 flex flex-wrap items-end justify-between gap-4">
        <div className="min-w-0">
          <p className="eyebrow">{t("eyebrow", { category })}</p>
          <h1 className="display mt-2 text-4xl sm:text-5xl">{productName}</h1>
          <nav aria-label={t("links.title")} className="-mx-4 mt-4 flex gap-2 overflow-x-auto px-4 scrollbar-none sm:mx-0 sm:flex-wrap sm:px-0">
            {[
              { href: `/solishtirish?products=${slug}`, icon: GitCompareArrows, label: t("links.compare") },
              { href: `/xarita?product=${slug}`, icon: MapPinned, label: t("links.map") },
              { href: `/statistika?category=${product.category}`, icon: Grid3x3, label: t("links.heatmap") },
              { href: `/tahlilchi?q=${encodeURIComponent(t("links.askQuestion", { product: productName.toLocaleLowerCase(locale) }))}`, icon: MessageCircleQuestion, label: t("links.ask") },
            ].map(({ href, icon: Icon, label }) => (
              <Link
                key={href}
                href={href}
                className="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-full bg-muted/70 px-3.5 text-sm whitespace-nowrap transition-colors hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
              >
                <Icon className="size-4 text-muted-foreground" aria-hidden />
                {label}
              </Link>
            ))}
          </nav>
        </div>
        <div className="flex flex-wrap gap-2">
          <Link href={offersHref} className={buttonVariants({ variant: "outline", size: "pill", className: "h-10 px-4" })}>
            <Store /> {t("offersLink")}
          </Link>
          <a href={csv} download className={buttonVariants({ variant: "outline", size: "pill", className: "h-10 px-4" })}>
            <Download /> {t("csv")}
          </a>
        </div>
      </Reveal>

      <MarketNavProvider>
        <div className="mt-6">
          <AnalyticsFilters basePath={basePath} provinces={provinces} province={provinceId} />
        </div>
        <Suspense fallback={null}>
          <ReferencePrices slug={slug} region={provinceId} trade={query.trade} perUnit={perUnit} className="mt-4 rounded-3xl border bg-card px-5 py-4" />
        </Suspense>

        <MarketPending className="mt-6 space-y-4">
          <Quality data={data} />
          <AdviceCard data={data} slug={slug} trade={query.trade ?? "ulgurji"} />

          <div className="grid gap-4 lg:grid-cols-[22rem_1fr]">
            <NowCard data={data} scopeName={scopeName} perUnit={perUnit} />
            <section className="min-w-0 rounded-3xl border bg-card p-5 sm:p-6">
              <CardHeading title={t("chart.title")} demoShare={data.quality.demo_share} />
              <p className="mt-0.5 text-sm text-muted-foreground">{t("chart.subtitle")}</p>
              <div className="mt-4">
                {data.series.length ? (
                  <PriceHistory data={data} productName={productName} />
                ) : (
                  <p className="py-10 text-center text-sm text-muted-foreground">{t("chart.empty")}</p>
                )}
              </div>
              {/* An older API sends a forecast without its method scores and drivers. */}
              {data.forecast?.methods && data.forecast.weeks.every((w) => w.drivers) && <ForecastExplain forecast={data.forecast} />}
            </section>
          </div>

          <div className="grid gap-4 lg:grid-cols-3">
            {data.seasonality && (
              <Card title={t("season.title")} subtitle={t("season.subtitle")} demoShare={data.quality.history_demo_share} className="lg:col-span-2">
                <SeasonChart season={data.seasonality} />
              </Card>
            )}
            <div className={cn("grid gap-4", !data.seasonality && "lg:col-span-3 sm:grid-cols-2")}>
              {data.weekdays && (
                <Card title={t("season.weekdays")} subtitle={t("season.weekdaysSub", { weeks: data.weekdays.weeks })} demoShare={data.quality.history_demo_share}>
                  <WeekdayChart weekdays={data.weekdays} />
                </Card>
              )}
              <SupplyCard data={data} unit={unit} />
            </div>
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <Card title={t("regions.title")} subtitle={t("regions.subtitle")}>
              {data.provinces.length ? (
                <BarList
                  max={Math.max(...data.provinces.map((p) => p.median))}
                  rows={data.provinces.map((p) => ({
                    key: String(p.id),
                    label: names.region(p.name),
                    value: p.median,
                    display: formatSom(p.median),
                    note: p.diff != null && p.id !== provinceId ? fmt.percent(p.diff, { signed: true }) : undefined,
                    noteTone: p.diff == null ? "muted" : p.diff < 0 ? "cheap" : "dear",
                    highlight: p.id === provinceId,
                    href: `${basePath}?${new URLSearchParams({ ...(query.trade === "chakana" ? { trade: "chakana" } : {}), ...(query.period !== "1y" ? { period: query.period } : {}), region: String(p.id) })}`,
                  }))}
                />
              ) : (
                <p className="text-sm text-muted-foreground">{t("regions.empty")}</p>
              )}
              {provinceId > 0 && (data.districts.length > 0 || data.markets.length > 0) && (
                <div className="mt-6 grid gap-6 sm:grid-cols-2">
                  {[
                    { title: t("regions.districts"), rows: data.districts, market: false },
                    { title: t("regions.markets"), rows: data.markets, market: true },
                  ].map((g) =>
                    g.rows.length ? (
                      <div key={g.title}>
                        <p className="text-xs font-medium text-muted-foreground">{g.title}</p>
                        <ul className="mt-2 space-y-1.5 text-sm">
                          {g.rows.map((r) => (
                            <li key={r.id} className="flex justify-between gap-2">
                              <span className="truncate">{g.market ? names.market(r.name) : names.region(r.name)}</span>
                              <span className="num shrink-0 font-medium">{formatSom(r.median)}</span>
                            </li>
                          ))}
                        </ul>
                      </div>
                    ) : null,
                  )}
                </div>
              )}
              {provinceId > 0 && (
                <Link href={`/hudud/${provinceId}`} className="mt-5 inline-flex items-center gap-1 text-sm font-medium underline-offset-4 hover:underline">
                  {t("regions.statsLink")} <ArrowRight className="size-3.5" />
                </Link>
              )}
            </Card>
            <ArbitrageCard data={data} unit={unit} basePath={basePath} />
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <JumpsCard data={data} />
            <RecordsCard data={data} />
          </div>

          {(data.reasons.length > 0 || data.weather || data.fx) && (
            <div className="grid gap-4 lg:grid-cols-2">
              {data.reasons.length > 0 && <ReasonsCard data={data} />}
              {data.weather && data.weather.points.length > 1 && (
                <Card title={t("weather.title")}>
                  <WeatherBlock weather={data.weather} national={!provinceId} />
                </Card>
              )}
              {data.fx && (
                <Card title={t("fx.title")} subtitle={t("fx.subtitle")}>
                  <FxBlock fx={data.fx} />
                </Card>
              )}
            </div>
          )}

          <p className="text-center text-xs text-muted-foreground">
            {tcodes("period." + query.period as Parameters<typeof tcodes>[0])} · {fmt.range(`${data.scope.from}T00:00:00Z`, `${data.scope.to}T00:00:00Z`)}
          </p>
        </MarketPending>
      </MarketNavProvider>
    </Page>
  );
}

/** A card's title, with the demo mark on the right when its numbers include demo data. */
function CardHeading({ title, demoShare }: { title: string; demoShare?: number }) {
  return (
    <div className="flex items-start justify-between gap-3">
      <h2 className="font-semibold tracking-tight">{title}</h2>
      <DemoBadge compact share={demoShare ?? 0} className="mt-0.5" />
    </div>
  );
}

function Card({
  title,
  subtitle,
  demoShare,
  className,
  children,
}: {
  title: string;
  subtitle?: string;
  /** Share of demo data behind the card's numbers (the API's demo_share), marked when above 0. */
  demoShare?: number;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <section className={cn("min-w-0 rounded-3xl border bg-card p-5 sm:p-6", className)}>
      <CardHeading title={title} demoShare={demoShare} />
      {subtitle && <p className="mt-0.5 text-sm text-muted-foreground">{subtitle}</p>}
      <div className="mt-4">{children}</div>
    </section>
  );
}

async function Quality({ data }: { data: AnalyticsOutputBody }) {
  const [t, tcodes, fmt] = await Promise.all([getTranslations("analytics"), getTranslations("codes"), getFormat()]);
  const q = data.quality;
  const sources = Object.entries(q.by_source ?? {}).sort((a, b) => b[1] - a[1]);
  type CodeKey = Parameters<typeof tcodes>[0];
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm text-muted-foreground">
      <DemoBadge share={q.demo_share} />
      <span>{t("quality.live", { count: q.offers_now })}</span>
      <span>{t("quality.deals", { count: q.confirmed_deals_30d })}</span>
      {q.last_price_at && <span>{t("quality.last", { when: fmt.dayAgo(q.last_price_at) })}</span>}
      {sources.length > 0 && (
        <span className="flex flex-wrap items-center gap-1.5" aria-label={t("quality.sources")}>
          {sources.map(([s, n]) => (
            <span key={s} className="rounded-full bg-muted px-2 py-0.5 text-xs">
              {tcodes.has(`source.${s}` as CodeKey) ? tcodes(`source.${s}` as CodeKey) : s} · <span className="num">{n}</span>
            </span>
          ))}
        </span>
      )}
    </div>
  );
}

async function AdviceCard({ data, slug, trade }: { data: AnalyticsOutputBody; slug: string; trade: "ulgurji" | "chakana" }) {
  const [t, tcodes, fmt, locale] = await Promise.all([getTranslations("analytics"), getTranslations("codes"), getFormat(), getLocale()]);
  const adviceText = await getProductAdviceText(slug, data.scope.province_id ?? 0, trade, locale as "uz" | "ru" | "en");
  const { advice } = data;
  const holiday = (title: string) => {
    const key = holidayKey(title);
    return key ? t(`holidays.${key}` as Parameters<typeof t>[0]) : title;
  };
  const sentence = (i: AdviceItem) => {
    const share = fmt.percent(i.share ?? 0, { digits: 0 });
    switch (i.code) {
      case "trend_up":
      case "trend_down":
      case "below_year":
      case "above_year":
        return t(`advice.items.${i.code}`, { share });
      case "season_dearer":
      case "season_cheaper":
        return t(`advice.items.${i.code}`, { share, month: fmt.month(i.month ?? 1), fromMonth: fmt.month(i.from_month ?? 1) });
      case "holiday_soon":
        return t("advice.items.holiday_soon", { title: holiday(i.title ?? ""), days: i.days ?? 0 });
      case "holiday_now":
        return t("advice.items.holiday_now", { title: holiday(i.title ?? ""), until: fmt.dayMonth(`${i.until}T00:00:00Z`) });
      default:
        return t(`advice.items.${i.code}`);
    }
  };
  // Older API builds send only the Uzbek sentences.
  const reasons = advice.items?.length ? advice.items.map(sentence) : advice.reasons;
  const verdicts = [
    { key: "buyer", icon: ShoppingBasket, label: t("advice.buyer"), value: advice.buyer, text: tcodes(`adviceBuyer.${advice.buyer}`), go: advice.buyer === "buy_now" },
    { key: "seller", icon: Store, label: t("advice.seller"), value: advice.seller, text: tcodes(`adviceSeller.${advice.seller}`), go: advice.seller === "sell_now" },
  ];
  return (
    <section className="rounded-3xl border bg-card p-5 sm:p-6">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="font-semibold tracking-tight">{t("advice.title")}</h2>
        {data.pressure && (
          <span className="flex items-center gap-2 text-sm text-muted-foreground">
            {t("advice.pressure")}
            <ZoneBadge zone={data.pressure.zone} />
          </span>
        )}
      </div>
      <div className="mt-4 grid gap-3 sm:grid-cols-2">
        {verdicts.map((v) => {
          const Icon = v.value === "neutral" ? Minus : v.go ? ArrowRight : Clock3;
          return (
            <div
              key={v.key}
              className={cn("flex items-center gap-4 rounded-2xl px-4 py-3.5", v.value === "neutral" ? "bg-muted" : v.go ? "bg-primary/10" : "bg-warm-soft")}
            >
              <v.icon className="size-5 shrink-0 text-muted-foreground" strokeWidth={1.5} />
              <div className="min-w-0 flex-1">
                <p className="text-xs text-muted-foreground">{v.label}</p>
                <p
                  className={cn(
                    "mt-0.5 flex items-center gap-1.5 text-lg font-semibold tracking-tight",
                    v.value === "neutral" ? "text-foreground" : v.go ? "text-primary" : "text-warm-foreground",
                  )}
                >
                  <Icon className="size-4" />
                  {v.text}
                </p>
              </div>
            </div>
          );
        })}
      </div>
      {adviceText ? (
        <>
          <p className="mt-4 leading-relaxed whitespace-pre-line" data-advice-mode={adviceText.mode}>
            {adviceText.text}
          </p>
          {reasons.length > 0 && (
            <details className="group mt-3 text-sm">
              <summary className="cursor-pointer text-muted-foreground transition-colors hover:text-foreground">{t("advice.reasons")}</summary>
              <ReasonList reasons={reasons} />
            </details>
          )}
        </>
      ) : reasons.length ? (
        <ReasonList reasons={reasons} />
      ) : (
        <p className="mt-4 text-sm text-muted-foreground">{t("advice.noReasons")}</p>
      )}
      <p className="mt-4 text-xs text-muted-foreground">{t("advice.disclaimer")}</p>
    </section>
  );
}

function ReasonList({ reasons }: { reasons: string[] }) {
  return (
    <ul className="mt-4 space-y-1.5 text-sm">
      {reasons.map((r) => (
        <li key={r} className="flex gap-2">
          <span aria-hidden className="mt-2 size-1 shrink-0 rounded-full bg-muted-foreground" />
          {r}
        </li>
      ))}
    </ul>
  );
}

async function NowCard({ data, scopeName, perUnit }: { data: AnalyticsOutputBody; scopeName: string; perUnit: string }) {
  const [t, tcodes, fmt] = await Promise.all([getTranslations("analytics"), getTranslations("codes"), getFormat()]);
  const changes = [
    { key: "d1", label: t("now.d1"), v: data.changes.d1 },
    { key: "d7", label: t("now.d7"), v: data.changes.d7 },
    { key: "d30", label: t("now.d30"), v: data.changes.d30 },
    { key: "y1", label: t("now.y1"), v: data.changes.y1 },
  ];
  const last = data.series[data.series.length - 1];
  const median = data.now?.median ?? last?.median;
  return (
    <section className="rounded-3xl border bg-card p-5 sm:p-6">
      <div className="flex items-start justify-between gap-3">
        <p className="text-sm text-muted-foreground">{t("now.median", { scope: scopeName })}</p>
        <DemoBadge compact share={Math.max(data.quality.demo_share, data.quality.history_demo_share)} />
      </div>
      {median != null ? (
        <p className="mt-2 text-5xl leading-none font-semibold tracking-[-0.035em]">
          {formatSom(median)}
          <span className="ml-2 text-base font-normal tracking-normal text-muted-foreground">{perUnit}</span>
        </p>
      ) : (
        <p className="display mt-2 text-3xl">—</p>
      )}
      {data.now ? (
        <p className="mt-3 text-sm text-muted-foreground">{t("now.range", { from: formatSom(data.now.p25), to: formatSom(data.now.p75) })}</p>
      ) : (
        <p className="mt-3 text-xs leading-relaxed text-muted-foreground">{t("now.notEnough")}</p>
      )}
      {data.trend && (
        <p className="mt-3 flex items-center gap-1.5 text-sm">
          {data.trend.direction === "up" ? (
            <ArrowUpRight className="size-4 text-warm-foreground" />
          ) : data.trend.direction === "down" ? (
            <ArrowDownRight className="size-4 text-primary" />
          ) : (
            <Minus className="size-4 text-muted-foreground" />
          )}
          {t("now.trendWeekly", { trend: tcodes(`trend.${data.trend.direction}`), share: fmt.percent(data.trend.weekly, { signed: true }) })}
        </p>
      )}
      <p className="mt-6 text-xs font-medium text-muted-foreground">{t("now.changes")}</p>
      <dl className="mt-2 grid grid-cols-4 gap-2">
        {changes.map((c) => (
          <div key={c.key} className="rounded-2xl bg-muted/60 px-2 py-2 text-center">
            <dt className="text-[11px] text-muted-foreground">{c.label}</dt>
            <dd
              className={cn(
                "num mt-0.5 text-sm font-semibold",
                c.v == null ? "text-muted-foreground" : c.v > 0.0005 ? "text-warm-foreground" : c.v < -0.0005 ? "text-primary" : "",
              )}
            >
              {c.v == null ? t("now.noChange") : fmt.percent(c.v, { signed: true })}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

async function SupplyCard({ data, unit }: { data: AnalyticsOutputBody; unit: string }) {
  const [t, fmt] = await Promise.all([getTranslations("analytics"), getFormat()]);
  const s = data.supply;
  return (
    <section className="rounded-3xl border bg-card p-5 sm:p-6">
      <h2 className="font-semibold tracking-tight">{t("supply.title")}</h2>
      <p className="mt-3 text-2xl font-semibold tracking-tight">{t("supply.sellers", { count: s.sellers })}</p>
      <ul className="mt-2 space-y-1 text-sm text-muted-foreground">
        {s.sellers_change != null && (
          <li className={s.sellers_change > 0 ? "text-primary" : s.sellers_change < 0 ? "text-warm-foreground" : ""}>
            {t("supply.sellersChange", { share: fmt.percent(s.sellers_change, { signed: true }) })}
          </li>
        )}
        <li>{t("supply.listings", { count: s.listings })}</li>
        {s.stock != null && <li>{t("supply.stock", { amount: formatSom(s.stock), unit, count: s.stocked })}</li>}
        {data.retail_markup != null && <li>{t("supply.retail", { share: fmt.percent(data.retail_markup) })}</li>}
        {data.bargain && <li>{t("supply.bargain", { share: fmt.percent(Math.abs(data.bargain.avg_discount)), count: data.bargain.deals })}</li>}
      </ul>
    </section>
  );
}

async function ArbitrageCard({ data, unit, basePath }: { data: AnalyticsOutputBody; unit: string; basePath: string }) {
  const [t, names] = await Promise.all([getTranslations("analytics"), getCatalogNames()]);
  const fmt = await getFormat();
  return (
    <section className="min-w-0 rounded-3xl border bg-card p-5 sm:p-6">
      <h2 className="font-semibold tracking-tight">{t("arbitrage.title")}</h2>
      <p className="mt-0.5 text-sm text-muted-foreground">{t("arbitrage.subtitle")}</p>
      <div className="mt-4">
        <TransportRate basePath={basePath} value={data.transport_rate} label={t("arbitrage.rate", { unit })} />
      </div>
      {data.arbitrage.length ? (
        <ul className="mt-4 space-y-2.5">
          {data.arbitrage.map((a) => (
            <li key={`${a.buy_id}-${a.sell_id}`} className="rounded-2xl bg-muted/60 px-4 py-3">
              <div className="flex items-baseline justify-between gap-3">
                <p className="min-w-0 truncate text-sm font-medium">
                  {t("arbitrage.route", { buy: names.region(a.buy), sell: names.region(a.sell) })}
                </p>
                <p className="num shrink-0 text-sm font-semibold text-primary">{fmt.percent(a.net_pct, { signed: true })}</p>
              </div>
              <p className="num mt-1 text-xs text-muted-foreground">
                {t("arbitrage.detail", { buyPrice: formatSom(a.buy_price), sellPrice: formatSom(a.sell_price), transport: formatSom(a.transport), km: formatSom(a.distance_km) })}
                {" · "}
                {t("arbitrage.net", { net: formatSom(a.net), unit })}
              </p>
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-4 text-sm text-muted-foreground">{t("arbitrage.empty")}</p>
      )}
      <p className="mt-3 text-xs text-muted-foreground">{t("arbitrage.approx")}</p>
    </section>
  );
}

/** An event's name: known holidays translated, weather built from its numbers, other titles as entered. */
async function getEventText() {
  const [t, tcodes, names] = await Promise.all([getTranslations("analytics"), getTranslations("codes"), getCatalogNames()]);
  return (e: EventRef) => {
    if (e.id === 0 && (e.kind === "frost" || e.kind === "heat")) {
      const temp = e.title.match(/-?\d+(?:[.,]\d+)?(?=\s*°)/)?.[0] ?? "";
      const place = e.province ? ` (${names.region(e.province)})` : "";
      return t(`jumps.${e.kind}`, { temp }) + place;
    }
    const key = e.kind === "holiday" ? holidayKey(e.title) : undefined;
    const kind = tcodes.has(`eventKind.${e.kind}` as Parameters<typeof tcodes>[0]) ? tcodes(`eventKind.${e.kind}` as Parameters<typeof tcodes>[0]) : e.kind;
    return `${kind}: ${key ? t(`holidays.${key}` as Parameters<typeof t>[0]) : e.title}`;
  };
}

async function JumpsCard({ data }: { data: AnalyticsOutputBody }) {
  const [t, fmt, eventText] = await Promise.all([getTranslations("analytics"), getFormat(), getEventText()]);
  return (
    <section className="min-w-0 rounded-3xl border bg-card p-5 sm:p-6">
      <CardHeading title={t("jumps.title")} demoShare={data.quality.history_demo_share} />
      <p className="mt-0.5 text-sm text-muted-foreground">{t("jumps.subtitle")}</p>
      {data.jumps.length ? (
        <ul className="mt-4 space-y-3">
          {[...data.jumps].reverse().map((j) => (
            <li key={j.week} className="flex gap-3">
              <span
                className={cn(
                  "num mt-0.5 inline-flex h-7 min-w-16 shrink-0 items-center justify-center rounded-full px-2 text-xs font-semibold",
                  j.change > 0 ? "bg-warm-soft text-warm-foreground" : "bg-primary/10 text-primary",
                )}
              >
                {fmt.percent(j.change, { signed: true })}
              </span>
              <div className="min-w-0 text-sm">
                <p className="font-medium">{t("jumps.week", { date: fmt.date(`${j.week}T00:00:00Z`) })}</p>
                {j.events.length ? (
                  <p className="text-muted-foreground">
                    {t("jumps.possible")} {j.events.map(eventText).join("; ")}
                  </p>
                ) : (
                  <p className="text-muted-foreground">{t("jumps.noCause")}</p>
                )}
              </div>
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-4 text-sm text-muted-foreground">{t("jumps.empty")}</p>
      )}
    </section>
  );
}

async function RecordsCard({ data }: { data: AnalyticsOutputBody }) {
  const [t, fmt, names] = await Promise.all([getTranslations("analytics"), getFormat(), getCatalogNames()]);
  const { records } = data;
  const tile = (label: string, r: typeof records.max, tone: string) =>
    r ? (
      <div className="rounded-2xl bg-muted/60 px-4 py-3">
        <p className="text-xs text-muted-foreground">{label}</p>
        <p className={cn("num mt-1 text-xl font-semibold", tone)}>{formatSom(r.price)}</p>
        <p className="mt-0.5 text-xs text-muted-foreground">
          {fmt.date(`${r.date}T00:00:00Z`)} · {names.region(r.province_name)}
        </p>
      </div>
    ) : null;
  return (
    <section className="min-w-0 rounded-3xl border bg-card p-5 sm:p-6">
      <CardHeading title={t("records.title")} demoShare={data.quality.history_demo_share} />
      <div className="mt-4 grid gap-2 sm:grid-cols-2">
        {tile(t("records.max"), records.max, "text-warm-foreground")}
        {tile(t("records.min"), records.min, "text-primary")}
      </div>
      {records.by_year.length > 0 && (
        <div className="mt-5 overflow-x-auto">
          <p className="text-xs font-medium text-muted-foreground">{t("records.byYear")}</p>
          <table className="mt-2 w-full text-sm">
            <thead className="text-left text-xs text-muted-foreground">
              <tr>
                <th className="py-1.5 pr-3 font-medium">{t("records.year")}</th>
                <th className="py-1.5 pr-3 text-right font-medium">{t("records.min")}</th>
                <th className="py-1.5 text-right font-medium">{t("records.max")}</th>
              </tr>
            </thead>
            <tbody className="num">
              {[...records.by_year].reverse().map((y) => (
                <tr key={y.year} className="border-t">
                  <td className="py-1.5 pr-3">{y.year}</td>
                  <td className="py-1.5 pr-3 text-right">{y.min ? formatSom(y.min.price) : "—"}</td>
                  <td className="py-1.5 text-right">{y.max ? formatSom(y.max.price) : "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

async function ReasonsCard({ data }: { data: AnalyticsOutputBody }) {
  const [t, tcodes] = await Promise.all([getTranslations("analytics"), getTranslations("codes")]);
  type CodeKey = Parameters<typeof tcodes>[0];
  const label = (code: string, fallback: string) => (tcodes.has(`sellerReason.${code}` as CodeKey) ? tcodes(`sellerReason.${code}` as CodeKey) : fallback);
  const max = Math.max(...data.reasons.map((r) => r.count));
  return (
    <section className="min-w-0 rounded-3xl border bg-card p-5 sm:p-6">
      <h2 className="font-semibold tracking-tight">{t("reasons.title")}</h2>
      <p className="mt-0.5 text-sm text-muted-foreground">{t("reasons.subtitle")}</p>
      <div className="mt-4 grid gap-6 sm:grid-cols-2">
        {(["up", "down"] as const).map((dir) => {
          const rows = data.reasons.filter((r) => r.direction === dir);
          if (!rows.length) return null;
          return (
            <div key={dir}>
              <p className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
                {dir === "up" ? <ArrowUpRight className="size-3.5" /> : <ArrowDownRight className="size-3.5" />}
                {t(`reasons.${dir}`)}
              </p>
              <BarList
                className="mt-2"
                max={max}
                rows={rows.map((r) => ({
                  key: r.code,
                  label: label(r.code, r.label),
                  value: r.count,
                  display: String(r.count),
                  color: dir === "up" ? "var(--viz-dear)" : "var(--viz-cheap)",
                }))}
              />
            </div>
          );
        })}
      </div>
    </section>
  );
}

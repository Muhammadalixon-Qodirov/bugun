import { ArrowDownRight, ArrowUpRight, ChartSpline, ChevronRight, Flame, Info, MapPin, Store } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";

import { ContactButton } from "@/components/contact-button";
import { EmptyState } from "@/components/empty-state";
import { TradeSwitch } from "@/components/trade-switch";
import { Reveal, Stagger, StaggerItem } from "@/components/motion";
import { OfferPhotos } from "@/components/offer-photos";
import { ReferencePrices } from "@/components/reference-prices";
import { Page } from "@/components/page";
import { buttonVariants } from "@/components/ui/button";
import { WatchButton } from "@/components/watch-button";
import { ZoneBadge } from "@/components/zone-badge";
import { getCatalogNames } from "@/lib/i18n/catalog.server";
import { getFormat } from "@/lib/i18n/format.server";
import { formatSom } from "@/lib/labels";
import { getProductPressure } from "@/lib/server/analytics";
import { getCatalogData, getCurrentUser, getProductPage, getRegionId } from "@/lib/server-api";
import { cn } from "@/lib/utils";

export async function generateMetadata({ params }: PageProps<"/mahsulot/[slug]">): Promise<Metadata> {
  const { slug } = await params;
  const [data, t, fmt, names] = await Promise.all([getProductPage(slug, 0, "ulgurji"), getTranslations("product"), getFormat(), getCatalogNames()]);
  if (!data) return { title: t("metaFallback") };
  const name = names.product(data.product.slug, data.product.name);
  const title = t("metaTitle", { name });
  const description = data.price.enough
    ? t("metaPrice", { price: formatSom(data.price.median ?? 0), unit: fmt.unit(data.product.base_unit), offers: data.price.offers })
    : t("metaOffers", { name });
  return { title, description, openGraph: { title, description, type: "website" } };
}

export default async function ProductPage({ params, searchParams }: PageProps<"/mahsulot/[slug]">) {
  const [{ slug }, sp] = await Promise.all([params, searchParams]);
  const trade = sp.trade === "chakana" ? "chakana" : "ulgurji";
  const [savedRegion, catalog, t, ts, tc, fmt, names, me] = await Promise.all([
    getRegionId(),
    getCatalogData(),
    getTranslations("product"),
    getTranslations("store"),
    getTranslations("common"),
    getFormat(),
    getCatalogNames(),
    getCurrentUser(),
  ]);
  // A region in the link (the bot's price alerts send ?region=<province or 0>) wins over the saved one.
  const asked = typeof sp.region === "string" && /^\d+$/.test(sp.region) ? Number(sp.region) : null;
  const region = asked != null && (asked === 0 || catalog.regions.some((r) => r.id === asked)) ? asked : savedRegion;
  const [data, pressure] = await Promise.all([getProductPage(slug, region, trade), getProductPressure(slug, region, trade)]);
  if (!data) notFound();

  const { product, price, offers } = data;
  const unit = fmt.unit(product.base_unit);
  const perUnit = tc("perUnit", { unit });
  const productName = names.product(product.slug, product.name);
  const categoryUz = catalog.categories.find((c) => c.slug === product.category)?.name;
  const category = categoryUz ? names.category(product.category, categoryUz) : "";
  const regionUz = catalog.regions.find((r) => r.id === region)?.name;
  const regionName = regionUz ? names.region(regionUz) : tc("wholeCountry");
  const scopeName = data.price_scope ? names.region(data.price_scope.name) : tc("wholeCountry");
  const widened = price.enough && region !== 0 && data.price_scope?.id !== region;
  const regionNames = new Map(catalog.regions.map((r) => [r.id, r.name]));
  const provinceName = (id: number) => {
    const uz = regionNames.get(id);
    return uz ? names.region(uz).replace(/ (viloyati|Region)$/, "") : "";
  };
  const maxProvince = Math.max(1, ...data.provinces.map((p) => p.median));
  const provinces = catalog.regions.filter((r) => r.parent_id == null).map((r) => ({ id: r.id, name: r.name }));
  const province = catalog.regions.find((r) => r.id === region)?.parent_id ?? region;
  const interest = data.interest;
  const interestText = interest
    ? [
        interest.visitors_today != null && t("interest.visitors", { count: interest.visitors_today }),
        interest.deals_7d != null && t("interest.deals", { count: interest.deals_7d }),
        interest.deal_median_7d != null && t("interest.dealMedian", { price: formatSom(interest.deal_median_7d) }),
      ]
        .filter(Boolean)
        .join(" · ")
    : "";
  const analysisHref = `/mahsulot/${slug}/tahlil?${new URLSearchParams({ ...(province ? { region: String(province) } : {}), ...(trade === "chakana" ? { trade } : {}) })}`;

  return (
    <Page className="py-5 sm:py-8">
      <nav className="flex flex-wrap items-center gap-1 text-[13px] text-muted-foreground" aria-label={t("breadcrumb")}>
        <Link href="/" className="transition-colors hover:text-foreground">{tc("market")}</Link>
        <ChevronRight className="size-3.5 opacity-60" />
        <Link href={`/?category=${product.category}`} className="transition-colors hover:text-foreground">{category}</Link>
      </nav>

      <Reveal className="mt-6 flex flex-wrap items-end justify-between gap-5">
        <div className="min-w-0">
          <p className="eyebrow">{t("eyebrow", { category })}</p>
          <h1 className="display mt-2 text-4xl sm:text-5xl">{productName}</h1>
        </div>
        <TradeSwitch slug={slug} value={trade} />
      </Reveal>

      <Reveal delay={0.03} className="mt-4 flex flex-wrap gap-2">
        <Link href={analysisHref} className={buttonVariants({ variant: "solid", size: "pill", className: "h-10 px-4" })}>
          <ChartSpline /> {t("analysis")}
        </Link>
        <WatchButton
          product={{ id: product.id, name: productName }}
          provinces={provinces}
          province={province}
          trade={trade}
          signedIn={!!me}
          botEnabled={me?.bot_enabled ?? false}
        />
      </Reveal>

      <div className="mt-8 grid gap-4 lg:grid-cols-[1fr_22rem]">
        <Reveal delay={0.05} className="rounded-3xl border bg-card p-6 sm:p-8">
          {price.enough ? (
            <>
              <p className="text-sm text-muted-foreground">{t("median", { scope: scopeName })}</p>
              <p className="num mt-2 text-5xl leading-none font-semibold tracking-[-0.035em] sm:text-7xl">
                {formatSom(price.median ?? 0)}
                <span className="ml-2 font-sans text-lg font-normal tracking-normal text-muted-foreground">{perUnit}</span>
              </p>
              <PriceRange min={price.min ?? 0} p25={price.p25 ?? 0} median={price.median ?? 0} p75={price.p75 ?? 0} max={price.max ?? 0} label={(from, to) => t("middleHalf", { from, to })} />
              <p className="mt-6 text-xs text-muted-foreground">
                {t("sellers", { count: price.offers })}
                {price.first_seen && price.last_seen ? ` · ${fmt.range(price.first_seen, price.last_seen)}` : ""} · {t("source")}
              </p>
              {widened && (
                <p className="mt-4 flex items-start gap-2 rounded-2xl bg-warm-soft px-4 py-3 text-sm text-warm-foreground">
                  <Info className="mt-0.5 size-4 shrink-0" />
                  {t("widened", { region: regionName, scope: scopeName })}
                </p>
              )}
            </>
          ) : (
            <div className="py-2">
              <p className="text-sm text-muted-foreground">{t("average")}</p>
              <p className="display mt-2 text-3xl">{t("collecting")}</p>
              <p className="mt-3 max-w-md text-sm leading-relaxed text-muted-foreground">
                {t("collectingText", { offers: price.offers })}
              </p>
            </div>
          )}
          {/* Official exchange prices stream in on their own and never hold the page up. */}
          <Suspense fallback={null}>
            <ReferencePrices slug={slug} region={region} trade={trade} perUnit={perUnit} className="mt-5 border-t pt-4" />
          </Suspense>
          {(interestText || pressure?.enough) && (
            <div className="mt-5 flex flex-wrap items-center gap-x-4 gap-y-2 border-t pt-4 text-sm">
              {interestText && (
                <span className="flex items-center gap-1.5 text-muted-foreground">
                  <Flame className="size-4 text-warm" />
                  {interestText}
                </span>
              )}
              {pressure?.enough && (
                <span className="flex flex-wrap items-center gap-2 text-muted-foreground">
                  {t("pressure")}
                  <ZoneBadge zone={pressure.zone} withHint />
                  <span className="num w-full text-xs">
                    {t("pressureDetail", {
                      now: formatSom(pressure.demand_now),
                      usual: fmt.decimal(pressure.demand_avg, 1),
                      sellers: formatSom(pressure.sellers_now),
                      sellersUsual: fmt.decimal(pressure.sellers_avg, 1),
                    })}
                  </span>
                </span>
              )}
            </div>
          )}
        </Reveal>

        <Reveal delay={0.1} className="rounded-3xl border bg-card p-6">
          <h2 className="font-semibold tracking-tight">{t("byProvince")}</h2>
          {data.provinces.length === 0 ? (
            <p className="mt-3 text-sm text-muted-foreground">{t("noData")}</p>
          ) : (
            <ul className="mt-4 space-y-3.5">
              {data.provinces.map((p) => (
                <li key={p.region_id} className="text-sm">
                  <div className="flex justify-between gap-2">
                    <span className="truncate text-muted-foreground">{provinceName(p.region_id)}</span>
                    <span className="num font-medium">{formatSom(p.median)}</span>
                  </div>
                  <div className="mt-1.5 h-1 overflow-hidden rounded-full bg-muted">
                    <div className="h-full rounded-full bg-foreground/70" style={{ width: `${(100 * p.median) / maxProvince}%` }} />
                  </div>
                </li>
              ))}
            </ul>
          )}
        </Reveal>
      </div>

      <section className="mt-16">
        <div className="mb-5 flex flex-wrap items-end justify-between gap-2">
          <h2 className="display text-2xl sm:text-[1.75rem]">{t("offers")}</h2>
          <p className="text-sm text-muted-foreground">
            {t("offersMeta", { region: regionName, count: offers.length })}
          </p>
        </div>
        {offers.length === 0 ? (
          <EmptyState
            icon={<MapPin strokeWidth={1.5} />}
            title={t("emptyTitle")}
            description={t("emptyText")}
          />
        ) : (
          <Stagger className="grid gap-4 md:grid-cols-2">
            {offers.map((o, i) => (
              <StaggerItem key={o.id} className="flex min-w-0 gap-4 rounded-3xl border bg-card p-3 transition-[box-shadow,transform] duration-150 hover:shadow-lg hover:shadow-black/[0.04] pointer-coarse:active:scale-[0.99]">
                <div className="relative aspect-square w-24 shrink-0 self-start overflow-hidden rounded-2xl sm:w-36" data-offer={o.id}>
                  <OfferPhotos photos={o.photos} alt={`${productName} — ${o.seller_name}`} category={product.category} />
                  {i === 0 && offers.length > 1 && (
                    <span className="absolute top-2 left-2 rounded-full bg-foreground px-2 py-0.5 text-[11px] font-medium text-background">
                      {t("cheapest")}
                    </span>
                  )}
                </div>
                <div className="flex min-w-0 flex-1 flex-col gap-1.5 py-1 pr-1">
                  <div className="flex items-start justify-between gap-2">
                    <p className="num text-2xl leading-tight font-semibold">
                      {formatSom(o.price_som)}
                      <span className="ml-1 font-sans text-sm font-normal text-muted-foreground">{perUnit}</span>
                    </p>
                    {o.diff_percent != null && o.diff_percent !== 0 && (
                      <span
                        className={cn(
                          "mt-1 flex shrink-0 items-center rounded-full px-2 py-0.5 text-xs font-medium",
                          o.diff_percent < 0 ? "bg-primary/10 text-primary" : "bg-warm-soft text-warm-foreground",
                        )}
                        title={t("diffTitle")}
                      >
                        {o.diff_percent < 0 ? <ArrowDownRight className="size-3" /> : <ArrowUpRight className="size-3" />}
                        {Math.abs(o.diff_percent)}%
                      </span>
                    )}
                  </div>
                  <div className="flex min-w-0 items-center gap-1.5">
                    <Link href={`/elon/${o.id}`} className="truncate font-medium underline-offset-4 hover:underline">
                      {o.seller_name}
                    </Link>
                    <Link
                      href={`/sotuvchi/${o.seller_id}`}
                      title={ts("storeOf", { seller: o.seller_name })}
                      aria-label={ts("storeOf", { seller: o.seller_name })}
                      className="-my-1 inline-flex h-7 shrink-0 items-center gap-1 rounded-full px-2 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                    >
                      <Store className="size-3.5" aria-hidden />
                      {ts("store")}
                    </Link>
                  </div>
                  <p className="flex items-center gap-1 truncate text-xs text-muted-foreground">
                    <MapPin className="size-3 shrink-0" />
                    {[o.market_name && names.market(o.market_name), o.region_name && names.region(o.region_name)].filter(Boolean).join(", ")}
                  </p>
                  <p className="truncate text-xs text-muted-foreground">
                    {[o.variety, o.min_order != null && t("minOrder", { amount: formatSom(o.min_order), unit }), o.stock != null && t("stock", { amount: formatSom(o.stock), unit })]
                      .filter(Boolean)
                      .join(" · ")}
                  </p>
                  <div className="mt-auto flex flex-wrap items-center justify-between gap-2 pt-2">
                    <span className="text-xs text-muted-foreground">{fmt.date(o.updated_at)}</span>
                    <ContactButton listingId={o.id} hasTelegram={o.has_telegram} hasPhone={o.has_phone} />
                  </div>
                </div>
              </StaggerItem>
            ))}
          </Stagger>
        )}
      </section>
    </Page>
  );
}

/** Min–max line with the middle half (p25–p75) highlighted and the median marked. */
function PriceRange({
  min,
  p25,
  median,
  p75,
  max,
  label,
}: {
  min: number;
  p25: number;
  median: number;
  p75: number;
  max: number;
  label: (from: string, to: string) => string;
}) {
  const span = Math.max(1, max - min);
  const pos = (v: number) => `${(100 * (v - min)) / span}%`;
  return (
    <div className="mt-8">
      <div className="relative h-1.5 rounded-full bg-muted">
        <div className="absolute inset-y-0 rounded-full bg-foreground/20" style={{ left: pos(p25), right: `calc(100% - ${pos(p75)})` }} />
        <div className="absolute top-1/2 size-3.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary ring-4 ring-card" style={{ left: pos(median) }} />
      </div>
      <div className="num mt-2.5 flex justify-between text-xs text-muted-foreground">
        <span>{formatSom(min)}</span>
        <span className="font-sans">{label(formatSom(p25), formatSom(p75))}</span>
        <span>{formatSom(max)}</span>
      </div>
    </div>
  );
}

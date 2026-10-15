import { ArrowDownRight, ArrowUpRight, ChartSpline, ChevronRight, Eye, MapPin, Scale, ShieldCheck, Star, Store } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { ContactButton } from "@/components/contact-button";
import { ListingCard } from "@/components/listing-card";
import { ListingGallery } from "@/components/listing-gallery";
import { ShareButton } from "@/components/share-button";
import { Reveal } from "@/components/motion";
import { Page } from "@/components/page";
import { Rail } from "@/components/rail";
import { getCatalogNames } from "@/lib/i18n/catalog.server";
import { getFormat } from "@/lib/i18n/format.server";
import { formatSom } from "@/lib/labels";
import { previewImages } from "@/lib/media";
import { getCatalogData, getListingPage, getRegionId } from "@/lib/server-api";
import { cn } from "@/lib/utils";

function parseId(raw: string) {
  const id = Number(raw);
  return Number.isInteger(id) && id > 0 ? id : null;
}

export async function generateMetadata({ params }: PageProps<"/elon/[id]">): Promise<Metadata> {
  const id = parseId((await params).id);
  const data = id ? await getListingPage(id, 0) : null;
  const [t, tc, fmt, names] = await Promise.all([getTranslations("listing"), getTranslations("common"), getFormat(), getCatalogNames()]);
  if (!data) return { title: t("metaFallback") };
  const l = data.listing;
  const product = names.product(l.product_slug, l.product_name);
  const title = `${product}${l.variety ? ` ${l.variety}` : ""} — ${formatSom(l.price_som)} ${tc("perUnit", { unit: fmt.unit(l.unit) })}`;
  const description = [
    `${l.trade_type === "ulgurji" ? tc("wholesale") : tc("retail")} · ${[l.region_name, l.province_name].filter(Boolean).map((n) => names.region(n as string)).join(", ")}`,
    l.diff_percent != null && l.diff_percent < 0 ? t("metaBelow", { percent: Math.abs(l.diff_percent) }) : null,
    l.description,
  ]
    .filter(Boolean)
    .join(". ")
    .slice(0, 200);
  return { title, description, openGraph: { title, description, type: "website", ...previewImages(l.photos[0]) } };
}

export default async function ListingPage({ params }: PageProps<"/elon/[id]">) {
  const id = parseId((await params).id);
  if (!id) notFound();
  const [region, catalog, t, ts, tc, fmt, names] = await Promise.all([
    getRegionId(),
    getCatalogData(),
    getTranslations("listing"),
    getTranslations("store"),
    getTranslations("common"),
    getFormat(),
    getCatalogNames(),
  ]);
  const data = await getListingPage(id, region);
  if (!data) notFound();

  const { listing: l, seller, price } = data;
  const unit = fmt.unit(l.unit);
  const category = catalog.categories.find((c) => c.slug === l.category);
  const categoryName = category ? names.category(category.slug, category.name) : "";
  const product = names.product(l.product_slug, l.product_name);
  const scope = data.price_scope?.name ? names.region(data.price_scope.name) : tc("wholeCountry");
  const trade = l.trade_type === "ulgurji" ? tc("wholesale") : tc("retail");
  const diff = l.diff_percent;
  const title = `${product}${l.variety ? ` · ${l.variety}` : ""}`;
  const compareHref = `/mahsulot/${l.product_slug}${l.trade_type === "chakana" ? "?trade=chakana" : ""}`;
  const analysisParams: Record<string, string> = {};
  if (data.price_scope?.id) analysisParams.region = String(data.price_scope.id);
  if (l.trade_type === "chakana") analysisParams.trade = "chakana";
  const analysisQuery = new URLSearchParams(analysisParams).toString();
  const analysisHref = `/mahsulot/${l.product_slug}/tahlil${analysisQuery ? `?${analysisQuery}` : ""}`;

  const specs: [string, React.ReactNode][] = [
    [t("specs.trade"), trade],
    ...(l.min_order != null ? [[t("specs.minOrder"), `${formatSom(l.min_order)} ${unit}`] as [string, string]] : []),
    ...(l.stock != null ? [[t("specs.stock"), `${formatSom(l.stock)} ${unit}`] as [string, string]] : []),
    [t("specs.region"), [l.region_name, l.province_name].filter(Boolean).map((n) => names.region(n as string)).join(", ")],
    ...(seller.market_name ? [[t("specs.market"), names.market(seller.market_name)] as [string, string]] : []),
    [t("specs.updated"), `${fmt.dayAgo(l.updated_at)} · ${fmt.date(l.updated_at)}`],
    [t("specs.validUntil"), fmt.until(l.valid_until)],
  ];

  return (
    <Page className="py-5 pb-28 sm:py-8 lg:pb-8">
      <nav className="flex flex-wrap items-center gap-1 text-[13px] text-muted-foreground" aria-label={t("breadcrumb")}>
        <Link href="/" className="transition-colors hover:text-foreground">{tc("market")}</Link>
        <ChevronRight className="size-3.5 opacity-60" />
        <Link href={`/?category=${l.category}`} className="transition-colors hover:text-foreground">{categoryName}</Link>
        <ChevronRight className="size-3.5 opacity-60" />
        <Link href={`/?product=${l.product_slug}`} className="transition-colors hover:text-foreground">{product}</Link>
      </nav>

      <div className="mt-5 grid gap-8 lg:grid-cols-[1.2fr_1fr] lg:gap-14">
        <Reveal className="lg:sticky lg:top-24 lg:self-start">
          <ListingGallery photos={l.photos} alt={title} category={l.category} />
        </Reveal>

        <Reveal delay={0.05}>
          <div className="flex items-start gap-3">
            <div className="min-w-0 flex-1">
              <p className="eyebrow">
                {categoryName} · {trade}
              </p>
              <h1 className="display mt-2 text-3xl sm:text-4xl">{title}</h1>
              <p className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-muted-foreground">
                <span className="flex items-center gap-1.5"><MapPin className="size-3.5" />{names.region(l.region_name)}</span>
                <span className="flex items-center gap-1.5"><Eye className="size-3.5" />{t("views", { count: l.views })}</span>
                <span className="num">#{l.id}</span>
              </p>
            </div>
            <ShareButton title={title} />
          </div>

          <div className="mt-7 rounded-3xl border bg-card p-6 shadow-[0_1px_2px_color-mix(in_oklch,var(--foreground)_4%,transparent)]">
            <p className="num text-[2.75rem] leading-none font-semibold tracking-[-0.03em]">
              {formatSom(l.price_som)}
              <span className="ml-2 font-sans text-base font-normal tracking-normal text-muted-foreground">{t("perUnitSpaced", { unit })}</span>
            </p>
            {price.enough ? (
              <div className="mt-5 space-y-4">
                <div className="flex flex-wrap items-center gap-x-3 gap-y-2 text-sm">
                  {diff != null && (
                    <span
                      className={cn(
                        "inline-flex items-center gap-1 rounded-full px-2.5 py-1 font-medium",
                        diff < 0 ? "bg-primary/10 text-primary" : diff > 0 ? "bg-warm-soft text-warm-foreground" : "bg-secondary",
                      )}
                    >
                      {diff < 0 ? <ArrowDownRight className="size-4" /> : diff > 0 ? <ArrowUpRight className="size-4" /> : null}
                      {diff === 0 ? t("atMarket") : t(diff < 0 ? "belowMarket" : "aboveMarket", { percent: Math.abs(diff) })}
                    </span>
                  )}
                  <span className="text-muted-foreground">
                    {t.rich("average", {
                      price: formatSom(price.median ?? 0),
                      scope,
                      b: (chunks) => <span className="num font-medium text-foreground">{chunks}</span>,
                    })}
                  </span>
                </div>
                <MarketBar min={price.min ?? 0} max={price.max ?? 0} median={price.median ?? 0} value={l.price_som} labels={{ median: t("barMedian"), value: t("barThis") }} />
                <p className="text-xs text-muted-foreground">{t("basedOn", { count: price.offers })}</p>
              </div>
            ) : (
              <p className="mt-4 text-sm text-muted-foreground">{t("collecting", { count: price.offers })}</p>
            )}

            <div className="mt-6 grid grid-cols-2 gap-2.5">
              <Link
                href={compareHref}
                className="group relative flex min-h-28 flex-col justify-between gap-3 rounded-2xl bg-muted/70 p-4 transition-colors hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
              >
                <span className="flex items-center justify-between">
                  <span className="grid size-9 place-items-center rounded-xl bg-card shadow-xs">
                    <Scale strokeWidth={1.75} className="size-[18px]" />
                  </span>
                  <ArrowUpRight className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5" aria-hidden />
                </span>
                <span>
                  <span className="block text-sm leading-snug font-semibold">{t("tools.compare")}</span>
                  <span className="mt-0.5 block text-xs leading-snug text-muted-foreground">{t("tools.compareHint")}</span>
                </span>
              </Link>
              <Link
                href={analysisHref}
                className="group relative flex min-h-28 flex-col justify-between gap-3 overflow-hidden rounded-2xl bg-primary/10 p-4 text-primary ring-1 ring-primary/15 transition-colors ring-inset hover:bg-primary/15 focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none dark:bg-primary/15 dark:hover:bg-primary/20"
              >
                <TrendSketch />
                <span className="relative flex items-center justify-between">
                  <span className="grid size-9 place-items-center rounded-xl bg-primary text-primary-foreground">
                    <ChartSpline strokeWidth={1.75} className="size-[18px]" />
                  </span>
                  <ArrowUpRight className="size-4 opacity-80 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5" aria-hidden />
                </span>
                <span className="relative">
                  <span className="block text-sm leading-snug font-semibold">{t("tools.analysis")}</span>
                  <span className="mt-0.5 block text-xs leading-snug text-foreground/60">{t("tools.analysisHint")}</span>
                </span>
              </Link>
            </div>

            <div className="mt-3 hidden lg:flex">
              <ContactButton block listingId={l.id} hasTelegram={seller.has_telegram} hasPhone={seller.has_phone} />
            </div>

            <div className="mt-6 border-t pt-3">
              <Link
                href={`/sotuvchi/${seller.id}`}
                title={ts("storeOf", { seller: seller.name })}
                className="group -mx-3 flex items-center gap-3 rounded-2xl px-3 py-2 transition-colors hover:bg-muted/60 focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
              >
                <span className="grid size-11 shrink-0 place-items-center rounded-full bg-secondary text-base font-semibold">
                  {seller.name.slice(0, 1)}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium underline-offset-4 group-hover:underline">{seller.name}</p>
                  <p className="text-xs text-muted-foreground">
                    {t("sellerSince", { date: fmt.date(seller.since), count: seller.listings })}
                  </p>
                  {seller.rating && (
                    <p className="mt-1 flex items-center gap-1 text-sm font-medium">
                      <Star className="size-4 fill-warm text-warm" aria-hidden />
                      <span className="num">{t("rating.summary", { average: fmt.decimal(seller.rating.average, 1), count: seller.rating.count })}</span>
                    </p>
                  )}
                </div>
                <Store strokeWidth={1.5} className="size-5 text-muted-foreground" />
                <ChevronRight className="-ml-1 size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
              </Link>
            </div>
            {seller.rating && seller.rating.comments.length > 0 && (
              <ul className="mt-4 space-y-3" aria-label={t("rating.title")}>
                {seller.rating.comments.map((c) => (
                  <li key={c.created_at} className="rounded-2xl bg-muted/60 px-4 py-3 text-sm">
                    <div className="flex items-center justify-between gap-2">
                      <span className="flex gap-0.5" role="img" aria-label={t("rating.stars", { stars: c.stars })}>
                        {[1, 2, 3, 4, 5].map((i) => (
                          <Star key={i} className={cn("size-3.5", i <= c.stars ? "fill-warm text-warm" : "text-muted-foreground/40")} />
                        ))}
                      </span>
                      <span className="text-xs text-muted-foreground">{fmt.dayAgo(c.created_at)}</span>
                    </div>
                    {c.comment && <p className="mt-1.5 leading-relaxed">{c.comment}</p>}
                  </li>
                ))}
              </ul>
            )}
          </div>

          <p className="mt-3 flex items-start gap-2 px-1 text-xs leading-relaxed text-muted-foreground">
            <ShieldCheck className="mt-px size-3.5 shrink-0" />
            {t("safety")}
          </p>

          <section className="mt-10">
            <h2 className="text-lg font-semibold tracking-tight">{t("details")}</h2>
            <dl className="mt-3 divide-y border-y text-sm">
              {specs.map(([k, v]) => (
                <div key={k} className="flex justify-between gap-4 py-3">
                  <dt className="text-muted-foreground">{k}</dt>
                  <dd className="text-right font-medium">{v}</dd>
                </div>
              ))}
            </dl>
          </section>

          {l.description && (
            <section className="mt-10">
              <h2 className="text-lg font-semibold tracking-tight">{t("description")}</h2>
              <p className="mt-3 leading-relaxed whitespace-pre-line text-muted-foreground">{l.description}</p>
            </section>
          )}

        </Reveal>
      </div>

      <div className="app-bottom-bar fixed inset-x-0 bottom-0 z-40 border-t bg-background px-4 pt-3 pb-[max(0.75rem,var(--app-safe-bottom))] transition-transform duration-200 ease-out lg:hidden">
        <div className="flex items-center justify-between gap-3">
          <p className="num min-w-0 truncate text-xl font-semibold">
            {formatSom(l.price_som)}
            <span className="ml-1 font-sans text-xs font-normal text-muted-foreground">{tc("perUnit", { unit })}</span>
          </p>
          <ContactButton listingId={l.id} hasTelegram={seller.has_telegram} hasPhone={seller.has_phone} />
        </div>
      </div>

      {data.other_offers.length > 0 && (
        <Rail title={t("otherSellers")} hint={t("otherSellersHint")} allLabel={t("all")} href={`/?product=${l.product_slug}&sort=price_asc`}>
          {data.other_offers.map((o) => (
            <ListingCard key={o.id} l={o} />
          ))}
        </Rail>
      )}
      {data.from_seller.length > 0 && (
        <Rail title={t("fromSeller", { seller: seller.name })} allLabel={t("all")} href={`/sotuvchi/${seller.id}`}>
          {data.from_seller.map((o) => (
            <ListingCard key={o.id} l={o} />
          ))}
        </Rail>
      )}
    </Page>
  );
}

/** A faint price line running into a dashed forecast, behind the analysis tile's text. */
function TrendSketch() {
  return (
    <svg aria-hidden viewBox="0 0 160 64" preserveAspectRatio="none" className="pointer-events-none absolute top-3.5 right-12 left-16 h-9 opacity-40">
      <path d="M0 50 C14 46 20 52 32 44 S52 30 64 36 S84 22 98 26 S112 14 120 18" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      <path d="M120 18 C132 22 142 10 160 8" fill="none" stroke="currentColor" strokeWidth="2" strokeDasharray="4 4" strokeLinecap="round" />
      <circle cx="120" cy="18" r="3" fill="currentColor" />
    </svg>
  );
}

/** Where this price sits in the market's min–max range. */
function MarketBar({
  min,
  max,
  median,
  value,
  labels,
}: {
  min: number;
  max: number;
  median: number;
  value: number;
  labels: { median: string; value: string };
}) {
  const lo = Math.min(min, value);
  const hi = Math.max(max, value);
  const span = Math.max(1, hi - lo);
  const pos = (v: number) => `${(100 * (v - lo)) / span}%`;
  return (
    <div>
      <div className="relative h-1.5 rounded-full bg-muted">
        <span className="absolute top-1/2 h-3.5 w-px -translate-y-1/2 bg-foreground/40" style={{ left: pos(median) }} title={labels.median} />
        <span
          className="absolute top-1/2 size-3.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary ring-4 ring-card"
          style={{ left: pos(value) }}
          title={labels.value}
        />
      </div>
      <div className="num mt-2 flex justify-between text-xs text-muted-foreground">
        <span>{formatSom(lo)}</span>
        <span>{formatSom(hi)}</span>
      </div>
    </div>
  );
}

import { CalendarDays, ChevronRight, MapPin, ShieldCheck, Star, Store, TrendingUp } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { categoryIcons } from "@/components/category-icon";
import { EmptyState } from "@/components/empty-state";
import { Reveal } from "@/components/motion";
import { Page } from "@/components/page";
import { ShareButton } from "@/components/share-button";
import { StoreListings } from "@/components/store/store-listings";
import { StoreReviews } from "@/components/store/store-reviews";
import { buttonVariants } from "@/components/ui/button";
import { getCatalogNames } from "@/lib/i18n/catalog.server";
import { getFormat } from "@/lib/i18n/format.server";
import { previewImages } from "@/lib/media";
import { getSellerPage } from "@/lib/server/sellers";
import { getCatalogData } from "@/lib/server-api";

/** "Faol sotuvchi": set a price within this many days. */
const ACTIVE_DAYS = 7;

export async function generateMetadata({ params }: PageProps<"/sotuvchi/[id]">): Promise<Metadata> {
  const data = await getSellerPage((await params).id);
  const [t, names] = await Promise.all([getTranslations("store"), getCatalogNames()]);
  if (!data) return { title: t("metaFallback") };
  const s = data.seller;
  const title = t("metaTitle", { name: s.name });
  const place = [s.market_name && names.market(s.market_name), s.region_name && names.region(s.region_name)].filter(Boolean).join(", ");
  const description = t("metaDescription", { name: s.name, place, count: s.listings });
  const cover = data.listings.find((l) => l.cover_key)?.cover_key;
  return { title, description, openGraph: { title, description, type: "profile", ...previewImages(cover) } };
}

export default async function SellerPage({ params }: PageProps<"/sotuvchi/[id]">) {
  const [data, catalog, t, tc, fmt, names] = await Promise.all([
    getSellerPage((await params).id),
    getCatalogData(),
    getTranslations("store"),
    getTranslations("common"),
    getFormat(),
    getCatalogNames(),
  ]);
  if (!data) notFound();

  const { seller: s, listings } = data;
  const place = [s.region_name, s.province_name].filter(Boolean).map((n) => names.region(n as string)).join(", ");
  const categories = catalog.categories
    .filter((c) => s.categories.includes(c.slug))
    .map((c) => ({ slug: c.slug, name: names.category(c.slug, c.name) }));
  const listed = categories.filter((c) => listings.some((l) => l.category === c.slug));
  const active = s.last_active != null && new Date().getTime() - Date.parse(s.last_active) < ACTIVE_DAYS * 86_400_000;

  const stats: { label: string; value: React.ReactNode }[] = [
    { label: t("stats.listings"), value: s.listings },
    { label: t("stats.products"), value: s.products },
    { label: t("stats.lastActive"), value: s.last_active ? fmt.dayAgo(s.last_active) : t("stats.never") },
    {
      label: t("stats.rating"),
      value: s.rating ? (
        <span className="inline-flex items-center gap-1.5">
          <Star className="size-5 fill-warm text-warm" aria-hidden />
          {fmt.decimal(s.rating.average, 1)}
        </span>
      ) : (
        t("stats.noRating")
      ),
    },
  ];

  return (
    <Page className="py-5 sm:py-8">
      <nav className="flex flex-wrap items-center gap-1 text-[13px] text-muted-foreground" aria-label={t("breadcrumb")}>
        <Link href="/" className="transition-colors hover:text-foreground">{tc("market")}</Link>
        <ChevronRight className="size-3.5 opacity-60" />
        <span className="max-w-[60vw] truncate text-foreground" aria-current="page">{s.name}</span>
      </nav>

      <Reveal className="mt-5 rounded-3xl border bg-card p-5 shadow-[0_1px_2px_color-mix(in_oklch,var(--foreground)_4%,transparent)] sm:p-7">
        <div className="grid gap-6 lg:grid-cols-[1fr_auto] lg:gap-10">
          <div className="flex min-w-0 items-start gap-4 sm:gap-5">
            <span
              className="display grid size-16 shrink-0 place-items-center rounded-full bg-secondary text-3xl sm:size-20 sm:text-4xl"
              aria-hidden
            >
              {s.name.slice(0, 1).toUpperCase()}
            </span>
            <div className="min-w-0 flex-1">
              <div className="flex items-start gap-3">
                <div className="min-w-0 flex-1">
                  <p className="eyebrow">{t("eyebrow")}</p>
                  <h1 className="display mt-1.5 text-3xl break-words sm:text-4xl">{s.name}</h1>
                </div>
                <ShareButton title={t("metaTitle", { name: s.name })} />
              </div>
              <p className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-sm text-muted-foreground">
                {place && (
                  <span className="flex items-center gap-1.5"><MapPin className="size-3.5 shrink-0" />{place}</span>
                )}
                {s.market_name && (
                  <span className="flex items-center gap-1.5"><Store className="size-3.5 shrink-0" />{names.market(s.market_name)}</span>
                )}
                <span className="flex items-center gap-1.5"><CalendarDays className="size-3.5 shrink-0" />{fmt.since(s.since)}</span>
              </p>
              {(s.rating || active) && (
                <div className="mt-4 flex flex-wrap items-center gap-2 text-sm">
                  {s.rating && (
                    <a
                      href="#reviews"
                      className="inline-flex h-8 items-center gap-1.5 rounded-full bg-warm-soft px-3 font-medium text-warm-foreground transition-opacity hover:opacity-80"
                    >
                      <Star className="size-4 fill-current" aria-hidden />
                      <span className="num">{t("ratingSummary", { average: fmt.decimal(s.rating.average, 1), count: s.rating.count })}</span>
                    </a>
                  )}
                  {active && (
                    <span
                      className="inline-flex h-8 items-center gap-1.5 rounded-full bg-primary/10 px-3 font-medium text-primary"
                      title={t("activeBadgeHint")}
                    >
                      <TrendingUp className="size-4" aria-hidden />
                      {t("activeBadge")}
                    </span>
                  )}
                </div>
              )}
            </div>
          </div>

          <dl className="grid grid-cols-2 gap-px self-start overflow-hidden rounded-2xl border bg-border lg:w-[22rem]">
            {stats.map((st) => (
              <div key={st.label} className="bg-card px-4 py-3.5">
                <dt className="text-xs text-muted-foreground">{st.label}</dt>
                <dd className="num mt-1 text-xl font-semibold tracking-tight">{st.value}</dd>
              </div>
            ))}
          </dl>
        </div>

        {categories.length > 0 && (
          <div className="mt-6 flex flex-wrap items-center gap-2 border-t pt-5">
            <span className="sr-only">{t("categories")}</span>
            {categories.map((c) => {
              const Icon = categoryIcons[c.slug];
              return (
                <span key={c.slug} className="inline-flex h-8 items-center gap-1.5 rounded-full bg-muted px-3 text-sm">
                  {Icon && <Icon className="size-4 text-muted-foreground" strokeWidth={1.75} aria-hidden />}
                  {c.name}
                </span>
              );
            })}
          </div>
        )}
        <p className="mt-5 flex items-start gap-2 text-xs leading-relaxed text-muted-foreground">
          <ShieldCheck className="mt-px size-3.5 shrink-0" />
          {t("contactNote")}
        </p>
      </Reveal>

      <section className="pt-12 sm:pt-16" aria-label={t("listingsTitle")}>
        {listings.length > 0 ? (
          <StoreListings listings={listings} categories={listed} />
        ) : (
          <EmptyState
            icon={<Store strokeWidth={1.5} />}
            title={t("emptyTitle")}
            description={t("emptyText")}
            action={
              <Link href="/" className={buttonVariants({ variant: "solid", size: "pill" })}>
                {t("toMarket")}
              </Link>
            }
          />
        )}
      </section>

      {s.rating && <StoreReviews rating={s.rating} />}
    </Page>
  );
}

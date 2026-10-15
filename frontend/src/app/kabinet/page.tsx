import { BarChart3, ChevronRight, Clock, MessageSquareText, Plus, ShieldCheck, ShoppingBasket, Store } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { WeeklyReport } from "@/components/cabinet/weekly-report";
import { Reveal } from "@/components/motion";
import { Page } from "@/components/page";
import { PageHeader } from "@/components/page-header";
import { RfqInbox } from "@/components/rfq/rfq-inbox";
import { buttonVariants } from "@/components/ui/button";
import { getFormat } from "@/lib/i18n/format.server";
import { listingStats } from "@/lib/listings";
import { isEntrepreneur, isMarketAdmin } from "@/lib/roles";
import { getRfqInbox } from "@/lib/server/rfq";
import { getCatalogData, getCurrentUser, getMyListings } from "@/lib/server-api";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("cabinet");
  return { title: t("metaTitle"), robots: { index: false } };
}

export default async function CabinetPage() {
  const me = await getCurrentUser();
  if (!me) redirect("/kirish?keyin=/kabinet");
  if (!me.onboarded) redirect("/boshlash");

  // Entrepreneurs sell here (incoming requests, listings) and buy on /sorovlar.
  const seller = isEntrepreneur(me.role);
  const privileged = isMarketAdmin(me.role);
  const reports = me.is_admin || (privileged && me.role_approved);
  const [t, tp, fmt, catalog, inbox, listings] = await Promise.all([
    getTranslations("cabinet"),
    getTranslations("profile.page"),
    getFormat(),
    getCatalogData(),
    seller ? getRfqInbox() : Promise.resolve([]),
    seller ? getMyListings() : Promise.resolve([]),
  ]);

  const provinces = catalog.regions.filter((r) => r.parent_id == null).map((r) => ({ id: r.id, name: r.name }));
  const district = catalog.regions.find((r) => r.id === me.region_id);
  const ownProvince = district?.parent_id ?? district?.id;
  // A market administration reports on its own province only; everyone else may pick.
  const reportProvinces = privileged && !me.is_admin ? provinces.filter((p) => p.id === ownProvince) : provinces;
  const stats = listingStats(listings);

  return (
    <Page className="max-w-6xl py-10 sm:py-14">
      <PageHeader
        eyebrow={me.role ? fmt.role(me.role) : undefined}
        title={t("title")}
        description={seller ? t("sellerDescription") : t("description")}
      />

      {seller && (
        <div className="mt-10 grid gap-8 lg:grid-cols-[minmax(0,1fr)_19rem] lg:gap-10">
          <Reveal delay={0.05} className="min-w-0">
            <RfqInbox initial={inbox} />
          </Reveal>
          <Reveal delay={0.1} className="space-y-3 lg:sticky lg:top-24 lg:self-start">
            <Link href="/mahsulotlarim" className="group block rounded-3xl border bg-card p-5 transition-colors hover:border-foreground/25">
              <div className="flex items-center gap-3">
                <span className="grid size-11 shrink-0 place-items-center rounded-2xl bg-muted">
                  <Store strokeWidth={1.5} className="size-5" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block font-semibold tracking-tight">{t("listings.title")}</span>
                  <span className="text-sm text-muted-foreground">{t("listings.live", { count: stats.live, total: stats.total })}</span>
                </span>
                <ChevronRight className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
              </div>
              {stats.attention > 0 && <p className="mt-3 rounded-xl bg-warm-soft px-3 py-2 text-xs text-warm-foreground">{t("listings.attention", { count: stats.attention })}</p>}
            </Link>
            <Link href="/sorovlar" className="group block rounded-3xl border bg-card p-5 transition-colors hover:border-foreground/25">
              <div className="flex items-center gap-3">
                <span className="grid size-11 shrink-0 place-items-center rounded-2xl bg-muted">
                  <ShoppingBasket strokeWidth={1.5} className="size-5" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block font-semibold tracking-tight">{t("buying.title")}</span>
                  <span className="text-sm text-muted-foreground">{t("buying.text")}</span>
                </span>
                <ChevronRight className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
              </div>
            </Link>
            <div className="flex gap-2">
              <Link href="/mahsulotlarim/yangi" className={buttonVariants({ variant: "solid", size: "pill", className: "flex-1" })}>
                <Plus /> {t("listings.new")}
              </Link>
              <Link href="/narx" className={buttonVariants({ variant: "outline", size: "pill", className: "flex-1" })}>
                <MessageSquareText /> {t("listings.text")}
              </Link>
            </div>
            <div className="rounded-3xl border border-dashed p-5 text-sm leading-relaxed text-muted-foreground">
              <p className="font-medium text-foreground">{t("how.title")}</p>
              <ol className="mt-2 list-decimal space-y-1 pl-4">
                <li>{t("how.one")}</li>
                <li>{t("how.two")}</li>
                <li>{t("how.three")}</li>
              </ol>
            </div>
          </Reveal>
        </div>
      )}

      {privileged && !me.role_approved && !me.is_admin && (
        <Reveal delay={0.05} className="mt-10">
          <section className="flex flex-col items-start gap-4 rounded-3xl border bg-card p-6 sm:flex-row sm:p-8">
            <span className="grid size-12 shrink-0 place-items-center rounded-2xl bg-warm-soft text-warm-foreground">
              <Clock className="size-5" />
            </span>
            <div className="min-w-0">
              <h2 className="text-lg font-semibold tracking-tight">{tp("pendingApproval")}</h2>
              <p className="mt-1 max-w-xl text-sm leading-relaxed text-muted-foreground">{t("pending.text")}</p>
              <Link href="/statistika" className={buttonVariants({ variant: "outline", size: "pill", className: "mt-5" })}>
                <BarChart3 /> {t("pending.statistics")}
              </Link>
            </div>
          </section>
        </Reveal>
      )}

      {reports && (
        <Reveal delay={0.08} className="mt-10 grid gap-4 lg:grid-cols-[minmax(0,1fr)_19rem]">
          <WeeklyReport provinces={reportProvinces} initial={ownProvince} />
          {me.is_admin && (
            <Link href="/admin" className="group flex items-center gap-3 self-start rounded-3xl border bg-card p-5 transition-colors hover:border-foreground/25">
              <span className="grid size-11 shrink-0 place-items-center rounded-2xl bg-muted">
                <ShieldCheck strokeWidth={1.5} className="size-5" />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block font-semibold tracking-tight">{t("admin.title")}</span>
                <span className="text-sm text-muted-foreground">{t("admin.text")}</span>
              </span>
              <ChevronRight className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
            </Link>
          )}
        </Reveal>
      )}
    </Page>
  );
}

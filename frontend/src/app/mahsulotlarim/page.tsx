import { MessageSquareText, Plus, Store } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { EmptyState } from "@/components/empty-state";
import { Reveal } from "@/components/motion";
import { MyListings } from "@/components/my-listings";
import { Page } from "@/components/page";
import { PageHeader } from "@/components/page-header";
import { Stat } from "@/components/stat";
import { buttonVariants } from "@/components/ui/button";
import { formatSom } from "@/lib/labels";
import { listingStats } from "@/lib/listings";
import { isEntrepreneur } from "@/lib/roles";
import { getCurrentUser, getMyListings } from "@/lib/server-api";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("mine");
  return { title: t("meta.title") };
}

export default async function MyListingsPage() {
  const me = await getCurrentUser();
  if (!me) redirect("/kirish?keyin=/mahsulotlarim");
  if (!me.onboarded) redirect("/boshlash");
  if (!isEntrepreneur(me.role)) redirect("/kabinet");

  const [listings, t] = await Promise.all([getMyListings(), getTranslations("mine")]);
  const stats = listingStats(listings);

  return (
    <Page className="max-w-6xl py-10 sm:py-14">
      <PageHeader
        eyebrow={t("eyebrow")}
        title={t("title")}
        description={t("description")}
        action={
          <div className="flex gap-2">
            <Link href="/narx" className={buttonVariants({ variant: "outline", size: "pill", className: "h-11" })}>
              <MessageSquareText /> {t("textPrice")}
            </Link>
            <Link href="/mahsulotlarim/yangi" className={buttonVariants({ variant: "solid", size: "pill", className: "h-11" })}>
              <Plus /> {t("newListing")}
            </Link>
          </div>
        }
      />

      {listings.length === 0 ? (
        <Reveal delay={0.05} className="mt-10">
          <EmptyState
            icon={<Store strokeWidth={1.5} />}
            title={t("empty.title")}
            description={t("empty.description")}
            action={
              <Link href="/mahsulotlarim/yangi" className={buttonVariants({ variant: "solid", size: "pill", className: "h-11 px-6" })}>
                <Plus /> {t("empty.action")}
              </Link>
            }
          />
        </Reveal>
      ) : (
        <>
          <Reveal delay={0.04} className="mt-10 grid grid-cols-2 gap-3 lg:grid-cols-4">
            <Stat label={t("stats.live")} value={stats.live} hint={t("stats.liveHint", { count: stats.total })} />
            <Stat label={t("stats.views")} value={formatSom(stats.views)} hint={t("stats.viewsHint")} />
            <Stat label={t("stats.attention")} value={stats.attention} hint={t("stats.attentionHint")} />
            <Stat label={t("stats.review")} value={stats.review} hint={t("stats.reviewHint")} />
          </Reveal>
          <Reveal delay={0.08} className="mt-8">
            <MyListings listings={listings} />
          </Reveal>
        </>
      )}
    </Page>
  );
}

import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { AdminBoard } from "@/components/admin-board";
import { Reveal } from "@/components/motion";
import { Page } from "@/components/page";
import { PageHeader } from "@/components/page-header";
import { Stat } from "@/components/stat";
import { formatSom } from "@/lib/labels";
import { getAdminBoard } from "@/lib/server/admin";
import { getCurrentUser } from "@/lib/server-api";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("admin");
  return { title: t("meta.title"), robots: { index: false } };
}

export default async function AdminPage({ searchParams }: PageProps<"/admin">) {
  const me = await getCurrentUser();
  // Not advertised to anyone else, signed in or not: the page simply does not exist for them.
  if (!me?.is_admin) notFound();
  const [data, t, { tab }] = await Promise.all([getAdminBoard(), getTranslations("admin"), searchParams]);
  // The visitor is an admin, so a failed call is an outage or a rate limit, not a missing page.
  if (!data) throw new Error("Could not load the admin data");
  const { summary, ...board } = data;

  return (
    <Page className="max-w-6xl py-10 sm:py-14">
      <PageHeader
        eyebrow={t("eyebrow")}
        title={t("title")}
        description={t("description")}
      />
      <Reveal delay={0.04} className="mt-10 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat label={t("stats.review")} value={summary.review_listings} hint={t("stats.reviewHint")} />
        <Stat label={t("stats.roles")} value={summary.pending_roles} hint={t("stats.rolesHint")} />
        <Stat label={t("stats.live")} value={formatSom(summary.live_listings)} hint={t("stats.liveHint")} />
        <Stat label={t("stats.users")} value={formatSom(summary.users)} hint={t("stats.usersHint", { count: summary.sellers })} />
      </Reveal>
      <Reveal delay={0.08} className="mt-8">
        <AdminBoard data={board} initialTab={typeof tab === "string" ? tab : undefined} />
      </Reveal>
    </Page>
  );
}

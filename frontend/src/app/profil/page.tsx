import { BadgeCheck, CalendarDays, Clock, MapPin, Phone, Plus, Store } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { BotCard } from "@/components/bot-card";
import { ContactPrefs } from "@/components/contact-prefs";
import { DeleteAccount } from "@/components/delete-account";
import { Reveal } from "@/components/motion";
import { Page } from "@/components/page";
import { ProfileForm } from "@/components/profile-form";
import { SectionNav, SettingsSection } from "@/components/settings";
import { WatchList } from "@/components/watch-list";
import { Stat } from "@/components/stat";
import { buttonVariants } from "@/components/ui/button";
import { getCatalogNames } from "@/lib/i18n/catalog.server";
import { getFormat } from "@/lib/i18n/format.server";
import { formatSom } from "@/lib/labels";
import { listingStats } from "@/lib/listings";
import { isEntrepreneur, isMarketAdmin } from "@/lib/roles";
import { getCatalogData, getCurrentUser, getLimits, getMyListings } from "@/lib/server-api";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("profile.page");
  return { title: t("metaTitle") };
}

function initials(name: string) {
  return name
    .split(/\s+/)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() ?? "")
    .join("");
}

export default async function ProfilePage() {
  const [me, catalog] = await Promise.all([getCurrentUser(), getCatalogData()]);
  if (!me) redirect("/kirish?keyin=/profil");
  if (!me.onboarded) redirect("/boshlash");
  const [t, tw, fmt, names, limits] = await Promise.all([
    getTranslations("profile.page"),
    getTranslations("watch.list"),
    getFormat(),
    getCatalogNames(),
    getLimits(),
  ]);

  const seller = isEntrepreneur(me.role);
  const stats = seller ? listingStats(await getMyListings()) : null;
  const district = catalog.regions.find((r) => r.id === me.region_id);
  const province = catalog.regions.find((r) => r.id === district?.parent_id);
  const region = [district?.name, province?.name].filter((n): n is string => Boolean(n)).map(names.region).join(", ");
  const marketName = catalog.markets.find((m) => m.id === me.market_id)?.name;
  const market = marketName && names.market(marketName);
  const privileged = isMarketAdmin(me.role);

  const sections = [
    { id: "malumotlar", label: t("nav.details") },
    ...(seller ? [{ id: "aloqa", label: t("nav.contact") }] : []),
    { id: "kuzatuv", label: tw("title") },
    { id: "bot", label: t("nav.bot") },
    { id: "hisob", label: t("nav.account") },
  ];

  return (
    <Page className="max-w-6xl py-8 sm:py-12">
      <Reveal className="flex flex-col gap-6 rounded-3xl border bg-card p-6 sm:flex-row sm:items-center sm:p-8">
        <span className="grid size-16 shrink-0 place-items-center rounded-full bg-foreground text-xl font-semibold text-background sm:size-20 sm:text-2xl">
          {initials(me.public_name || me.display_name)}
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="display truncate text-2xl sm:text-3xl">{me.public_name || me.display_name}</h1>
            {me.role && <span className="rounded-full bg-muted px-2.5 py-0.5 text-xs font-medium">{fmt.role(me.role)}</span>}
            {privileged && (
              <span className="flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-xs">
                {me.role_approved ? <BadgeCheck className="size-3" /> : <Clock className="size-3" />}
                {me.role_approved ? t("approved") : t("pendingApproval")}
              </span>
            )}
          </div>
          <p className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-muted-foreground">
            {(region || market) && (
              <span className="flex items-center gap-1.5">
                <MapPin className="size-3.5" /> {[region, market].filter(Boolean).join(" · ")}
              </span>
            )}
            {me.phone_masked && (
              <span className="flex items-center gap-1.5">
                <Phone className="size-3.5" /> <span className="num">{me.phone_masked}</span>
              </span>
            )}
            <span className="flex items-center gap-1.5">
              <CalendarDays className="size-3.5" /> {fmt.since(me.created_at)}
            </span>
          </p>
        </div>
        {seller && (
          <div className="flex flex-wrap gap-2">
            <Link href={`/sotuvchi/${me.id}`} className={buttonVariants({ variant: "outline", size: "pill" })}>
              <Store /> {t("myStore")}
            </Link>
            <Link href="/mahsulotlarim" className={buttonVariants({ variant: "outline", size: "pill" })}>
              {t("myProducts")}
            </Link>
            <Link href="/mahsulotlarim/yangi" className={buttonVariants({ variant: "solid", size: "pill" })}>
              <Plus /> {t("newListing")}
            </Link>
          </div>
        )}
      </Reveal>

      {stats && (
        <Reveal delay={0.04} className="mt-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
          <Stat label={t("stats.live")} value={stats.live} hint={t("stats.liveHint", { total: stats.total })} />
          <Stat label={t("stats.views")} value={formatSom(stats.views)} hint={t("stats.viewsHint")} />
          <Stat label={t("stats.attention")} value={stats.attention} hint={t("stats.attentionHint")} />
          <Stat label={t("stats.review")} value={stats.review} hint={t("stats.reviewHint")} />
        </Reveal>
      )}

      <div className="mt-10 grid gap-8 lg:grid-cols-[13rem_1fr] lg:gap-12">
        <aside className="lg:sticky lg:top-24 lg:self-start">
          <SectionNav items={sections} />
        </aside>
        <div className="min-w-0 space-y-6">
          <SettingsSection id="malumotlar" title={t("details")} description={t("detailsHint")}>
            <ProfileForm catalog={catalog} me={me} submitLabel={t("saveChanges")} />
          </SettingsSection>
          {seller && (
            <SettingsSection id="aloqa" title={t("contact")} description={t("contactHint")}>
              <ContactPrefs
                telegram={me.contact_telegram}
                phone={me.contact_phone}
                username={me.telegram_username}
                hasPhone={Boolean(me.phone_masked)}
              />
            </SettingsSection>
          )}
          <SettingsSection id="kuzatuv" title={tw("title")} description={tw("hint", { max: limits.watches_max })}>
            <WatchList />
          </SettingsSection>
          <SettingsSection id="bot" title={t("bot")} description={t("botHint")}>
            <BotCard enabled={me.bot_enabled} botUsername={process.env.NEXT_PUBLIC_TELEGRAM_BOT_USERNAME ?? ""} />
          </SettingsSection>
          <SettingsSection id="hisob" title={t("deleteTitle")} description={t("deleteHint")} danger>
            <DeleteAccount />
          </SettingsSection>
        </div>
      </div>
    </Page>
  );
}

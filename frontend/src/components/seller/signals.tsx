"use client";

import { Bot, Eye, Handshake, Hourglass, Pause, Phone, Trophy, XCircle } from "lucide-react";
import { useTranslations } from "next-intl";

import type { MyListing } from "@/lib/api/gen/types.gen";
import { type Format } from "@/lib/i18n/format";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { pausedUntil } from "@/lib/listings";
import { cn } from "@/lib/utils";

/** "14:12", or "27-sen, 14:12" when it is not today in Tashkent. */
export function clockOrDay(fmt: Format, iso: string) {
  const sameDay = fmt.dayMonth(iso) === fmt.dayMonth(new Date().toISOString());
  return sameDay ? fmt.time(iso) : `${fmt.dayMonth(iso)}, ${fmt.time(iso)}`;
}

/** "9-o'rin / 12": the listing's rank among the province's sellers (1 = cheapest). */
export function PositionChip({ l, className }: { l: MyListing; className?: string }) {
  const t = useTranslations("mine.signals");
  const fmt = useFormat();
  const p = l.position;
  if (!p) return null;
  const hint = t("positionHint", {
    total: p.total,
    price: formatSom(p.median),
    diff: fmt.percent(p.diff_percent / 100, { signed: true, digits: 0 }),
  });
  return (
    <span
      title={hint}
      aria-label={`${t("position", { rank: p.rank, total: p.total })}. ${hint}`}
      className={cn("inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs font-medium whitespace-nowrap", className)}
    >
      <Trophy className="size-3 text-muted-foreground" aria-hidden />
      <span className="num">{t("position", { rank: p.rank, total: p.total })}</span>
    </span>
  );
}

/** "Avto 5 500 – 7 000", plus "Pauza: 14:12 gacha" while a price set by hand holds it. */
export function AutoBadge({ l, className }: { l: MyListing; className?: string }) {
  const t = useTranslations("mine.signals");
  const fmt = useFormat();
  if (l.pricing.mode !== "auto") return null;
  const { price_min: min, price_max: max } = l.pricing;
  const paused = pausedUntil(l);
  return (
    <span className={cn("inline-flex flex-wrap items-center gap-1.5", className)}>
      <span
        title={t("autoHint")}
        className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium whitespace-nowrap text-primary"
      >
        <Bot className="size-3" aria-hidden />
        {t("auto")}
        {min != null && max != null && <span className="num font-normal">{t("range", { min: formatSom(min), max: formatSom(max) })}</span>}
      </span>
      {paused && (
        <span className="inline-flex items-center gap-1 text-xs whitespace-nowrap text-warm-foreground">
          <Pause className="size-3" aria-hidden />
          {t("paused", { time: clockOrDay(fmt, paused) })}
        </span>
      )}
    </span>
  );
}

/** Unique views, contacts and deals over the last 7 days. */
export function Funnel({ l, className }: { l: MyListing; className?: string }) {
  const t = useTranslations("mine.signals");
  const f = l.funnel_7d ?? { views: 0, contacts: 0, deals: 0 };
  const items = [
    { icon: Eye, value: f.views, key: "views" },
    { icon: Phone, value: f.contacts, key: "contacts" },
    { icon: Handshake, value: f.deals, key: "deals" },
  ];
  return (
    <span
      role="img"
      aria-label={t("funnelLabel", f)}
      title={t("funnelLabel", f)}
      className={cn("inline-flex items-center gap-2.5 text-xs text-muted-foreground", className)}
    >
      {items.map(({ icon: Icon, value, key }) => (
        <span key={key} className={cn("inline-flex items-center gap-1", value > 0 && "text-foreground")}>
          <Icon className="size-3.5 text-muted-foreground" aria-hidden />
          <span className="num">{formatSom(value)}</span>
        </span>
      ))}
    </span>
  );
}

/** Why a listing waits for review, or what to do with a rejected one. Nothing for other listings. */
export function StatusNote({ l, className }: { l: MyListing; className?: string }) {
  const t = useTranslations("mine.signals");
  const tc = useTranslations("codes");
  if (l.status === "rejected") {
    return (
      <p className={cn("flex gap-2 rounded-2xl bg-destructive/5 px-3.5 py-2.5 text-xs text-destructive", className)}>
        <XCircle className="mt-px size-3.5 shrink-0" aria-hidden />
        <span>
          <span className="font-medium">{t("rejectedTitle")}</span> {t("rejectedHint")}
          {l.review_note && <span className="mt-1 block">{t("adminNote", { note: l.review_note })}</span>}
        </span>
      </p>
    );
  }
  if (l.status !== "review") return null;
  const reason = l.review_reason;
  return (
    <p className={cn("flex gap-2 rounded-2xl bg-warm-soft px-3.5 py-2.5 text-xs text-warm-foreground", className)}>
      <Hourglass className="mt-px size-3.5 shrink-0" aria-hidden />
      <span>
        <span className="font-medium">{reason ? t("reviewTitle", { reason: tc(`reviewReason.${reason}.title`) }) : t("reviewTitleNoReason")}</span>{" "}
        {reason && `${tc(`reviewReason.${reason}.hint`)} `}
        {t("reviewHint")}
      </span>
    </p>
  );
}

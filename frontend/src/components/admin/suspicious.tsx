"use client";

import { ChevronDown, Info, UserSearch, Users } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { EmptyState } from "@/components/empty-state";
import { ease } from "@/components/motion";
import type { SellerFeatures, SuspiciousOutputBody, SuspiciousSeller } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { cn } from "@/lib/utils";

// Feature rows in the detail, in the order the forest reads them.
const featureKeys = ["prices", "review_share", "deviation", "below_share", "above_share", "per_listing", "products", "max_listings_day"] as const;
type FeatureKey = (typeof featureKeys)[number];

/** Sellers the Isolation Forest finds unlike the rest: advice for a closer look, never a block. */
export function AdminSuspicious({ data }: { data: SuspiciousOutputBody }) {
  const t = useTranslations("admin.suspicious");
  const fmt = useFormat();
  const threshold = fmt.decimal(data.threshold, 2);

  return (
    <div>
      <div className="flex gap-3 rounded-3xl bg-muted/60 p-4 text-sm sm:p-5">
        <Info className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
        <div className="space-y-1">
          <p>
            <span className="font-semibold">{t("adviceTitle")}.</span> {t("advice")}
          </p>
          {data.computed_at && (
            <p className="text-muted-foreground">
              {t("meta", { scored: data.scored, threshold, when: `${fmt.date(data.computed_at)}, ${fmt.time(data.computed_at)}` })}
            </p>
          )}
        </div>
      </div>
      <div className="mt-5">
        {data.scored === 0 ? (
          <EmptyState icon={<Users strokeWidth={1.5} />} title={t("notEnoughTitle")} description={t("notEnoughDescription", { min: data.min_sellers })} />
        ) : data.sellers.length === 0 ? (
          <EmptyState icon={<UserSearch strokeWidth={1.5} />} title={t("emptyTitle")} description={t("emptyDescription", { threshold })} />
        ) : (
          <ul className="divide-y overflow-hidden rounded-3xl border bg-card">
            {data.sellers.map((s) => (
              <li key={s.id}>
                <SellerRow s={s} threshold={data.threshold} />
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

/** A plain-language value for each feature. */
function useFeatureValue() {
  const fmt = useFormat();
  return (k: FeatureKey, f: SellerFeatures) => {
    switch (k) {
      case "review_share":
      case "below_share":
      case "above_share":
        return fmt.percent(f[k], { digits: 0 });
      // A log ratio: 0.2 is about 22% away from the median.
      case "deviation":
        return `≈ ${fmt.percent(Math.expm1(f.deviation), { digits: 0 })}`;
      case "per_listing":
        return fmt.decimal(f.per_listing, 1);
      default:
        return String(f[k]);
    }
  };
}

function ScoreBar({ score, threshold }: { score: number; threshold: number }) {
  const t = useTranslations("admin.suspicious");
  const fmt = useFormat();
  return (
    <div className="w-full sm:w-44" title={t("scoreHint", { threshold: fmt.decimal(threshold, 2) })}>
      <div className="flex items-baseline justify-between text-xs text-muted-foreground">
        <span>{t("score")}</span>
        <span className="num text-base font-semibold text-foreground">{fmt.decimal(score, 2)}</span>
      </div>
      <div
        className="relative mt-1 h-2 overflow-hidden rounded-full bg-muted"
        role="meter"
        aria-label={t("score")}
        aria-valuemin={0}
        aria-valuemax={1}
        aria-valuenow={score}
      >
        <div className={cn("h-full rounded-full", score >= 0.7 ? "bg-destructive" : "bg-warm-foreground")} style={{ width: `${score * 100}%` }} />
        <span className="absolute inset-y-0 w-px bg-foreground/40" style={{ left: `${threshold * 100}%` }} aria-hidden />
      </div>
    </div>
  );
}

function SellerRow({ s, threshold }: { s: SuspiciousSeller; threshold: number }) {
  const t = useTranslations("admin.suspicious");
  const tcodes = useTranslations("codes");
  const value = useFeatureValue();
  const [open, setOpen] = useState(false);
  const detailId = `seller-features-${s.id}`;
  const reasons = s.reasons.length > 0 ? s.reasons : ["combination"];
  const reasonLabel = (code: string) => {
    const key = `reasons.${code}` as Parameters<typeof t>[0];
    return t.has(key) ? t(key) : code;
  };

  return (
    <div className="p-4 sm:px-5">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-5">
        <div className="min-w-0 flex-1">
          <p className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
            <Link href={`/sotuvchi/${s.id}`} className="font-medium underline-offset-4 hover:underline">
              {s.name}
            </Link>
            {s.username && <span className="text-sm text-muted-foreground">@{s.username}</span>}
            {s.demo && <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">{tcodes("demo")}</span>}
          </p>
          <p className="mt-0.5 text-xs text-muted-foreground">{t("live", { count: s.live_listings })}</p>
          <ul className="mt-2 flex flex-wrap gap-1.5">
            {reasons.map((r) => (
              <li key={r} className="rounded-full bg-warm-soft px-2.5 py-0.5 text-xs font-medium text-warm-foreground">
                {reasonLabel(r)}
              </li>
            ))}
          </ul>
        </div>
        <div className="flex items-end gap-3 sm:items-center">
          <ScoreBar score={s.score} threshold={threshold} />
          <button
            type="button"
            aria-expanded={open}
            aria-controls={detailId}
            aria-label={t("detailsFor", { name: s.name })}
            onClick={() => setOpen((v) => !v)}
            className="flex h-10 shrink-0 items-center gap-1 rounded-full px-3 text-sm text-muted-foreground transition-colors outline-none hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            {t("details")}
            <ChevronDown className={cn("size-4 transition-transform", open && "rotate-180")} aria-hidden />
          </button>
        </div>
      </div>
      <AnimatePresence initial={false}>
        {open && (
          <m.div
            id={detailId}
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.2, ease }}
            className="overflow-hidden"
          >
            <dl className="mt-4 grid gap-x-6 gap-y-2 rounded-2xl bg-muted/60 p-4 text-sm sm:grid-cols-2">
              {featureKeys.map((k) => {
                const stands = s.reasons.includes(k);
                return (
                  <div key={k} className="flex items-baseline justify-between gap-3">
                    <dt className="text-muted-foreground">{t(`features.${k}`)}</dt>
                    <dd className={cn("num shrink-0 text-right", stands && "font-semibold text-warm-foreground")}>
                      {value(k, s.features)}
                      {stands && <span className="sr-only"> ({t("standsOut")})</span>}
                    </dd>
                  </div>
                );
              })}
            </dl>
          </m.div>
        )}
      </AnimatePresence>
    </div>
  );
}

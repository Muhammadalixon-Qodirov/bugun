"use client";

import { ArrowRight, CircleCheck, Hourglass, Plus, Sprout, Trophy, XCircle } from "lucide-react";
import { m } from "motion/react";
import Link from "next/link";
import { useTranslations } from "next-intl";

import { useCatalogNames } from "@/components/catalog-names";
import { CategoryIcon } from "@/components/category-icon";
import { ease } from "@/components/motion";
import { Button, buttonVariants } from "@/components/ui/button";
import type { Saved } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

/** What happened to each confirmed price: its rank in the province, a review hold, or a rejection. */
export function PriceResult({ saved, onAgain }: { saved: Saved[]; onAgain: () => void }) {
  const t = useTranslations("price.result");
  return (
    <div className="overflow-hidden rounded-3xl border bg-card">
      <header className="flex items-center gap-2 border-b px-5 py-4">
        <CircleCheck className="size-4 text-primary" aria-hidden />
        <h2 className="font-medium" role="status">
          {t("title", { count: saved.length })}
        </h2>
      </header>
      <ul className="divide-y">
        {saved.map((s, i) => (
          <m.li
            key={s.listing_id}
            initial={{ opacity: 0, x: -8 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ delay: 0.08 * i, duration: 0.35, ease }}
            className="flex gap-3 px-5 py-4"
          >
            <ResultRow s={s} />
          </m.li>
        ))}
      </ul>
      <footer className="flex flex-wrap gap-2 border-t bg-muted/30 px-5 py-4">
        <Button variant="solid" size="pill" className="h-11 flex-1 sm:flex-none sm:px-6" onClick={onAgain}>
          <Plus /> {t("again")}
        </Button>
        <Link href="/mahsulotlarim" className={buttonVariants({ variant: "outline", size: "pill", className: "h-11 flex-1 sm:flex-none" })}>
          {t("mine")} <ArrowRight />
        </Link>
      </footer>
    </div>
  );
}

function ResultRow({ s }: { s: Saved }) {
  const t = useTranslations("price.result");
  const tc = useTranslations("common");
  const fmt = useFormat();
  const names = useCatalogNames();
  const unit = fmt.unit(s.unit);
  const change = s.prev_price && s.prev_price !== s.price_som ? (s.price_som - s.prev_price) / s.prev_price : null;
  const trade = s.trade_type === "ulgurji" ? tc("wholesaleLower") : s.trade_type === "chakana" ? tc("retailLower") : s.trade_type;

  let outcome: { icon: typeof Trophy; tone: string; text: React.ReactNode };
  if (s.status === "rejected") {
    outcome = { icon: XCircle, tone: "bg-destructive/5 text-destructive", text: t("rejected") };
  } else if (s.status === "review") {
    outcome = { icon: Hourglass, tone: "bg-warm-soft text-warm-foreground", text: t("review") };
  } else if (s.position) {
    const p = s.position;
    outcome = {
      icon: Trophy,
      tone: "bg-muted text-foreground",
      text: t.rich("position", {
        rank: p.rank,
        total: p.total,
        price: formatSom(p.median),
        diff: fmt.percent(p.diff_percent / 100, { signed: true, digits: 0 }),
        b: (c) => <span className="num font-semibold">{c}</span>,
      }),
    };
  } else {
    outcome = { icon: Sprout, tone: "bg-muted text-muted-foreground", text: t("fewPrices") };
  }

  return (
    <>
      <CategoryIcon category={s.category} />
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-baseline justify-between gap-x-3">
          <span className="font-medium">
            {names.product(undefined, s.product_name)} <span className="text-xs font-normal text-muted-foreground">· {trade}</span>
          </span>
          <span className="num text-xl font-semibold">
            {formatSom(s.price_som)}
            <span className="ml-1 font-sans text-sm font-normal text-muted-foreground">{tc("perUnit", { unit })}</span>
          </span>
        </div>
        {s.prev_price != null && (
          <p className="mt-1 text-xs text-muted-foreground">
            {change == null ? (
              t("unchanged")
            ) : (
              <>
                {t("previous")}{" "}
                <span className="num">
                  {formatSom(s.prev_price)} → {formatSom(s.price_som)}
                </span>
                ,{" "}
                <span className={cn("num font-medium", change < 0 ? "text-primary" : "text-warm-foreground")}>
                  {fmt.percent(change, { signed: true, digits: 0 })}
                </span>
              </>
            )}
          </p>
        )}
        <p className={cn("mt-2.5 flex gap-2 rounded-xl px-3.5 py-2.5 text-sm", outcome.tone)}>
          <outcome.icon className="mt-0.5 size-4 shrink-0" aria-hidden />
          <span>{outcome.text}</span>
        </p>
      </div>
    </>
  );
}

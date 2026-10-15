import { Star } from "lucide-react";
import { getTranslations } from "next-intl/server";

import type { SellerRating } from "@/lib/api/gen/types.gen";
import { getFormat } from "@/lib/i18n/format.server";
import { cn } from "@/lib/utils";

function Stars({ value, label, className }: { value: number; label: string; className?: string }) {
  return (
    <span className="flex gap-0.5" role="img" aria-label={label}>
      {[1, 2, 3, 4, 5].map((i) => (
        <Star key={i} className={cn(className, i <= Math.round(value) ? "fill-warm text-warm" : "text-muted-foreground/40")} />
      ))}
    </span>
  );
}

/** Buyers' ratings of the seller: the average and the latest comments. */
export async function StoreReviews({ rating }: { rating: SellerRating }) {
  const [t, fmt] = await Promise.all([getTranslations("store"), getFormat()]);
  const average = fmt.decimal(rating.average, 1);
  return (
    <section id="reviews" className="scroll-mt-24 pt-14 sm:pt-20">
      <h2 className="display text-2xl sm:text-[1.75rem]">{t("reviews.title")}</h2>
      <p className="mt-1.5 text-sm text-muted-foreground">{t("reviews.note")}</p>
      <div className="mt-5 grid gap-4 md:grid-cols-[16rem_1fr]">
        <div className="self-start rounded-3xl border bg-card p-6">
          <p className="num text-5xl leading-none font-semibold tracking-[-0.03em]">{average}</p>
          <div className="mt-3">
            <Stars value={rating.average} label={t("reviews.stars", { stars: average })} className="size-5" />
          </div>
          <p className="mt-2 text-sm text-muted-foreground">{t("ratingSummary", { average, count: rating.count })}</p>
        </div>
        {rating.comments.length > 0 && (
          <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {rating.comments.map((c) => (
              <li key={c.created_at} className="rounded-3xl border bg-card p-5 text-sm">
                <div className="flex items-center justify-between gap-2">
                  <Stars value={c.stars} label={t("reviews.stars", { stars: c.stars })} className="size-3.5" />
                  <span className="text-xs text-muted-foreground">{fmt.dayAgo(c.created_at)}</span>
                </div>
                <p className="mt-2.5 leading-relaxed">{c.comment}</p>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}

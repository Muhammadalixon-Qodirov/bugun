"use client";

import { useMutation } from "@tanstack/react-query";
import { Eye, EyeOff, MessageSquareQuote, Star } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { useCatalogNames } from "@/components/catalog-names";
import { EmptyState } from "@/components/empty-state";
import { ease } from "@/components/motion";
import { Button } from "@/components/ui/button";
import { adminDecideRatingMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { HeldRating } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { cn } from "@/lib/utils";

/** Low ratings whose comments wait for an admin before they are shown. */
export function AdminRatings({ ratings }: { ratings: HeldRating[] }) {
  const t = useTranslations("admin.ratings");
  if (ratings.length === 0) {
    return <EmptyState icon={<MessageSquareQuote strokeWidth={1.5} />} title={t("emptyTitle")} description={t("emptyDescription")} />;
  }
  return (
    <div>
      <p className="mb-4 max-w-2xl text-sm text-muted-foreground">{t("help")}</p>
      <ul className="space-y-3">
        <AnimatePresence initial={false}>
          {ratings.map((r) => (
            <m.li
              key={r.id}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0, height: 0, marginTop: 0, transition: { duration: 0.25, ease } }}
              className="overflow-hidden"
            >
              <RatingCard r={r} />
            </m.li>
          ))}
        </AnimatePresence>
      </ul>
    </div>
  );
}

function Stars({ stars, className }: { stars: number; className?: string }) {
  const t = useTranslations("admin.ratings");
  return (
    <span role="img" aria-label={t("stars", { stars })} className={cn("flex items-center gap-0.5", className)}>
      {[1, 2, 3, 4, 5].map((i) => (
        <Star key={i} aria-hidden className={cn("size-4", i <= stars ? "fill-warm text-warm" : "text-muted-foreground/40")} />
      ))}
    </span>
  );
}

function RatingCard({ r }: { r: HeldRating }) {
  const router = useRouter();
  const t = useTranslations("admin.ratings");
  const fmt = useFormat();
  const names = useCatalogNames();
  const errors = useApiErrors();
  const decide = useMutation({
    ...adminDecideRatingMutation(),
    onSuccess: (_, v) => {
      toast.success(v.body.publish ? t("published") : t("hidden"));
      router.refresh();
    },
    onError: (err) => errors.show(err),
  });
  return (
    <article className="flex flex-col gap-4 rounded-3xl border bg-card p-4 sm:flex-row sm:items-start sm:p-5">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <Stars stars={r.stars} />
          <span className="text-xs text-muted-foreground">
            #{r.id} · {fmt.date(r.created_at)}
          </span>
        </div>
        {r.comment ? (
          <blockquote className="mt-2 text-[15px] leading-relaxed">“{r.comment}”</blockquote>
        ) : (
          <p className="mt-2 text-sm text-muted-foreground italic">{t("noComment")}</p>
        )}
        <p className="mt-2 text-sm text-muted-foreground">
          <span className="font-medium text-foreground">{r.seller_name}</span> · {names.product(undefined, r.product_name)}
        </p>
      </div>
      <div className="flex shrink-0 gap-2 sm:flex-col">
        <Button variant="solid" size="pill" disabled={decide.isPending} onClick={() => decide.mutate({ path: { id: r.id }, body: { publish: true } })}>
          <Eye /> {t("publish")}
        </Button>
        <Button variant="outline" size="pill" disabled={decide.isPending} onClick={() => decide.mutate({ path: { id: r.id }, body: { publish: false } })}>
          <EyeOff /> {t("hide")}
        </Button>
      </div>
    </article>
  );
}

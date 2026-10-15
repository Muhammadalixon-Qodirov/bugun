"use client";

import { useMutation } from "@tanstack/react-query";
import { ArrowDownRight, ArrowUpRight, Clock, Phone, UserRound } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import { useTranslations } from "next-intl";

import { TelegramIcon } from "@/components/telegram-icon";
import { Button, buttonVariants } from "@/components/ui/button";
import { offerContact } from "@/lib/api/gen/sdk.gen";
import type { Rfq } from "@/lib/api/gen/types.gen";
import { problemOf, useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

/** Open, ended or closed, with a dot so the state never rests on colour alone. */
export function RfqStatusBadge({ status, className }: { status: Rfq["status"]; className?: string }) {
  const t = useTranslations("codes.rfqStatus");
  return (
    <span
      className={cn(
        "inline-flex h-6 shrink-0 items-center gap-1.5 rounded-full px-2.5 text-xs font-medium",
        status === "open" ? "bg-primary/10 text-primary" : "bg-muted text-muted-foreground",
        className,
      )}
    >
      <span className={cn("size-1.5 rounded-full", status === "open" ? "bg-primary" : "bg-muted-foreground/60")} aria-hidden />
      {t(status)}
    </span>
  );
}

/** "2 500 kg", "12,5 kg": whole amounts grouped like prices, fractions with their decimals. */
export function useQuantity() {
  const fmt = useFormat();
  return (q: number, unit: string) => `${Number.isInteger(q) ? formatSom(q) : fmt.decimal(q, 2)} ${fmt.unit(unit)}`;
}

/** "Until 29 September 2026, 14:00": the deadline in Tashkent time, with a clock icon. */
export function Deadline({ iso, className }: { iso: string; className?: string }) {
  const t = useTranslations("rfq");
  const fmt = useFormat();
  return (
    <span className={cn("inline-flex items-center gap-1.5", className)}>
      <Clock className="size-3.5 shrink-0" />
      {t("deadline", { date: fmt.date(iso), time: fmt.time(iso) })}
    </span>
  );
}

/** "6% cheaper than the market" from a whole-number percent; cheaper in green, dearer in warm. */
export function MarketDiff({ percent, className }: { percent?: number; className?: string }) {
  const t = useTranslations("rfq.diff");
  if (percent == null) return null;
  if (percent === 0) return <span className={cn("text-xs text-muted-foreground", className)}>{t("same")}</span>;
  const cheaper = percent < 0;
  const Icon = cheaper ? ArrowDownRight : ArrowUpRight;
  return (
    <span className={cn("inline-flex items-center gap-0.5 text-xs font-medium", cheaper ? "text-primary" : "text-warm-foreground", className)}>
      <Icon className="size-3.5" />
      {cheaper ? t("cheaper", { percent: Math.abs(percent) }) : t("dearer", { percent })}
    </span>
  );
}

/**
 * The other side's contact after an agreed deal. A hidden contact is explained
 * in place rather than as an error toast.
 */
export function OfferContact({ offerId, block }: { offerId: number; block?: boolean }) {
  const t = useTranslations("rfq.contact");
  const errors = useApiErrors();
  const reveal = useMutation({
    mutationFn: async () => (await offerContact({ path: { id: offerId }, throwOnError: true })).data,
    onError: (err) => {
      if (problemOf(err).code !== "contact_hidden") errors.show(err, "contact");
    },
  });
  const hidden = reveal.isError && problemOf(reveal.error).code === "contact_hidden";
  const size = block ? "h-11 px-5" : "h-10 px-4";

  return (
    <AnimatePresence mode="wait" initial={false}>
      {reveal.data ? (
        <m.div key="shown" initial={{ opacity: 0, y: 4 }} animate={{ opacity: 1, y: 0 }} className="flex flex-wrap items-center gap-2">
          <span className="flex min-w-0 items-center gap-1.5 text-sm font-medium">
            <UserRound className="size-4 shrink-0 text-muted-foreground" />
            <span className="truncate">{reveal.data.name}</span>
          </span>
          {reveal.data.telegram_url && (
            <a
              href={reveal.data.telegram_url}
              target="_blank"
              rel="noreferrer"
              className={buttonVariants({ className: cn("rounded-full bg-telegram text-white hover:bg-telegram-hover", size) })}
            >
              <TelegramIcon className="size-4" /> {t("telegram")}
            </a>
          )}
          {reveal.data.phone && (
            <a href={`tel:${reveal.data.phone}`} className={buttonVariants({ variant: "outline", className: cn("num rounded-full", size) })}>
              <Phone /> {reveal.data.phone}
            </a>
          )}
          {!reveal.data.telegram_url && !reveal.data.phone && <span className="text-sm text-muted-foreground">{t("none")}</span>}
        </m.div>
      ) : hidden ? (
        <m.p key="hidden" initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="text-sm text-muted-foreground">
          {errors.message(reveal.error)}
        </m.p>
      ) : (
        <m.div key="button" exit={{ opacity: 0 }}>
          <Button variant="default" className={cn("rounded-full", size)} disabled={reveal.isPending} onClick={() => reveal.mutate()}>
            <Phone /> {reveal.isPending ? t("opening") : t("show")}
          </Button>
        </m.div>
      )}
    </AnimatePresence>
  );
}

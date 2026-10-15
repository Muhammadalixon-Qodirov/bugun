"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, Hourglass, MapPin, MessageSquareText, PiggyBank, Store, Trophy, X } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { useCatalogNames } from "@/components/catalog-names";
import { EmptyState } from "@/components/empty-state";
import { Deadline, MarketDiff, OfferContact, RfqStatusBadge, useQuantity } from "@/components/rfq/rfq-parts";
import { Stat } from "@/components/stat";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import {
  cancelRfqMutation,
  chooseOfferMutation,
  getRfqOptions,
  listMyRfqsQueryKey,
} from "@/lib/api/gen/@tanstack/react-query.gen";
import type { RfqDetailOutputBody, RfqOffer } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

/** One request as its buyer sees it: the terms, the market context and every offer, cheapest first. */
export function RfqDetail({ id, initial }: { id: number; initial: RfqDetailOutputBody }) {
  const t = useTranslations("rfq.detail");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const fmt = useFormat();
  const qty = useQuantity();
  const queryClient = useQueryClient();
  const errors = useApiErrors();
  const options = getRfqOptions({ path: { id } });
  const key = options.queryKey;
  const { data } = useQuery({ ...options, initialData: initial });
  const { rfq, offers, median, savings } = data;
  const unit = fmt.unit(rfq.unit);
  const [confirmClose, setConfirmClose] = useState(false);

  const refresh = () =>
    Promise.all([queryClient.invalidateQueries({ queryKey: key }), queryClient.invalidateQueries({ queryKey: listMyRfqsQueryKey() })]);

  const choose = useMutation({
    ...chooseOfferMutation(),
    onSuccess: (res) => {
      queryClient.setQueryData(key, (old) =>
        old ? { ...old, offers: old.offers.map((o) => (o.id === res.id ? { ...o, status: "chosen" as const } : o)) } : old,
      );
      toast.success(t("chosenToast"));
    },
    onError: (err) => errors.show(err, "change"),
    onSettled: refresh,
  });
  const cancel = useMutation({
    ...cancelRfqMutation(),
    onSuccess: () => {
      queryClient.setQueryData(key, (old) => (old ? { ...old, rfq: { ...old.rfq, status: "cancelled" as const } } : old));
      setConfirmClose(false);
      toast.success(t("closedToast"));
    },
    onError: (err) => errors.show(err, "change"),
    onSettled: refresh,
  });

  const live = offers.filter((o) => o.status !== "rejected");
  const best = live[0];

  return (
    <div className="space-y-8">
      <section className="rounded-3xl border bg-card p-5 sm:p-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="display truncate text-3xl sm:text-4xl">{names.product(undefined, rfq.product_name)}</h1>
              <RfqStatusBadge status={rfq.status} />
            </div>
            <p className="num mt-2 text-lg font-medium">{qty(rfq.quantity, rfq.unit)}</p>
            <p className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-muted-foreground">
              <span className="inline-flex items-center gap-1.5">
                <MapPin className="size-3.5" /> {names.region(rfq.region_name)}
              </span>
              <Deadline iso={rfq.deadline} />
              {rfq.max_price_som != null && (
                <span>
                  {t("maxPrice")}: <span className="num font-medium text-foreground">{formatSom(rfq.max_price_som)}</span> {tc("perUnit", { unit })}
                </span>
              )}
            </p>
            {rfq.note && (
              <p className="mt-3 flex max-w-2xl gap-2 text-sm leading-relaxed">
                <MessageSquareText className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                <span className="break-words">{rfq.note}</span>
              </p>
            )}
          </div>
          {rfq.status === "open" && (
            <Button variant="outline" size="pill" onClick={() => setConfirmClose(true)}>
              <X /> {t("close")}
            </Button>
          )}
        </div>
      </section>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat label={t("stats.offers")} value={live.length} hint={t("stats.offersHint")} />
        <Stat
          label={t("stats.best")}
          value={best ? formatSom(best.price_som) : "—"}
          hint={best ? tc("perUnit", { unit }) : t("stats.none")}
        />
        <Stat label={t("stats.median")} value={median ? formatSom(median) : "—"} hint={median ? tc("perUnit", { unit }) : t("stats.medianNone")} />
        {savings ? (
          <div className="rounded-2xl bg-primary/10 px-5 py-4 text-primary">
            <p className="flex items-center gap-1.5 text-sm">
              <PiggyBank className="size-4" /> {t("stats.savings")}
            </p>
            <p className="num mt-1 text-2xl font-semibold tracking-tight">{formatSom(savings)}</p>
            <p className="mt-0.5 text-xs">{t("stats.savingsHint")}</p>
          </div>
        ) : (
          <Stat label={t("stats.total")} value={best ? formatSom(best.price_som * rfq.quantity) : "—"} hint={t("stats.totalHint")} />
        )}
      </div>

      <section aria-labelledby="offers-title">
        <h2 id="offers-title" className="display mb-4 text-2xl">
          {t("offersTitle")}
        </h2>
        {offers.length === 0 ? (
          <EmptyState
            icon={<Hourglass strokeWidth={1.5} />}
            title={rfq.status === "open" ? t("empty.openTitle") : t("empty.closedTitle")}
            description={rfq.status === "open" ? t("empty.openText") : t("empty.closedText")}
            className="py-12"
          />
        ) : (
          <ul className="space-y-3">
            <AnimatePresence initial={false}>
              {offers.map((o) => (
                <m.li key={o.id} layout="position" initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
                  <OfferCard
                    offer={o}
                    unit={unit}
                    rawUnit={rfq.unit}
                    quantity={rfq.quantity}
                    cheapest={live.length > 1 && o.id === best?.id}
                    canChoose={rfq.status !== "cancelled"}
                    choosing={choose.isPending && choose.variables?.path.id === o.id}
                    onChoose={() => choose.mutate({ path: { id: o.id } })}
                  />
                </m.li>
              ))}
            </AnimatePresence>
          </ul>
        )}
      </section>

      <Dialog open={confirmClose} onOpenChange={setConfirmClose}>
        <DialogContent>
          <DialogTitle>{t("closeDialog.title")}</DialogTitle>
          <DialogDescription>{t("closeDialog.text")}</DialogDescription>
          <div className="mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <DialogClose render={<Button variant="outline" size="pill" />}>{tc("cancel")}</DialogClose>
            <Button variant="destructive" size="pill" disabled={cancel.isPending} onClick={() => cancel.mutate({ path: { id } })}>
              {t("closeDialog.confirm")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function OfferCard({
  offer: o,
  unit,
  rawUnit,
  quantity,
  cheapest,
  canChoose,
  choosing,
  onChoose,
}: {
  offer: RfqOffer;
  unit: string;
  rawUnit: string;
  quantity: number;
  cheapest: boolean;
  canChoose: boolean;
  choosing: boolean;
  onChoose: () => void;
}) {
  const t = useTranslations("rfq.detail");
  const ts = useTranslations("codes.offerStatus");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const qty = useQuantity();
  const rejected = o.status === "rejected";
  const place = [o.seller_region && names.region(o.seller_region), o.seller_market && names.market(o.seller_market)].filter(Boolean);

  return (
    <article
      className={cn(
        "grid gap-4 rounded-3xl border bg-card p-5 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center sm:p-6",
        cheapest && "border-primary/40 ring-1 ring-primary/30",
        o.status === "confirmed" && "border-primary/40",
        rejected && "opacity-60",
      )}
    >
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          {cheapest && (
            <span className="inline-flex h-6 items-center gap-1 rounded-full bg-primary px-2.5 text-xs font-medium text-primary-foreground">
              <Trophy className="size-3" /> {t("cheapest")}
            </span>
          )}
          {o.status !== "sent" && (
            <span
              className={cn(
                "inline-flex h-6 items-center gap-1 rounded-full px-2.5 text-xs font-medium",
                o.status === "confirmed" ? "bg-primary/10 text-primary" : "bg-muted text-muted-foreground",
              )}
            >
              {o.status === "confirmed" && <Check className="size-3" />}
              {o.status === "chosen" ? t("chosenBadge") : ts(o.status)}
            </span>
          )}
        </div>
        <p className="mt-2 flex flex-wrap items-baseline gap-x-2">
          <span className={cn("num text-2xl font-semibold tracking-tight", rejected && "line-through")}>{formatSom(o.price_som)}</span>
          <span className="text-sm text-muted-foreground">{tc("perUnit", { unit })}</span>
          <MarketDiff percent={o.diff_percent} />
        </p>
        <p className="num mt-0.5 text-sm text-muted-foreground">{t("total", { sum: formatSom(o.price_som * quantity) })}</p>
        <p className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
          <span className="inline-flex min-w-0 items-center gap-1.5 font-medium">
            <Store className="size-4 shrink-0 text-muted-foreground" />
            <span className="truncate">{o.seller_name}</span>
          </span>
          {place.length > 0 && (
            <span className="inline-flex items-center gap-1 text-muted-foreground">
              <MapPin className="size-3.5" /> {place.join(" · ")}
            </span>
          )}
        </p>
        {o.quantity != null && <p className="mt-1 text-xs text-muted-foreground">{t("canSupply", { amount: qty(o.quantity, rawUnit) })}</p>}
      </div>

      <div className="sm:justify-self-end">
        {o.status === "sent" && canChoose && (
          <Button variant="solid" size="pill" className="w-full sm:w-auto" disabled={choosing} onClick={onChoose}>
            <Check /> {t("choose")}
          </Button>
        )}
        {o.status === "chosen" && (
          <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
            <Hourglass className="size-4" /> {t("waiting")}
          </p>
        )}
        {o.status === "confirmed" && <OfferContact offerId={o.id} />}
        {rejected && <p className="text-sm text-muted-foreground">{t("rejected")}</p>}
      </div>
    </article>
  );
}

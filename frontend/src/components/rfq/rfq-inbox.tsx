"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, Inbox, Loader2, MapPin, MessageSquareText, PackageX, Pencil, Send, Sparkles, X } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { useCatalogNames } from "@/components/catalog-names";
import { EmptyState } from "@/components/empty-state";
import { ease } from "@/components/motion";
import { Deadline, OfferContact, RfqStatusBadge, useQuantity } from "@/components/rfq/rfq-parts";
import { Button, buttonVariants } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  answerOfferMutation,
  offerRfqMutation,
  rfqInboxOptions,
} from "@/lib/api/gen/@tanstack/react-query.gen";
import type { InboxRfq } from "@/lib/api/gen/types.gen";
import { problemOf, useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom, groupDigits } from "@/lib/labels";
import { cn } from "@/lib/utils";

type Patch = Pick<InboxRfq, "offer_id" | "offer_status" | "offer_price">;

/** Requests sent to the seller; the ones where the buyer chose their offer come first (the API orders them). */
export function RfqInbox({ initial }: { initial: InboxRfq[] }) {
  const t = useTranslations("cabinet.inbox");
  const { data } = useQuery({ ...rfqInboxOptions(), initialData: { rfqs: initial } });
  const rfqs = data.rfqs;
  const chosen = rfqs.filter((r) => r.offer_status === "chosen").length;

  return (
    <section aria-labelledby="inbox-title">
      <div className="mb-4 flex flex-wrap items-end justify-between gap-2">
        <h2 id="inbox-title" className="display text-2xl">
          {t("title")}
        </h2>
        {chosen > 0 && (
          <span className="inline-flex h-7 items-center rounded-full bg-primary px-3 text-sm font-medium text-primary-foreground">
            {t("chosenCount", { count: chosen })}
          </span>
        )}
      </div>
      <InboxList rfqs={rfqs} />
    </section>
  );
}

function InboxList({ rfqs }: { rfqs: InboxRfq[] }) {
  const t = useTranslations("cabinet.inbox");
  if (rfqs.length === 0) {
    return (
      <EmptyState
        icon={<Inbox strokeWidth={1.5} />}
        title={t("emptyTitle")}
        description={t("emptyText")}
        className="py-12"
        action={
          <Link href="/mahsulotlarim" className={buttonVariants({ variant: "outline", size: "pill" })}>
            {t("emptyAction")}
          </Link>
        }
      />
    );
  }
  return (
    <ul className="space-y-3">
      <AnimatePresence initial={false}>
        {rfqs.map((r) => (
          <m.li key={r.id} layout="position" initial={{ opacity: 0 }} animate={{ opacity: 1 }} transition={{ duration: 0.25, ease }}>
            <InboxCard r={r} />
          </m.li>
        ))}
      </AnimatePresence>
    </ul>
  );
}

/** Updates one request in the cached inbox, so the card changes before the refetch lands. */
function usePatchInbox() {
  const queryClient = useQueryClient();
  const key = rfqInboxOptions().queryKey;
  return {
    patch: (id: number, patch: Patch) =>
      queryClient.setQueryData(key, (old) => (old ? { rfqs: old.rfqs.map((r) => (r.id === id ? { ...r, ...patch } : r)) } : old)),
    refresh: () => queryClient.invalidateQueries({ queryKey: key }),
  };
}

function InboxCard({ r }: { r: InboxRfq }) {
  const t = useTranslations("cabinet.inbox");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const fmt = useFormat();
  const qty = useQuantity();
  const unit = fmt.unit(r.unit);
  const chosen = r.offer_status === "chosen";
  const confirmed = r.offer_status === "confirmed";

  return (
    <article
      className={cn(
        "overflow-hidden rounded-3xl border bg-card",
        chosen && "border-primary/50 ring-1 ring-primary/30",
        r.offer_status === "rejected" && "opacity-70",
      )}
    >
      {chosen && (
        <p className="flex items-center gap-2 bg-primary px-5 py-2.5 text-sm font-medium text-primary-foreground sm:px-6">
          <Sparkles className="size-4" /> {t("chosenBanner")}
        </p>
      )}
      <div className="p-5 sm:p-6">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="truncate text-lg font-semibold tracking-tight">{names.product(undefined, r.product_name)}</h3>
              {r.status !== "open" && <RfqStatusBadge status={r.status} />}
            </div>
            <p className="num mt-0.5 font-medium">{qty(r.quantity, r.unit)}</p>
          </div>
          {r.max_price_som != null && (
            <p className="text-right text-sm">
              <span className="block text-xs text-muted-foreground">{t("maxPrice")}</span>
              <span className="num font-semibold">{formatSom(r.max_price_som)}</span>{" "}
              <span className="text-xs text-muted-foreground">{tc("perUnit", { unit })}</span>
            </p>
          )}
        </div>
        <p className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-muted-foreground">
          <span className="inline-flex items-center gap-1.5">
            <MapPin className="size-3.5" /> {names.region(r.region_name)}
          </span>
          <Deadline iso={r.deadline} />
        </p>
        {r.note && (
          <p className="mt-3 flex gap-2 rounded-2xl bg-muted/60 px-4 py-3 text-sm leading-relaxed">
            <MessageSquareText className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            <span className="break-words">{r.note}</span>
          </p>
        )}

        <div className="mt-5 border-t pt-5">
          {chosen ? (
            <ChosenActions r={r} unit={unit} />
          ) : confirmed ? (
            <div className="space-y-3">
              <p className="flex flex-wrap items-center gap-1.5 text-sm">
                <Check className="size-4 text-primary" />
                {t.rich("agreed", {
                  price: formatSom(r.offer_price ?? 0),
                  unit,
                  b: (c) => <span className="num font-semibold">{c}</span>,
                })}
              </p>
              {r.offer_id != null && <OfferContact offerId={r.offer_id} />}
            </div>
          ) : r.offer_status === "rejected" ? (
            <p className="text-sm text-muted-foreground">{t("youDeclined")}</p>
          ) : r.status !== "open" ? (
            <p className="text-sm text-muted-foreground">
              {r.offer_status === "sent" && r.offer_price != null
                ? t("closedWithOffer", { price: formatSom(r.offer_price), unit })
                : t("closed")}
            </p>
          ) : (
            <OfferArea r={r} unit={unit} />
          )}
        </div>
      </div>
    </article>
  );
}

/** Price form, "I don't have it", or the sent offer with a way to change it. */
function OfferArea({ r, unit }: { r: InboxRfq; unit: string }) {
  const t = useTranslations("cabinet.inbox");
  const [editing, setEditing] = useState(r.offer_status !== "sent");

  if (r.offer_status === "sent" && !editing) {
    return (
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="flex items-center gap-1.5 text-sm">
          <Send className="size-4 text-muted-foreground" />
          {t.rich("sent", { price: formatSom(r.offer_price ?? 0), unit, b: (c) => <span className="num font-semibold">{c}</span> })}
        </p>
        <Button variant="outline" size="pill" onClick={() => setEditing(true)}>
          <Pencil /> {t("changePrice")}
        </Button>
      </div>
    );
  }
  return <OfferForm r={r} unit={unit} onDone={() => setEditing(false)} onCancel={r.offer_status === "sent" ? () => setEditing(false) : undefined} />;
}

function OfferForm({ r, unit, onDone, onCancel }: { r: InboxRfq; unit: string; onDone: () => void; onCancel?: () => void }) {
  const t = useTranslations("cabinet.inbox");
  const tc = useTranslations("common");
  const fmt = useFormat();
  const errors = useApiErrors();
  const { patch, refresh } = usePatchInbox();
  const [price, setPrice] = useState(r.offer_price ? groupDigits(String(r.offer_price)) : "");
  const [quantity, setQuantity] = useState("");
  const [fieldErrors, setFieldErrors] = useState<{ price?: string; quantity?: string }>({});
  const offer = useMutation({
    ...offerRfqMutation(),
    onSuccess: (res) => {
      patch(r.id, { offer_id: res.id, offer_status: res.status, offer_price: res.price_som });
      toast.success(res.status === "declined" ? t("declinedToast") : t("sentToast"));
      onDone();
    },
    onError: (err) => {
      const fields = errors.fields(err);
      const next: typeof fieldErrors = {};
      for (const f of fields) {
        if (f.field === "price_som") next.price = f.message;
        if (f.field === "quantity") next.quantity = f.message;
      }
      setFieldErrors(next);
      if (fields.length === 0) errors.show(err);
      // The request may have closed or been chosen meanwhile: show its real state.
      if (["rfq_closed", "offer_locked", "rfq_not_found", "rfq_not_eligible"].includes(problemOf(err).code ?? "")) void refresh();
    },
  });

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const p = Number(price.replace(/\s/g, ""));
    const q = quantity.trim() ? Number(quantity.replace(/\s/g, "").replace(",", ".")) : undefined;
    const next: typeof fieldErrors = {};
    if (!price.trim()) next.price = t("validation.priceRequired");
    else if (!(p >= 50 && p <= 50_000_000)) next.price = t("validation.priceRange");
    if (q !== undefined && !(q > 0 && q <= 10_000_000)) next.quantity = t("validation.quantity");
    setFieldErrors(next);
    if (next.price || next.quantity) return;
    offer.mutate({ path: { id: r.id }, body: { price_som: p, quantity: q } });
  };
  const priceId = `offer-price-${r.id}`;
  const qtyId = `offer-qty-${r.id}`;

  return (
    <form onSubmit={submit} noValidate className="space-y-4">
      {r.offer_status === "declined" && <p className="text-sm text-muted-foreground">{t("markedNone")}</p>}
      <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_11rem]">
        <div>
          <label htmlFor={priceId} className="text-sm font-medium">
            {t("price")}
          </label>
          <div className="relative mt-1.5">
            <Input
              id={priceId}
              inputMode="numeric"
              placeholder="4 500"
              value={price}
              onChange={(e) => setPrice(groupDigits(e.target.value))}
              aria-invalid={Boolean(fieldErrors.price)}
              aria-describedby={fieldErrors.price ? `${priceId}-error` : undefined}
              className="num h-12 rounded-xl pr-24 text-lg font-semibold md:text-lg"
            />
            <span className="pointer-events-none absolute top-1/2 right-4 -translate-y-1/2 text-sm text-muted-foreground">{tc("perUnit", { unit })}</span>
          </div>
          {fieldErrors.price && (
            <p id={`${priceId}-error`} className="mt-1.5 text-sm text-destructive">
              {fieldErrors.price}
            </p>
          )}
        </div>
        <div>
          <label htmlFor={qtyId} className="text-sm font-medium">
            {t("quantity")} <span className="font-normal text-muted-foreground">· {tc("optional")}</span>
          </label>
          <div className="relative mt-1.5">
            <Input
              id={qtyId}
              inputMode="decimal"
              placeholder={fmt.decimal(r.quantity, 2)}
              value={quantity}
              onChange={(e) => setQuantity(e.target.value)}
              aria-invalid={Boolean(fieldErrors.quantity)}
              aria-describedby={fieldErrors.quantity ? `${qtyId}-error` : undefined}
              className="num h-12 rounded-xl pr-14"
            />
            <span className="pointer-events-none absolute top-1/2 right-4 -translate-y-1/2 text-sm text-muted-foreground">{unit}</span>
          </div>
          {fieldErrors.quantity && (
            <p id={`${qtyId}-error`} className="mt-1.5 text-sm text-destructive">
              {fieldErrors.quantity}
            </p>
          )}
        </div>
      </div>
      <div className="flex flex-wrap gap-2">
        <Button type="submit" variant="solid" size="pill" disabled={offer.isPending} className="flex-1 sm:flex-none">
          {offer.isPending && offer.variables?.body.none !== true ? <Loader2 className="animate-spin" /> : <Send />}
          {r.offer_status === "sent" ? t("updateOffer") : t("sendOffer")}
        </Button>
        {onCancel ? (
          <Button type="button" variant="ghost" size="pill" onClick={onCancel}>
            {tc("cancel")}
          </Button>
        ) : (
          r.offer_status !== "declined" && (
            <Button
              type="button"
              variant="outline"
              size="pill"
              disabled={offer.isPending}
              className="flex-1 sm:flex-none"
              onClick={() => offer.mutate({ path: { id: r.id }, body: { none: true } })}
            >
              <PackageX /> {t("none")}
            </Button>
          )
        )}
      </div>
    </form>
  );
}

/** The buyer chose this seller's offer: agree (a deal, contacts open) or decline. */
function ChosenActions({ r, unit }: { r: InboxRfq; unit: string }) {
  const t = useTranslations("cabinet.inbox");
  const tc = useTranslations("common");
  const errors = useApiErrors();
  const { patch, refresh } = usePatchInbox();
  const [confirmDecline, setConfirmDecline] = useState(false);
  const answer = useMutation({
    ...answerOfferMutation(),
    onSuccess: (res) => {
      patch(r.id, { offer_status: res.status });
      setConfirmDecline(false);
      toast.success(res.status === "confirmed" ? t("agreedToast") : t("declinedDealToast"));
    },
    onError: (err) => errors.show(err, "change"),
    onSettled: refresh,
  });
  if (r.offer_id == null) return null;
  const offerId = r.offer_id;

  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <p className="text-sm">
        {t.rich("chosenText", { price: formatSom(r.offer_price ?? 0), unit, b: (c) => <span className="num font-semibold">{c}</span> })}
      </p>
      <div className="flex gap-2">
        <Button
          variant="default"
          size="pill"
          className="flex-1 sm:flex-none"
          disabled={answer.isPending}
          onClick={() => answer.mutate({ path: { id: offerId }, body: { agree: true } })}
        >
          {answer.isPending && answer.variables?.body.agree ? <Loader2 className="animate-spin" /> : <Check />} {t("agree")}
        </Button>
        <Button variant="outline" size="pill" className="flex-1 sm:flex-none" disabled={answer.isPending} onClick={() => setConfirmDecline(true)}>
          <X /> {t("decline")}
        </Button>
      </div>
      <Dialog open={confirmDecline} onOpenChange={setConfirmDecline}>
        <DialogContent>
          <DialogTitle>{t("declineDialog.title")}</DialogTitle>
          <DialogDescription>{t("declineDialog.text")}</DialogDescription>
          <div className="mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <DialogClose render={<Button variant="outline" size="pill" />}>{tc("cancel")}</DialogClose>
            <Button
              variant="destructive"
              size="pill"
              disabled={answer.isPending}
              onClick={() => answer.mutate({ path: { id: offerId }, body: { agree: false } })}
            >
              {t("declineDialog.confirm")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

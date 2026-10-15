"use client";

import { useMutation } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { markListingSoldMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { MyListing } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom, groupDigits } from "@/lib/labels";
import { formatAmount } from "@/lib/listings";

/** "Sotildi": records a sale from this listing, and takes it off the stock when stock is set. */
export function SoldDialog({ l, open, onOpenChange }: { l: MyListing; open: boolean; onOpenChange: (open: boolean) => void }) {
  const t = useTranslations("mine.sold");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogTitle>{t("title")}</DialogTitle>
        <DialogDescription>{t("description")}</DialogDescription>
        {/* The popup unmounts when closed, so the form starts empty every time. */}
        <SoldForm l={l} onDone={() => onOpenChange(false)} />
      </DialogContent>
    </Dialog>
  );
}

type Errors = { quantity?: string; price?: string };

function SoldForm({ l, onDone }: { l: MyListing; onDone: () => void }) {
  const t = useTranslations("mine.sold");
  const te = useTranslations("errors");
  const tc = useTranslations("common");
  const fmt = useFormat();
  const router = useRouter();
  const errors = useApiErrors();
  const unit = fmt.unit(l.unit);
  const [quantity, setQuantity] = useState("");
  const [price, setPrice] = useState("");
  const [invalid, setInvalid] = useState<Errors>({});

  const sold = useMutation({
    ...markListingSoldMutation(),
    onSuccess: (res) => {
      if (res.sold_out) {
        toast.success(t("soldOut"), {
          description: res.confirmed ? t("confirmed") : undefined,
          action: { label: t("updateStock"), onClick: () => router.push(`/mahsulotlarim/${l.id}`) },
        });
      } else {
        toast.success(res.confirmed ? t("confirmed") : t("recorded"));
      }
      onDone();
      // The response is the thin single-listing shape: reload the list for rank, funnel and median.
      router.refresh();
    },
    onError: (err) => {
      const fields = errors.fields(err);
      const next: Errors = {};
      for (const f of fields) {
        if (f.field === "quantity") next.quantity = f.message;
        if (f.field === "price_som") next.price = f.message;
      }
      if (next.quantity || next.price) setInvalid(next);
      else errors.show(err, "save");
    },
  });

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const q = Number(quantity.replace(/\s/g, "").replace(",", "."));
    const p = price.trim() === "" ? undefined : Number(price.replace(/\s/g, ""));
    const next: Errors = {};
    if (!(q > 0 && q <= 10_000_000)) next.quantity = te("fields.invalid_amount");
    if (p !== undefined && !(p >= 50 && p <= 50_000_000)) next.price = te("fields.invalid_price");
    setInvalid(next);
    if (next.quantity || next.price) return;
    sold.mutate({ path: { id: l.id }, body: { quantity: q, price_som: p } });
  };

  return (
    <form onSubmit={submit} noValidate className="mt-5">
      <FieldGroup>
        <Field data-invalid={!!invalid.quantity}>
          <FieldLabel htmlFor={`sold-qty-${l.id}`}>{t("quantity", { unit })}</FieldLabel>
          <div className="relative">
            <Input
              id={`sold-qty-${l.id}`}
              inputMode="decimal"
              autoFocus
              value={quantity}
              onChange={(e) => setQuantity(e.target.value)}
              placeholder={l.stock != null ? formatAmount(l.stock, fmt.decimal) : "100"}
              aria-invalid={!!invalid.quantity}
              className="num h-11 rounded-xl pr-16"
            />
            <span className="pointer-events-none absolute top-1/2 right-4 -translate-y-1/2 text-sm text-muted-foreground">{unit}</span>
          </div>
          {l.stock != null && <FieldDescription>{t("max", { amount: formatAmount(l.stock, fmt.decimal), unit })}</FieldDescription>}
          {invalid.quantity && <FieldError>{invalid.quantity}</FieldError>}
        </Field>
        <Field data-invalid={!!invalid.price}>
          <FieldLabel htmlFor={`sold-price-${l.id}`}>
            {t("price", { unit })} <span className="font-normal text-muted-foreground">· {tc("optional")}</span>
          </FieldLabel>
          <div className="relative">
            <Input
              id={`sold-price-${l.id}`}
              inputMode="numeric"
              value={price}
              onChange={(e) => setPrice(groupDigits(e.target.value))}
              placeholder={formatSom(l.price_som)}
              aria-invalid={!!invalid.price}
              className="num h-11 rounded-xl pr-24"
            />
            <span className="pointer-events-none absolute top-1/2 right-4 -translate-y-1/2 text-sm text-muted-foreground">
              {tc("perUnit", { unit })}
            </span>
          </div>
          <FieldDescription>{t("priceHint")}</FieldDescription>
          {invalid.price && <FieldError>{invalid.price}</FieldError>}
        </Field>
      </FieldGroup>
      <div className="mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <Button type="button" variant="ghost" size="pill" className="h-11" onClick={onDone}>
          {tc("cancel")}
        </Button>
        <Button type="submit" variant="solid" size="pill" className="h-11 sm:px-6" disabled={sold.isPending}>
          {sold.isPending && <Loader2 className="animate-spin" />}
          {sold.isPending ? tc("saving") : t("submit")}
        </Button>
      </div>
    </form>
  );
}

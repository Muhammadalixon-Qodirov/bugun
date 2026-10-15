"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Bot, Info, Loader2, Pause } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { useLimits } from "@/components/limits";
import { ease } from "@/components/motion";
import { clockOrDay } from "@/components/seller/signals";
import { Button } from "@/components/ui/button";
import { Field, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { getMyListingQueryKey, setListingPricingMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { ListingPricing, MyListing } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom, groupDigits } from "@/lib/labels";
import { pausedUntil } from "@/lib/listings";
import { cn } from "@/lib/utils";

const goals = ["fast", "balanced", "profit"] as const;

/** Current price ±10%, rounded outwards so the price itself stays inside. */
function suggestedRange(price: number) {
  const step = price >= 10_000 ? 100 : price >= 1_000 ? 10 : 1;
  return {
    min: Math.max(50, Math.floor((price * 0.9) / step) * step),
    max: Math.min(50_000_000, Math.ceil((price * 1.1) / step) * step),
  };
}

const digits = (v: string) => (v.trim() === "" ? undefined : Number(v.replace(/\s/g, "")));

type Errors = { min?: string; max?: string };

/** The switch, range and goal for automatic pricing of one listing. */
export function AutoPricing({ l }: { l: MyListing }) {
  const t = useTranslations("autoPricing");
  const te = useTranslations("errors");
  const tg = useTranslations("codes.goal");
  const tc = useTranslations("common");
  const fmt = useFormat();
  const errors = useApiErrors();
  const queryClient = useQueryClient();
  const unit = fmt.unit(l.unit);
  const stored = l.pricing;
  const { max_ratio: maxRatio, daily_step: dailyStep, manual_pause_hours: pauseHours } = useLimits().auto_price;

  const [on, setOn] = useState(stored.mode === "auto");
  const [goal, setGoal] = useState<ListingPricing["goal"]>(stored.goal);
  const [min, setMin] = useState(stored.price_min != null ? groupDigits(String(stored.price_min)) : "");
  const [max, setMax] = useState(stored.price_max != null ? groupDigits(String(stored.price_max)) : "");
  const [invalid, setInvalid] = useState<Errors>({});
  const paused = pausedUntil(l);

  const dirty =
    on !== (stored.mode === "auto") || goal !== stored.goal || digits(min) !== stored.price_min || digits(max) !== stored.price_max;

  const save = useMutation({
    ...setListingPricingMutation(),
    onSuccess: (res) => {
      queryClient.setQueryData(getMyListingQueryKey({ path: { id: l.id } }), res);
      toast.success(res.listing.pricing.mode === "auto" ? t("savedOn") : t("savedOff"));
    },
    onError: (err) => {
      const next: Errors = {};
      for (const f of errors.fields(err)) {
        if (f.field === "price_min") next.min = f.message;
        if (f.field === "price_max") next.max = f.message;
      }
      if (next.min || next.max) setInvalid(next);
      else errors.show(err, "save");
    },
  });

  const toggle = () => {
    const next = !on;
    setOn(next);
    setInvalid({});
    // First time on: start from the current price ±10% for the seller to adjust.
    if (next && min === "" && max === "") {
      const r = suggestedRange(l.price_som);
      setMin(groupDigits(String(r.min)));
      setMax(groupDigits(String(r.max)));
    }
  };

  const validate = (lo?: number, hi?: number): Errors => {
    if (!on) return {};
    const e: Errors = {};
    if (lo == null) e.min = te("fields.required");
    else if (lo < 50 || lo > 50_000_000) e.min = te("fields.invalid_price");
    if (hi == null) e.max = te("fields.required");
    else if (hi < 50 || hi > 50_000_000) e.max = te("fields.invalid_price");
    if (e.min || e.max || lo == null || hi == null) return e;
    if (hi < lo) return { max: te("fields.below_min") };
    if (hi > lo * maxRatio) return { max: te("fields.range_too_wide", { max_ratio: maxRatio }) };
    if (l.price_som < lo || l.price_som > hi) return { min: te("fields.price_outside_range", { price: formatSom(l.price_som) }) };
    return {};
  };

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const lo = digits(min);
    const hi = digits(max);
    const found = validate(lo, hi);
    setInvalid(found);
    if (found.min || found.max) return;
    // goal is required by the API; a manual save keeps the range so switching back restores it.
    const keepRange = lo != null && hi != null && hi >= lo && hi <= lo * maxRatio;
    save.mutate({
      path: { id: l.id },
      body: {
        mode: on ? "auto" : "manual",
        goal,
        price_min: on || keepRange ? lo : undefined,
        price_max: on || keepRange ? hi : undefined,
      },
    });
  };

  return (
    <form onSubmit={submit} noValidate aria-labelledby="auto-title" className="rounded-3xl border bg-card">
      <header className="flex items-start gap-4 border-b px-5 py-5 sm:px-6">
        <span className="grid size-10 shrink-0 place-items-center rounded-2xl bg-muted">
          <Bot strokeWidth={1.5} className="size-5" />
        </span>
        <div className="min-w-0 flex-1">
          <h2 id="auto-title" className="font-semibold tracking-tight">
            {t("title")}
          </h2>
          <p className="mt-0.5 text-sm text-muted-foreground">{t("hint")}</p>
        </div>
        <button
          type="button"
          role="switch"
          aria-checked={on}
          aria-label={t("switch")}
          onClick={toggle}
          className="grid min-h-10 min-w-12 shrink-0 place-items-center rounded-full outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <span className={cn("relative h-6 w-11 rounded-full transition-colors", on ? "bg-primary" : "bg-input")}>
            <span
              className={cn(
                "absolute top-0.5 size-5 rounded-full shadow transition-all",
                on ? "left-[22px] bg-primary-foreground" : "left-0.5 bg-background dark:bg-muted-foreground",
              )}
            />
          </span>
        </button>
      </header>

      <div className="space-y-5 px-5 py-5 sm:px-6">
        {paused && on && stored.mode === "auto" && (
          <p className="flex gap-2 rounded-2xl bg-warm-soft px-4 py-3 text-sm text-warm-foreground">
            <Pause className="mt-0.5 size-4 shrink-0" aria-hidden />
            {t("paused", { time: clockOrDay(fmt, paused), hours: Math.round(pauseHours) })}
          </p>
        )}

        <AnimatePresence initial={false}>
          {on && (
            <m.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: "auto" }}
              exit={{ opacity: 0, height: 0 }}
              transition={{ duration: 0.25, ease }}
              className="-m-1 overflow-hidden p-1"
            >
              <div className="space-y-5">
                <div className="grid gap-4 sm:grid-cols-2">
                  <RangeInput id="price-min" label={t("min", { unit })} value={min} onChange={setMin} error={invalid.min} />
                  <RangeInput id="price-max" label={t("max", { unit })} value={max} onChange={setMax} error={invalid.max} />
                </div>
                <p className="text-xs text-muted-foreground">{t("current", { price: formatSom(l.price_som), unit })}</p>

                <fieldset>
                  <legend className="mb-2 text-sm font-medium">{t("goal")}</legend>
                  <div role="radiogroup" aria-label={t("goal")} className="grid gap-2 sm:grid-cols-3">
                    {goals.map((g) => (
                      <button
                        key={g}
                        type="button"
                        role="radio"
                        aria-checked={goal === g}
                        onClick={() => setGoal(g)}
                        className={cn(
                          "rounded-2xl border p-3.5 text-left transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                          goal === g ? "border-foreground/80 bg-muted/40" : "hover:border-foreground/25",
                        )}
                      >
                        <span className="flex items-center gap-2 font-medium">
                          <span
                            className={cn("grid size-4 place-items-center rounded-full border", goal === g && "border-primary bg-primary")}
                            aria-hidden
                          >
                            {goal === g && <span className="size-1.5 rounded-full bg-primary-foreground" />}
                          </span>
                          {tg(`${g}.label`)}
                        </span>
                        <span className="mt-1 block text-xs text-muted-foreground">{tg(`${g}.hint`)}</span>
                      </button>
                    ))}
                  </div>
                </fieldset>

                <p className="flex gap-2 rounded-2xl bg-muted px-4 py-3 text-sm text-muted-foreground">
                  <Info className="mt-0.5 size-4 shrink-0" aria-hidden />
                  {t("warning", { step: Math.round(dailyStep * 1000) / 10 })}
                </p>
              </div>
            </m.div>
          )}
        </AnimatePresence>

        {!on && <p className="text-sm text-muted-foreground">{t("offNote")}</p>}

        <div className="flex justify-end">
          <Button type="submit" variant="solid" size="pill" className="h-11 w-full sm:w-auto sm:px-6" disabled={!dirty || save.isPending}>
            {save.isPending && <Loader2 className="animate-spin" />}
            {save.isPending ? tc("saving") : tc("save")}
          </Button>
        </div>
      </div>
    </form>
  );
}

function RangeInput({
  id,
  label,
  value,
  onChange,
  error,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (v: string) => void;
  error?: string;
}) {
  const tc = useTranslations("common");
  return (
    <Field data-invalid={!!error}>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <div className="relative">
        <Input
          id={id}
          inputMode="numeric"
          value={value}
          onChange={(e) => onChange(groupDigits(e.target.value))}
          aria-invalid={!!error}
          className="num h-11 rounded-xl pr-14 text-base"
        />
        <span className="pointer-events-none absolute top-1/2 right-4 -translate-y-1/2 text-sm text-muted-foreground">{tc("som")}</span>
      </div>
      {error && <FieldError>{error}</FieldError>}
    </Field>
  );
}

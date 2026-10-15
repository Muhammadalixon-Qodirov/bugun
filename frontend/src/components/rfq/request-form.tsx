"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, ArrowRight, CircleCheck, Info, Loader2, Search, Send } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useMemo, useState } from "react";
import { Controller, useForm, useWatch } from "react-hook-form";
import { z } from "zod";

import { useCatalogNames } from "@/components/catalog-names";
import { CategoryIcon } from "@/components/category-icon";
import { useLimits } from "@/components/limits";
import { ease } from "@/components/motion";
import { Segmented } from "@/components/segmented";
import { Button, buttonVariants } from "@/components/ui/button";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { createRfqMutation, listMyRfqsQueryKey, metaLimitsQueryKey } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { Category, CreateRfqOutputBody, Product } from "@/lib/api/gen/types.gen";
import { problemOf, useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { groupDigits } from "@/lib/labels";
import { cn } from "@/lib/utils";

const NOTE_MAX = 300;
// More choices than this get a number field instead of buttons.
const MAX_DAY_BUTTONS = 10;

type T = ReturnType<typeof useTranslations<"rfq">>;
type DayRange = { min: number; max: number };

function makeSchema(t: T, days: DayRange) {
  return z.object({
    quantity: z
      .string()
      .trim()
      .min(1, t("validation.quantityRequired"))
      .refine((v) => {
        const n = Number(v.replace(/\s/g, "").replace(",", "."));
        return n > 0 && n <= 10_000_000;
      }, t("validation.quantity")),
    days: z
      .string()
      .trim()
      .refine((v) => /^\d+$/.test(v) && Number(v) >= days.min && Number(v) <= days.max, t("validation.days", days)),
    maxPrice: z
      .string()
      .trim()
      .refine((v) => {
        if (v === "") return true;
        const n = Number(v.replace(/\s/g, ""));
        return n >= 50 && n <= 50_000_000;
      }, t("validation.maxPrice")),
    note: z.string().trim().max(NOTE_MAX, t("validation.noteMax", { max: NOTE_MAX })),
  });
}
type Values = z.infer<ReturnType<typeof makeSchema>>;

function norm(s: string) {
  return s.toLowerCase().replace(/[‘’`ʻʼ']/g, "");
}

/** Values to start the form with, e.g. from the assistant's proposal (all optional, checked as usual). */
export type RequestPrefill = { product?: string; quantity?: string; days?: string };

/** The buyer's new request: product, quantity, deadline and optional price cap and note. */
export function RequestForm({ products, categories, openCount, prefill }: { products: Product[]; categories: Category[]; openCount: number; prefill?: RequestPrefill }) {
  const t = useTranslations("rfq");
  const [product, setProduct] = useState<Product | null>(() => products.find((p) => p.slug === prefill?.product) ?? null);
  const [sent, setSent] = useState<CreateRfqOutputBody | null>(null);
  const maxOpen = useLimits().rfq.max_open;
  const full = openCount >= maxOpen;

  return (
    <section aria-labelledby="new-request" className="rounded-3xl border bg-card p-5 sm:p-6">
      <AnimatePresence mode="wait" initial={false}>
        {sent ? (
          <m.div key="sent" initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }} transition={{ duration: 0.3, ease }}>
            <Sent
              result={sent}
              canSendMore={openCount < maxOpen}
              onAnother={() => {
                setSent(null);
                setProduct(null);
              }}
            />
          </m.div>
        ) : (
          <m.div key="form" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} transition={{ duration: 0.2 }}>
            <h2 id="new-request" className="font-semibold tracking-tight">
              {t("form.title")}
            </h2>
            <p className="mt-0.5 text-sm text-muted-foreground">{t("form.hint")}</p>
            {full && (
              <p className="mt-5 flex gap-2.5 rounded-2xl bg-warm-soft px-4 py-3 text-sm text-warm-foreground">
                <Info className="mt-0.5 size-4 shrink-0" />
                {t("form.limit", { max: maxOpen })}
              </p>
            )}
            <fieldset disabled={full} className="mt-5 min-w-0 disabled:opacity-60">
              {product ? (
                <Details product={product} prefill={prefill?.product === product.slug ? prefill : undefined} onBack={() => setProduct(null)} onSent={setSent} />
              ) : (
                <ProductPicker products={products} categories={categories} onPick={setProduct} />
              )}
            </fieldset>
          </m.div>
        )}
      </AnimatePresence>
    </section>
  );
}

function ProductPicker({ products, categories, onPick }: { products: Product[]; categories: Category[]; onPick: (p: Product) => void }) {
  const t = useTranslations("rfq.form");
  const names = useCatalogNames();
  const fmt = useFormat();
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<string | null>(null);
  // Match the shown (translated) name and the Uzbek one, whichever the buyer types.
  const q = norm(query);
  const visible = products.filter(
    (p) => (!category || p.category === category) && (!q || norm(names.product(p.slug, p.name)).includes(q) || norm(p.name).includes(q)),
  );

  return (
    <div className="space-y-3">
      <label className="relative block">
        <span className="sr-only">{t("product")}</span>
        <Search className="pointer-events-none absolute top-1/2 left-4 size-4 -translate-y-1/2 text-muted-foreground" />
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("search")}
          className="h-12 w-full rounded-2xl border bg-background pl-11 text-base outline-none focus:border-foreground/40 focus:ring-4 focus:ring-foreground/5"
        />
      </label>
      <div className="-mx-5 flex gap-1.5 overflow-x-auto px-5 [scrollbar-width:none] sm:-mx-6 sm:px-6">
        {[{ slug: null, name: t("all") } as { slug: string | null; name: string }, ...categories].map((c) => (
          <button
            key={c.slug ?? "all"}
            type="button"
            aria-pressed={category === c.slug}
            onClick={() => setCategory(c.slug)}
            className={cn(
              "h-9 shrink-0 rounded-full border px-3.5 text-sm transition-colors",
              category === c.slug ? "border-foreground bg-foreground text-background" : "bg-background text-muted-foreground hover:text-foreground",
            )}
          >
            {c.slug ? names.category(c.slug, c.name) : c.name}
          </button>
        ))}
      </div>
      <div className="-mx-1 max-h-72 overflow-y-auto overscroll-contain px-1 py-1" data-lenis-prevent>
        {visible.length === 0 ? (
          <p className="rounded-2xl border border-dashed p-6 text-center text-sm text-muted-foreground">{t("empty")}</p>
        ) : (
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
            {visible.map((p) => (
              <button
                key={p.id}
                type="button"
                onClick={() => onPick(p)}
                className="flex min-h-14 items-center gap-2.5 rounded-2xl border bg-background p-2.5 text-left transition-colors outline-none hover:border-foreground/30 focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                <CategoryIcon category={p.category} className="size-9 rounded-xl" />
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium">{names.product(p.slug, p.name)}</span>
                  <span className="text-xs text-muted-foreground">{fmt.unit(p.base_unit)}</span>
                </span>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function Details({
  product,
  prefill,
  onBack,
  onSent,
}: {
  product: Product;
  prefill?: RequestPrefill;
  onBack: () => void;
  onSent: (r: CreateRfqOutputBody) => void;
}) {
  const t = useTranslations("rfq");
  const tc = useTranslations("common");
  const errors = useApiErrors();
  const names = useCatalogNames();
  const fmt = useFormat();
  const queryClient = useQueryClient();
  const unit = fmt.unit(product.base_unit);
  const limits = useLimits().rfq;
  const range = useMemo(() => ({ min: limits.min_days, max: limits.max_days }), [limits.min_days, limits.max_days]);
  const dayOptions = Array.from({ length: Math.max(0, range.max - range.min + 1) }, (_, i) => String(range.min + i));
  const schema = useMemo(() => makeSchema(t, range), [t, range]);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      quantity: prefill?.quantity && Number(prefill.quantity) > 0 ? prefill.quantity : "",
      days: String(Math.min(range.max, Math.max(range.min, Number(prefill?.days) || limits.default_days))),
      maxPrice: "",
      note: "",
    },
  });
  const create = useMutation({
    ...createRfqMutation(),
    onSettled: () => queryClient.invalidateQueries({ queryKey: listMyRfqsQueryKey() }),
  });

  const onSubmit = form.handleSubmit(async (v) => {
    try {
      const res = await create.mutateAsync({
        body: {
          product_id: product.id,
          quantity: Number(v.quantity.replace(/\s/g, "").replace(",", ".")),
          days: Number(v.days),
          max_price_som: v.maxPrice ? Number(v.maxPrice.replace(/\s/g, "")) : undefined,
          note: v.note || undefined,
        },
      });
      onSent(res);
    } catch (err) {
      const map: Record<string, keyof Values> = { quantity: "quantity", days: "days", max_price_som: "maxPrice", note: "note" };
      const problem = problemOf(err);
      const fields = errors.fields(err);
      for (const [i, { field, message }] of fields.entries()) {
        const f = problem.fields[i];
        // The allowed deadline changed since the page loaded: say the API's range and load the new one.
        if (field === "days" && f?.code === "out_of_range") {
          form.setError("days", { message: t("validation.days", { min: Number(f.params.min), max: Number(f.params.max) }) });
          void queryClient.invalidateQueries({ queryKey: metaLimitsQueryKey() });
        } else if (map[field]) form.setError(map[field], { message });
      }
      if (fields.length === 0 || problemOf(err).code !== "validation_failed") errors.show(err, "save");
    }
  });

  const [days, note] = useWatch({ control: form.control, name: ["days", "note"] });

  return (
    <form onSubmit={onSubmit} noValidate>
      <div className="flex items-center gap-3 rounded-2xl bg-muted/60 p-3">
        <CategoryIcon category={product.category} className="size-11 rounded-xl" />
        <div className="min-w-0 flex-1">
          <p className="text-xs text-muted-foreground">{t("form.product")}</p>
          <p className="truncate font-semibold tracking-tight">{names.product(product.slug, product.name)}</p>
        </div>
        <Button type="button" variant="outline" className="h-10 rounded-full px-4" onClick={onBack}>
          <ArrowLeft /> {t("form.change")}
        </Button>
      </div>

      <FieldGroup className="mt-5">
        <div className="grid gap-5 sm:grid-cols-2">
          <Controller
            name="quantity"
            control={form.control}
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid}>
                <FieldLabel htmlFor="rfq-quantity">{t("form.quantity")}</FieldLabel>
                <div className="relative">
                  <Input
                    id="rfq-quantity"
                    inputMode="decimal"
                    autoFocus
                    placeholder="500"
                    className="num h-12 rounded-xl pr-16 text-lg font-semibold md:text-lg"
                    aria-invalid={fieldState.invalid}
                    {...field}
                  />
                  <span className="pointer-events-none absolute top-1/2 right-4 -translate-y-1/2 text-sm text-muted-foreground">{unit}</span>
                </div>
                {fieldState.error && <FieldError errors={[fieldState.error]} />}
              </Field>
            )}
          />
          <Controller
            name="maxPrice"
            control={form.control}
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid}>
                <FieldLabel htmlFor="rfq-max-price">
                  {t("form.maxPrice")} <span className="font-normal text-muted-foreground">· {tc("optional")}</span>
                </FieldLabel>
                <div className="relative">
                  <Input
                    id="rfq-max-price"
                    inputMode="numeric"
                    placeholder="4 500"
                    className="num h-12 rounded-xl pr-24 text-lg font-semibold md:text-lg"
                    aria-invalid={fieldState.invalid}
                    {...field}
                    onChange={(e) => field.onChange(groupDigits(e.target.value))}
                  />
                  <span className="pointer-events-none absolute top-1/2 right-4 -translate-y-1/2 text-sm text-muted-foreground">
                    {tc("perUnit", { unit })}
                  </span>
                </div>
                {fieldState.error ? <FieldError errors={[fieldState.error]} /> : <FieldDescription>{t("form.maxPriceHint")}</FieldDescription>}
              </Field>
            )}
          />
        </div>

        <Controller
          name="days"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel>
                {t("form.days")}{" "}
                {/^\d+$/.test(days) && <span className="num font-normal text-muted-foreground">· {t("form.daysValue", { count: Number(days) })}</span>}
              </FieldLabel>
              {dayOptions.length <= MAX_DAY_BUTTONS ? (
                <Segmented
                  label={t("form.days")}
                  size="sm"
                  className="w-full sm:max-w-sm"
                  value={field.value}
                  onChange={field.onChange}
                  options={dayOptions.map((d) => ({ value: d, label: <span className="num">{d}</span> }))}
                />
              ) : (
                <Input
                  inputMode="numeric"
                  aria-label={t("form.days")}
                  className="num h-12 w-32 rounded-xl text-lg font-semibold md:text-lg"
                  aria-invalid={fieldState.invalid}
                  {...field}
                />
              )}
              {fieldState.error ? <FieldError errors={[fieldState.error]} /> : <FieldDescription>{t("form.daysHint")}</FieldDescription>}
            </Field>
          )}
        />

        <Controller
          name="note"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor="rfq-note">
                {t("form.note")} <span className="font-normal text-muted-foreground">· {tc("optional")}</span>
              </FieldLabel>
              <Textarea
                id="rfq-note"
                rows={3}
                maxLength={NOTE_MAX}
                placeholder={t("form.notePlaceholder")}
                className="rounded-xl"
                aria-invalid={fieldState.invalid}
                aria-describedby="rfq-note-count"
                {...field}
              />
              <div className="flex justify-between gap-3">
                {fieldState.error ? <FieldError errors={[fieldState.error]} /> : <FieldDescription>{t("form.noteHint")}</FieldDescription>}
                <span id="rfq-note-count" className={cn("num shrink-0 text-xs", note.length >= NOTE_MAX ? "text-warm-foreground" : "text-muted-foreground")}>
                  {note.length}/{NOTE_MAX}
                </span>
              </div>
            </Field>
          )}
        />
      </FieldGroup>

      <Button type="submit" variant="solid" size="pill-lg" disabled={create.isPending} className="mt-6 w-full sm:w-auto">
        {create.isPending ? <Loader2 className="animate-spin" /> : <Send />}
        {create.isPending ? t("form.sending") : t("form.submit")}
      </Button>
    </form>
  );
}

function Sent({ result, canSendMore, onAnother }: { result: CreateRfqOutputBody; canSendMore: boolean; onAnother: () => void }) {
  const t = useTranslations("rfq.sent");
  const none = result.sellers === 0;
  return (
    <div className="flex flex-col items-start gap-4 sm:flex-row">
      <span className={cn("grid size-12 shrink-0 place-items-center rounded-2xl", none ? "bg-warm-soft text-warm-foreground" : "bg-primary/10 text-primary")}>
        {none ? <Info className="size-5" /> : <CircleCheck className="size-5" />}
      </span>
      <div className="min-w-0 flex-1">
        <h2 className="text-lg font-semibold tracking-tight">{none ? t("noneTitle") : t("title", { count: result.sellers })}</h2>
        <p className="mt-1 max-w-lg text-sm leading-relaxed text-muted-foreground">{none ? t("noneText") : t("text")}</p>
        <div className="mt-5 flex flex-wrap gap-2">
          <Link href={`/sorovlar/${result.rfq.id}`} className={buttonVariants({ variant: "solid", size: "pill" })}>
            {t("open")} <ArrowRight />
          </Link>
          {canSendMore && (
            <Button variant="outline" size="pill" onClick={onAnother}>
              {t("another")}
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}

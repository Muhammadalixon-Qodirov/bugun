"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { ArrowLeft, ArrowRight, Check, Landmark, Lock, Package, Store } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useMemo, useState } from "react";
import { Controller, useForm, useWatch } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";

import { categoryIcons } from "@/components/category-icon";
import { useCatalogNames } from "@/components/catalog-names";
import { ease } from "@/components/motion";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { updateProfileMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { GetCatalogResponse, Me } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { SELF_ROLES, selfRole } from "@/lib/roles";
import { cn } from "@/lib/utils";

const roles = [
  { value: "entrepreneur", icon: Store },
  { value: "market_admin", icon: Landmark },
] as const;

type T = ReturnType<typeof useTranslations<"profile.form">>;

function makeSchema(t: T) {
  return z
    .object({
      role: z.enum(SELF_ROLES, { error: t("errors.role") }),
      region_id: z.string().min(1, t("errors.region")),
      market_id: z.string().optional(),
      categories: z.array(z.string()),
      public_name: z.string().trim().max(60, t("errors.publicNameMax", { max: 60 })),
      consent: z.boolean().refine((v) => v, t("errors.consent")),
    })
    .superRefine((v, ctx) => {
      if (v.role === "market_admin" && !v.market_id) {
        ctx.addIssue({ code: "custom", path: ["market_id"], message: t("errors.market") });
      }
    });
}

type Values = z.infer<ReturnType<typeof makeSchema>>;

export function ProfileForm({
  catalog,
  me,
  submitLabel,
  redirectTo,
  wizard,
  intro,
}: {
  catalog: GetCatalogResponse;
  me: Me;
  submitLabel: string;
  redirectTo?: string;
  /** Onboarding: one question per step instead of the full form. */
  wizard?: boolean;
  /** Onboarding: greeting shown above the step list. */
  intro?: React.ReactNode;
}) {
  const router = useRouter();
  const t = useTranslations("profile.form");
  const tc = useTranslations("common");
  const fmt = useFormat();
  const names = useCatalogNames();
  const errors = useApiErrors();
  const schema = useMemo(() => makeSchema(t), [t]);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      role: selfRole(me.role),
      region_id: me.region_id ? String(me.region_id) : "",
      market_id: me.market_id ? String(me.market_id) : undefined,
      categories: me.categories,
      public_name: me.public_name ?? "",
      consent: Boolean(me.consent_at),
    },
  });
  const role = useWatch({ control: form.control, name: "role" });
  const regionId = useWatch({ control: form.control, name: "region_id" });

  const provinces = catalog.regions.filter((r) => r.parent_id == null);
  const [provinceId, setProvinceId] = useState(() => {
    const current = catalog.regions.find((r) => r.id === me.region_id);
    return current?.parent_id ? String(current.parent_id) : "";
  });
  const districts = catalog.regions.filter((r) => String(r.parent_id) === provinceId);
  const markets = catalog.markets.filter((m) => String(m.region_id) === regionId);
  // Markets are listed for the pilot region only, so only market staff must pick one.
  const needsMarket = role === "market_admin";
  const entrepreneur = role === "entrepreneur";
  const showMarket = needsMarket || (entrepreneur && markets.length > 0);

  const save = useMutation({
    ...updateProfileMutation(),
    onSuccess: () => {
      toast.success(t("saved"));
      if (redirectTo) router.push(redirectTo);
      router.refresh();
    },
    onError: (err) => {
      for (const { field, message } of errors.fields(err)) {
        if (field in form.getValues()) form.setError(field as keyof Values, { message });
      }
      errors.show(err, "save");
    },
  });

  const onSubmit = form.handleSubmit((v) =>
    save.mutate({
      body: {
        role: v.role,
        region_id: Number(v.region_id),
        market_id: showMarket && v.market_id ? Number(v.market_id) : undefined,
        categories: v.role === "entrepreneur" ? v.categories : [],
        public_name: v.public_name || undefined,
        consent: v.consent,
      },
    }),
  );

  const steps: { title: string; hint: string; fields: (keyof Values)[] }[] = [
    { title: t("steps.role"), hint: t("steps.roleHint"), fields: ["role"] },
    { title: t("steps.place"), hint: t("steps.placeHint"), fields: ["region_id", "market_id"] },
    ...(entrepreneur
      ? [{ title: t("steps.sell"), hint: t("steps.sellHint"), fields: ["categories", "public_name"] as (keyof Values)[] }]
      : []),
  ];
  const [step, setStep] = useState(0);
  const current = Math.min(step, steps.length - 1);
  const last = current === steps.length - 1;
  const next = async () => {
    if (await form.trigger(steps[current].fields)) setStep(current + 1);
  };

  const roleField = (
    <Controller
      name="role"
      control={form.control}
      render={({ field, fieldState }) => (
        <Field data-invalid={fieldState.invalid}>
          <div role="radiogroup" aria-label={t("roleGroup")} aria-invalid={fieldState.invalid} className="grid gap-3 sm:grid-cols-2">
            {roles.map((r) => {
              const checked = field.value === r.value;
              return (
                <m.button
                  key={r.value}
                  type="button"
                  role="radio"
                  aria-checked={checked}
                  onClick={() => {
                    field.onChange(r.value);
                    form.clearErrors("role");
                  }}
                  whileTap={{ scale: 0.98 }}
                  className={cn(
                    "relative isolate flex items-start gap-3 rounded-2xl border p-4 text-left transition-colors",
                    checked ? "border-transparent" : "bg-card hover:border-foreground/25",
                  )}
                >
                  {checked && (
                    <m.span
                      layoutId="role-highlight"
                      className="absolute inset-0 -z-10 rounded-2xl bg-card ring-2 ring-foreground"
                      transition={{ type: "spring", stiffness: 420, damping: 34 }}
                    />
                  )}
                  <span
                    className={cn(
                      "grid size-10 shrink-0 place-items-center rounded-xl transition-colors",
                      checked ? "bg-foreground text-background" : "bg-muted text-foreground",
                    )}
                  >
                    <r.icon strokeWidth={1.5} className="size-5" />
                  </span>
                  <span className="min-w-0 pr-6">
                    <span className="block font-medium">{fmt.role(r.value)}</span>
                    <span className="mt-0.5 block text-sm leading-relaxed text-muted-foreground">{t(`roles.${r.value}`)}</span>
                    <span className="mt-2.5 inline-flex rounded-full bg-muted px-2.5 py-0.5 text-xs font-medium text-foreground/80">
                      {t(`roleNotes.${r.value}`)}
                    </span>
                  </span>
                  <span
                    className={cn(
                      "absolute top-4 right-4 grid size-5 place-items-center rounded-full border transition-colors",
                      checked ? "border-foreground bg-foreground text-background" : "border-input",
                    )}
                  >
                    {checked && <Check className="size-3" />}
                  </span>
                </m.button>
              );
            })}
          </div>
          {fieldState.error && <FieldError errors={[fieldState.error]} />}
        </Field>
      )}
    />
  );

  const placeFields = (
    <div className="grid gap-4 sm:grid-cols-2">
      <Field>
        <FieldLabel htmlFor="province">{t("province")}</FieldLabel>
        <Select
          items={provinces.map((r) => ({ value: String(r.id), label: names.region(r.name) }))}
          value={provinceId || null}
          onValueChange={(v) => {
            setProvinceId(v ?? "");
            form.setValue("region_id", "");
            form.setValue("market_id", undefined);
          }}
        >
          <SelectTrigger id="province" className="h-11! w-full rounded-xl bg-card">
            <SelectValue placeholder={t("choose")} />
          </SelectTrigger>
          <SelectContent>
            {provinces.map((r) => (
              <SelectItem key={r.id} value={String(r.id)}>
                {names.region(r.name)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>

      <Controller
        name="region_id"
        control={form.control}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor="region">{t("district")}</FieldLabel>
            <Select
              items={districts.map((r) => ({ value: String(r.id), label: names.region(r.name) }))}
              value={field.value || null}
              onValueChange={(v) => {
                field.onChange(v ?? "");
                form.setValue("market_id", undefined);
                form.clearErrors("region_id");
              }}
              disabled={!provinceId}
            >
              <SelectTrigger id="region" className="h-11! w-full rounded-xl bg-card" aria-invalid={fieldState.invalid}>
                <SelectValue placeholder={provinceId ? t("choose") : t("chooseProvinceFirst")} />
              </SelectTrigger>
              <SelectContent>
                {districts.map((r) => (
                  <SelectItem key={r.id} value={String(r.id)}>
                    {names.region(r.name)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {fieldState.error && <FieldError errors={[fieldState.error]} />}
          </Field>
        )}
      />

      {showMarket && (
        <Controller
          name="market_id"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid} className="sm:col-span-2">
              <FieldLabel htmlFor="market">{needsMarket ? t("market") : t("marketOptional")}</FieldLabel>
              <Select
                items={markets.map((m) => ({ value: String(m.id), label: names.market(m.name) }))}
                value={field.value ?? null}
                onValueChange={(v) => {
                  field.onChange(v ?? undefined);
                  form.clearErrors("market_id");
                }}
                disabled={!regionId}
              >
                <SelectTrigger id="market" className="h-11! w-full rounded-xl bg-card" aria-invalid={fieldState.invalid}>
                  <SelectValue placeholder={regionId ? t("choose") : t("chooseDistrictFirst")} />
                </SelectTrigger>
                <SelectContent>
                  {markets.map((m) => (
                    <SelectItem key={m.id} value={String(m.id)}>
                      {names.market(m.name)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {regionId && markets.length === 0 && <FieldDescription>{t("noMarkets")}</FieldDescription>}
              {fieldState.error && <FieldError errors={[fieldState.error]} />}
            </Field>
          )}
        />
      )}
    </div>
  );

  const sellerFields = (
    <FieldGroup>
      <Controller
        name="categories"
        control={form.control}
        render={({ field, fieldState }) => (
          <FieldSet data-invalid={fieldState.invalid}>
            <FieldLegend variant="label">{t("categories")}</FieldLegend>
            <FieldDescription className="-mt-1 mb-1">{t("categoriesHint")}</FieldDescription>
            <div className="flex flex-wrap gap-2">
              {catalog.categories.map((c) => {
                const checked = field.value.includes(c.slug);
                const Icon = categoryIcons[c.slug] ?? Package;
                return (
                  <m.button
                    key={c.slug}
                    type="button"
                    role="checkbox"
                    aria-checked={checked}
                    whileTap={{ scale: 0.95 }}
                    onClick={() => {
                      field.onChange(checked ? field.value.filter((x) => x !== c.slug) : [...field.value, c.slug]);
                      form.clearErrors("categories");
                    }}
                    className={cn(
                      "flex h-10 items-center gap-2 rounded-full border px-4 text-sm transition-colors",
                      checked
                        ? "border-foreground bg-foreground text-background"
                        : "bg-card text-muted-foreground hover:border-foreground/25 hover:text-foreground",
                    )}
                  >
                    <Icon strokeWidth={1.5} className="size-4" />
                    {names.category(c.slug, c.name)}
                  </m.button>
                );
              })}
            </div>
            {fieldState.error && <FieldError errors={[fieldState.error]} />}
          </FieldSet>
        )}
      />
      <Controller
        name="public_name"
        control={form.control}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor="public_name">{t("publicName")}</FieldLabel>
            <Input id="public_name" {...field} placeholder={t("publicNamePlaceholder")} className="h-11 rounded-xl bg-card" aria-invalid={fieldState.invalid} />
            <FieldDescription>{t("publicNameHint")}</FieldDescription>
            {fieldState.error && <FieldError errors={[fieldState.error]} />}
          </Field>
        )}
      />
    </FieldGroup>
  );

  // Consent is asked once; after that the checkbox would only be noise on the profile page.
  const consentField = (
    <Controller
      name="consent"
      control={form.control}
      render={({ field, fieldState }) => (
        <Field orientation="horizontal" data-invalid={fieldState.invalid} className="rounded-2xl bg-muted/60 p-4">
          <Checkbox id="consent" checked={field.value} onCheckedChange={(v) => {
              field.onChange(Boolean(v));
              form.clearErrors("consent");
            }} aria-invalid={fieldState.invalid} />
          <FieldContent>
            <FieldLabel htmlFor="consent" className="leading-relaxed font-normal">
              {t("consent")}
            </FieldLabel>
            {fieldState.error && <FieldError errors={[fieldState.error]} />}
          </FieldContent>
        </Field>
      )}
    />
  );
  const needsConsent = !me.consent_at;
  const consented = useWatch({ control: form.control, name: "consent" });

  const submitButton = (
    <Button
      type="submit"
      variant="solid"
      size="pill-lg"
      disabled={save.isPending || !consented}
    >
      {save.isPending ? tc("saving") : submitLabel}
    </Button>
  );

  if (wizard) {
    return (
      <div className="grid gap-10 lg:grid-cols-[17rem_1fr] lg:gap-20">
        <aside className="lg:sticky lg:top-28 lg:self-start">
          {intro}
          <ol className="mt-10 hidden space-y-1 lg:block" aria-label={t("stepsLabel")}>
            {steps.map((st, i) => {
              const state = i < current ? "done" : i === current ? "current" : "next";
              return (
                <li key={st.title} aria-current={state === "current" ? "step" : undefined} className="flex gap-3 py-2">
                  <span
                    className={cn(
                      "grid size-7 shrink-0 place-items-center rounded-full text-xs font-medium transition-colors duration-300",
                      state === "next" ? "border text-muted-foreground" : "bg-foreground text-background",
                    )}
                  >
                    {state === "done" ? <Check className="size-3.5" /> : i + 1}
                  </span>
                  <span className="pt-1">
                    <span className={cn("block text-sm font-medium transition-colors", state === "next" && "text-muted-foreground")}>{st.title}</span>
                    {state === "current" && <span className="mt-0.5 block text-xs leading-relaxed text-muted-foreground">{st.hint}</span>}
                  </span>
                </li>
              );
            })}
          </ol>
          <p className="mt-10 hidden items-start gap-2 text-xs leading-relaxed text-muted-foreground lg:flex">
            <Lock className="mt-px size-3.5 shrink-0" />
            {t("phoneNote")}
          </p>
        </aside>

        <form
          onSubmit={(e) => {
            // Enter on an early step moves forward instead of submitting half a profile.
            if (!last) {
              e.preventDefault();
              void next();
              return;
            }
            void onSubmit(e);
          }}
          noValidate
          className="rounded-3xl border bg-card p-6 sm:p-10"
        >
          <div className="flex items-center gap-3">
            <div className="flex flex-1 gap-1.5" aria-hidden>
              {steps.map((_, i) => (
                <span key={i} className={cn("h-1 flex-1 rounded-full transition-colors duration-500", i <= current ? "bg-foreground" : "bg-border")} />
              ))}
            </div>
            <span className="num text-sm text-muted-foreground">
              {current + 1} / {steps.length}
            </span>
          </div>

          <AnimatePresence mode="wait" initial={false}>
            <m.div
              key={current}
              initial={{ opacity: 0, x: 12 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -12 }}
              transition={{ duration: 0.22, ease }}
              className="mt-8"
            >
              <h2 className="display text-2xl sm:text-3xl">{steps[current].title}</h2>
              <p className="mt-2 text-muted-foreground">{steps[current].hint}</p>
              <div className="mt-8 space-y-6">
                {current === 0 && roleField}
                {current === 1 && placeFields}
                {current === 2 && sellerFields}
                {last && needsConsent && consentField}
              </div>
            </m.div>
          </AnimatePresence>

          <div className="mt-10 flex items-center justify-between gap-3 border-t pt-6">
            {current > 0 ? (
              <Button type="button" variant="ghost" size="pill-lg" className="px-5" onClick={() => setStep(current - 1)}>
                <ArrowLeft /> {tc("back")}
              </Button>
            ) : (
              <span />
            )}
            {last ? (
              submitButton
            ) : (
              <Button type="submit" variant="solid" size="pill-lg">
                {t("continue")} <ArrowRight />
              </Button>
            )}
          </div>
          {last && needsConsent && !consented && (
            <p className="mt-3 text-right text-xs text-muted-foreground">{t("consentNeeded")}</p>
          )}
        </form>
      </div>
    );
  }

  return (
    <form onSubmit={onSubmit} noValidate className="divide-y">
      <FormSection title={t("sections.role")} hint={t("sections.roleHint")}>
        {roleField}
      </FormSection>
      <FormSection title={t("sections.place")} hint={t("sections.placeHint")}>
        {placeFields}
      </FormSection>
      {entrepreneur && (
        <FormSection title={t("sections.sell")} hint={t("sections.sellHint")}>
          {sellerFields}
        </FormSection>
      )}
      <div className="space-y-5 pt-8">
        {needsConsent && consentField}
        {submitButton}
      </div>
    </form>
  );
}

function FormSection({ title, hint, children }: { title: string; hint: string; children: React.ReactNode }) {
  return (
    <section className="py-8 first:pt-0">
      <h3 className="font-semibold tracking-tight">{title}</h3>
      <p className="mt-0.5 mb-5 text-sm text-muted-foreground">{hint}</p>
      {children}
    </section>
  );
}

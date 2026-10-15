"use client";

import { useMutation } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { eventKinds, type EventKind, isEventKind } from "@/components/admin/event-kind";
import { useCatalogNames } from "@/components/catalog-names";
import { Segmented } from "@/components/segmented";
import { Button } from "@/components/ui/button";
import { DialogClose, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { adminCreateEventMutation, adminUpdateEventMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { EventBody, MarketEvent } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import type { AdminCatalog } from "@/lib/server/admin";

type Target = "all" | "category" | "product";
type Category = NonNullable<EventBody["category"]>;

const WHOLE_COUNTRY = "0";
const NOTE_MAX = 1000;

/**
 * Create or edit a market event. Optional fields left empty are omitted from
 * the body: the API rejects an empty ends_on, and PUT replaces the whole event.
 */
export function EventForm({ event, catalog, onDone }: { event?: MarketEvent; catalog: AdminCatalog; onDone: () => void }) {
  const t = useTranslations("admin.events.form");
  const ta = useTranslations("admin.events");
  const tcodes = useTranslations("codes");
  const tc = useTranslations("common");
  const terr = useTranslations("errors");
  const names = useCatalogNames();
  const errors = useApiErrors();

  const [title, setTitle] = useState(event?.title ?? "");
  const [kind, setKind] = useState<EventKind | null>(event && isEventKind(event.kind) ? event.kind : null);
  const [startsOn, setStartsOn] = useState(event?.starts_on ?? "");
  const [endsOn, setEndsOn] = useState(event?.ends_on ?? "");
  const [province, setProvince] = useState(event?.province_id ? String(event.province_id) : WHOLE_COUNTRY);
  const [target, setTarget] = useState<Target>(event?.product_id ? "product" : event?.category ? "category" : "all");
  const [category, setCategory] = useState<string | null>(event?.category ?? null);
  const [product, setProduct] = useState<string | null>(event?.product_id ? String(event.product_id) : null);
  const [sourceUrl, setSourceUrl] = useState(event?.source_url ?? "");
  const [note, setNote] = useState(event?.note ?? "");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  const onSuccess = () => {
    toast.success(event ? ta("updated") : ta("created"));
    onDone();
  };
  const onError = (err: unknown) => {
    const fields = errors.fields(err);
    if (fields.length === 0) errors.show(err, "save");
    setFieldErrors(Object.fromEntries(fields.map((f) => [f.field, f.message])));
  };
  const create = useMutation({ ...adminCreateEventMutation(), onSuccess, onError });
  const update = useMutation({ ...adminUpdateEventMutation(), onSuccess, onError });
  const pending = create.isPending || update.isPending;

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    // What the body can't be sent without is checked here; the rest is left to the API.
    const required = terr("fields.required");
    const missing: Record<string, string> = {
      ...(!kind ? { kind: required } : {}),
      ...(!startsOn ? { starts_on: required } : {}),
      ...(target === "product" && !product ? { product_id: required } : {}),
      ...(target === "category" && !category ? { category: required } : {}),
    };
    setFieldErrors(missing);
    if (!kind || Object.keys(missing).length > 0) return;
    const body: EventBody = {
      title: title.trim(),
      kind,
      starts_on: startsOn,
      ...(endsOn ? { ends_on: endsOn } : {}),
      ...(province !== WHOLE_COUNTRY ? { region_id: Number(province) } : {}),
      ...(target === "product" && product ? { product_id: Number(product) } : {}),
      ...(target === "category" && category ? { category: category as Category } : {}),
      ...(sourceUrl.trim() ? { source_url: sourceUrl.trim() } : {}),
      ...(note.trim() ? { note: note.trim() } : {}),
    };
    if (event) update.mutate({ path: { id: event.id }, body });
    else create.mutate({ body });
  };

  const error = (field: string) => fieldErrors[field];
  const invalid = (field: string) => (error(field) ? { "aria-invalid": true, "aria-describedby": `event-${field}-error` } : {});
  const errorText = (field: string) => error(field) && <FieldError id={`event-${field}-error`}>{error(field)}</FieldError>;

  const kindItems = eventKinds.map(({ kind: k }) => ({ value: k, label: tcodes(`eventKind.${k}`) }));
  const provinceItems = [
    { value: WHOLE_COUNTRY, label: tc("wholeCountry") },
    ...catalog.provinces.map((p) => ({ value: String(p.id), label: names.region(p.name) })),
  ];
  const categoryItems = catalog.categories.map((c) => ({ value: c.slug, label: names.category(c.slug, c.name) }));
  const productItems = catalog.products.map((p) => ({ value: String(p.id), label: names.product(p.slug, p.name) }));

  return (
    <form onSubmit={submit} noValidate>
      <DialogTitle>{event ? t("editTitle") : t("newTitle")}</DialogTitle>
      <DialogDescription>{t("description")}</DialogDescription>

      <div className="mt-5 grid gap-4">
        <Field data-invalid={!!error("title")}>
          <FieldLabel htmlFor="event-title">{t("title")}</FieldLabel>
          <Input
            id="event-title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            maxLength={200}
            placeholder={t("titlePlaceholder")}
            className="h-11 rounded-xl"
            {...invalid("title")}
          />
          {errorText("title")}
        </Field>

        <Field data-invalid={!!error("kind")}>
          <FieldLabel htmlFor="event-kind">{t("kind")}</FieldLabel>
          <Select items={kindItems} value={kind} onValueChange={(v) => setKind(v as EventKind | null)}>
            <SelectTrigger id="event-kind" className="h-11! w-full rounded-xl" {...invalid("kind")}>
              <SelectValue placeholder={t("choose")}>
                {(v: EventKind | null) => {
                  const k = eventKinds.find((x) => x.kind === v);
                  return k ? (
                    <>
                      <k.icon className="text-muted-foreground" /> {tcodes(`eventKind.${k.kind}`)}
                    </>
                  ) : (
                    t("choose")
                  );
                }}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              {eventKinds.map(({ kind: k, icon: Icon }) => (
                <SelectItem key={k} value={k}>
                  <Icon className="text-muted-foreground" /> {tcodes(`eventKind.${k}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {errorText("kind")}
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field data-invalid={!!error("starts_on")}>
            <FieldLabel htmlFor="event-starts">{t("startsOn")}</FieldLabel>
            <Input id="event-starts" type="date" value={startsOn} onChange={(e) => setStartsOn(e.target.value)} className="h-11 rounded-xl" {...invalid("starts_on")} />
            {errorText("starts_on")}
          </Field>
          <Field data-invalid={!!error("ends_on")}>
            <FieldLabel htmlFor="event-ends">
              {t("endsOn")} <span className="font-normal text-muted-foreground">({tc("optional")})</span>
            </FieldLabel>
            <Input
              id="event-ends"
              type="date"
              value={endsOn}
              min={startsOn || undefined}
              onChange={(e) => setEndsOn(e.target.value)}
              className="h-11 rounded-xl"
              {...invalid("ends_on")}
            />
            {errorText("ends_on") || <FieldDescription className="text-xs">{t("endsOnHint")}</FieldDescription>}
          </Field>
        </div>

        <Field data-invalid={!!error("region_id")}>
          <FieldLabel htmlFor="event-province">{t("province")}</FieldLabel>
          <Select items={provinceItems} value={province} onValueChange={(v) => setProvince(v ?? WHOLE_COUNTRY)}>
            <SelectTrigger id="event-province" className="h-11! w-full rounded-xl" {...invalid("region_id")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {provinceItems.map((p) => (
                <SelectItem key={p.value} value={p.value}>
                  {p.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {errorText("region_id")}
        </Field>

        <Field>
          <span className="text-sm font-medium">{t("target")}</span>
          <Segmented
            label={t("target")}
            className="w-full"
            value={target}
            onChange={setTarget}
            options={[
              { value: "all", label: t("targetAll") },
              { value: "category", label: t("targetCategory") },
              { value: "product", label: t("targetProduct") },
            ]}
          />
        </Field>

        {target === "category" && (
          <Field data-invalid={!!error("category")}>
            <FieldLabel htmlFor="event-category">{t("category")}</FieldLabel>
            <Select items={categoryItems} value={category} onValueChange={(v) => setCategory(v as string | null)}>
              <SelectTrigger id="event-category" className="h-11! w-full rounded-xl" {...invalid("category")}>
                <SelectValue placeholder={t("choose")} />
              </SelectTrigger>
              <SelectContent>
                {categoryItems.map((c) => (
                  <SelectItem key={c.value} value={c.value}>
                    {c.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {errorText("category")}
          </Field>
        )}

        {target === "product" && (
          <Field data-invalid={!!error("product_id")}>
            <FieldLabel htmlFor="event-product">{t("product")}</FieldLabel>
            <Select items={productItems} value={product} onValueChange={(v) => setProduct(v as string | null)}>
              <SelectTrigger id="event-product" className="h-11! w-full rounded-xl" {...invalid("product_id")}>
                <SelectValue placeholder={t("choose")} />
              </SelectTrigger>
              <SelectContent>
                {categoryItems.map((c) => (
                  <SelectGroup key={c.value}>
                    <SelectLabel>{c.label}</SelectLabel>
                    {catalog.products
                      .filter((p) => p.category === c.value)
                      .map((p) => (
                        <SelectItem key={p.id} value={String(p.id)}>
                          {names.product(p.slug, p.name)}
                        </SelectItem>
                      ))}
                  </SelectGroup>
                ))}
              </SelectContent>
            </Select>
            {errorText("product_id")}
          </Field>
        )}

        <Field data-invalid={!!error("source_url")}>
          <FieldLabel htmlFor="event-source">
            {t("sourceUrl")} <span className="font-normal text-muted-foreground">({tc("optional")})</span>
          </FieldLabel>
          <Input
            id="event-source"
            type="url"
            inputMode="url"
            value={sourceUrl}
            onChange={(e) => setSourceUrl(e.target.value)}
            maxLength={500}
            placeholder="https://"
            className="h-11 rounded-xl"
            {...invalid("source_url")}
          />
          {errorText("source_url")}
        </Field>

        <Field data-invalid={!!error("note")}>
          <FieldLabel htmlFor="event-note">
            {t("note")} <span className="font-normal text-muted-foreground">({tc("optional")})</span>
          </FieldLabel>
          <Textarea
            id="event-note"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            maxLength={NOTE_MAX}
            rows={3}
            placeholder={t("notePlaceholder")}
            className="rounded-xl"
            {...invalid("note")}
          />
          {errorText("note") || (
            <FieldDescription className="num text-right text-xs">{t("noteCount", { count: note.length })}</FieldDescription>
          )}
        </Field>
      </div>

      <div className="mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <DialogClose render={<Button type="button" variant="ghost" size="pill" />}>{tc("cancel")}</DialogClose>
        <Button type="submit" variant="solid" size="pill" disabled={pending}>
          {pending ? tc("saving") : event ? tc("save") : t("submitNew")}
        </Button>
      </div>
    </form>
  );
}

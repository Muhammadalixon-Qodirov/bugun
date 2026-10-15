"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { ArrowLeft, Bot, Check, ImagePlus, Images, Loader2, Package, Search, X } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useMemo, useRef, useState } from "react";
import { type Control, Controller, useForm, useWatch } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";

import { useCatalogNames } from "@/components/catalog-names";
import { CategoryIcon, categoryIcons } from "@/components/category-icon";
import { useLimits } from "@/components/limits";
import { ease } from "@/components/motion";
import { Photo } from "@/components/photo";
import { Segmented } from "@/components/segmented";
import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  createListingMutation,
  deleteListingPhotoMutation,
  updateListingMutation,
  uploadListingPhotoMutation,
} from "@/lib/api/gen/@tanstack/react-query.gen";
import type { Category, MyListing, Product } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { shrinkPhoto } from "@/lib/image";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom, groupDigits } from "@/lib/labels";
import { cn } from "@/lib/utils";

// The largest original we try to shrink; what the API accepts comes from its limits.
const MAX_INPUT_BYTES = 40 * 1024 * 1024;

type T = ReturnType<typeof useTranslations<"listingForm">>;

function makeSchema(t: T) {
  const optionalAmount = z
    .string()
    .trim()
    .refine((v) => v === "" || (Number(v.replace(",", ".")) > 0 && Number(v.replace(",", ".")) <= 10_000_000), t("validation.amount"));

  return z.object({
    variety: z.string().trim().max(60, t("validation.varietyMax")),
    price: z
      .string()
      .trim()
      .min(1, t("validation.priceRequired"))
      .refine((v) => /^\d[\d\s]*$/.test(v), t("validation.priceDigits"))
      .refine((v) => {
        const n = Number(v.replace(/\s/g, ""));
        return n >= 50 && n <= 50_000_000;
      }, t("validation.priceRange")),
    trade: z.enum(["ulgurji", "chakana"]),
    minOrder: optionalAmount,
    stock: optionalAmount,
    description: z.string().trim().max(1000, t("validation.descriptionMax")),
  });
}
type Values = z.infer<ReturnType<typeof makeSchema>>;

function amount(v: string) {
  return v.trim() === "" ? undefined : Number(v.replace(",", "."));
}


export function ListingForm({
  products,
  categories,
  listing,
}: {
  products: Product[];
  categories: Category[];
  listing?: MyListing;
}) {
  const [product, setProduct] = useState<Product | null>(
    listing ? (products.find((p) => p.id === listing.product_id) ?? null) : null,
  );

  return (
    <>
      {!listing && <Steps current={product ? 1 : 0} />}
      <AnimatePresence mode="wait" initial={false}>
      {product ? (
        <m.div key="form" initial={{ opacity: 0, x: 16 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -16 }} transition={{ duration: 0.3, ease }}>
          <DetailsForm product={product} listing={listing} onBack={listing ? undefined : () => setProduct(null)} />
        </m.div>
      ) : (
        <m.div key="pick" initial={{ opacity: 0, x: -16 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: 16 }} transition={{ duration: 0.3, ease }}>
          <ProductPicker products={products} categories={categories} onPick={setProduct} />
        </m.div>
      )}
      </AnimatePresence>
    </>
  );
}

function Steps({ current }: { current: number }) {
  const t = useTranslations("listingForm");
  return (
    <ol className="mb-8 flex items-center gap-3 text-sm">
      {[t("steps.product"), t("steps.details")].map((label, i) => (
        <li key={label} className="flex items-center gap-3">
          {i > 0 && <span className="h-px w-8 bg-border sm:w-12" aria-hidden />}
          <span className={cn("flex items-center gap-2", i === current ? "font-medium" : "text-muted-foreground")}>
            <span
              className={cn(
                "grid size-6 place-items-center rounded-full text-xs",
                i <= current ? "bg-foreground text-background" : "border",
              )}
            >
              {i < current ? <Check className="size-3.5" /> : i + 1}
            </span>
            {label}
          </span>
        </li>
      ))}
    </ol>
  );
}

function norm(s: string) {
  return s.toLowerCase().replace(/[‘’`ʻʼ']/g, "");
}

function ProductPicker({ products, categories, onPick }: { products: Product[]; categories: Category[]; onPick: (p: Product) => void }) {
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<string | null>(null);
  const t = useTranslations("listingForm");
  const names = useCatalogNames();
  const fmt = useFormat();
  // Match the shown (translated) name and the Uzbek one, whichever the seller types.
  const matches = (p: Product) => {
    const q = norm(query);
    return norm(names.product(p.slug, p.name)).includes(q) || norm(p.name).includes(q);
  };
  const visible = products.filter((p) => (!category || p.category === category) && (!query || matches(p)));

  return (
    <div className="space-y-4">
      <div>
        <h2 className="display text-2xl">{t("picker.title")}</h2>
        <p className="mt-1.5 text-sm text-muted-foreground">{t("picker.hint")}</p>
      </div>
      <div className="relative">
        <Search className="pointer-events-none absolute top-1/2 left-4 size-4 -translate-y-1/2 text-muted-foreground" />
        <input
          autoFocus
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("picker.search")}
          className="h-12 w-full rounded-2xl border bg-card pl-11 text-base outline-none focus:border-foreground/40 focus:ring-4 focus:ring-foreground/5"
        />
      </div>
      <div className="-mx-4 flex gap-1.5 overflow-x-auto px-4 [scrollbar-width:none] sm:mx-0 sm:px-0">
        {[{ slug: null, name: t("picker.all") } as { slug: string | null; name: string }, ...categories].map((c) => (
          <button
            key={c.slug ?? "all"}
            type="button"
            onClick={() => setCategory(c.slug)}
            className={cn(
              "shrink-0 rounded-full border px-3.5 py-1.5 text-sm transition-colors",
              category === c.slug ? "border-foreground bg-foreground text-background" : "bg-card text-muted-foreground hover:text-foreground",
            )}
          >
            {c.slug ? names.category(c.slug, c.name) : c.name}
          </button>
        ))}
      </div>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
        {visible.map((p) => (
          <m.button
            key={p.id}
            type="button"
            whileTap={{ scale: 0.97 }}
            onClick={() => onPick(p)}
            className="flex items-center gap-3 rounded-2xl border bg-card p-3 text-left transition-colors hover:border-foreground/30"
          >
            <CategoryIcon category={p.category} />
            <span className="min-w-0">
              <span className="block truncate font-medium">{names.product(p.slug, p.name)}</span>
              <span className="text-xs text-muted-foreground">1 {fmt.unit(p.base_unit)}</span>
            </span>
          </m.button>
        ))}
      </div>
      {visible.length === 0 && (
        <p className="rounded-2xl border border-dashed p-6 text-center text-sm text-muted-foreground">
          {t("picker.empty")}
        </p>
      )}
    </div>
  );
}

type Pending = { id: string; file: File; url: string };

function DetailsForm({ product, listing, onBack }: { product: Product; listing?: MyListing; onBack?: () => void }) {
  const router = useRouter();
  const t = useTranslations("listingForm");
  const tc = useTranslations("common");
  const errors = useApiErrors();
  const names = useCatalogNames();
  const fmt = useFormat();
  const limits = useLimits();
  const maxPhotos = limits.photos.max_per_listing;
  const unit = fmt.unit(product.base_unit);
  const productName = names.product(product.slug, product.name);
  const schema = useMemo(() => makeSchema(t), [t]);
  const fileInput = useRef<HTMLInputElement>(null);
  const [pending, setPending] = useState<Pending[]>([]);
  const [saved, setSaved] = useState(listing?.photos ?? []);
  // What the submit button says while saving: null when idle.
  const [busy, setBusy] = useState<string | null>(null);

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      variety: listing?.variety ?? "",
      price: listing ? groupDigits(String(listing.price_som)) : "",
      trade: listing?.trade_type ?? "ulgurji",
      minOrder: listing?.min_order != null ? String(listing.min_order) : "",
      stock: listing?.stock != null ? String(listing.stock) : "",
      description: listing?.description ?? "",
    },
  });

  const create = useMutation(createListingMutation());
  const update = useMutation(updateListingMutation());
  const upload = useMutation(uploadListingPhotoMutation());
  const removePhoto = useMutation(deleteListingPhotoMutation());

  const photoCount = saved.length + pending.length;
  const addFiles = async (files: FileList | null) => {
    if (!files) return;
    const next: Pending[] = [];
    for (const original of Array.from(files)) {
      if (photoCount + next.length >= maxPhotos) {
        toast.error(t("photoErrors.tooMany", { max: maxPhotos }));
        break;
      }
      if (!/^image\/(jpeg|png|webp)$/.test(original.type)) {
        toast.error(t("photoErrors.badType", { name: original.name }));
        continue;
      }
      // Big phone photos are shrunk here first, so only the result has to fit the limit.
      const f = original.size > MAX_INPUT_BYTES ? original : await shrinkPhoto(original);
      if (f.size > limits.photos.max_bytes) {
        toast.error(t("photoErrors.tooBig", { name: original.name }));
        continue;
      }
      next.push({ id: `${f.name}-${f.size}-${Math.random()}`, file: f, url: URL.createObjectURL(f) });
    }
    setPending((p) => [...p, ...next]);
  };

  const dropSaved = async (photoId: number) => {
    if (!listing) return;
    try {
      await removePhoto.mutateAsync({ path: { id: listing.id, photo_id: photoId } });
      setSaved((s) => s.filter((p) => p.id !== photoId));
    } catch (err) {
      errors.show(err, "deletePhoto");
    }
  };

  const onSubmit = form.handleSubmit(async (v) => {
    const body = {
      variety: v.variety || undefined,
      description: v.description || undefined,
      price_som: Number(v.price.replace(/\s/g, "")),
      trade_type: v.trade,
      min_order: amount(v.minOrder),
      stock: amount(v.stock),
    };
    setBusy(tc("saving"));
    try {
      const res = listing
        ? await update.mutateAsync({ path: { id: listing.id }, body })
        : await create.mutateAsync({ body: { product_id: product.id, ...body } });
      const id = res.listing.id;
      let failed = 0;
      for (const [i, p] of pending.entries()) {
        setBusy(pending.length > 1 ? t("busy.uploadingMany", { current: i + 1, total: pending.length }) : t("busy.uploadingOne"));
        try {
          await upload.mutateAsync({ path: { id }, body: { file: p.file } });
        } catch {
          failed++;
        }
      }
      if (failed) toast.error(t("toasts.photosFailed", { count: failed }));
      if (res.listing.status === "review") {
        toast.warning(t("toasts.review"));
      } else if (!listing && !res.created) {
        toast.success(t("toasts.updatedExisting"));
      } else {
        toast.success(listing ? t("toasts.saved") : t("toasts.posted"));
      }
      router.push("/mahsulotlarim");
      router.refresh();
    } catch (err) {
      const map: Record<string, keyof Values> = { price_som: "price", min_order: "minOrder", stock: "stock", variety: "variety", description: "description" };
      for (const { field, message } of errors.fields(err)) {
        if (map[field]) form.setError(map[field], { message });
      }
      errors.show(err, "save");
    } finally {
      setBusy(null);
    }
  });

  const submit = (className: string) => (
    <Button
      type="submit"
      variant="solid"
      size="pill-lg"
      disabled={busy != null}
      className={cn("w-full", className)}
    >
      {busy && <Loader2 className="animate-spin" />}
      {busy ?? (listing ? t("submit.save") : t("submit.post"))}
    </Button>
  );

  return (
    <form onSubmit={onSubmit} noValidate className="grid gap-8 lg:grid-cols-[1fr_19rem] lg:gap-12">
      <div className="min-w-0 space-y-4">
        <div className="flex items-center gap-3 rounded-3xl border bg-card p-4">
          <CategoryIcon category={product.category} className="size-12 rounded-2xl" />
          <div className="min-w-0 flex-1">
            <p className="text-xs text-muted-foreground">{t("product")}</p>
            <p className="truncate text-lg font-semibold tracking-tight">{productName}</p>
          </div>
          {onBack && (
            <Button type="button" variant="outline" size="sm" className="h-9 rounded-full px-4" onClick={onBack}>
              <ArrowLeft /> {t("change")}
            </Button>
          )}
        </div>

        <Block title={t("photos.title")} hint={t("photos.hint", { count: photoCount, max: maxPhotos })}>
          <div className="grid grid-cols-3 gap-2 sm:grid-cols-5">
            <AnimatePresence initial={false}>
              {saved.map((p, i) => (
                <m.div key={p.id} layout initial={{ opacity: 0, scale: 0.9 }} animate={{ opacity: 1, scale: 1 }} exit={{ opacity: 0, scale: 0.9 }} className="relative aspect-square overflow-hidden rounded-2xl border">
                  <Photo thumb k={p.key} alt={productName} sizes="(min-width: 640px) 8rem, 33vw" className="size-full" />
                  {i === 0 && <CoverBadge />}
                  <RemoveButton onClick={() => dropSaved(p.id)} />
                </m.div>
              ))}
              {pending.map((p, i) => (
                <m.div key={p.id} layout initial={{ opacity: 0, scale: 0.9 }} animate={{ opacity: 1, scale: 1 }} exit={{ opacity: 0, scale: 0.9 }} className="relative aspect-square overflow-hidden rounded-2xl border">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img src={p.url} alt="" className="size-full object-cover" />
                  {saved.length === 0 && i === 0 && <CoverBadge />}
                  <RemoveButton
                    onClick={() => {
                      URL.revokeObjectURL(p.url);
                      setPending((list) => list.filter((x) => x.id !== p.id));
                    }}
                  />
                </m.div>
              ))}
            </AnimatePresence>
            {photoCount < maxPhotos && (
              <button
                type="button"
                onClick={() => fileInput.current?.click()}
                className="flex aspect-square flex-col items-center justify-center gap-1.5 rounded-2xl border-2 border-dashed text-xs text-muted-foreground transition-colors hover:border-foreground/40 hover:text-foreground"
              >
                <ImagePlus strokeWidth={1.5} className="size-6" />
                {t("photos.add")}
              </button>
            )}
          </div>
          <input
            ref={fileInput}
            type="file"
            accept="image/jpeg,image/png,image/webp"
            multiple
            hidden
            onChange={(e) => {
              const input = e.target;
              void addFiles(input.files).finally(() => {
                input.value = "";
              });
            }}
          />
          <p className="mt-3 text-xs text-muted-foreground">
            {t("photos.note")}
          </p>
        </Block>

        <Block title={t("price.title")} hint={t("price.hint")}>
          <FieldGroup>
            <Controller
              name="trade"
              control={form.control}
              render={({ field }) => (
                <Field>
                  <FieldLabel>{t("price.trade")}</FieldLabel>
                  <Segmented
                    label={t("price.trade")}
                    value={field.value}
                    onChange={field.onChange}
                    options={[
                      { value: "ulgurji", label: tc("wholesale") },
                      { value: "chakana", label: tc("retail") },
                    ]}
                  />
                </Field>
              )}
            />
            <Controller
              name="price"
              control={form.control}
              render={({ field, fieldState }) => (
                <Field data-invalid={fieldState.invalid}>
                  <FieldLabel htmlFor="price">{t("price.label", { unit })}</FieldLabel>
                  <div className="relative">
                    <Input
                      id="price"
                      inputMode="numeric"
                      placeholder="4 500"
                      className="num h-14 rounded-2xl pr-24 text-2xl font-semibold md:text-2xl"
                      aria-invalid={fieldState.invalid}
                      {...field}
                      onChange={(e) => field.onChange(groupDigits(e.target.value))}
                    />
                    <span className="pointer-events-none absolute top-1/2 right-4 -translate-y-1/2 text-sm text-muted-foreground">
                      {t("price.suffix", { unit })}
                    </span>
                  </div>
                  {fieldState.error && <FieldError errors={[fieldState.error]} />}
                  <FieldDescription className="flex items-start gap-1.5">
                    <Bot className="mt-0.5 size-3.5 shrink-0" aria-hidden />
                    {!listing
                      ? t("autoNote.new")
                      : listing.pricing.mode === "auto"
                        ? t("autoNote.on", { hours: Math.round(limits.auto_price.manual_pause_hours) })
                        : t("autoNote.off")}
                  </FieldDescription>
                </Field>
              )}
            />
          </FieldGroup>
        </Block>

        <Block title={t("extra.title")} hint={t("extra.hint")}>
          <FieldGroup>
            <Controller
              name="variety"
              control={form.control}
              render={({ field, fieldState }) => (
                <Field data-invalid={fieldState.invalid}>
                  <FieldLabel htmlFor="variety">{t("extra.variety")}</FieldLabel>
                  <Input id="variety" placeholder={t("extra.varietyPlaceholder")} className="h-11 rounded-xl" aria-invalid={fieldState.invalid} {...field} />
                  {fieldState.error && <FieldError errors={[fieldState.error]} />}
                </Field>
              )}
            />
            <div className="grid gap-4 sm:grid-cols-2">
              <Controller
                name="minOrder"
                control={form.control}
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid}>
                    <FieldLabel htmlFor="minOrder">{t("extra.minOrder", { unit })}</FieldLabel>
                    <Input id="minOrder" inputMode="decimal" placeholder="100" className="h-11 rounded-xl" aria-invalid={fieldState.invalid} {...field} />
                    {fieldState.error && <FieldError errors={[fieldState.error]} />}
                  </Field>
                )}
              />
              <Controller
                name="stock"
                control={form.control}
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid}>
                    <FieldLabel htmlFor="stock">{t("extra.stock", { unit })}</FieldLabel>
                    <Input id="stock" inputMode="decimal" placeholder="2000" className="h-11 rounded-xl" aria-invalid={fieldState.invalid} {...field} />
                    {fieldState.error && <FieldError errors={[fieldState.error]} />}
                  </Field>
                )}
              />
            </div>
            <Controller
              name="description"
              control={form.control}
              render={({ field, fieldState }) => (
                <Field data-invalid={fieldState.invalid}>
                  <FieldLabel htmlFor="description">{t("extra.description")}</FieldLabel>
                  <Textarea id="description" rows={3} placeholder={t("extra.descriptionPlaceholder")} className="rounded-xl" {...field} />
                  <FieldDescription>{t("extra.descriptionNote")}</FieldDescription>
                  {fieldState.error && <FieldError errors={[fieldState.error]} />}
                </Field>
              )}
            />
          </FieldGroup>
        </Block>

        {submit("lg:hidden")}
      </div>

      <aside className="hidden lg:block">
        <div className="sticky top-24 space-y-5">
          <p className="eyebrow">{t("preview.heading")}</p>
          <Preview
            control={form.control}
            product={product}
            productName={productName}
            unit={unit}
            cover={pending[0]?.url}
            coverKey={saved[0]?.key}
            photos={photoCount}
          />
          {submit("")}
          <p className="text-center text-xs text-muted-foreground">
            {t("preview.reviewNote")}
          </p>
        </div>
      </aside>
    </form>
  );
}

function Block({ title, hint, children }: { title: string; hint: string; children: React.ReactNode }) {
  return (
    <section className="rounded-3xl border bg-card p-5 sm:p-6">
      <h2 className="font-semibold tracking-tight">{title}</h2>
      <p className="mt-0.5 mb-5 text-sm text-muted-foreground">{hint}</p>
      {children}
    </section>
  );
}

function CoverBadge() {
  const t = useTranslations("listingForm");
  return <span className="absolute bottom-1.5 left-1.5 rounded-full bg-black/55 px-2 py-0.5 text-[10px] font-medium text-white backdrop-blur">{t("photos.cover")}</span>;
}

/** Live listing card built from the form values, as buyers will see it in the market. */
function Preview({
  control,
  product,
  productName,
  unit,
  cover,
  coverKey,
  photos,
}: {
  control: Control<Values>;
  product: Product;
  productName: string;
  unit: string;
  cover?: string;
  coverKey?: string;
  photos: number;
}) {
  const [price, variety, trade, minOrder] = useWatch({ control, name: ["price", "variety", "trade", "minOrder"] });
  const n = Number(price.replace(/\s/g, ""));
  const Icon = categoryIcons[product.category] ?? Package;
  const t = useTranslations("listingForm");
  const tc = useTranslations("common");
  return (
    <div className="pointer-events-none select-none" aria-hidden>
      <div className="relative aspect-square overflow-hidden rounded-2xl bg-muted">
        {cover ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={cover} alt="" className="size-full object-cover" />
        ) : coverKey ? (
          <Photo thumb k={coverKey} alt="" className="size-full" />
        ) : (
          <div className="flex size-full flex-col items-center justify-center gap-3 bg-[radial-gradient(circle_at_50%_40%,var(--card),var(--muted)_70%)]">
            <Icon strokeWidth={1.25} className="size-12 text-muted-foreground/50" />
            <span className="text-sm font-medium text-muted-foreground">{productName}</span>
          </div>
        )}
        {trade === "ulgurji" && (
          <span className="absolute top-2.5 left-2.5 rounded-full bg-background/85 px-2 py-0.5 text-xs font-medium backdrop-blur-md">{tc("wholesale")}</span>
        )}
        {photos > 1 && (
          <span className="absolute right-2.5 bottom-2.5 flex items-center gap-1 rounded-full bg-black/50 px-2 py-0.5 text-[11px] font-medium text-white">
            <Images className="size-3" /> {photos}
          </span>
        )}
      </div>
      <div className="px-0.5 pt-3">
        <p className="num text-lg leading-tight font-semibold">
          {n > 0 ? formatSom(n) : "—"}
          <span className="ml-1 font-sans text-[13px] font-normal text-muted-foreground">{tc("perUnit", { unit })}</span>
        </p>
        <p className="mt-1 truncate text-[15px]">
          {productName}
          {variety.trim() && <span className="text-muted-foreground"> · {variety.trim()}</span>}
        </p>
        <p className="mt-3 text-xs text-muted-foreground">
          {t("preview.meta")}
          {minOrder.trim() && ` · ${t("preview.minOrder", { amount: minOrder.trim(), unit })}`}
        </p>
      </div>
    </div>
  );
}

function RemoveButton({ onClick }: { onClick: () => void }) {
  const t = useTranslations("listingForm");
  return (
    <button type="button" aria-label={t("photos.remove")} onClick={onClick} className="absolute top-1.5 right-1.5 grid size-7 place-items-center rounded-full bg-background/90 shadow backdrop-blur">
      <X className="size-4" />
    </button>
  );
}

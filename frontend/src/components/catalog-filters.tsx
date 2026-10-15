"use client";

import { ArrowDownUp, ChevronDown, Search, SlidersHorizontal, X } from "lucide-react";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { useMarketNav, useMarketParams } from "@/components/market-nav";
import { Segmented } from "@/components/segmented";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { groupDigits } from "@/lib/labels";
import { cn } from "@/lib/utils";

const sorts = ["new", "deal", "price_asc", "price_desc", "popular"] as const;

function useFilterParams(basePath: string) {
  const params = useMarketParams();
  const { go } = useMarketNav();
  const set = (patch: Record<string, string | null>) => {
    const next = new URLSearchParams(params);
    for (const [k, v] of Object.entries(patch)) {
      if (v) next.set(k, v);
      else next.delete(k);
    }
    const qs = next.toString();
    go(qs ? `${basePath}?${qs}` : basePath);
  };
  return { params, set };
}

/** Heading, result count, sort menu, active-filter chips and (on phones) the filter sheet. */
export function CatalogToolbar({
  basePath,
  total,
  regionName,
  products,
  categories,
}: {
  basePath: string;
  total: number;
  regionName: string;
  products: ProductOption[];
  categories: { slug: string; name: string }[];
}) {
  const t = useTranslations("market");
  const tc = useTranslations("common");
  const { params, set } = useFilterParams(basePath);
  const { pending } = useMarketNav();
  // Worked out from the (optimistic) query so the title changes with the tab.
  const heading = params.get("q")
    ? t("searchTitle", { q: params.get("q") ?? "" })
    : (products.find((p) => p.slug === params.get("product"))?.name ??
      categories.find((c) => c.slug === params.get("category"))?.name ??
      (params.get("sort") === "deal" ? t("dealsTitle") : t("allListings")));
  const sort = sorts.find((s) => s === params.get("sort")) ?? "new";
  const trade = params.get("trade");
  const min = params.get("min_price");
  const max = params.get("max_price");
  const q = params.get("q");
  const product = params.get("product");
  const som = (v: string) => Number(v).toLocaleString("ru-RU").replace(/\u00a0/g, " ");
  type Chip = { label: string; clear: Record<string, null> };
  const chips: Chip[] = [];
  if (q) chips.push({ label: `«${q}»`, clear: { q: null } });
  if (product) chips.push({ label: products.find((p) => p.slug === product)?.name ?? product, clear: { product: null } });
  if (trade) chips.push({ label: trade === "ulgurji" ? tc("wholesale") : tc("retail"), clear: { trade: null } });
  if (min || max) {
    chips.push({
      label: min && max ? t("priceRange", { min: som(min), max: som(max) }) : min ? t("priceMin", { min: som(min) }) : t("priceMax", { max: som(max ?? "") }),
      clear: { min_price: null, max_price: null },
    });
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end gap-x-4 gap-y-3">
        <div className="mr-auto min-w-0">
          <h1 className="display line-clamp-2 text-2xl break-words sm:text-[2rem]">{heading}</h1>
          <p className={cn("mt-1.5 text-sm text-muted-foreground transition-opacity duration-200", pending && "opacity-40")}>
            {t("count", { count: total, formatted: som(String(total)), region: regionName })}
          </p>
        </div>
        <div className="flex w-full items-center gap-2 sm:w-auto">
          <Sheet>
            <SheetTrigger className="relative flex h-10 flex-1 items-center justify-center gap-2 rounded-full border bg-card px-4 text-sm font-medium transition-colors hover:bg-muted sm:flex-none lg:hidden">
              <SlidersHorizontal className="size-4" /> {t("filters")}
              {chips.length > 0 && (
                <span className="grid size-5 place-items-center rounded-full bg-foreground text-[11px] text-background">{chips.length}</span>
              )}
            </SheetTrigger>
            <SheetContent side="bottom" className="max-h-[calc(var(--app-height)*0.85)] rounded-t-3xl pb-[var(--app-safe-bottom)]">
              <SheetHeader>
                <SheetTitle>{t("filters")}</SheetTitle>
              </SheetHeader>
              <div className="overflow-y-auto px-4 pb-6">
                <FilterPanel basePath={basePath} products={products} inSheet />
              </div>
            </SheetContent>
          </Sheet>
          <DropdownMenu>
            <DropdownMenuTrigger className="flex h-10 flex-1 items-center justify-center gap-2 rounded-full border bg-card px-4 text-sm transition-colors hover:bg-muted aria-expanded:bg-muted sm:flex-none">
              <ArrowDownUp className="size-4 text-muted-foreground" />
              <span className="sr-only text-muted-foreground sm:not-sr-only">{t("sortBy")}</span>
              <span className="font-medium">{t(`sort.${sort}`)}</span>
              <ChevronDown className="size-4 text-muted-foreground" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-52">
              <DropdownMenuRadioGroup value={sort} onValueChange={(v: string) => set({ sort: v === "new" ? null : v })}>
                {sorts.map((s) => (
                  <DropdownMenuRadioItem key={s} value={s}>
                    {t(`sort.${s}`)}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
      {chips.length > 0 && (
        <div className="flex flex-wrap items-center gap-2">
          {chips.map((c) => (
            <button
              key={c.label}
              type="button"
              onClick={() => set(c.clear)}
              title={c.label}
              className="flex h-8 max-w-full items-center gap-1.5 rounded-full border bg-card pr-2.5 pl-3.5 text-sm transition-colors hover:border-foreground/30"
            >
              <span className="max-w-[16rem] truncate">{c.label}</span>
              <X className="size-3.5 shrink-0 text-muted-foreground" />
            </button>
          ))}
          <button
            type="button"
            onClick={() => set({ q: null, product: null, trade: null, min_price: null, max_price: null })}
            className="px-2 text-sm font-medium underline-offset-4 hover:underline"
          >
            {tc("clearAll")}
          </button>
        </div>
      )}
    </div>
  );
}

/** A product with its name in the visitor's language; `uzName` keeps search working in Uzbek too. */
type ProductOption = { slug: string; name: string; uzName?: string; category: string };

/** Keeps a text field in step with the URL: typing edits it, a URL change resets it. */
function useUrlField(value: string) {
  const [text, setText] = useState(value);
  const [seen, setSeen] = useState(value);
  if (value !== seen) {
    setSeen(value);
    setText(value);
  }
  return [text, setText] as const;
}

/** Trade type, product and price range; a card beside the grid on desktop, a bottom sheet on phones. */
export function FilterPanel({ basePath, products, inSheet }: { basePath: string; products: ProductOption[]; inSheet?: boolean }) {
  const t = useTranslations("market");
  const tc = useTranslations("common");
  const { params, set } = useFilterParams(basePath);
  const trade = params.get("trade") ?? "";
  const category = params.get("category");
  const product = params.get("product");
  const [min, setMin] = useUrlField(groupDigits(params.get("min_price") ?? ""));
  const [max, setMax] = useUrlField(groupDigits(params.get("max_price") ?? ""));
  const choices = category ? products.filter((p) => p.category === category) : [];
  const count = [trade, product, params.get("min_price") || params.get("max_price")].filter(Boolean).length;

  return (
    <div className={cn("text-sm", !inSheet && "rounded-3xl border bg-card p-5")}>
      <div className={cn("flex items-center justify-between", inSheet && count === 0 && "hidden")}>
        <p className={cn("flex items-center gap-2 font-semibold", inSheet && "invisible")}>
          {t("filters")}
          {count > 0 && <span className="num grid size-5 place-items-center rounded-full bg-foreground text-[11px] text-background">{count}</span>}
        </p>
        {count > 0 && (
          <button
            type="button"
            onClick={() => set({ trade: null, product: null, min_price: null, max_price: null })}
            className="text-muted-foreground underline-offset-4 transition-colors hover:text-foreground hover:underline"
          >
            {tc("clear")}
          </button>
        )}
      </div>

      <section className="mt-5">
        <p className="eyebrow mb-2.5">{t("tradeType")}</p>
        <Segmented
          label={t("tradeType")}
          size="sm"
          value={trade}
          onChange={(v) => set({ trade: v || null })}
          options={[
            { value: "", label: tc("all") },
            { value: "ulgurji", label: tc("wholesale") },
            { value: "chakana", label: tc("retail") },
          ]}
        />
      </section>

      {choices.length > 0 && <ProductChips choices={choices} product={product} onPick={(slug) => set({ product: slug })} />}

      <section className="mt-6">
        <p className="eyebrow mb-2.5">{t("price")}</p>
        <PriceRange
          min={min}
          max={max}
          onMin={setMin}
          onMax={setMax}
          current={[params.get("min_price") ?? "", params.get("max_price") ?? ""]}
          onApply={(lo, hi) => set({ min_price: lo || null, max_price: hi || null })}
        />
      </section>
    </div>
  );
}

/** Products of the chosen category as chips; a filter box appears once there are many. */
function ProductChips({ choices, product, onPick }: { choices: ProductOption[]; product: string | null; onPick: (slug: string | null) => void }) {
  const t = useTranslations("market");
  const tc = useTranslations("common");
  const [query, setQuery] = useState("");
  const norm = (v: string) => v.toLowerCase().replace(/[‘’`ʻʼ']/g, "");
  const q = norm(query.trim());
  const shown = choices
    .filter((p) => p.slug === product || !q || [p.name, p.uzName ?? ""].some((n) => norm(n).includes(q)))
    // The chosen product stays first, so it never scrolls out of sight.
    .sort((a, b) => Number(b.slug === product) - Number(a.slug === product));
  return (
    <section className="mt-6">
      <p className="eyebrow mb-2.5">{t("product")}</p>
      {choices.length > 10 && (
        <label className="mb-2.5 flex h-9 items-center gap-2 rounded-full bg-muted px-3">
          <Search className="size-3.5 shrink-0 text-muted-foreground" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("findProduct")}
            aria-label={t("findProduct")}
            className="w-full min-w-0 bg-transparent text-[13px] outline-none placeholder:text-muted-foreground"
          />
        </label>
      )}
      <div className="flex max-h-56 flex-wrap gap-1.5 overflow-y-auto" data-lenis-prevent>
        {shown.map((p) => {
          const on = product === p.slug;
          return (
            <button
              key={p.slug}
              type="button"
              aria-pressed={on}
              title={p.name}
              onClick={() => onPick(on ? null : p.slug)}
              className={cn(
                "max-w-full truncate rounded-full border px-3 py-1.5 transition-colors duration-200",
                on ? "border-foreground bg-foreground text-background" : "text-muted-foreground hover:border-foreground/30 hover:text-foreground",
              )}
            >
              {p.name}
            </button>
          );
        })}
        {shown.length === 0 && <p className="py-1 text-[13px] text-muted-foreground">{tc("notFound")}</p>}
      </div>
    </section>
  );
}

const digits = (v: string) => v.replace(/\D/g, "");

/**
 * "dan – gacha" price fields in a narrow column. The field being typed in
 * widens to about three quarters of the row and the other one shrinks; on
 * blur both return to half and show their number from its first digit.
 */
function PriceRange({
  min,
  max,
  onMin,
  onMax,
  current,
  onApply,
}: {
  min: string;
  max: string;
  onMin: (v: string) => void;
  onMax: (v: string) => void;
  current: [string, string];
  onApply: (min: string, max: string) => void;
}) {
  const t = useTranslations("market");
  const [focus, setFocus] = useState<"min" | "max" | null>(null);
  const lo = digits(min);
  const hi = digits(max);
  const changed = lo !== current[0] || hi !== current[1];
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        // "dan" above "gacha" is a slip, not a request for nothing: swap them.
        const swap = lo && hi && Number(lo) > Number(hi);
        if (swap) {
          onMin(groupDigits(hi));
          onMax(groupDigits(lo));
        }
        onApply(swap ? hi : lo, swap ? lo : hi);
      }}
    >
      <div className="flex gap-2">
        <PriceInput label={t("from")} ariaLabel={t("priceFrom")} value={min} onChange={onMin} focus={focus} self="min" onFocusChange={setFocus} />
        <PriceInput label={t("to")} ariaLabel={t("priceTo")} value={max} onChange={onMax} focus={focus} self="max" onFocusChange={setFocus} />
      </div>
      <Button type="submit" variant="solid" size="pill" className="mt-3 w-full" disabled={!changed}>
        {t("show")}
      </Button>
    </form>
  );
}

function PriceInput({
  label,
  ariaLabel,
  value,
  onChange,
  focus,
  self,
  onFocusChange,
}: {
  label: string;
  ariaLabel: string;
  value: string;
  onChange: (v: string) => void;
  focus: "min" | "max" | null;
  self: "min" | "max";
  onFocusChange: (f: "min" | "max" | null) => void;
}) {
  const tc = useTranslations("common");
  const wide = focus === self;
  const narrow = focus != null && !wide;
  return (
    <label
      // flex-grow animates, so the two fields trade width smoothly (about 78% / 22%).
      style={{ flexGrow: wide ? 3.5 : 1 }}
      className={cn(
        "flex h-11 min-w-0 basis-0 items-center gap-1.5 rounded-xl border bg-background transition-[flex-grow,border-color,padding] duration-300 ease-out-soft focus-within:border-foreground/40",
        narrow ? "px-2" : "px-3",
      )}
    >
      <input
        value={value}
        onChange={(e) => onChange(groupDigits(e.target.value))}
        onFocus={() => onFocusChange(self)}
        onBlur={(e) => {
          // Typing leaves the field scrolled to its end; show the number from its first digit.
          e.currentTarget.scrollLeft = 0;
          onFocusChange(null);
        }}
        inputMode="numeric"
        enterKeyHint="search"
        placeholder={label}
        aria-label={ariaLabel}
        className="num w-full min-w-0 bg-transparent text-ellipsis outline-none placeholder:text-muted-foreground/70"
      />
      {wide && value && (
        <button
          type="button"
          aria-label={tc("clear")}
          // Keep focus in the field while clearing it.
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => onChange("")}
          className="grid size-5 shrink-0 place-items-center rounded-full bg-muted text-muted-foreground transition-colors hover:text-foreground"
        >
          <X className="size-3" />
        </button>
      )}
    </label>
  );
}

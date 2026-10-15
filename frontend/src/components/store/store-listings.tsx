"use client";

import { ArrowDownUp, ChevronDown, LayoutGrid, Package, Search, SearchX, X } from "lucide-react";
import { useTranslations } from "next-intl";
import { useMemo, useState } from "react";

import { useCatalogNames } from "@/components/catalog-names";
import { categoryIcons } from "@/components/category-icon";
import { EmptyState } from "@/components/empty-state";
import { ListingCard } from "@/components/listing-card";
import { Segmented } from "@/components/segmented";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { ListingCard as Card } from "@/lib/api/gen/types.gen";
import { cn } from "@/lib/utils";

const sorts = ["new", "price_asc", "price_desc"] as const;
type Sort = (typeof sorts)[number];
type Trade = "all" | "ulgurji" | "chakana";

const norm = (v: string) => v.toLowerCase().replace(/[‘’`ʻʼ']/g, "");

/** The seller's live listings with category, trade, sort and search filters (all in the browser). */
export function StoreListings({ listings, categories }: { listings: Card[]; categories: { slug: string; name: string }[] }) {
  const t = useTranslations("store");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const [category, setCategory] = useState("all");
  const [trade, setTrade] = useState<Trade>("all");
  const [sort, setSort] = useState<Sort>("new");
  const [query, setQuery] = useState("");

  const bothTrades = listings.some((l) => l.trade_type === "ulgurji") && listings.some((l) => l.trade_type === "chakana");
  const shown = useMemo(() => {
    const q = norm(query.trim());
    const out = listings.filter(
      (l) =>
        (category === "all" || l.category === category) &&
        (trade === "all" || l.trade_type === trade) &&
        (!q || [names.product(l.product_slug, l.product_name), l.product_name, l.variety ?? ""].some((n) => norm(n).includes(q))),
    );
    // The API sends newest first; price sorts keep that order among equal prices.
    if (sort !== "new") out.sort((a, b) => (sort === "price_asc" ? a.price_som - b.price_som : b.price_som - a.price_som));
    return out;
  }, [listings, category, trade, sort, query, names]);
  const filtered = category !== "all" || trade !== "all" || query.trim() !== "";
  const reset = () => {
    setCategory("all");
    setTrade("all");
    setQuery("");
  };

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end gap-x-4 gap-y-3">
        <div className="mr-auto min-w-0">
          <h2 className="display text-2xl sm:text-[1.75rem]">{t("listingsTitle")}</h2>
          <p className="mt-1 text-sm text-muted-foreground" aria-live="polite">
            {t("shown", { count: shown.length })}
          </p>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger className="flex h-10 items-center justify-center gap-2 rounded-full border bg-card px-4 text-sm transition-colors hover:bg-muted aria-expanded:bg-muted">
            <ArrowDownUp className="size-4 text-muted-foreground" />
            <span className="sr-only text-muted-foreground sm:not-sr-only">{t("sortBy")}</span>
            <span className="font-medium">{t(`sort.${sort}`)}</span>
            <ChevronDown className="size-4 text-muted-foreground" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-52">
            <DropdownMenuRadioGroup value={sort} onValueChange={(v: string) => setSort(sorts.find((s) => s === v) ?? "new")}>
              {sorts.map((s) => (
                <DropdownMenuRadioItem key={s} value={s}>
                  {t(`sort.${s}`)}
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {categories.length > 1 && (
        <div role="group" aria-label={t("categories")} className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1 scrollbar-none sm:mx-0 sm:flex-wrap sm:px-0">
          {[{ slug: "all", name: tc("all") }, ...categories].map((c) => {
            const on = category === c.slug;
            const Icon = c.slug === "all" ? LayoutGrid : (categoryIcons[c.slug] ?? Package);
            const count = c.slug === "all" ? listings.length : listings.filter((l) => l.category === c.slug).length;
            return (
              <button
                key={c.slug}
                type="button"
                aria-pressed={on}
                onClick={() => setCategory(c.slug)}
                className={cn(
                  "flex h-10 shrink-0 items-center gap-2 rounded-full border px-4 text-sm whitespace-nowrap transition-colors duration-200",
                  on ? "border-foreground bg-foreground text-background" : "bg-card text-muted-foreground hover:border-foreground/30 hover:text-foreground",
                )}
              >
                <Icon className="size-4" strokeWidth={1.75} />
                {c.name}
                <span className={cn("num text-xs", on ? "text-background/70" : "text-muted-foreground")}>{count}</span>
              </button>
            );
          })}
        </div>
      )}

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <label className="flex h-10 min-w-0 flex-1 items-center gap-2 rounded-full border bg-card px-4 transition-colors focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/50 sm:max-w-sm">
          <Search className="size-4 shrink-0 text-muted-foreground" />
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("search")}
            aria-label={t("search")}
            className="w-full min-w-0 bg-transparent text-base outline-none placeholder:text-muted-foreground md:text-sm [&::-webkit-search-cancel-button]:hidden"
          />
          {query && (
            <button
              type="button"
              onClick={() => setQuery("")}
              aria-label={t("clearSearch")}
              className="-mr-2 grid size-8 shrink-0 place-items-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            >
              <X className="size-4" />
            </button>
          )}
        </label>
        {bothTrades && (
          <Segmented<Trade>
            label={t("trade")}
            value={trade}
            onChange={setTrade}
            className="sm:w-80"
            options={[
              { value: "all", label: tc("all") },
              { value: "ulgurji", label: tc("wholesale") },
              { value: "chakana", label: tc("retail") },
            ]}
          />
        )}
      </div>

      {shown.length === 0 ? (
        <EmptyState
          icon={<SearchX strokeWidth={1.5} />}
          title={t("noMatchTitle")}
          description={t("noMatchText")}
          action={
            filtered && (
              <Button variant="outline" size="pill" onClick={reset}>
                {t("resetFilters")}
              </Button>
            )
          }
        />
      ) : (
        <div className="grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
          {shown.map((l) => (
            <ListingCard key={l.id} l={l} />
          ))}
        </div>
      )}
    </div>
  );
}

"use client";

import { useTranslations } from "next-intl";

import { categoryIcons } from "@/components/category-icon";
import { useCatalogNames } from "@/components/catalog-names";
import { Segmented } from "@/components/segmented";
import { useQueryNav } from "@/components/stats/query-nav";
import { CATEGORIES } from "@/lib/stats";
import { cn } from "@/lib/utils";

/** Wholesale / retail switch kept in the "trade" query param (wholesale is the default, so it is dropped). */
export function TradeToggle({ className }: { className?: string }) {
  const t = useTranslations("stats");
  const tc = useTranslations("common");
  const { params, set } = useQueryNav();
  return (
    <Segmented
      label={t("trade")}
      className={cn("w-full sm:w-64", className)}
      value={params.get("trade") === "chakana" ? "chakana" : "ulgurji"}
      onChange={(v) => set({ trade: v === "chakana" ? v : null })}
      options={[
        { value: "ulgurji", label: tc("wholesale") },
        { value: "chakana", label: tc("retail") },
      ]}
    />
  );
}

/** Category pills; "All" drops the param rather than sending an empty category. */
export function CategoryTabs({ categories }: { categories: { slug: string; name: string }[] }) {
  const t = useTranslations("stats");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const { params, set } = useQueryNav();
  const current = params.get("category") ?? "";
  const items = [{ slug: "", name: tc("all") }, ...categories.filter((c) => (CATEGORIES as readonly string[]).includes(c.slug))];
  return (
    <div role="radiogroup" aria-label={t("category")} className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1 [scrollbar-width:none] sm:mx-0 sm:flex-wrap sm:px-0">
      {items.map((c) => {
        const on = c.slug === current;
        const Icon = c.slug ? categoryIcons[c.slug] : null;
        return (
          <button
            key={c.slug || "all"}
            type="button"
            role="radio"
            aria-checked={on}
            onClick={() => set({ category: c.slug || null })}
            className={cn(
              "flex h-10 shrink-0 items-center gap-2 rounded-full border px-4 text-sm transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
              on ? "border-foreground bg-foreground font-medium text-background" : "bg-card hover:bg-muted",
            )}
          >
            {Icon && <Icon className="size-4" strokeWidth={1.75} />}
            {c.slug ? names.category(c.slug, c.name) : c.name}
          </button>
        );
      })}
    </div>
  );
}

"use client";

import { useMutation } from "@tanstack/react-query";
import { ArrowDownRight, ArrowUpRight, Package, RefreshCw, Search } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useMemo, useState } from "react";
import { toast } from "sonner";

import { categoryIcons } from "@/components/category-icon";
import { useCatalogNames } from "@/components/catalog-names";
import { ListingActions } from "@/components/listing-actions";
import { ease } from "@/components/motion";
import { Photo } from "@/components/photo";
import { AutoBadge, Funnel, PositionChip, StatusNote } from "@/components/seller/signals";
import { Button } from "@/components/ui/button";
import { refreshDueListingsMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { MyListing } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { type ListingState, needsRefresh, stateDots, stateOf } from "@/lib/listings";
import { cn } from "@/lib/utils";

const tabs: { key: "all" | "live" | "attention" | "review" | "hidden"; match: (s: ListingState) => boolean }[] = [
  { key: "all", match: () => true },
  { key: "live", match: (s) => s === "active" || s === "ending" },
  { key: "attention", match: (s) => s === "ending" || s === "expired" },
  { key: "review", match: (s) => s === "review" || s === "rejected" },
  { key: "hidden", match: (s) => s === "hidden" },
];

function norm(s: string) {
  return s.toLowerCase().replace(/[‘’`ʻʼ']/g, "");
}

/** The seller's listings: status tabs, a name filter, bulk renewal and one row per listing. */
export function MyListings({ listings }: { listings: MyListing[] }) {
  const router = useRouter();
  const t = useTranslations("mine");
  const errors = useApiErrors();
  const names = useCatalogNames();
  const [tab, setTab] = useState("all");
  const [query, setQuery] = useState("");
  const renew = useMutation({
    ...refreshDueListingsMutation(),
    onSuccess: ({ renewed }) => {
      toast.success(renewed > 0 ? t("renewedMany", { count: renewed }) : t("nothingToRenew"));
      router.refresh();
    },
    onError: (err) => errors.show(err, "renew"),
  });

  const rows = useMemo(() => listings.map((l) => ({ l, state: stateOf(l) })), [listings]);
  const counts = Object.fromEntries(tabs.map((tb) => [tb.key, rows.filter((r) => tb.match(r.state)).length]));
  const active = tabs.find((tb) => tb.key === tab) ?? tabs[0];
  const visible = rows.filter((r) => active.match(r.state) && (!query || norm(`${r.l.product_name} ${names.product(r.l.product_slug, r.l.product_name)} ${r.l.variety}`).includes(norm(query))));
  const stale = listings.filter(needsRefresh);

  return (
    <div>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="-mx-4 flex gap-1 overflow-x-auto px-4 scrollbar-none sm:mx-0 sm:px-0" role="tablist" aria-label={t("tabsLabel")}>
          {tabs.map((tb) => (
            <button
              key={tb.key}
              type="button"
              role="tab"
              aria-selected={tab === tb.key}
              onClick={() => setTab(tb.key)}
              className={cn(
                "relative isolate flex h-9 shrink-0 items-center gap-1.5 rounded-full px-3.5 text-sm transition-colors",
                tab === tb.key ? "font-medium text-background" : "text-muted-foreground hover:text-foreground",
              )}
            >
              {tab === tb.key && (
                <m.span layoutId="my-listings-tab" className="absolute inset-0 -z-10 rounded-full bg-foreground" transition={{ type: "spring", stiffness: 500, damping: 40 }} />
              )}
              {t(`tabs.${tb.key}`)}
              <span className={cn("num text-xs", tab === tb.key ? "text-background/70" : "text-muted-foreground/70")}>{counts[tb.key]}</span>
            </button>
          ))}
        </div>
        <label className="flex h-9 items-center gap-2 rounded-full bg-muted px-3.5 sm:ml-auto sm:w-56">
          <Search className="size-4 shrink-0 text-muted-foreground" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("searchPlaceholder")}
            aria-label={t("searchLabel")}
            className="w-full min-w-0 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
          />
        </label>
      </div>

      <AnimatePresence initial={false}>
        {stale.length > 0 && (
          <m.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.25, ease }}
            className="overflow-hidden"
          >
            <div className="mt-4 flex flex-col gap-3 rounded-2xl bg-warm-soft px-5 py-4 text-warm-foreground sm:flex-row sm:items-center">
              <p className="flex-1 text-sm">
                {t.rich("stale", { count: stale.length, b: (c) => <span className="font-semibold">{c}</span> })}
              </p>
              <Button variant="solid" size="pill" disabled={renew.isPending} onClick={() => renew.mutate({})}>
                <RefreshCw className={renew.isPending ? "animate-spin" : ""} /> {t("renewAll")}
              </Button>
            </div>
          </m.div>
        )}
      </AnimatePresence>

      <div className="mt-4 overflow-hidden rounded-3xl border bg-card">
        <div className="hidden grid-cols-[minmax(0,1fr)_8rem_9rem_11rem_13rem] gap-4 border-b px-5 py-3 text-xs font-medium text-muted-foreground md:grid">
          <span>{t("columns.product")}</span>
          <span className="text-right">{t("columns.price")}</span>
          <span className="text-right">{t("columns.funnel")}</span>
          <span>{t("columns.state")}</span>
          <span className="sr-only">{t("columns.actions")}</span>
        </div>
        {visible.length === 0 ? (
          <p className="px-5 py-14 text-center text-sm text-muted-foreground">
            {query ? t("emptySearch", { query }) : t("emptyTab")}
          </p>
        ) : (
          <ul className="divide-y">
            <AnimatePresence initial={false}>
              {visible.map(({ l, state }) => (
                <m.li
                  key={l.id}
                  layout="position"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  transition={{ duration: 0.18 }}
                  className="grid grid-cols-1 items-center gap-x-4 gap-y-3 px-4 py-4 transition-colors hover:bg-muted/30 sm:px-5 md:grid-cols-[minmax(0,1fr)_8rem_9rem_11rem_13rem]"
                >
                  <Row l={l} state={state} />
                </m.li>
              ))}
            </AnimatePresence>
          </ul>
        )}
      </div>
    </div>
  );
}

function Row({ l, state }: { l: MyListing; state: ListingState }) {
  const t = useTranslations("mine");
  const tc = useTranslations("common");
  const fmt = useFormat();
  const names = useCatalogNames();
  const Icon = categoryIcons[l.category] ?? Package;
  const unit = fmt.unit(l.unit);
  const dot = stateDots[state];
  const label = t(`states.${state}`);
  const name = names.product(l.product_slug, l.product_name);
  return (
    <>
      <Link href={`/mahsulot/${l.product_slug}`} className="flex min-w-0 items-center gap-3.5">
        <span className="grid size-14 shrink-0 place-items-center overflow-hidden rounded-xl bg-muted">
          {l.cover_key ? (
            <Photo thumb k={l.cover_key} alt={name} sizes="56px" className="size-full" />
          ) : (
            <Icon strokeWidth={1.25} className="size-6 text-muted-foreground/60" />
          )}
        </span>
        <span className="min-w-0">
          <span className="block truncate font-medium">
            {name}
            {l.variety && <span className="text-muted-foreground"> · {l.variety}</span>}
          </span>
          <span className="block text-xs text-muted-foreground capitalize">
            {tc(l.trade_type === "ulgurji" ? "wholesaleLower" : "retailLower")} · #{l.id}
          </span>
          {/* Phones: price and status sit under the name. */}
          <span className="mt-1 flex flex-wrap items-center gap-x-3 text-xs md:hidden">
            <span className="num text-sm font-semibold">
              {formatSom(l.price_som)} <span className="font-sans font-normal text-muted-foreground">{tc("perUnit", { unit })}</span>
            </span>
            <span className="flex items-center gap-1.5">
              <span className={cn("size-1.5 rounded-full", dot)} />
              {label}
            </span>
            <MarketDiff l={l} />
          </span>
          {/* Second line on phones: market rank, automatic pricing and the week's funnel. */}
          <span className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1.5 md:hidden">
            <PositionChip l={l} />
            <AutoBadge l={l} />
            <Funnel l={l} />
          </span>
        </span>
      </Link>
      <div className="hidden text-right md:block">
        <p className="num font-semibold">{formatSom(l.price_som)}</p>
        <p className="text-xs text-muted-foreground">{tc("perUnit", { unit })}</p>
        <MarketDiff l={l} className="mt-0.5 justify-end" />
        <PositionChip l={l} className="mt-1" />
      </div>
      <div className="hidden justify-end md:flex">
        <Funnel l={l} />
      </div>
      <div className="hidden text-sm md:block">
        <p className="flex items-center gap-1.5 font-medium">
          <span className={cn("size-1.5 rounded-full", dot)} />
          {label}
        </p>
        {!l.expired && <p className="text-xs text-muted-foreground">{fmt.until(l.valid_until)}</p>}
        <AutoBadge l={l} className="mt-1" />
      </div>
      <div className="flex justify-end">
        <ListingActions l={l} />
      </div>
      <StatusNote l={l} className="md:col-span-5" />
    </>
  );
}

/** "7% arzon" / "4% qimmat" against the market median, with the median in the tooltip. */
function MarketDiff({ l, className }: { l: MyListing; className?: string }) {
  const t = useTranslations("mine");
  const d = l.diff_percent;
  if (d == null || !l.median) return null;
  return (
    <span
      title={t(l.median_scope === "province" ? "medianProvince" : "medianCountry", { price: formatSom(l.median) })}
      className={cn(
        "flex items-center gap-0.5 text-xs font-medium whitespace-nowrap",
        d < 0 ? "text-primary" : d > 0 ? "text-warm-foreground" : "text-muted-foreground",
        className,
      )}
    >
      {d < 0 ? <ArrowDownRight className="size-3" /> : d > 0 ? <ArrowUpRight className="size-3" /> : null}
      {d === 0 ? t("atMarket") : t(d < 0 ? "belowMarket" : "aboveMarket", { percent: Math.abs(d) })}
    </span>
  );
}

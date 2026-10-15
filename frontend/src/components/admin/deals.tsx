"use client";

import { useQuery } from "@tanstack/react-query";
import { Check, Handshake, Minus } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { useCatalogNames } from "@/components/catalog-names";
import { EmptyState } from "@/components/empty-state";
import { useLimits } from "@/components/limits";
import { Segmented } from "@/components/segmented";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { adminDealsOptions } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { AdminDeal } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

type Filter = "all" | "reported" | "confirmed" | "disputed";

/** Recent deals with a status filter: a table on wide screens, cards on phones. */
export function AdminDeals({ deals }: { deals: AdminDeal[] }) {
  const t = useTranslations("admin.deals");
  const tc = useTranslations("common");
  const [filter, setFilter] = useState<Filter>("all");
  // "All" comes with the page; a status is fetched when it is picked.
  const filtered = useQuery({ ...adminDealsOptions({ query: filter === "all" ? undefined : { status: filter } }), enabled: filter !== "all" });
  const rows = filter === "all" ? deals : filtered.data?.deals;
  // The API returns at most this many deals, newest first.
  const limit = useLimits().admin_deals_limit;

  return (
    <div>
      <div className="-mx-4 overflow-x-auto px-4 scrollbar-none sm:mx-0 sm:px-0">
        <Segmented
          label={t("filterLabel")}
          size="sm"
          className="w-max min-w-full sm:min-w-0 [&>button]:px-3"
          value={filter}
          onChange={setFilter}
          options={[
            { value: "all", label: tc("all") },
            { value: "reported", label: t("filters.reported") },
            { value: "confirmed", label: t("filters.confirmed") },
            { value: "disputed", label: t("filters.disputed") },
          ]}
        />
      </div>
      <div className="mt-5">
        {!rows ? (
          <Skeleton className="h-48 rounded-3xl" />
        ) : rows.length === 0 ? (
          <EmptyState
            icon={<Handshake strokeWidth={1.5} />}
            title={filter === "all" ? t("emptyTitle") : t("emptyFiltered")}
            description={filter === "all" ? t("emptyDescription") : undefined}
          />
        ) : (
          <>
            <DealTable deals={rows} />
            <ul className="space-y-3 md:hidden">
              {rows.map((d) => (
                <li key={d.id}>
                  <DealCard d={d} />
                </li>
              ))}
            </ul>
            {rows.length >= limit && <p className="mt-3 text-center text-xs text-muted-foreground">{t("latest", { count: rows.length })}</p>}
          </>
        )}
      </div>
    </div>
  );
}

/** Formatting shared by the table and the cards. */
function useDealText() {
  const tcodes = useTranslations("codes");
  const tc = useTranslations("common");
  const fmt = useFormat();
  const names = useCatalogNames();
  return {
    product: (d: AdminDeal) => names.product(undefined, d.product_name),
    region: (d: AdminDeal) => names.region(d.region_name),
    source: (d: AdminDeal) => tcodes(`dealSource.${d.source}`),
    status: (d: AdminDeal) => {
      const key = `dealStatus.${d.status}` as Parameters<typeof tcodes>[0];
      return tcodes.has(key) ? tcodes(key) : d.status;
    },
    quantity: (d: AdminDeal) =>
      d.quantity == null ? null : `${Number.isInteger(d.quantity) ? formatSom(d.quantity) : fmt.decimal(d.quantity, 2)} ${fmt.unit(d.unit)}`,
    price: (d: AdminDeal) => (d.price_som == null ? null : { value: formatSom(d.price_som), unit: tc("perUnit", { unit: fmt.unit(d.unit) }) }),
    date: (iso: string) => fmt.date(iso),
  };
}

function ProductName({ d, text }: { d: AdminDeal; text: ReturnType<typeof useDealText> }) {
  const name = text.product(d);
  return d.listing_id ? (
    <Link href={`/elon/${d.listing_id}`} className="font-medium underline-offset-4 hover:underline">
      {name}
    </Link>
  ) : (
    <span className="font-medium">{name}</span>
  );
}

function StatusPill({ d, text }: { d: AdminDeal; text: ReturnType<typeof useDealText> }) {
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap",
        d.status === "confirmed" ? "bg-primary/10 text-primary" : d.status === "disputed" ? "bg-destructive/10 text-destructive" : "bg-warm-soft text-warm-foreground",
      )}
    >
      {text.status(d)}
    </span>
  );
}

/** One side of the deal: a check mark and the day it confirmed, or a dash. */
function Side({ label, at, missing }: { label: string; at?: string; missing?: string }) {
  const t = useTranslations("admin.deals");
  const fmt = useFormat();
  return (
    <span className="flex items-center gap-1.5 text-xs">
      {at ? <Check className="size-3.5 shrink-0 text-primary" aria-hidden /> : <Minus className="size-3.5 shrink-0 text-muted-foreground/60" aria-hidden />}
      <span className={at ? "text-foreground" : "text-muted-foreground"}>{label}</span>
      <span className="text-muted-foreground">{at ? t("confirmedOn", { date: fmt.dayMonth(at) }) : (missing ?? t("notConfirmed"))}</span>
    </span>
  );
}

function Confirmations({ d }: { d: AdminDeal }) {
  const t = useTranslations("admin.deals");
  return (
    <span className="flex flex-col gap-1">
      <Side label={t("buyer")} at={d.buyer_confirmed_at} missing={d.has_buyer ? undefined : t("noBuyer")} />
      <Side label={t("seller")} at={d.seller_confirmed_at} />
    </span>
  );
}

function DealTable({ deals }: { deals: AdminDeal[] }) {
  const t = useTranslations("admin.deals.columns");
  const text = useDealText();
  return (
    <div className="hidden overflow-hidden rounded-3xl border bg-card md:block">
      <Table>
        <TableHeader>
          <TableRow className="text-xs hover:bg-transparent [&>th]:text-muted-foreground">
            <TableHead className="pl-5">{t("date")}</TableHead>
            <TableHead>{t("product")}</TableHead>
            <TableHead>{t("seller")}</TableHead>
            <TableHead>{t("region")}</TableHead>
            <TableHead>{t("source")}</TableHead>
            <TableHead className="text-right">{t("quantity")}</TableHead>
            <TableHead className="text-right">{t("price")}</TableHead>
            <TableHead>{t("confirmed")}</TableHead>
            <TableHead className="pr-5">{t("status")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {deals.map((d) => {
            const price = text.price(d);
            return (
              <TableRow key={d.id}>
                <TableCell className="py-3 pl-5 text-muted-foreground">{text.date(d.created_at)}</TableCell>
                <TableCell>
                  <ProductName d={d} text={text} />
                </TableCell>
                <TableCell>{d.seller_name}</TableCell>
                <TableCell className="text-muted-foreground">{text.region(d)}</TableCell>
                <TableCell className="text-muted-foreground">{text.source(d)}</TableCell>
                <TableCell className="num text-right">{text.quantity(d) ?? "—"}</TableCell>
                <TableCell className="text-right">
                  {price ? (
                    <>
                      <span className="num font-medium">{price.value}</span> <span className="text-xs text-muted-foreground">{price.unit}</span>
                    </>
                  ) : (
                    "—"
                  )}
                </TableCell>
                <TableCell>
                  <Confirmations d={d} />
                </TableCell>
                <TableCell className="pr-5">
                  <StatusPill d={d} text={text} />
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}

function DealCard({ d }: { d: AdminDeal }) {
  const text = useDealText();
  const price = text.price(d);
  const quantity = text.quantity(d);
  return (
    <article className="rounded-3xl border bg-card p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <ProductName d={d} text={text} />
          <p className="mt-0.5 text-sm text-muted-foreground">
            {d.seller_name} · {text.region(d)}
          </p>
        </div>
        <StatusPill d={d} text={text} />
      </div>
      <p className="mt-3 flex flex-wrap items-baseline gap-x-3 gap-y-1">
        {price && (
          <span>
            <span className="num text-lg font-semibold">{price.value}</span> <span className="text-sm text-muted-foreground">{price.unit}</span>
          </span>
        )}
        {quantity && <span className="num text-sm text-muted-foreground">{quantity}</span>}
      </p>
      <div className="mt-3 flex flex-wrap items-end justify-between gap-3 border-t pt-3">
        <Confirmations d={d} />
        <p className="text-xs text-muted-foreground">
          {text.source(d)} · {text.date(d.created_at)}
        </p>
      </div>
    </article>
  );
}

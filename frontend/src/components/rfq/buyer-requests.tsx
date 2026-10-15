"use client";

import { useQuery } from "@tanstack/react-query";
import { ChevronRight, Inbox } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";

import { useCatalogNames } from "@/components/catalog-names";
import { EmptyState } from "@/components/empty-state";
import { useLimits } from "@/components/limits";
import { RequestForm, type RequestPrefill } from "@/components/rfq/request-form";
import { Deadline, RfqStatusBadge, useQuantity } from "@/components/rfq/rfq-parts";
import { listMyRfqsOptions } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { Category, MyRfq, Product } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

/** The buyer's page body: the new request form and the list of their requests, kept in sync. */
export function BuyerRequests({
  products,
  categories,
  initial,
  prefill,
}: {
  products: Product[];
  categories: Category[];
  initial: MyRfq[];
  prefill?: RequestPrefill;
}) {
  const t = useTranslations("rfq");
  const { data } = useQuery({ ...listMyRfqsOptions(), initialData: { rfqs: initial } });
  const rfqs = data.rfqs;
  const open = rfqs.filter((r) => r.status === "open").length;
  const maxOpen = useLimits().rfq.max_open;

  return (
    <div className="space-y-10">
      <RequestForm products={products} categories={categories} openCount={open} prefill={prefill} />
      <section aria-labelledby="my-requests">
        <div className="mb-4 flex items-end justify-between gap-3">
          <h2 id="my-requests" className="display text-2xl">
            {t("list.title")}
          </h2>
          {rfqs.length > 0 && (
            <p className="num text-sm text-muted-foreground">{t("list.openCount", { count: open, max: maxOpen })}</p>
          )}
        </div>
        {rfqs.length === 0 ? (
          <EmptyState icon={<Inbox strokeWidth={1.5} />} title={t("list.emptyTitle")} description={t("list.emptyText")} className="py-12" />
        ) : (
          <div className="overflow-hidden rounded-3xl border bg-card">
            <div className="hidden grid-cols-[minmax(0,1fr)_7rem_9rem_14rem_1.25rem] gap-4 border-b px-5 py-3 text-xs font-medium text-muted-foreground md:grid">
              <span>{t("list.product")}</span>
              <span className="text-right">{t("list.offers")}</span>
              <span className="text-right">{t("list.best")}</span>
              <span>{t("list.deadline")}</span>
              <span />
            </div>
            <ul className="divide-y">
              {rfqs.map((r) => (
                <li key={r.id}>
                  <RequestRow r={r} />
                </li>
              ))}
            </ul>
          </div>
        )}
      </section>
    </div>
  );
}

function RequestRow({ r }: { r: MyRfq }) {
  const t = useTranslations("rfq.list");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const fmt = useFormat();
  const qty = useQuantity();
  const unit = fmt.unit(r.unit);
  return (
    <Link
      href={`/sorovlar/${r.id}`}
      className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 gap-y-2 px-4 py-4 transition-colors outline-none hover:bg-muted/30 focus-visible:bg-muted/40 sm:px-5 md:grid-cols-[minmax(0,1fr)_7rem_9rem_14rem_1.25rem]"
    >
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <p className="truncate font-medium">{names.product(undefined, r.product_name)}</p>
          <RfqStatusBadge status={r.status} />
        </div>
        <p className="num mt-0.5 text-sm text-muted-foreground">{qty(r.quantity, r.unit)}</p>
      </div>
      <ChevronRight className="size-4 justify-self-end text-muted-foreground md:order-last" aria-hidden />
      <p className={cn("text-sm md:text-right", r.offers === 0 && "text-muted-foreground")}>
        <span className="num md:hidden">{t("offersShort", { count: r.offers })}</span>
        <span className="num hidden font-medium md:inline">{r.offers}</span>
      </p>
      <p className="text-sm md:text-right">
        {r.best_price != null ? (
          <>
            <span className="text-muted-foreground md:hidden">{t("bestShort")} </span>
            <span className="num font-semibold">{formatSom(r.best_price)}</span>{" "}
            <span className="text-xs text-muted-foreground">{tc("perUnit", { unit })}</span>
          </>
        ) : (
          <span className="text-muted-foreground">{t("noOffers")}</span>
        )}
      </p>
      <Deadline iso={r.deadline} className="col-span-2 text-sm text-muted-foreground md:col-span-1" />
    </Link>
  );
}

"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { BellOff, BellRing } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { useCatalogNames } from "@/components/catalog-names";
import { Skeleton } from "@/components/ui/skeleton";
import { deleteWatchMutation, listWatchesOptions, listWatchesQueryKey } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { ListWatchesResponse } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";

/** The visitor's price watches with a way to stop each one. */
export function WatchList() {
  const t = useTranslations("watch.list");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const fmt = useFormat();
  const errors = useApiErrors();
  const queryClient = useQueryClient();
  const watches = useQuery(listWatchesOptions());
  const remove = useMutation({
    ...deleteWatchMutation(),
    onSuccess: (_, vars) => {
      queryClient.setQueryData<ListWatchesResponse>(listWatchesQueryKey(), (old) =>
        old ? { watches: old.watches.filter((w) => w.id !== vars.path.id) } : old,
      );
      toast.success(t("removed"));
    },
    onError: (err) => errors.show(err, "delete"),
  });

  if (watches.isPending) return <Skeleton className="h-16 rounded-2xl" />;
  const list = watches.data?.watches ?? [];
  if (!list.length) return <p className="text-sm text-muted-foreground">{t("empty")}</p>;

  return (
    <ul className="divide-y">
      {list.map((w) => (
        <li key={w.id} className="flex items-center gap-3 py-3 first:pt-0 last:pb-0">
          <BellRing className="size-5 shrink-0 text-primary" strokeWidth={1.5} />
          <div className="min-w-0 flex-1">
            <Link href={`/mahsulot/${w.product_slug}?region=${w.province_id ?? 0}${w.trade_type === "chakana" ? "&trade=chakana" : ""}`} className="font-medium underline-offset-4 hover:underline">
              {names.product(w.product_slug, w.product_name)}
            </Link>
            <p className="truncate text-xs text-muted-foreground">
              {[
                w.province_name ? names.region(w.province_name) : tc("wholeCountry"),
                w.trade_type === "chakana" ? tc("retailLower") : tc("wholesaleLower"),
                w.max_price ? t("below", { price: formatSom(w.max_price) }) : t("anyDrop"),
                // The bot also alerts when the median falls 5% from its recent high.
                w.peak_median && t("medianDrop", { price: formatSom(w.peak_median * 0.95) }),
                w.notified_at && t("notified", { when: fmt.dayAgo(w.notified_at) }),
              ]
                .filter(Boolean)
                .join(" · ")}
            </p>
          </div>
          <button
            type="button"
            onClick={() => remove.mutate({ path: { id: w.id } })}
            disabled={remove.isPending && remove.variables?.path.id === w.id}
            aria-label={t("remove")}
            title={t("remove")}
            className="grid size-9 shrink-0 place-items-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:opacity-50"
          >
            <BellOff className="size-4" />
          </button>
        </li>
      ))}
    </ul>
  );
}

"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { History, Undo2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { SignedBar } from "@/components/analytics/forecast";
import { useLimits } from "@/components/limits";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  getMyListingQueryKey,
  listPriceChangesOptions,
  listPriceChangesQueryKey,
  revertPriceChangeMutation,
} from "@/lib/api/gen/@tanstack/react-query.gen";
import type { PriceChange } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

/** The automatic price changes of one listing, newest first, with "Undo" on the latest. */
export function PriceHistory({ id, unit }: { id: number; unit: string }) {
  const t = useTranslations("autoPricing.history");
  const errors = useApiErrors();
  const queryClient = useQueryClient();
  const pauseHours = Math.round(useLimits().auto_price.manual_pause_hours);
  const { data, isError } = useQuery(listPriceChangesOptions({ path: { id } }));

  const revert = useMutation({
    ...revertPriceChangeMutation(),
    onSuccess: async (res) => {
      queryClient.setQueryData(getMyListingQueryKey({ path: { id } }), res);
      await queryClient.invalidateQueries({ queryKey: listPriceChangesQueryKey({ path: { id } }) });
      toast.success(t("reverted", { hours: pauseHours }));
    },
    onError: (err) => {
      errors.show(err, "change");
      void queryClient.invalidateQueries({ queryKey: listPriceChangesQueryKey({ path: { id } }) });
    },
  });

  return (
    <section aria-labelledby="history-title" className="rounded-3xl border bg-card">
      <header className="border-b px-5 py-5 sm:px-6">
        <h2 id="history-title" className="font-semibold tracking-tight">
          {t("title")}
        </h2>
        <p className="mt-0.5 text-sm text-muted-foreground">{t("hint")}</p>
      </header>
      {isError ? (
        <p className="px-5 py-10 text-center text-sm text-muted-foreground">{t("failed")}</p>
      ) : !data ? (
        <div className="space-y-3 p-5 sm:p-6">
          <Skeleton className="h-16 rounded-2xl" />
          <Skeleton className="h-16 rounded-2xl" />
        </div>
      ) : data.changes.length === 0 ? (
        <div className="flex flex-col items-center gap-2 px-5 py-12 text-center">
          <History strokeWidth={1.5} className="size-7 text-muted-foreground/70" aria-hidden />
          <p className="text-sm text-muted-foreground">{t("empty")}</p>
        </div>
      ) : (
        <ul className="divide-y">
          {data.changes.map((c) => (
            <ChangeRow
              key={c.id}
              c={c}
              unit={unit}
              busy={revert.isPending}
              onRevert={() => revert.mutate({ path: { id: c.id } })}
            />
          ))}
        </ul>
      )}
    </section>
  );
}

function ChangeRow({ c, unit, busy, onRevert }: { c: PriceChange; unit: string; busy: boolean; onRevert: () => void }) {
  const t = useTranslations("autoPricing.history");
  const tf = useTranslations("codes.factor");
  const tc = useTranslations("common");
  const fmt = useFormat();
  const change = (c.new_price - c.old_price) / c.old_price;
  const reverted = c.status === "reverted";
  const factors = c.factors.filter((f) => f.share !== 0);
  const parts = somParts(c);
  const widest = Math.max(...parts.map((p) => Math.abs(p.share)), 0.001);
  const signedSom = (v: number) => `${v > 0 ? "+" : v < 0 ? "−" : ""}${formatSom(Math.abs(v))}`;

  return (
    <li className={cn("flex flex-col gap-3 px-5 py-4 sm:flex-row sm:items-start sm:px-6", reverted && "text-muted-foreground")}>
      <div className="min-w-0 flex-1">
        <p className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <span className="num text-lg font-semibold">
            <span className={cn(reverted && "line-through decoration-1")}>
              {formatSom(c.old_price)} → {formatSom(c.new_price)}
            </span>
            <span className="ml-1 font-sans text-xs font-normal text-muted-foreground">{tc("perUnit", { unit })}</span>
          </span>
          <span className={cn("num text-sm font-medium", reverted ? "" : change < 0 ? "text-primary" : "text-warm-foreground")}>
            {fmt.percent(change, { signed: true })}
          </span>
        </p>
        <p className="mt-0.5 text-xs text-muted-foreground">
          <time dateTime={c.created_at}>
            {fmt.dayMonth(c.created_at)}, {fmt.time(c.created_at)}
          </time>
          {" · "}
          {t("median", { price: formatSom(c.median), count: c.peers })}
          {reverted && (
            <>
              {" · "}
              <span className="font-medium">
                {c.reverted_at ? t("revertedAt", { time: `${fmt.dayMonth(c.reverted_at)}, ${fmt.time(c.reverted_at)}` }) : t("revertedLabel")}
              </span>
            </>
          )}
        </p>
        {parts.length > 0 ? (
          <div className="mt-3 max-w-md rounded-2xl bg-muted/50 px-3.5 py-3">
            <p className="text-xs font-medium">{t("contrib")}</p>
            <p className="mt-0.5 text-xs text-muted-foreground">{t("contribHint")}</p>
            <ul className="mt-2.5 space-y-1.5" aria-label={t("contrib")}>
              {parts.map((p) => (
                <SignedBar
                  key={p.code}
                  label={tf(`${p.code}.${p.share > 0 ? "up" : "down"}`)}
                  share={p.share}
                  max={widest}
                  value={signedSom(p.som)}
                  note={fmt.percent(p.share, { signed: true })}
                  muted={reverted}
                  stack
                />
              ))}
              <li className="flex items-baseline justify-between gap-3 border-t border-foreground/10 pt-1.5 text-xs">
                <span className="font-medium">{t("total")}</span>
                <span className="num">
                  <span className="font-semibold">{signedSom(c.new_price - c.old_price)}</span>
                  <span className="ml-1.5 text-muted-foreground">{fmt.percent(change, { signed: true })}</span>
                </span>
              </li>
            </ul>
          </div>
        ) : factors.length > 0 && (
          <ul className="mt-2.5 flex flex-wrap gap-1.5" aria-label={t("factors")}>
            {factors.map((f) => (
              <li key={f.code} className="inline-flex items-center gap-1.5 rounded-full bg-muted px-2.5 py-1 text-xs">
                <span className={reverted ? "" : "text-foreground"}>{tf(`${f.code}.${f.share > 0 ? "up" : "down"}`)}</span>
                <span className={cn("num font-medium", !reverted && (f.share < 0 ? "text-primary" : "text-warm-foreground"))}>
                  {fmt.percent(f.share, { signed: true })}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
      {c.can_revert && (
        <Button variant="outline" size="pill" className="h-10 self-start" disabled={busy} onClick={onRevert}>
          <Undo2 /> {t("undo")}
        </Button>
      )}
    </li>
  );
}

/**
 * The exact Shapley shares of a change in so'm, biggest first. The API rounds
 * shares to 0.1%, so the rounding remainder goes to the biggest part and the
 * parts add up to the change to the so'm.
 */
function somParts(c: PriceChange) {
  const total = c.new_price - c.old_price;
  const parts = c.contrib
    .filter((f) => f.share !== 0)
    .map((f) => ({ code: f.code, share: f.share, som: Math.round(f.share * c.old_price) }))
    .sort((a, b) => Math.abs(b.share) - Math.abs(a.share));
  if (parts.length) parts[0].som += total - parts.reduce((s, p) => s + p.som, 0);
  return parts;
}

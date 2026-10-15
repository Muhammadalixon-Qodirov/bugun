"use client";

import { Images, MapPin, Package } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";

import { useCatalogNames } from "@/components/catalog-names";
import { categoryIcons } from "@/components/category-icon";
import { Photo } from "@/components/photo";
import type { ListingCard as Card } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

/** Marketplace card for one seller's listing. */
export function ListingCard({ l, className }: { l: Card; className?: string }) {
  const t = useTranslations("card");
  const tc = useTranslations("common");
  const fmt = useFormat();
  const names = useCatalogNames();
  const unit = fmt.unit(l.unit);
  const product = names.product(l.product_slug, l.product_name);
  const diff = l.diff_percent;
  const Icon = categoryIcons[l.category] ?? Package;
  return (
    <Link
      href={`/elon/${l.id}`}
      // Touch: the card presses in like an app tile.
      className={cn("group flex h-full flex-col transition-transform duration-150 ease-out pointer-coarse:active:scale-[0.97]", className)}
    >
      <div className="relative aspect-square overflow-hidden rounded-2xl bg-muted">
        {l.cover_key ? (
          <Photo
            thumb
            k={l.cover_key}
            alt={l.product_name}
            className="size-full transition-transform duration-400 ease-out-soft group-hover:scale-[1.02]"
          />
        ) : (
          <div className="flex size-full flex-col items-center justify-center gap-3 bg-[radial-gradient(circle_at_50%_40%,var(--card),var(--muted)_70%)] p-4 text-center">
            <Icon strokeWidth={1.25} className="size-12 text-muted-foreground/50" />
            <span className="line-clamp-1 text-sm font-medium text-muted-foreground">{product}</span>
          </div>
        )}
        <div className="pointer-events-none absolute inset-0 rounded-2xl ring-1 ring-foreground/5 ring-inset" />
        <div className="absolute top-2.5 left-2.5 flex flex-wrap gap-1.5">
          {diff != null && diff <= -3 && (
            <span className="rounded-full bg-primary px-2 py-0.5 text-xs font-semibold text-primary-foreground" title={t("belowTitle")}>
              {t("below", { percent: Math.abs(diff) })}
            </span>
          )}
          {l.trade_type === "ulgurji" && (
            <span className="rounded-full bg-background/90 px-2 py-0.5 text-xs font-medium">{tc("wholesale")}</span>
          )}
        </div>
        {l.photo_count > 1 && (
          <span className="absolute right-2.5 bottom-2.5 flex items-center gap-1 rounded-full bg-black/60 px-2 py-0.5 text-[11px] font-medium text-white">
            <Images className="size-3" /> {l.photo_count}
          </span>
        )}
      </div>
      <div className="flex flex-1 flex-col px-0.5 pt-3">
        <p className="num text-lg leading-tight font-semibold">
          {formatSom(l.price_som)}
          <span className="ml-1 font-sans text-[13px] font-normal text-muted-foreground">{tc("perUnit", { unit })}</span>
        </p>
        <p className="mt-1 line-clamp-1 text-[15px] leading-snug">
          {product}
          {l.variety && <span className="text-muted-foreground"> · {l.variety}</span>}
        </p>
        {l.median ? (
          <p className="mt-0.5 text-xs text-muted-foreground">
            {t("marketAverage", { price: formatSom(l.median) })}
          </p>
        ) : null}
        <div className="mt-auto space-y-0.5 pt-3 text-xs text-muted-foreground">
          <p className="flex items-center gap-1">
            <MapPin className="size-3 shrink-0" />
            <span className="truncate">{names.region(l.region_name)}</span>
            <span aria-hidden>·</span>
            <span className="shrink-0">{fmt.dayAgo(l.updated_at)}</span>
          </p>
          <p className="truncate">
            {l.seller_name}
            {l.min_order != null && (
              <>
                {" "}
                · {t("minOrder", { amount: formatSom(l.min_order), unit })}
              </>
            )}
          </p>
        </div>
      </div>
    </Link>
  );
}

export function ListingGridSkeleton({ count = 8 }: { count?: number }) {
  return (
    <div className="grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 xl:grid-cols-4">
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="animate-pulse">
          <div className="aspect-square rounded-2xl bg-muted" />
          <div className="mt-3 h-5 w-1/2 rounded bg-muted" />
          <div className="mt-2 h-4 w-3/4 rounded bg-muted" />
          <div className="mt-4 h-3 w-2/3 rounded bg-muted" />
        </div>
      ))}
    </div>
  );
}

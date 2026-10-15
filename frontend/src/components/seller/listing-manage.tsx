"use client";

import { useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { ListingForm } from "@/components/listing-form";
import { Segmented } from "@/components/segmented";
import { AutoPricing } from "@/components/seller/auto-pricing";
import { ListingStats } from "@/components/seller/listing-stats";
import { PriceHistory } from "@/components/seller/price-history";
import { StatusNote } from "@/components/seller/signals";
import { getMyListingOptions } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { Category, MyListing, Product } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { type ManageTab, manageTabs } from "@/lib/listings";

/** The seller's page for one listing: the edit form, its statistics, and automatic pricing with history. */
export function ListingManage({
  listing,
  products,
  categories,
  initialTab,
}: {
  listing: MyListing;
  products: Product[];
  categories: Category[];
  initialTab: ManageTab;
}) {
  const t = useTranslations("listingForm.manage");
  const fmt = useFormat();
  const [tab, setTab] = useState<ManageTab>(initialTab);
  // Pricing and undo update this copy, so the form and the pricing panel see the new price.
  const { data } = useQuery({ ...getMyListingOptions({ path: { id: listing.id } }), initialData: { listing, created: false } });
  const l = data.listing;

  const choose = (next: ManageTab) => {
    setTab(next);
    const url = new URL(window.location.href);
    if (next === "details") url.searchParams.delete("bolim");
    else url.searchParams.set("bolim", manageTabs[next]);
    window.history.replaceState(null, "", url);
  };

  return (
    <div className="space-y-8">
      <Segmented
        label={t("label")}
        value={tab}
        onChange={choose}
        className="w-full sm:w-fit"
        options={[
          { value: "details", label: t("details") },
          { value: "stats", label: t("stats") },
          { value: "pricing", label: t("pricing") },
        ]}
      />

      {/* The form stays mounted while hidden so unsaved edits survive a look at the other tabs. */}
      <div hidden={tab !== "details"} className="space-y-6">
        <StatusNote l={listing} className="text-sm" />
        {/* A new price from undo remounts the form with it. */}
        <ListingForm key={l.price_som} products={products} categories={categories} listing={l} />
      </div>
      {tab === "stats" && <ListingStats id={l.id} />}
      {tab === "pricing" && (
        <div className="space-y-6">
          <AutoPricing l={l} />
          <PriceHistory id={l.id} unit={fmt.unit(l.unit)} />
        </div>
      )}
    </div>
  );
}

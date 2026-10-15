import type { Metadata } from "next";
import { notFound, redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { Reveal } from "@/components/motion";
import { Page } from "@/components/page";
import { PageHeader } from "@/components/page-header";
import { ListingManage } from "@/components/seller/listing-manage";
import { getCatalogNames } from "@/lib/i18n/catalog.server";
import { getFormat } from "@/lib/i18n/format.server";
import { type ManageTab, manageTabs } from "@/lib/listings";
import { getCatalogData, getCurrentUser, getOwnListing } from "@/lib/server-api";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("listingForm.pages.edit");
  return { title: t("title") };
}

export default async function EditListingPage({ params, searchParams }: PageProps<"/mahsulotlarim/[id]">) {
  const [{ id }, { bolim }] = await Promise.all([params, searchParams]);
  const tab = (Object.keys(manageTabs) as ManageTab[]).find((k) => manageTabs[k] === bolim) ?? "details";
  const listingId = Number(id);
  if (!Number.isInteger(listingId) || listingId < 1) notFound();
  const [me, catalog] = await Promise.all([getCurrentUser(), getCatalogData()]);
  if (!me) redirect(`/kirish?keyin=/mahsulotlarim/${listingId}`);
  const listing = await getOwnListing(listingId);
  if (!listing) notFound();

  const [t, fmt, names] = await Promise.all([getTranslations("listingForm.pages.edit"), getFormat(), getCatalogNames()]);
  const product = names.product(listing.product_slug, listing.product_name);

  return (
    <Page className="max-w-5xl py-10 sm:py-14">
      <PageHeader
        eyebrow={`${fmt.role("entrepreneur")} · #${listing.id}`}
        title={listing.variety ? `${product} · ${listing.variety}` : product}
        description={t("description")}
      />
      <Reveal delay={0.1} className="mt-10">
        <ListingManage listing={listing} products={catalog.products} categories={catalog.categories} initialTab={tab} />
      </Reveal>
    </Page>
  );
}

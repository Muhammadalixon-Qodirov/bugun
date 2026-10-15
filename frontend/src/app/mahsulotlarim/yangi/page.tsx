import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { ListingForm } from "@/components/listing-form";
import { Reveal } from "@/components/motion";
import { Page } from "@/components/page";
import { PageHeader } from "@/components/page-header";
import { getFormat } from "@/lib/i18n/format.server";
import { isEntrepreneur } from "@/lib/roles";
import { getCatalogData, getCurrentUser } from "@/lib/server-api";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("listingForm.pages.new");
  return { title: t("meta") };
}

export default async function NewListingPage() {
  const [me, catalog] = await Promise.all([getCurrentUser(), getCatalogData()]);
  if (!me) redirect("/kirish?keyin=/mahsulotlarim/yangi");
  if (!me.onboarded) redirect("/boshlash");
  if (!isEntrepreneur(me.role)) redirect("/kabinet");

  const [t, fmt] = await Promise.all([getTranslations("listingForm.pages.new"), getFormat()]);

  return (
    <Page className="max-w-5xl py-10 sm:py-14">
      <PageHeader eyebrow={fmt.role("entrepreneur")} title={t("title")} description={t("description")} />
      <Reveal delay={0.1} className="mt-10">
        <ListingForm products={catalog.products} categories={catalog.categories} />
      </Reveal>
    </Page>
  );
}

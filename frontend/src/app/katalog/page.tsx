import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

import { MarketView } from "@/components/market-view";
import { Page } from "@/components/page";

export async function generateMetadata(): Promise<Metadata> {
  return { title: (await getTranslations("site"))("catalog") };
}

export default async function CatalogPage({ searchParams }: PageProps<"/katalog">) {
  return (
    <Page className="py-4 sm:py-6">
      <MarketView searchParams={await searchParams} basePath="/katalog" />
    </Page>
  );
}

import { ShoppingBasket } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { EmptyState } from "@/components/empty-state";
import { Reveal } from "@/components/motion";
import { Page } from "@/components/page";
import { PageHeader } from "@/components/page-header";
import { BuyerRequests } from "@/components/rfq/buyer-requests";
import { buttonVariants } from "@/components/ui/button";
import { getMyRfqs } from "@/lib/server/rfq";
import { isEntrepreneur } from "@/lib/roles";
import { getCatalogData, getCurrentUser } from "@/lib/server-api";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("rfq");
  return { title: t("meta.title"), robots: { index: false } };
}

export default async function RequestsPage({ searchParams }: PageProps<"/sorovlar">) {
  const [me, params] = await Promise.all([getCurrentUser(), searchParams]);
  // A request the assistant proposed arrives filled in: product, quantity, days.
  const one = (v: string | string[] | undefined) => (typeof v === "string" ? v : "");
  const prefill = { product: one(params.mahsulot), quantity: one(params.miqdor), days: one(params.kun) };
  const query = new URLSearchParams(Object.entries({ mahsulot: prefill.product, miqdor: prefill.quantity, kun: prefill.days }).filter(([, v]) => v)).toString();
  if (!me) redirect(`/kirish?keyin=${encodeURIComponent(`/sorovlar${query ? `?${query}` : ""}`)}`);
  if (!me.onboarded) redirect("/boshlash");
  const t = await getTranslations("rfq");

  if (!isEntrepreneur(me.role)) {
    return (
      <Page className="max-w-4xl py-10 sm:py-14">
        <PageHeader eyebrow={t("eyebrow")} title={t("title")} />
        <Reveal delay={0.05} className="mt-10">
          <EmptyState
            icon={<ShoppingBasket strokeWidth={1.5} />}
            title={t("notBuyer.title")}
            description={t("notBuyer.text")}
            action={
              <Link href="/kabinet" className={buttonVariants({ variant: "solid", size: "pill", className: "h-11 px-6" })}>
                {t("notBuyer.action")}
              </Link>
            }
          />
        </Reveal>
      </Page>
    );
  }

  const [catalog, rfqs] = await Promise.all([getCatalogData(), getMyRfqs()]);

  return (
    <Page className="max-w-5xl py-10 sm:py-14">
      <PageHeader eyebrow={t("eyebrow")} title={t("title")} description={t("description")} />
      <Reveal delay={0.05} className="mt-10">
        <BuyerRequests products={catalog.products} categories={catalog.categories} initial={rfqs} prefill={prefill} />
      </Reveal>
    </Page>
  );
}

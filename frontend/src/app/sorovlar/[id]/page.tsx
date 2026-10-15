import { ArrowLeft, SearchX } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { EmptyState } from "@/components/empty-state";
import { Reveal } from "@/components/motion";
import { Page } from "@/components/page";
import { RfqDetail } from "@/components/rfq/rfq-detail";
import { buttonVariants } from "@/components/ui/button";
import { getCatalogNames } from "@/lib/i18n/catalog.server";
import { getRfqDetail } from "@/lib/server/rfq";
import { getCurrentUser } from "@/lib/server-api";

function parseId(raw: string) {
  const id = Number(raw);
  return Number.isInteger(id) && id > 0 ? id : null;
}

export async function generateMetadata({ params }: PageProps<"/sorovlar/[id]">): Promise<Metadata> {
  const id = parseId((await params).id);
  const [t, names, me] = await Promise.all([getTranslations("rfq"), getCatalogNames(), getCurrentUser()]);
  const data = id && me ? await getRfqDetail(id) : null;
  return {
    title: data ? t("meta.detail", { product: names.product(undefined, data.rfq.product_name) }) : t("meta.title"),
    robots: { index: false },
  };
}

export default async function RequestPage({ params }: PageProps<"/sorovlar/[id]">) {
  const raw = (await params).id;
  const me = await getCurrentUser();
  if (!me) redirect(`/kirish?keyin=/sorovlar/${encodeURIComponent(raw)}`);
  if (!me.onboarded) redirect("/boshlash");
  const id = parseId(raw);
  const [t, data] = await Promise.all([getTranslations("rfq"), id ? getRfqDetail(id) : null]);

  return (
    <Page className="max-w-5xl py-8 sm:py-12">
      <Link href="/sorovlar" className="inline-flex h-10 items-center gap-1.5 rounded-full text-sm text-muted-foreground transition-colors hover:text-foreground">
        <ArrowLeft className="size-4" /> {t("detail.back")}
      </Link>
      <Reveal className="mt-4">
        {id && data ? (
          <RfqDetail id={id} initial={data} />
        ) : (
          <EmptyState
            icon={<SearchX strokeWidth={1.5} />}
            title={t("detail.missingTitle")}
            description={t("detail.missingText")}
            action={
              <Link href="/sorovlar" className={buttonVariants({ variant: "solid", size: "pill", className: "h-11 px-6" })}>
                {t("detail.missingAction")}
              </Link>
            }
          />
        )}
      </Reveal>
    </Page>
  );
}

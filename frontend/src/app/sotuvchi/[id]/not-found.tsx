import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Page } from "@/components/page";
import { buttonVariants } from "@/components/ui/button";

export default async function SellerNotFound() {
  const t = await getTranslations("store");
  return (
    <Page className="grid min-h-[calc(var(--app-height)-4rem)] place-items-center py-16 text-center">
      <div className="reveal max-w-md">
        <p className="eyebrow">404</p>
        <h1 className="display mt-3 text-4xl sm:text-5xl">{t("notFoundTitle")}</h1>
        <p className="mt-4 leading-relaxed text-muted-foreground">{t("notFoundText")}</p>
        <div className="mt-8 flex justify-center">
          <Link href="/" className={buttonVariants({ variant: "solid", size: "pill", className: "h-11 px-6" })}>
            {t("toMarket")}
          </Link>
        </div>
      </div>
    </Page>
  );
}

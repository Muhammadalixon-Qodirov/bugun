import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Page } from "@/components/page";
import { buttonVariants } from "@/components/ui/button";

export default async function NotFound() {
  const t = await getTranslations("site");
  return (
    <Page className="grid min-h-[calc(var(--app-height)-4rem)] place-items-center py-16 text-center">
      <div className="reveal max-w-md">
        <p className="eyebrow">404</p>
        <h1 className="display mt-3 text-4xl sm:text-5xl">{t("notFound.title")}</h1>
        <p className="mt-4 leading-relaxed text-muted-foreground">
          {t("notFound.text")}
        </p>
        <div className="mt-8 flex flex-wrap justify-center gap-2">
          <Link href="/" className={buttonVariants({ variant: "solid", size: "pill", className: "h-11 px-6" })}>
            {t("backToMarket")}
          </Link>
          <Link href="/?sort=deal" className={buttonVariants({ variant: "outline", size: "pill", className: "h-11 px-6" })}>
            {t("notFound.deals")}
          </Link>
        </div>
      </div>
    </Page>
  );
}

import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Page } from "@/components/page";
import { buttonVariants } from "@/components/ui/button";

export default async function RegionNotFound() {
  const t = await getTranslations("stats");
  return (
    <Page className="grid min-h-[calc(var(--app-height)-4rem)] place-items-center py-16 text-center">
      <div className="reveal max-w-md">
        <p className="eyebrow">404</p>
        <h1 className="display mt-3 text-4xl sm:text-5xl">{t("region.notFoundTitle")}</h1>
        <p className="mt-4 leading-relaxed text-muted-foreground">{t("region.notFoundText")}</p>
        <div className="mt-8 flex flex-wrap justify-center gap-2">
          <Link href="/statistika" className={buttonVariants({ variant: "solid", size: "pill", className: "h-11 px-6" })}>
            {t("links.heat")}
          </Link>
          <Link href="/xarita" className={buttonVariants({ variant: "outline", size: "pill", className: "h-11 px-6" })}>
            {t("links.map")}
          </Link>
        </div>
      </div>
    </Page>
  );
}

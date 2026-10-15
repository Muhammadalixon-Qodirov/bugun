import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Logo } from "@/components/logo";
import { getCatalogNames } from "@/lib/i18n/catalog.server";
import { getCatalogData } from "@/lib/server-api";

export async function SiteFooter() {
  const [{ categories }, t, names] = await Promise.all([getCatalogData(), getTranslations("footer"), getCatalogNames()]);
  const botUsername = process.env.NEXT_PUBLIC_TELEGRAM_BOT_USERNAME;
  return (
    <footer className="mt-24 hidden border-t sm:block">
      <div className="mx-auto grid max-w-7xl gap-10 px-6 py-14 text-sm md:grid-cols-[1.4fr_1fr_1fr_1fr_1fr]">
        <div>
          <Logo />
          <p className="mt-4 max-w-xs leading-relaxed text-muted-foreground">
            {t("about")}
          </p>
        </div>
        <div>
          <p className="eyebrow mb-4">{t("categories")}</p>
          <ul className="space-y-2.5 text-muted-foreground">
            {categories.map((c) => (
              <li key={c.slug}>
                <Link href={`/katalog?category=${c.slug}`} className="transition-colors hover:text-foreground">
                  {names.category(c.slug, c.name)}
                </Link>
              </li>
            ))}
          </ul>
        </div>
        <div>
          <p className="eyebrow mb-4">{t("forBuyers")}</p>
          <ul className="space-y-2.5 text-muted-foreground">
            <li><Link href="/katalog?sort=deal" className="transition-colors hover:text-foreground">{t("deals")}</Link></li>
            <li><Link href="/katalog?sort=price_asc" className="transition-colors hover:text-foreground">{t("cheapest")}</Link></li>
            <li><Link href="/katalog?trade=ulgurji" className="transition-colors hover:text-foreground">{t("wholesalePrices")}</Link></li>
            <li><Link href="/sorovlar" className="transition-colors hover:text-foreground">{t("requestQuote")}</Link></li>
          </ul>
        </div>
        <div>
          <p className="eyebrow mb-4">{t("analytics")}</p>
          <ul className="space-y-2.5 text-muted-foreground">
            <li><Link href="/statistika" className="transition-colors hover:text-foreground">{t("heatmap")}</Link></li>
            <li><Link href="/solishtirish" className="transition-colors hover:text-foreground">{t("compare")}</Link></li>
            <li><Link href="/xarita" className="transition-colors hover:text-foreground">{t("map")}</Link></li>
            <li><Link href="/tahlilchi" className="transition-colors hover:text-foreground">{t("analyst")}</Link></li>
          </ul>
        </div>
        <div>
          <p className="eyebrow mb-4">{t("forSellers")}</p>
          <ul className="space-y-2.5 text-muted-foreground">
            <li><Link href="/mahsulotlarim/yangi" className="transition-colors hover:text-foreground">{t("postListing")}</Link></li>
            <li><Link href="/narx" className="transition-colors hover:text-foreground">{t("priceByText")}</Link></li>
            {botUsername && (
              <li>
                <a href={`https://t.me/${botUsername}`} target="_blank" rel="noreferrer" className="transition-colors hover:text-foreground">
                  {t("bot")}
                </a>
              </li>
            )}
          </ul>
        </div>
      </div>
      <div className="border-t">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-x-8 gap-y-2 px-6 py-5 text-xs text-muted-foreground">
          <p>© {new Date().getFullYear()} Bugun</p>
          <p className="max-w-2xl">
            {t("disclaimer")}
          </p>
        </div>
      </div>
    </footer>
  );
}

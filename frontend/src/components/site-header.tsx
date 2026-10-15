import { ChartSpline, Plus } from "lucide-react";
import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";

import { HeaderSearch } from "@/components/header-search";
import { LocaleSwitcher } from "@/components/locale-switcher";
import { Logo } from "@/components/logo";
import { RegionPicker } from "@/components/region-picker";
import { ThemeSwitcher } from "@/components/theme-switcher";
import { BottomNav } from "@/components/site-nav";
import { buttonVariants } from "@/components/ui/button";
import { UserMenu } from "@/components/user-menu";
import { isEntrepreneur } from "@/lib/roles";
import { getCatalogData, getCurrentUser, getRegionId } from "@/lib/server-api";

export async function SiteHeader() {
  const [me, catalog, regionId, t] = await Promise.all([getCurrentUser(), getCatalogData(), getRegionId(), getTranslations("nav")]);
  const seller = isEntrepreneur(me?.role);
  const navUser = me ? { onboarded: me.onboarded, entrepreneur: seller } : null;

  return (
    <>
      <header className="vt-site-header sticky top-0 z-40 pt-[var(--app-safe-top)] border-b border-border/70 bg-background sm:bg-background/90 sm:backdrop-blur-md">
        <div className="mx-auto flex h-16 max-w-7xl items-center gap-3 px-4 sm:gap-4 sm:px-6 lg:gap-8">
          <Link href="/" aria-label={t("home")} className="shrink-0">
            <Logo />
          </Link>
          <Suspense fallback={<div className="hidden flex-1 md:block" />}>
            <HeaderSearch className="hidden max-w-2xl flex-1 md:block" products={catalog.products} categories={catalog.categories} />
          </Suspense>
          <div className="ml-auto flex min-w-0 items-center gap-1 sm:gap-2">
            <Link href="/statistika" className={buttonVariants({ variant: "ghost", size: "pill", className: "hidden px-4 xl:inline-flex" })}>
              <ChartSpline /> {t("statistics")}
            </Link>
            <ThemeSwitcher />
            <LocaleSwitcher />
            <RegionPicker regions={catalog.regions} current={regionId} />
            {seller && (
              <Link
                href="/mahsulotlarim/yangi"
                className={buttonVariants({ variant: "ghost", size: "pill", className: "hidden px-4 lg:inline-flex" })}
              >
                <Plus /> {t("postListing")}
              </Link>
            )}
            {me ? (
              <UserMenu name={me.display_name} onboarded={me.onboarded} role={me.role} admin={me.is_admin} />
            ) : (
              <Link
                href="/kirish"
                // Phones have the tab bar's sign-in tab instead.
                className={buttonVariants({ variant: "solid", size: "pill", className: "hidden sm:inline-flex" })}
              >
                {t("signIn")}
              </Link>
            )}
          </div>
        </div>
        <div className="px-4 pb-3 md:hidden">
          <Suspense fallback={<div className="h-11" />}>
            <HeaderSearch products={catalog.products} categories={catalog.categories} />
          </Suspense>
        </div>
      </header>
      <Suspense>
        <BottomNav user={navUser} />
      </Suspense>
    </>
  );
}

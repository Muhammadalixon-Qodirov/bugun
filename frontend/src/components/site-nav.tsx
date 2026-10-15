"use client";

import { BadgePercent, ChartSpline, Home, Inbox, Package, Plus, UserRound } from "lucide-react";
import { m } from "motion/react";
import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";

import { haptic } from "@/lib/telegram";
import { cn } from "@/lib/utils";

type NavUser = { onboarded: boolean; entrepreneur: boolean } | null;

function isActive(pathname: string, deal: boolean, href: string) {
  if (href === "/") return pathname === "/" && !deal;
  if (href === "/?sort=deal") return pathname === "/" && deal;
  // Statistics covers the analytics pages that hang off it.
  if (href === "/statistika") return ["/statistika", "/hudud", "/solishtirish", "/xarita"].some((p) => pathname.startsWith(p)) || pathname.endsWith("/tahlil");
  return pathname.startsWith(href);
}

/**
 * Phone tab bar. Entrepreneurs get a central "add price" button plus their
 * listings and requests (and give up the deals tab for them); everyone
 * reaches the statistics. Tabs share the width so six items fit a 360px phone.
 */
export function BottomNav({ user }: { user: NavUser }) {
  const t = useTranslations("nav");
  const pathname = usePathname();
  const deal = useSearchParams().get("sort") === "deal";
  // Listing pages have their own sticky price-and-contact bar.
  if (pathname.startsWith("/elon/")) return null;
  const tabs = [
    { href: "/", label: t("tabHome"), icon: Home },
    ...(user?.entrepreneur ? [] : [{ href: "/?sort=deal", label: t("tabDeals"), icon: BadgePercent }]),
    { href: "/statistika", label: t("tabStats"), icon: ChartSpline },
    ...(user?.entrepreneur
      ? [
          { href: "/mahsulotlarim", label: t("tabMine"), icon: Package },
          { href: "/sorovlar", label: t("tabRequests"), icon: Inbox },
        ]
      : []),
    { href: user?.onboarded ? "/profil" : "/kirish", label: user ? t("tabProfile") : t("signIn"), icon: UserRound },
  ];
  return (
    <nav className="vt-site-tabbar fixed inset-x-3 bottom-[calc(0.75rem+var(--app-safe-bottom))] transition-transform duration-200 ease-out z-40 sm:hidden" aria-label={t("mainMenu")}>
      <div className="flex items-center justify-around gap-0.5 rounded-full border bg-card p-1.5 shadow-lg shadow-black/5">
        {tabs.slice(0, 2).map((tab) => (
          <Tab key={tab.href} {...tab} active={isActive(pathname, deal, tab.href)} />
        ))}
        {user?.entrepreneur && (
          <Link
            href="/mahsulotlarim/yangi"
            aria-label={t("addProduct")}
            className="grid size-12 shrink-0 place-items-center rounded-full bg-foreground text-background shadow-md shadow-black/15 transition-transform active:scale-95"
          >
            <Plus className="size-5" />
          </Link>
        )}
        {tabs.slice(2).map((tab) => (
          <Tab key={tab.href} {...tab} active={isActive(pathname, deal, tab.href)} />
        ))}
      </div>
    </nav>
  );
}

function Tab({ href, label, icon: Icon, active }: { href: string; label: string; icon: typeof Home; active: boolean }) {
  return (
    <Link
      href={href}
      onClick={() => !active && haptic()}
      className={cn(
        "relative isolate flex min-w-0 max-w-20 flex-1 flex-col items-center gap-0.5 rounded-full px-0.5 py-1.5 text-[11px] font-medium transition-colors",
        active ? "text-foreground" : "text-muted-foreground",
      )}
    >
      {active && (
        <m.span
          layoutId="tab-pill"
          className="absolute inset-0 -z-10 rounded-full bg-muted"
          transition={{ type: "spring", stiffness: 420, damping: 34 }}
        />
      )}
      <Icon className="size-5" />
      <span className="max-w-full truncate">{label}</span>
    </Link>
  );
}

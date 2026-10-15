"use client";

import { m } from "motion/react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { createContext, use, useCallback, useEffect, useOptimistic, useRef, useTransition } from "react";

import { haptic } from "@/lib/telegram";
import { cn } from "@/lib/utils";

type MarketNav = { pending: boolean; target: string | null; go: (href: string) => void };

const MarketNavContext = createContext<MarketNav | null>(null);

/**
 * Every market filter (category, sort, trade, price) navigates through one
 * transition. The URL being loaded is kept as an optimistic value, so tabs and
 * toggles switch the moment they are pressed while the grid below stays on
 * screen, dimmed, until the new listings arrive.
 */
export function MarketNavProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [target, setTarget] = useOptimistic<string | null>(null);
  const go = useCallback(
    (href: string) =>
      startTransition(() => {
        setTarget(href);
        router.replace(href, { scroll: false });
      }),
    [router, setTarget],
  );
  return <MarketNavContext value={{ pending, target, go }}>{children}</MarketNavContext>;
}

export function useMarketNav() {
  const nav = use(MarketNavContext);
  if (!nav) throw new Error("useMarketNav must be used inside MarketNavProvider");
  return nav;
}

/** The market's query: the one being loaded if a filter was just pressed, else the current URL's. */
export function useMarketParams() {
  const { target } = useMarketNav();
  const current = useSearchParams();
  return target == null ? current : new URLSearchParams(target.split("?")[1] ?? "");
}

/** Dims its content while a filter change is loading. */
export function MarketPending({ className, children }: { className?: string; children: React.ReactNode }) {
  const { pending } = useMarketNav();
  return (
    <div aria-busy={pending} className={cn("transition-opacity duration-200", pending && "opacity-50", className)}>
      {children}
    </div>
  );
}

/** A real link (middle-click and crawlers work) that navigates inside the market transition. */
export function MarketLink({ href, className, children, ...rest }: React.ComponentProps<typeof Link> & { href: string }) {
  const { go } = useMarketNav();
  return (
    <Link
      href={href}
      scroll={false}
      className={className}
      onClick={(e) => {
        if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return;
        e.preventDefault();
        haptic();
        go(href);
      }}
      {...rest}
    >
      {children}
    </Link>
  );
}

type Tab = { key: string; label: string; icon: React.ReactNode; accent?: boolean };

/**
 * The category row. "all", a category slug or "deal" (below-market offers);
 * pressing the active tab again returns to "all".
 */
export function CategoryTabs({ basePath, tabs }: { basePath: string; tabs: Tab[] }) {
  const params = useMarketParams();
  const category = params.get("category");
  const deal = params.get("sort") === "deal";
  const activeKey = category ?? (deal ? "deal" : "all");
  const row = useRef<HTMLDivElement>(null);
  // On phones the row scrolls sideways: keep the active tab in view, without moving the page.
  useEffect(() => {
    const el = row.current;
    const tab = el?.querySelector<HTMLElement>('[aria-current="page"]');
    if (!el || !tab || el.scrollWidth <= el.clientWidth) return;
    el.scrollTo({ left: tab.offsetLeft - (el.clientWidth - tab.offsetWidth) / 2, behavior: "smooth" });
  }, [activeKey]);
  const hrefFor = (key: string) =>
    key === activeKey || key === "all" ? basePath : key === "deal" ? `${basePath}?sort=deal` : `${basePath}?category=${key}`;

  return (
    <div ref={row} className="relative flex gap-7 overflow-x-auto px-4 scrollbar-none sm:gap-9 sm:px-0 lg:justify-between">
      {tabs.map((t) => {
        const active = t.key === activeKey;
        return (
          <MarketLink
            key={t.key}
            href={hrefFor(t.key)}
            aria-current={active ? "page" : undefined}
            className={cn(
              "group relative flex shrink-0 flex-col items-center gap-2 pt-2 pb-3 text-xs font-medium whitespace-nowrap transition-colors duration-200 sm:text-[13px]",
              active ? "text-foreground" : "text-muted-foreground hover:text-foreground",
              t.accent && !active && "[&_svg]:text-primary",
            )}
          >
            <span className="transition-transform duration-200 ease-out group-hover:-translate-y-px">{t.icon}</span>
            {t.label}
            {active && (
              <m.span
                layoutId="category-underline"
                className="absolute inset-x-0 -bottom-px h-0.5 rounded-full bg-foreground"
                transition={{ type: "spring", stiffness: 520, damping: 40 }}
              />
            )}
          </MarketLink>
        );
      })}
    </div>
  );
}

"use client";

import { ArrowUpRight, Search, TrendingUp } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { useId, useMemo, useRef, useState } from "react";

import { useCatalogNames } from "@/components/catalog-names";
import { CategoryIcon } from "@/components/category-icon";
import type { Category, Product } from "@/lib/api/gen/types.gen";
import { cn } from "@/lib/utils";

const popular = ["kartoshka", "piyoz", "guruch", "mol-goshti", "olma", "tuxum", "sement", "shakar"];

function norm(s: string) {
  return s.toLowerCase().replace(/[‘’`ʻʼ']/g, "").trim();
}

type Option = { key: string; label: string; hint: string; category: string; href: string };

/** The market search box with product suggestions and keyboard navigation. */
export function HeaderSearch({ className, products, categories }: { className?: string; products: Product[]; categories: Category[] }) {
  const t = useTranslations("search");
  const locale = useLocale();
  const names = useCatalogNames();
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const listId = useId();
  const box = useRef<HTMLFormElement>(null);
  // Results open on the market page the visitor is on (home or /katalog), else on home.
  const market = pathname === "/katalog" ? "/katalog" : "/";
  const [q, setQ] = useState(pathname === market ? (params.get("q") ?? "") : "");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(-1);

  const catName = useMemo(() => new Map(categories.map((c) => [c.slug, names.category(c.slug, c.name)])), [categories, names]);
  const options: Option[] = useMemo(() => {
    const n = norm(q);
    // Matches the name in the visitor's language and the Uzbek one.
    const rank = (p: Product) => {
      const hits = [norm(names.product(p.slug, p.name)), norm(p.name)].map((s) => s.indexOf(n)).filter((i) => i >= 0);
      return hits.length ? Math.min(...hits) : -1;
    };
    const list = n
      ? products.filter((p) => rank(p) >= 0).sort((a, b) => rank(a) - rank(b))
      : popular.map((s) => products.find((p) => p.slug === s)).filter((p): p is Product => Boolean(p));
    return list.slice(0, 7).map((p) => ({
      key: p.slug,
      label: names.product(p.slug, p.name),
      hint: catName.get(p.category) ?? "",
      category: p.category,
      href: `${market}?product=${p.slug}`,
    }));
  }, [q, products, catName, market, names]);

  const go = (href: string) => {
    setOpen(false);
    setActive(-1);
    router.push(href);
  };
  const submit = () => {
    if (active >= 0 && options[active]) return go(options[active].href);
    // Free-text search runs on Uzbek names, so in other languages a matching product is the better answer.
    if (locale !== "uz" && q.trim() && options[0]) return go(options[0].href);
    const next = new URLSearchParams();
    if (q.trim()) next.set("q", q.trim());
    go(`${market}?${next}`);
  };

  return (
    <form
      ref={box}
      role="search"
      className={cn("relative", className)}
      onSubmit={(e) => {
        e.preventDefault();
        submit();
      }}
      onBlur={(e) => {
        if (!box.current?.contains(e.relatedTarget as Node)) setOpen(false);
      }}
    >
      <input
        value={q}
        onChange={(e) => {
          setQ(e.target.value);
          setOpen(true);
          setActive(-1);
        }}
        onFocus={() => setOpen(true)}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") {
            e.preventDefault();
            setOpen(true);
            setActive((i) => Math.min(options.length - 1, i + 1));
          } else if (e.key === "ArrowUp") {
            e.preventDefault();
            setActive((i) => Math.max(-1, i - 1));
          } else if (e.key === "Escape") {
            setOpen(false);
          }
        }}
        placeholder={t("placeholder")}
        aria-label={t("label")}
        role="combobox"
        aria-expanded={open}
        aria-controls={listId}
        aria-activedescendant={active >= 0 ? `${listId}-${active}` : undefined}
        autoComplete="off"
        enterKeyHint="search"
        className="peer h-11 w-full min-w-0 rounded-full border border-transparent bg-muted pr-4 pl-11 text-[15px] outline-none transition-[background-color,border-color,box-shadow] placeholder:text-muted-foreground hover:bg-[color-mix(in_oklch,var(--muted),var(--foreground)_4%)] focus:border-border focus:bg-card focus:shadow-[0_8px_30px_-12px_color-mix(in_oklch,var(--foreground)_25%,transparent)]"
      />
      <button
        type="submit"
        aria-label={t("label")}
        className="absolute inset-y-0 left-0 grid w-11 place-items-center rounded-full text-muted-foreground transition-colors peer-focus:text-foreground"
      >
        <Search className="size-[1.125rem]" />
      </button>

      <AnimatePresence>
        {open && options.length > 0 && (
          <m.ul
            id={listId}
            role="listbox"
            initial={{ opacity: 0, y: -4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
            transition={{ duration: 0.15 }}
            className="absolute inset-x-0 top-full z-50 mt-2 overflow-hidden rounded-2xl border bg-popover p-1.5 shadow-xl shadow-black/10"
          >
            <li className="flex items-center gap-1.5 px-2.5 pt-1 pb-1.5 text-xs text-muted-foreground">
              {q.trim() ? (
                <>{t("products")}</>
              ) : (
                <>
                  <TrendingUp className="size-3.5" /> {t("popular")}
                </>
              )}
            </li>
            {options.map((o, i) => (
              <li key={o.key} id={`${listId}-${i}`} role="option" aria-selected={i === active}>
                <button
                  type="button"
                  onMouseEnter={() => setActive(i)}
                  onClick={() => go(o.href)}
                  className={cn("flex w-full items-center gap-3 rounded-xl px-2.5 py-2 text-left text-sm", i === active && "bg-muted")}
                >
                  <CategoryIcon category={o.category} className="size-8 rounded-lg [&_svg]:size-4" />
                  <span className="flex-1 truncate font-medium">{o.label}</span>
                  <span className="text-xs text-muted-foreground">{o.hint}</span>
                  <ArrowUpRight className="size-4 text-muted-foreground" />
                </button>
              </li>
            ))}
            {q.trim() && (
              <li>
                <button type="submit" className="flex w-full items-center gap-3 rounded-xl px-2.5 py-2 text-left text-sm text-primary hover:bg-muted">
                  <Search className="ml-2 size-4" /> {t("allFor", { q: q.trim() })}
                </button>
              </li>
            )}
          </m.ul>
        )}
      </AnimatePresence>
    </form>
  );
}

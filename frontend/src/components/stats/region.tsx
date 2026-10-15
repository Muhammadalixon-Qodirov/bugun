"use client";

import { ArrowDownRight, ArrowUpDown, ArrowUpRight, ChevronDown, MapPin, Search } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useMemo, useState, useTransition } from "react";

import { useCatalogNames } from "@/components/catalog-names";
import { LineChart } from "@/components/charts/line-chart";
import { Picker } from "@/components/stats/picker";
import { useQueryNav } from "@/components/stats/query-nav";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ZoneBadge } from "@/components/zone-badge";
import type { IndexPoint, RegionProduct } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { norm } from "@/lib/stats";
import { cn } from "@/lib/utils";

/** Jumps to another province's page, keeping the trade type. */
export function ProvincePicker({ provinces, current }: { provinces: { id: number; name: string }[]; current: number }) {
  const t = useTranslations("stats");
  const names = useCatalogNames();
  const router = useRouter();
  const { params } = useQueryNav();
  const [pending, startTransition] = useTransition();
  const items = provinces.map((p) => ({ value: String(p.id), label: names.region(p.name), alias: p.name }));
  return (
    <Picker
      items={items}
      selected={[String(current)]}
      onPick={(v) => {
        const trade = params.get("trade");
        startTransition(() => router.push(`/hudud/${v}${trade ? `?trade=${trade}` : ""}`));
      }}
      triggerLabel={t("region.choose")}
      placeholder={t("region.search")}
      triggerClassName={cn(
        "flex h-11 items-center gap-2 rounded-full border bg-card px-4 text-sm font-medium transition-colors outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50 data-[popup-open]:bg-muted",
        pending && "opacity-60",
      )}
      trigger={
        <>
          <MapPin className="size-4 text-muted-foreground" />
          {t("region.other")}
          <ChevronDown className="size-4 text-muted-foreground" />
        </>
      }
    />
  );
}

/** The food-basket index, province against the country, both starting at 100. */
export function IndexChart({ points, province }: { points: IndexPoint[]; province: string }) {
  const t = useTranslations("stats");
  const fmt = useFormat();
  const at = (w: string) => Date.parse(`${w}T12:00:00Z`);
  const month = (ms: number) => {
    const d = new Date(ms);
    return d.getUTCMonth() === 0 ? String(d.getUTCFullYear()) : fmt.monthShort(d.getUTCMonth() + 1);
  };
  const series = [
    { id: "region", label: province, color: "var(--viz-1)", points: points.map((p) => ({ x: at(p.week), y: p.region })) },
    { id: "country", label: t("region.country"), color: "var(--viz-2)", points: points.map((p) => ({ x: at(p.week), y: p.country ?? null })) },
  ];
  const last = points.at(-1);
  return (
    <div>
      <LineChart
        series={series}
        height={240}
        yFormat={(v) => fmt.decimal(v, 0)}
        xFormat={month}
        titleFormat={(ms) => t("region.weekOf", { date: fmt.date(new Date(ms).toISOString()) })}
        ariaLabel={t("region.indexAria", { region: province })}
      />
      <ul className="mt-4 flex flex-wrap gap-x-6 gap-y-2 text-sm">
        {series.map((s) => {
          const v = s.id === "region" ? last?.region : last?.country;
          return (
            <li key={s.id} className="flex items-center gap-2">
              <span aria-hidden className="h-0.5 w-4 rounded-full" style={{ backgroundColor: s.color }} />
              <span className="text-muted-foreground">{s.label}</span>
              {v != null && <span className="num font-medium">{fmt.decimal(v, 1)}</span>}
              {v != null && <span className="num text-xs text-muted-foreground">({fmt.percent(v / 100 - 1, { signed: true })})</span>}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

const sorts = ["name", "median", "vs_country", "week", "sellers"] as const;
type Sort = (typeof sorts)[number];
const PAGE = 25;

/** A signed share with an arrow: cheap (primary) below zero, dear (warm) above. Never colour alone. */
export function Signed({ value, className }: { value?: number; className?: string }) {
  const fmt = useFormat();
  if (value == null) return <span className="text-muted-foreground">—</span>;
  const flat = Math.abs(value) < 0.005;
  const Icon = value < 0 ? ArrowDownRight : ArrowUpRight;
  return (
    <span className={cn("num inline-flex items-center gap-0.5 whitespace-nowrap", flat ? "text-muted-foreground" : value < 0 ? "text-primary" : "text-warm-foreground", className)}>
      {!flat && <Icon className="size-3.5" />}
      {fmt.percent(value, { signed: true })}
    </span>
  );
}

/** Every product with enough prices in the province: search, category filter and sorting. */
export function RegionProducts({
  products,
  categories,
  provinceId,
}: {
  products: RegionProduct[];
  categories: { slug: string; name: string }[];
  provinceId: number;
}) {
  const t = useTranslations("stats");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const fmt = useFormat();
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("");
  const [sort, setSort] = useState<Sort>("name");
  const [all, setAll] = useState(false);

  const rows = useMemo(() => {
    const q = norm(query.trim());
    const list = products
      .map((p) => ({ ...p, label: names.product(p.slug, p.name) }))
      .filter((p) => (!category || p.category === category) && (!q || norm(p.label).includes(q) || norm(p.name).includes(q)));
    const key = (p: RegionProduct) => (sort === "median" ? p.median : sort === "sellers" ? p.sellers : sort === "vs_country" ? (p.vs_country ?? 0) : (p.week ?? 0));
    return sort === "name" ? list.sort((a, b) => a.label.localeCompare(b.label)) : list.sort((a, b) => key(b) - key(a));
  }, [products, names, query, category, sort]);
  const present = categories.filter((c) => products.some((p) => p.category === c.slug));
  const shown = all ? rows : rows.slice(0, PAGE);

  return (
    <div>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <label className="flex h-10 flex-1 items-center gap-2 rounded-full border bg-card px-4 focus-within:ring-3 focus-within:ring-ring/40">
          <Search className="size-4 shrink-0 text-muted-foreground" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("region.searchProduct")}
            aria-label={t("region.searchProduct")}
            className="w-full min-w-0 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
          />
        </label>
        <div className="flex gap-2">
          <DropdownMenu>
            <DropdownMenuTrigger className="flex h-10 flex-1 items-center justify-center gap-2 rounded-full border bg-card px-4 text-sm transition-colors hover:bg-muted aria-expanded:bg-muted sm:flex-none">
              <span className="max-w-[9rem] truncate font-medium">{category ? names.category(category, present.find((c) => c.slug === category)?.name ?? category) : t("region.allCategories")}</span>
              <ChevronDown className="size-4 text-muted-foreground" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-52">
              <DropdownMenuRadioGroup value={category} onValueChange={(v: string) => setCategory(v)}>
                <DropdownMenuRadioItem value="">{t("region.allCategories")}</DropdownMenuRadioItem>
                {present.map((c) => (
                  <DropdownMenuRadioItem key={c.slug} value={c.slug}>
                    {names.category(c.slug, c.name)}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>
          <DropdownMenu>
            <DropdownMenuTrigger className="flex h-10 flex-1 items-center justify-center gap-2 rounded-full border bg-card px-4 text-sm transition-colors hover:bg-muted aria-expanded:bg-muted sm:flex-none">
              <ArrowUpDown className="size-4 text-muted-foreground" />
              <span className="font-medium">{t(`region.sort.${sort}`)}</span>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuRadioGroup value={sort} onValueChange={(v: string) => setSort(v as Sort)}>
                {sorts.map((s) => (
                  <DropdownMenuRadioItem key={s} value={s}>
                    {t(`region.sort.${s}`)}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {rows.length === 0 ? (
        <p className="py-10 text-center text-sm text-muted-foreground">{tc("notFound")}</p>
      ) : (
        <div className="relative mt-4 overflow-x-auto overscroll-x-contain rounded-2xl border">
          <table className="w-full border-separate border-spacing-0 text-sm">
            <thead>
              <tr className="text-xs text-muted-foreground">
                <th scope="col" className="sticky left-0 z-10 border-b bg-card px-4 py-2.5 text-left font-medium">{t("region.col.product")}</th>
                <th scope="col" className="border-b px-3 py-2.5 text-right font-medium">{t("region.col.median")}</th>
                <th scope="col" className="border-b px-3 py-2.5 text-right font-medium">{t("region.col.vsCountry")}</th>
                <th scope="col" className="border-b px-3 py-2.5 text-right font-medium">{t("region.col.week")}</th>
                <th scope="col" className="border-b px-3 py-2.5 text-left font-medium">{t("region.col.zone")}</th>
                <th scope="col" className="border-b px-4 py-2.5 text-right font-medium">{t("region.col.sellers")}</th>
              </tr>
            </thead>
            <tbody>
              {shown.map((p) => (
                <tr
                  key={p.id}
                  className="group cursor-pointer transition-colors hover:bg-muted"
                  onClick={(e) => {
                    if (!(e.target as HTMLElement).closest("a")) router.push(`/mahsulot/${p.slug}/tahlil?region=${provinceId}`);
                  }}
                >
                  <th scope="row" className="sticky left-0 z-10 border-b bg-card px-4 py-2.5 text-left font-normal group-hover:bg-muted">
                    <Link
                      href={`/mahsulot/${p.slug}/tahlil?region=${provinceId}`}
                      className="block max-w-44 truncate rounded-sm font-medium outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                    >
                      {p.label}
                    </Link>
                  </th>
                  <td className="border-b px-3 py-2.5 text-right whitespace-nowrap">
                    <span className="num font-medium">{formatSom(p.median)}</span>{" "}
                    <span className="text-xs text-muted-foreground">{tc("perUnit", { unit: fmt.unit(p.unit) })}</span>
                  </td>
                  <td className="border-b px-3 py-2.5 text-right">
                    <Signed value={p.vs_country} />
                  </td>
                  <td className="border-b px-3 py-2.5 text-right">
                    <Signed value={p.week} />
                  </td>
                  <td className="border-b px-3 py-2">
                    <ZoneBadge zone={p.zone} className="whitespace-nowrap" />
                  </td>
                  <td className="num border-b px-4 py-2.5 text-right">{p.sellers}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {!all && rows.length > PAGE && (
        <Button variant="outline" size="pill" className="mt-4" onClick={() => setAll(true)}>
          {t("region.showAll", { count: rows.length })}
        </Button>
      )}
    </div>
  );
}

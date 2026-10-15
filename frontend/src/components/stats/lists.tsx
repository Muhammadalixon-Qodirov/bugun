"use client";

import { Hourglass } from "lucide-react";
import { useTranslations } from "next-intl";

import { useCatalogNames } from "@/components/catalog-names";
import { BarList } from "@/components/charts/bar-list";
import type { DemandItem, Mover } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

/** A titled card; the stats pages' building block. */
export function Card({ title, hint, className, children }: { title: string; hint?: React.ReactNode; className?: string; children: React.ReactNode }) {
  return (
    <section className={cn("min-w-0 rounded-3xl border bg-card p-5 sm:p-6", className)}>
      <h2 className="font-semibold tracking-tight">{title}</h2>
      {hint && <p className="mt-1 text-sm text-muted-foreground">{hint}</p>}
      <div className="mt-4">{children}</div>
    </section>
  );
}

/** "Collecting data" inside a card: lists that fill as history builds up. */
export function Collecting({ text }: { text?: string }) {
  const t = useTranslations("stats");
  return (
    <div className="flex items-start gap-3 rounded-2xl bg-muted/60 px-4 py-4 text-sm">
      <Hourglass className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
      <div>
        <p className="font-medium">{t("collecting")}</p>
        <p className="mt-0.5 text-muted-foreground">{text ?? t("collectingHint")}</p>
      </div>
    </div>
  );
}

const analyticsHref = (slug: string, region?: number) => `/mahsulot/${slug}/tahlil${region ? `?region=${region}` : ""}`;

/** Week's biggest price moves, up or down, with the signed change written out. */
export function MoverList({ items, dir }: { items: Mover[]; dir: "up" | "down" }) {
  const t = useTranslations("stats");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const fmt = useFormat();
  if (items.length === 0) return <p className="text-sm text-muted-foreground">{t("movers.none")}</p>;
  return (
    <BarList
      rows={items.map((m) => ({
        key: String(m.product.id),
        label: names.product(m.product.slug, m.product.name),
        value: m.week,
        display: fmt.percent(m.week, { signed: true }),
        note: `${formatSom(m.country)} ${tc("perUnit", { unit: fmt.unit(m.product.unit) })}`,
        noteTone: "muted",
        color: dir === "up" ? "var(--viz-dear)" : "var(--viz-cheap)",
        href: analyticsHref(m.product.slug),
      }))}
    />
  );
}

/** Top demand: "most requested" ranks by this week's score, "rising" by growth against a usual week. */
export function DemandList({ items, kind, region }: { items: DemandItem[]; kind: "top" | "rising"; region?: number }) {
  const t = useTranslations("stats");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const fmt = useFormat();
  if (items.length === 0) return <Collecting text={t("demand.collectingHint")} />;
  return (
    <BarList
      rows={items.map((d) => ({
        key: String(d.product.id),
        label: names.product(d.product.slug, d.product.name),
        value: kind === "top" ? d.score : (d.growth ?? 0),
        display: kind === "top" ? t("demand.score", { score: formatSom(d.score) }) : d.growth != null ? fmt.percent(d.growth, { signed: true }) : t("demand.new"),
        note: d.median != null ? `${formatSom(d.median)} ${tc("perUnit", { unit: fmt.unit(d.product.unit) })}` : undefined,
        noteTone: "muted",
        href: analyticsHref(d.product.slug, region),
      }))}
    />
  );
}

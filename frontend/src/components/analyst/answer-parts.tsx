"use client";

import { ChevronDown, Database, Table2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { useCatalogNames } from "@/components/catalog-names";
import { DemoBadge } from "@/components/demo-badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { Source, Table as AnswerTable } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

const day = (d: string) => `${d}T00:00:00Z`;

/** Sources that ran without an error: what the numbers in an answer rest on. */
export function usableSources(sources: Source[]) {
  return sources.filter((s) => !s.error);
}

/**
 * The grey line under an answer: how many prices, over which dates, and a
 * Demo mark when part of the data is demo. Sources that failed are left out.
 */
export function SourceLine({ sources }: { sources: Source[] }) {
  const t = useTranslations("analyst.source");
  const fmt = useFormat();
  const ok = usableSources(sources).filter((s) => s.tool !== "platform_help");
  if (ok.length === 0) return null;
  const observations = ok.reduce((n, s) => n + s.observations, 0);
  const froms = ok.map((s) => s.data_from).filter((d): d is string => !!d).sort();
  const tos = ok.map((s) => s.data_to).filter((d): d is string => !!d).sort();
  const demo = Math.max(0, ...ok.map((s) => s.demo_share));
  return (
    <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground" data-testid="chat-source-line">
      {observations > 0 && <span className="num">{t("basedOn", { count: observations })}</span>}
      {froms.length > 0 && tos.length > 0 && (
        <>
          {observations > 0 && <span aria-hidden>·</span>}
          <span className="num">{fmt.range(day(froms[0]), day(tos[tos.length - 1]))}</span>
        </>
      )}
      <DemoBadge share={demo} compact />
    </p>
  );
}

/** A disclosure under an answer: a small toggle that shows its body when opened. */
function Disclosure({ icon, label, children }: { icon: React.ReactNode; label: string; children: React.ReactNode }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="w-full min-w-0">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs text-muted-foreground transition-colors hover:border-foreground/30 hover:text-foreground"
      >
        {icon}
        {label}
        <ChevronDown className={cn("size-3 transition-transform", open && "rotate-180")} aria-hidden />
      </button>
      {open && <div className="mt-2">{children}</div>}
    </div>
  );
}

/** The functions an answer used, with their parameters, dates and data quality. */
export function SourcesDisclosure({ sources }: { sources: Source[] }) {
  const t = useTranslations("analyst");
  if (sources.length === 0) return null;
  return (
    <Disclosure icon={<Database className="size-3" aria-hidden />} label={t("sourcesTitle")}>
      <ul className="divide-y rounded-2xl border bg-background/60 px-4">
        {sources.map((s, i) => (
          <SourceRow key={i} s={s} />
        ))}
      </ul>
    </Disclosure>
  );
}

function SourceRow({ s }: { s: Source }) {
  const t = useTranslations("analyst.source");
  const tc = useTranslations("common");
  const fmt = useFormat();
  const names = useCatalogNames();
  const tool = `tools.${s.tool}` as Parameters<typeof t>[0];
  const param = (key: string) => {
    const v = s.params[key];
    if (typeof v !== "string" || !v) return null;
    if (key === "product") return names.product(v, v);
    if (key === "region") return names.region(v);
    if (key === "trade") return v === "ulgurji" ? tc("wholesaleLower") : v === "chakana" ? tc("retailLower") : v;
    return v;
  };
  const params = (["product", "region", "trade"] as const).flatMap((k) => {
    const v = param(k);
    return v ? [{ k, v }] : [];
  });
  return (
    <li className="flex flex-col gap-1 py-2.5 text-xs sm:flex-row sm:items-baseline sm:gap-4">
      <span className="shrink-0 font-medium text-foreground sm:w-40">{t.has(tool) ? t(tool) : s.tool}</span>
      <div className="min-w-0 flex-1 text-muted-foreground">
        {params.length > 0 && <p className="text-foreground/80">{params.map((p) => p.v).join(" · ")}</p>}
        <p className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1">
          {s.data_from && s.data_to && <span>{t("period", { period: fmt.range(day(s.data_from), day(s.data_to)) })}</span>}
          <span className="num">{t("observations", { count: s.observations })}</span>
          {s.error ? (
            <span className="rounded-full bg-destructive/5 px-2 py-0.5 text-destructive">{t(`errors.${s.error}`)}</span>
          ) : !s.enough ? (
            <span className="rounded-full bg-warm-soft px-2 py-0.5 text-warm-foreground">{t("notEnough")}</span>
          ) : null}
          <DemoBadge share={s.demo_share} compact />
        </p>
      </div>
    </li>
  );
}

/** The rows behind an answer, behind a "Table" toggle; columns are translated, prices get digit groups. */
export function TableDisclosure({ table }: { table: AnswerTable }) {
  const t = useTranslations("analyst");
  if (table.rows.length === 0) return null;
  return (
    <Disclosure icon={<Table2 className="size-3" aria-hidden />} label={t("tableToggle", { count: table.rows.length })}>
      <AnswerTableView table={table} />
    </Disclosure>
  );
}

function AnswerTableView({ table }: { table: AnswerTable }) {
  const t = useTranslations("analyst");
  const fmt = useFormat();
  const names = useCatalogNames();
  const numeric = table.columns.map((_, c) => table.rows.some((r) => typeof r[c] === "number"));
  const column = (key: string) => {
    const k = `columns.${key}` as Parameters<typeof t>[0];
    return t.has(k) ? t(k) : key;
  };
  const cell = (key: string, v: unknown) => {
    if (v == null || v === "") return "—";
    if (typeof v === "number") {
      if (key === "demo_share") return fmt.percent(v, { digits: 0 });
      return Number.isInteger(v) || Math.abs(v) >= 100 ? formatSom(Math.round(v)) : fmt.decimal(v, 2);
    }
    if (typeof v === "string") {
      if (/^\d{4}-\d{2}-\d{2}$/.test(v)) return fmt.date(day(v));
      if (key === "area" || key === "region" || key === "place") return names.region(v);
      if (key === "unit") return fmt.unit(v);
      return v;
    }
    return String(v);
  };
  return (
    <div className="overflow-x-auto rounded-2xl border bg-background/60" data-testid="chat-table">
      <Table>
        <TableHeader>
          <TableRow>
            {table.columns.map((c, i) => (
              <TableHead key={c} className={cn("whitespace-nowrap", numeric[i] && "text-right")}>
                {column(c)}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {table.rows.map((r, ri) => (
            <TableRow key={ri}>
              {table.columns.map((c, ci) => (
                <TableCell key={c} className={cn("whitespace-nowrap", numeric[ci] && "num text-right")}>
                  {cell(c, r[ci])}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

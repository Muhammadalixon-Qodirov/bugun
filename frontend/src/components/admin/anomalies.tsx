"use client";

import { useMutation } from "@tanstack/react-query";
import { ArrowDownRight, ArrowUpRight, Check, Hourglass, Info, SearchCheck, Undo2 } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { useCatalogNames } from "@/components/catalog-names";
import { DemoBadge } from "@/components/demo-badge";
import { EmptyState } from "@/components/empty-state";
import { useLimits } from "@/components/limits";
import { Segmented } from "@/components/segmented";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { adminDecideAnomalyMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { PriceAnomaly } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

type Status = PriceAnomaly["status"];
type Filter = Status | "all";

/**
 * One viloyat-day of one product and trade type. Both checks may flag the
 * same day; the charts drop the day when any of its flags is confirmed or
 * settled and still open, so the admin decides the day as a whole.
 */
type Group = { key: string; first: PriceAnomaly; flags: PriceAnomaly[]; status: Status; excluded: boolean; demoShare: number };

function groupFlags(items: PriceAnomaly[]): Group[] {
  const groups = new Map<string, PriceAnomaly[]>();
  for (const a of items) {
    const key = `${a.product_id}|${a.trade_type}|${a.province_id}|${a.day}`;
    groups.set(key, [...(groups.get(key) ?? []), a]);
  }
  return [...groups].map(([key, flags]) => ({
    key,
    first: flags[0],
    flags,
    status: flags.some((f) => f.status === "open") ? "open" : flags.some((f) => f.status === "confirmed") ? "confirmed" : "dismissed",
    excluded: flags.some((f) => f.status === "confirmed" || (f.status === "open" && f.settled)),
    demoShare: Math.max(...flags.map((f) => f.demo_share)),
  }));
}

/** Suspicious price days: the rule, a status filter, and confirm or dismiss for each day. */
export function AdminAnomalies({ anomalies }: { anomalies: PriceAnomaly[] }) {
  const t = useTranslations("admin.anomalies");
  const tc = useTranslations("common");
  const groups = groupFlags(anomalies);
  const [filter, setFilter] = useState<Filter>(groups.some((g) => g.status === "open") || groups.length === 0 ? "open" : "all");
  const rows = filter === "all" ? groups : groups.filter((g) => g.status === filter);
  const count = (s: Status) => groups.filter((g) => g.status === s).length;
  // The API returns at most this many flags, newest first.
  const limit = useLimits().admin_anomalies_limit;

  return (
    <div>
      <div className="flex gap-3 rounded-3xl bg-muted/60 p-4 text-sm sm:p-5">
        <Info className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
        <div className="space-y-1">
          <p>{t("rule")}</p>
          <p className="text-muted-foreground">{t("semantics")}</p>
        </div>
      </div>
      {groups.length === 0 ? (
        <EmptyState className="mt-5" icon={<SearchCheck strokeWidth={1.5} />} title={t("emptyTitle")} description={t("emptyDescription")} />
      ) : (
        <>
          <div className="-mx-4 mt-5 overflow-x-auto px-4 scrollbar-none sm:mx-0 sm:px-0">
            <Segmented
              label={t("filterLabel")}
              size="sm"
              className="w-max min-w-full sm:min-w-0 [&>button]:px-3"
              value={filter}
              onChange={setFilter}
              options={[
                ...(["open", "confirmed", "dismissed"] as const).map((s) => ({
                  value: s,
                  label: (
                    <>
                      {t(`filters.${s}`)} <span className="num text-xs opacity-60">{count(s)}</span>
                    </>
                  ),
                })),
                { value: "all", label: tc("all") },
              ]}
            />
          </div>
          <div className="mt-5">
            {rows.length === 0 ? (
              <EmptyState icon={<SearchCheck strokeWidth={1.5} />} title={t("emptyFiltered")} />
            ) : (
              <>
                <AnomalyTable groups={rows} />
                <ul className="space-y-3 md:hidden">
                  {rows.map((g) => (
                    <li key={g.key}>
                      <AnomalyCard g={g} />
                    </li>
                  ))}
                </ul>
              </>
            )}
            {anomalies.length >= limit && <p className="mt-3 text-center text-xs text-muted-foreground">{t("latest", { count: anomalies.length })}</p>}
          </div>
        </>
      )}
    </div>
  );
}

function useAnomalyText() {
  const t = useTranslations("admin.anomalies");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const fmt = useFormat();
  return {
    product: (a: PriceAnomaly) => names.product(a.product_slug, a.product_name),
    province: (a: PriceAnomaly) => names.region(a.province_name),
    trade: (a: PriceAnomaly) => tc(a.trade_type === "ulgurji" ? "wholesaleLower" : "retailLower"),
    day: (a: PriceAnomaly) => fmt.date(a.day),
    z: (a: PriceAnomaly) => `z ${a.z < 0 ? "−" : ""}${fmt.decimal(Math.abs(a.z), 1)}`,
    diff: (a: PriceAnomaly) => (a.reference > 0 ? a.value / a.reference - 1 : 0),
    percent: (share: number) => fmt.percent(share, { signed: true, digits: 0 }),
    layer: (a: PriceAnomaly) => t(`layers.${a.layer}`),
    layerHint: (a: PriceAnomaly) => t(`layersHint.${a.layer}`),
  };
}

/** Confirm or dismiss every flag of the day; a decided day offers the other choice. */
function Decide({ g, className }: { g: Group; className?: string }) {
  const router = useRouter();
  const t = useTranslations("admin.anomalies");
  const errors = useApiErrors();
  const decide = useMutation({ ...adminDecideAnomalyMutation(), onError: (err) => errors.show(err) });
  const run = async (status: "confirmed" | "dismissed") => {
    let ok = true;
    try {
      for (const f of g.flags) {
        await decide.mutateAsync({
          body: { product_id: f.product_id, trade_type: f.trade_type === "chakana" ? "chakana" : "ulgurji", province_id: f.province_id, day: f.day, layer: f.layer, status },
        });
      }
    } catch {
      // Shown by onError; the page still refreshes to show what did go through.
      ok = false;
    }
    if (ok) toast.success(t(status));
    router.refresh();
  };
  const busy = decide.isPending;
  return (
    <div className={cn("flex gap-2", className)}>
      {g.status !== "confirmed" && (
        <Button variant={g.status === "open" ? "solid" : "outline"} size="pill" disabled={busy} title={t("confirmHint")} onClick={() => run("confirmed")}>
          <Check /> {t("confirm")}
        </Button>
      )}
      {g.status !== "dismissed" && (
        <Button variant="outline" size="pill" disabled={busy} title={t("dismissHint")} onClick={() => run("dismissed")}>
          {g.status === "confirmed" ? <Undo2 /> : null} {t("dismiss")}
        </Button>
      )}
    </div>
  );
}

function StatePills({ g }: { g: Group }) {
  const t = useTranslations("admin.anomalies");
  const fmt = useFormat();
  const provisional = g.status === "open" && g.flags.every((f) => !f.settled);
  const decidedAt = g.flags.find((f) => f.decided_at)?.decided_at;
  return (
    <span className="flex flex-wrap items-center gap-1.5">
      <span
        className={cn(
          "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap",
          g.status === "open" ? "bg-warm-soft text-warm-foreground" : g.status === "confirmed" ? "bg-foreground/10 text-foreground" : "bg-muted text-muted-foreground",
        )}
      >
        {t(`status.${g.status}`)}
      </span>
      {provisional && (
        <span title={t("provisionalHint")} className="inline-flex items-center gap-1 rounded-full bg-muted px-2.5 py-0.5 text-xs text-muted-foreground whitespace-nowrap">
          <Hourglass className="size-3" aria-hidden /> {t("provisional")}
        </span>
      )}
      <DemoBadge compact share={g.demoShare} />
      <span className="w-full text-xs text-muted-foreground">
        {t(g.excluded ? "excluded" : "inCharts")}
        {decidedAt && ` · ${t("decidedAt", { when: fmt.dayMonth(decidedAt) })}`}
      </span>
    </span>
  );
}

function Diff({ a, text }: { a: PriceAnomaly; text: ReturnType<typeof useAnomalyText> }) {
  const d = text.diff(a);
  return (
    <span className={cn("inline-flex items-center gap-0.5 text-xs font-medium", d < 0 ? "text-primary" : "text-warm-foreground")}>
      {d < 0 ? <ArrowDownRight className="size-3.5" aria-hidden /> : <ArrowUpRight className="size-3.5" aria-hidden />}
      {text.percent(d)}
    </span>
  );
}

function ProductCell({ a, text }: { a: PriceAnomaly; text: ReturnType<typeof useAnomalyText> }) {
  return (
    <>
      <Link
        href={`/mahsulot/${a.product_slug}/tahlil?${new URLSearchParams({ region: String(a.province_id), ...(a.trade_type === "chakana" ? { trade: "chakana" } : {}) })}`}
        className="font-medium underline-offset-4 hover:underline"
      >
        {text.product(a)}
      </Link>
      <span className="block text-xs text-muted-foreground">
        {text.province(a)} · {text.trade(a)}
      </span>
    </>
  );
}

function AnomalyTable({ groups }: { groups: Group[] }) {
  const t = useTranslations("admin.anomalies.columns");
  const tc = useTranslations("common");
  const text = useAnomalyText();
  return (
    <div className="hidden overflow-hidden rounded-3xl border bg-card md:block">
      <Table>
        <TableHeader>
          <TableRow className="text-xs hover:bg-transparent [&>th]:text-muted-foreground">
            <TableHead className="pl-5">{t("day")}</TableHead>
            <TableHead>{t("product")}</TableHead>
            <TableHead>{t("check")}</TableHead>
            <TableHead className="text-right">{t("value")}</TableHead>
            <TableHead className="text-right">{t("reference")}</TableHead>
            <TableHead>{t("state")}</TableHead>
            <TableHead className="pr-5" />
          </TableRow>
        </TableHeader>
        <TableBody>
          {groups.map((g) => (
            <TableRow key={g.key} className="align-top">
              <TableCell className="py-3 pl-5 whitespace-nowrap text-muted-foreground">{text.day(g.first)}</TableCell>
              <TableCell className="py-3">
                <ProductCell a={g.first} text={text} />
              </TableCell>
              <TableCell className="py-3">
                <ul className="space-y-1.5">
                  {g.flags.map((f) => (
                    <li key={f.layer} title={text.layerHint(f)}>
                      <span className="block text-sm">{text.layer(f)}</span>
                      <span className="num text-xs text-muted-foreground">{text.z(f)}</span>
                    </li>
                  ))}
                </ul>
              </TableCell>
              <TableCell className="py-3 text-right">
                <span className="num font-medium">{formatSom(g.first.value)}</span>{" "}
                <span className="text-xs text-muted-foreground">{tc("som")}</span>
              </TableCell>
              <TableCell className="py-3 text-right">
                <ul className="space-y-1.5">
                  {g.flags.map((f) => (
                    <li key={f.layer}>
                      <span className="num block">{formatSom(f.reference)}</span>
                      <Diff a={f} text={text} />
                    </li>
                  ))}
                </ul>
              </TableCell>
              <TableCell className="max-w-48 py-3 whitespace-normal">
                <StatePills g={g} />
              </TableCell>
              <TableCell className="py-3 pr-5">
                <Decide g={g} className="justify-end" />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function AnomalyCard({ g }: { g: Group }) {
  const tc = useTranslations("common");
  const t = useTranslations("admin.anomalies.columns");
  const text = useAnomalyText();
  return (
    <article className="rounded-3xl border bg-card p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <ProductCell a={g.first} text={text} />
        </div>
        <span className="shrink-0 text-xs text-muted-foreground">{text.day(g.first)}</span>
      </div>
      <p className="mt-3 text-sm">
        <span className="text-muted-foreground">{t("value")}: </span>
        <span className="num text-lg font-semibold">{formatSom(g.first.value)}</span> <span className="text-sm text-muted-foreground">{tc("som")}</span>
      </p>
      <ul className="mt-2 space-y-2">
        {g.flags.map((f) => (
          <li key={f.layer} className="rounded-2xl bg-muted/60 px-3 py-2 text-sm">
            <p className="font-medium">{text.layer(f)}</p>
            <p className="mt-0.5 flex flex-wrap items-center gap-x-3 text-xs text-muted-foreground">
              <span>
                {t("reference")}: <span className="num text-foreground">{formatSom(f.reference)}</span>
              </span>
              <Diff a={f} text={text} />
              <span className="num">{text.z(f)}</span>
            </p>
          </li>
        ))}
      </ul>
      <div className="mt-3 border-t pt-3">
        <StatePills g={g} />
        <Decide g={g} className="mt-3 [&>button]:flex-1" />
      </div>
    </article>
  );
}

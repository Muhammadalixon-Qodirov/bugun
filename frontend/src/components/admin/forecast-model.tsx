"use client";

import { useQuery } from "@tanstack/react-query";
import { BrainCircuit } from "lucide-react";
import { useTranslations } from "next-intl";

import { EmptyState } from "@/components/empty-state";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { adminForecastModelOptions } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { ModelRun } from "@/lib/api/gen/types.gen";
import { problemOf } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

/** The nightly forecast model: its latest training and past-year backtest, per week ahead, and what it leans on. */
export function AdminForecastModel() {
  const t = useTranslations("admin.forecastModel");
  const { data, isPending, error } = useQuery({ ...adminForecastModelOptions(), retry: false });

  if (error) {
    const status = problemOf(error).status;
    return (
      <EmptyState
        icon={<BrainCircuit strokeWidth={1.5} />}
        title={t(status === 404 ? "unavailableTitle" : "failedTitle")}
        description={t(status === 404 ? "unavailable" : "failedText")}
      />
    );
  }

  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-lg font-semibold tracking-tight">{t("title")}</h2>
        <p className="mt-1 max-w-prose text-sm text-muted-foreground">{t("hint")}</p>
      </div>
      {isPending || !data ? (
        <div className="h-40 animate-pulse rounded-3xl bg-muted" aria-busy />
      ) : data.runs.length === 0 ? (
        <EmptyState icon={<BrainCircuit strokeWidth={1.5} />} title={t("emptyTitle")} description={t("empty")} />
      ) : (
        <div className="grid gap-6 lg:grid-cols-2">
          {data.runs.map((run) => (
            <RunCard key={`${run.kind}-${run.started_at}`} run={run} />
          ))}
        </div>
      )}
    </div>
  );
}

function RunCard({ run }: { run: ModelRun }) {
  const t = useTranslations("admin.forecastModel");
  const tDriver = useTranslations("analytics.forecast.driver");
  const fmt = useFormat();
  const groups = Object.entries(run.importance?.groups ?? {}).sort((a, b) => b[1] - a[1]);
  const groupTotal = groups.reduce((s, [, v]) => s + v, 0) || 1;
  const features = run.importance?.features ?? [];
  const featureMax = Math.max(1e-9, ...features.map((f) => f.weight));
  const group = (code: string) => {
    const key = code as Parameters<typeof tDriver>[0];
    return tDriver.has(key) ? tDriver(key) : code;
  };

  return (
    <section className="space-y-5 rounded-3xl border p-5" data-testid={`forecast-run-${run.kind}`}>
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 className="font-semibold tracking-tight">{t(run.kind)}</h3>
        <p className="text-xs text-muted-foreground">{run.finished_at ? t("finished", { date: `${fmt.date(run.finished_at)} ${fmt.time(run.finished_at)}` }) : t("running")}</p>
      </div>
      <p className="flex flex-wrap gap-x-3 gap-y-1 text-sm text-muted-foreground">
        {run.rows != null && <span className="num">{t("rows", { count: formatSom(run.rows) })}</span>}
        {run.weeks != null && <span className="num">{t("weeks", { count: run.weeks })}</span>}
      </p>
      {run.error && <p className="rounded-2xl bg-destructive/5 px-4 py-3 text-sm text-destructive">{t("failed", { error: run.error })}</p>}

      {(run.horizons?.length ?? 0) > 0 && (
        <div className="space-y-2">
          <h4 className="eyebrow text-muted-foreground">{t("horizons")}</h4>
          <div className="overflow-x-auto rounded-2xl border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-4">{t("cols.horizon")}</TableHead>
                  <TableHead className="text-right">{t("cols.trees")}</TableHead>
                  <TableHead className="text-right">{t("cols.train")}</TableHead>
                  <TableHead className="text-right">{t("cols.valid")}</TableHead>
                  <TableHead className="pr-4 text-right">{t("cols.mape")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {run.horizons!.map((h) => (
                  <TableRow key={h.horizon}>
                    <TableCell className="pl-4">{t("horizonValue", { count: h.horizon })}</TableCell>
                    <TableCell className="num text-right">{formatSom(h.trees)}</TableCell>
                    <TableCell className="num text-right">{formatSom(h.train_rows)}</TableCell>
                    <TableCell className="num text-right">{formatSom(h.valid_rows)}</TableCell>
                    <TableCell className="num pr-4 text-right font-medium">{fmt.percent(h.valid_mape, { digits: 1 })}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </div>
      )}

      {groups.length > 0 && (
        <div className="space-y-2">
          <h4 className="eyebrow text-muted-foreground">{t("groups")}</h4>
          <div className="flex h-3 overflow-hidden rounded-full bg-muted" aria-hidden>
            {groups.map(([code, v], i) => (
              <span key={code} className={cn("h-full", palette[i % palette.length])} style={{ width: `${(v / groupTotal) * 100}%` }} />
            ))}
          </div>
          <ul className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
            {groups.map(([code, v], i) => (
              <li key={code} className="inline-flex items-center gap-1.5">
                <span className={cn("size-2 rounded-full", palette[i % palette.length])} aria-hidden />
                {group(code)} <span className="num text-foreground">{fmt.percent(v / groupTotal, { digits: 0 })}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {features.length > 0 && (
        <div className="space-y-2">
          <h4 className="eyebrow text-muted-foreground">{t("features")}</h4>
          <ul className="space-y-1.5">
            {features.slice(0, 10).map((f) => (
              <li key={f.feature} className="grid grid-cols-[minmax(0,11rem)_1fr] items-center gap-3 text-xs">
                <span className="truncate font-mono text-muted-foreground" title={`${f.feature} · ${group(f.group)}`}>
                  {f.feature}
                </span>
                <span className="h-1.5 overflow-hidden rounded-full bg-muted">
                  <span className="block h-full rounded-full bg-primary/70" style={{ width: `${(f.weight / featureMax) * 100}%` }} />
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}

const palette = ["bg-primary", "bg-chart-2", "bg-chart-3", "bg-chart-4", "bg-chart-5", "bg-muted-foreground/60"];

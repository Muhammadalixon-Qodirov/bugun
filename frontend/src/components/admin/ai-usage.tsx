"use client";

import { useQuery } from "@tanstack/react-query";
import { Cpu } from "lucide-react";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { EmptyState } from "@/components/empty-state";
import { Segmented } from "@/components/segmented";
import { Stat } from "@/components/stat";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { adminAiUsageOptions } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { Totals } from "@/lib/api/gen/types.gen";
import { problemOf } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";

const ranges = [7, 30, 90] as const;

/** Calls to the language, speech and vision models: totals, per feature, per model, per day, and failures. */
export function AdminAiUsage() {
  const t = useTranslations("admin.aiUsage");
  const fmt = useFormat();
  const [days, setDays] = useState<(typeof ranges)[number]>(30);
  const { data, isPending, error } = useQuery({ ...adminAiUsageOptions({ query: { days } }), retry: false });

  const feature = (code: string) => {
    const key = `features.${code}` as Parameters<typeof t>[0];
    return t.has(key) ? t(key) : code;
  };
  const usd = (n: number) => `$${n < 1 ? n.toFixed(4) : n.toFixed(2)}`;
  const n = (v: number) => formatSom(Math.round(v));

  if (error) {
    const status = problemOf(error).status;
    return <EmptyState icon={<Cpu strokeWidth={1.5} />} title={t(status === 404 ? "unavailableTitle" : "failedTitle")} description={t(status === 404 ? "unavailable" : "failed")} />;
  }

  const cols = (row: Totals) => [n(row.calls), n(row.errors), n(row.input_tokens), n(row.output_tokens), n(row.audio_seconds), `${n(row.avg_latency_ms)} ms`, usd(row.cost_usd)];
  const head = [t("cols.calls"), t("cols.errors"), t("cols.input"), t("cols.output"), t("cols.audio"), t("cols.latency"), t("cols.cost")];
  const table = (first: string, rows: { key: string; label: string; totals: Totals }[]) => (
    <div className="overflow-x-auto rounded-3xl border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="pl-5">{first}</TableHead>
            {head.map((h) => (
              <TableHead key={h} className="text-right">
                {h}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((r) => (
            <TableRow key={r.key}>
              <TableCell className="pl-5 font-medium">{r.label}</TableCell>
              {cols(r.totals).map((v, i) => (
                <TableCell key={i} className="num text-right">
                  {v}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold tracking-tight">{t("title")}</h2>
          <p className="mt-1 text-sm text-muted-foreground">{data ? t("since", { date: fmt.date(`${data.since}T00:00:00Z`) }) : t("hint")}</p>
        </div>
        <Segmented
          value={String(days)}
          onChange={(v) => setDays(Number(v) as (typeof ranges)[number])}
          options={ranges.map((d) => ({ value: String(d), label: t("days", { count: d }) }))}
          label={t("range")}
        />
      </div>

      {isPending || !data ? (
        <div className="h-40 animate-pulse rounded-3xl bg-muted" aria-busy />
      ) : data.total.calls === 0 ? (
        <EmptyState icon={<Cpu strokeWidth={1.5} />} title={t("emptyTitle")} description={t("empty")} />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <Stat label={t("cols.calls")} value={n(data.total.calls)} hint={t("errorsHint", { count: data.total.errors })} />
            <Stat label={t("tokens")} value={n(data.total.input_tokens + data.total.output_tokens)} hint={t("tokensHint", { input: n(data.total.input_tokens), output: n(data.total.output_tokens) })} />
            <Stat label={t("cols.latency")} value={`${n(data.total.avg_latency_ms)} ms`} hint={t("audioHint", { seconds: n(data.total.audio_seconds) })} />
            <Stat label={t("cols.cost")} value={usd(data.total.cost_usd)} hint={t("costHint")} />
          </div>

          <section className="space-y-3">
            <h3 className="eyebrow text-muted-foreground">{t("byFeature")}</h3>
            {table(t("cols.feature"), data.by_feature.map((f) => ({ key: f.feature, label: feature(f.feature), totals: f })))}
          </section>
          <section className="space-y-3">
            <h3 className="eyebrow text-muted-foreground">{t("byModel")}</h3>
            {table(t("cols.model"), data.by_model.map((m) => ({ key: m.model, label: m.model || "—", totals: m })))}
          </section>
          <section className="space-y-3">
            <h3 className="eyebrow text-muted-foreground">{t("byDay")}</h3>
            {table(t("cols.day"), data.by_day.map((d) => ({ key: d.day, label: fmt.date(`${d.day}T00:00:00Z`), totals: d })))}
          </section>
          <section className="space-y-3">
            <h3 className="eyebrow text-muted-foreground">{t("errorsTitle")}</h3>
            {data.errors.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("noErrors")}</p>
            ) : (
              <ul className="divide-y overflow-hidden rounded-3xl border">
                {data.errors.map((e) => (
                  <li key={`${e.feature}|${e.code}`} className="flex items-baseline gap-4 px-5 py-3 text-sm">
                    <span className="font-medium">{feature(e.feature)}</span>
                    <code className="min-w-0 flex-1 text-muted-foreground">{e.code}</code>
                    <span className="num">{n(e.calls)}</span>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </>
      )}
    </div>
  );
}

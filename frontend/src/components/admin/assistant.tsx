"use client";

import { useQuery } from "@tanstack/react-query";
import { MessagesSquare } from "lucide-react";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { EmptyState } from "@/components/empty-state";
import { Segmented } from "@/components/segmented";
import { Stat } from "@/components/stat";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { adminChatStatsOptions, adminChatUnansweredOptions } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { UnansweredGroup } from "@/lib/api/gen/types.gen";
import { problemOf } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

const ranges = [7, 30, 90] as const;

const reasonTone: Record<UnansweredGroup["reason"], string> = {
  no_data: "bg-warm-soft text-warm-foreground",
  no_tool: "bg-primary/10 text-primary",
  off_topic: "bg-muted text-muted-foreground",
  tool_error: "bg-destructive/5 text-destructive",
  unclear: "bg-muted text-muted-foreground",
};

/** How the chat assistant is doing: answer rate, clarifications, tools used, and what it could not answer. */
export function AdminAssistant() {
  const t = useTranslations("admin.assistant");
  const tTools = useTranslations("analyst.source");
  const fmt = useFormat();
  const [days, setDays] = useState<(typeof ranges)[number]>(7);
  const stats = useQuery({ ...adminChatStatsOptions({ query: { days } }), retry: false });
  const unanswered = useQuery({ ...adminChatUnansweredOptions({ query: { days } }), retry: false });

  const n = (v: number) => formatSom(Math.round(v));
  const share = (part: number, whole: number) => (whole > 0 ? fmt.percent(part / whole, { digits: 0 }) : "—");
  const tool = (code: string) => {
    const key = `tools.${code}` as Parameters<typeof tTools>[0];
    return tTools.has(key) ? tTools(key) : code;
  };

  if (stats.error) {
    const status = problemOf(stats.error).status;
    return <EmptyState icon={<MessagesSquare strokeWidth={1.5} />} title={t(status === 404 ? "unavailableTitle" : "failedTitle")} description={t(status === 404 ? "unavailable" : "failed")} />;
  }

  const s = stats.data;
  const maxCalls = Math.max(1, ...(s?.tools.map((x) => x.calls) ?? [1]));
  const questions = [...(unanswered.data?.questions ?? [])].sort((a, b) => b.asked - a.asked);

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold tracking-tight">{t("title")}</h2>
          <p className="mt-1 text-sm text-muted-foreground">{t("hint")}</p>
        </div>
        <Segmented
          value={String(days)}
          onChange={(v) => setDays(Number(v) as (typeof ranges)[number])}
          options={ranges.map((d) => ({ value: String(d), label: t("days", { count: d }) }))}
          label={t("range")}
        />
      </div>

      {stats.isPending || !s ? (
        <div className="h-40 animate-pulse rounded-3xl bg-muted" aria-busy />
      ) : s.answers === 0 ? (
        <EmptyState icon={<MessagesSquare strokeWidth={1.5} />} title={t("emptyTitle")} description={t("empty")} />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4" data-testid="assistant-stats">
            <Stat label={t("answeredShare")} value={share(s.answered, s.answers)} hint={t("answeredHint", { answered: n(s.answered), answers: n(s.answers) })} />
            <Stat label={t("conversations")} value={n(s.conversations)} hint={t("answersHint", { count: s.answers })} />
            <Stat label={t("clarified")} value={n(s.clarified)} hint={t("clarifiedHint")} />
            <Stat label={t("unanswered")} value={n(s.unanswered)} hint={t("insufficientHint", { count: s.insufficient })} />
            <Stat label={t("byModel")} value={share(s.by_model, s.answers)} hint={t("fallbacksHint", { count: s.fallbacks })} />
            <Stat label={t("latency")} value={fmt.decimal(s.avg_latency_ms / 1000, 1) + " s"} hint={t("latencyHint")} />
          </div>

          {s.tools.length > 0 && (
            <section className="space-y-3">
              <h3 className="eyebrow text-muted-foreground">{t("toolsTitle")}</h3>
              <ul className="space-y-2 rounded-3xl border p-5">
                {s.tools.map((x) => (
                  <li key={x.tool} className="grid grid-cols-[minmax(0,10rem)_1fr_auto] items-center gap-3 text-sm sm:grid-cols-[14rem_1fr_auto]">
                    <span className="truncate">{tool(x.tool)}</span>
                    <span className="h-2 overflow-hidden rounded-full bg-muted">
                      <span className="block h-full rounded-full bg-primary/70" style={{ width: `${(x.calls / maxCalls) * 100}%` }} />
                    </span>
                    <span className="num w-10 text-right text-muted-foreground">{n(x.calls)}</span>
                  </li>
                ))}
              </ul>
            </section>
          )}
        </>
      )}

      <section className="space-y-3">
        <h3 className="eyebrow text-muted-foreground">{t("unansweredTitle")}</h3>
        <p className="max-w-prose text-sm text-muted-foreground">{t("unansweredHint")}</p>
        {unanswered.isPending ? (
          <div className="h-24 animate-pulse rounded-3xl bg-muted" aria-busy />
        ) : questions.length === 0 ? (
          <p className="rounded-3xl border px-5 py-4 text-sm text-muted-foreground">{t("noUnanswered")}</p>
        ) : (
          <div className="overflow-x-auto rounded-3xl border" data-testid="assistant-unanswered">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="pl-5">{t("cols.question")}</TableHead>
                  <TableHead className="text-right">{t("cols.asked")}</TableHead>
                  <TableHead>{t("cols.reason")}</TableHead>
                  <TableHead>{t("cols.topic")}</TableHead>
                  <TableHead className="pr-5">{t("cols.last")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {questions.map((q) => (
                  <TableRow key={`${q.question}|${q.last_at}`}>
                    <TableCell className="max-w-80 pl-5 whitespace-normal">{q.question}</TableCell>
                    <TableCell className="num text-right font-medium">{n(q.asked)}</TableCell>
                    <TableCell>
                      <span className={cn("rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap", reasonTone[q.reason])}>{t(`reasons.${q.reason}`)}</span>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{q.topic || "—"}</TableCell>
                    <TableCell className="pr-5 whitespace-nowrap text-muted-foreground">{fmt.date(q.last_at)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </section>
    </div>
  );
}

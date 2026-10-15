"use client";

import { AlertTriangle, BellRing, CircleCheck, ShieldCheck } from "lucide-react";
import { useTranslations } from "next-intl";

import { EmptyState } from "@/components/empty-state";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { Alert, JobRun, MonitoringOutputBody } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import { cn } from "@/lib/utils";

// A job still running after this long counts as stuck (backend monitor.stuckAfter).
const STUCK_AFTER_MS = 2 * 60 * 60 * 1000;

type JobStatus = "ok" | "running" | "stuck" | "failed" | "overdue";

/** System health: open alerts, those resolved this week, and the last run of every job. */
export function AdminMonitoring({ data, now }: { data: MonitoringOutputBody; now: string }) {
  const t = useTranslations("admin.monitoring");
  const text = useMonitoringText(data.jobs, now);
  const open = data.alerts.filter((a) => !a.resolved_at);
  const resolved = data.alerts.filter((a) => a.resolved_at);
  const lastCheck = data.jobs.find((j) => j.job === "monitoring");

  return (
    <div className="space-y-10">
      <section aria-labelledby="monitoring-alerts">
        <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
          <h2 id="monitoring-alerts" className="text-lg font-semibold tracking-tight">
            {t("alertsTitle")}
          </h2>
          {lastCheck && <p className="text-sm text-muted-foreground">{t("lastCheck", { when: text.ago(lastCheck.started_at) })}</p>}
        </div>
        <p className="mt-1 max-w-2xl text-sm text-muted-foreground">{t("alertsHint")}</p>
        <div className="mt-4">
          {open.length === 0 ? (
            <EmptyState icon={<ShieldCheck strokeWidth={1.5} />} title={t("allClearTitle")} description={t("allClearDescription")} className="py-10" />
          ) : (
            <ul className="space-y-3">
              {open.map((a) => (
                <li key={a.id}>
                  <AlertCard a={a} text={text} />
                </li>
              ))}
            </ul>
          )}
        </div>
        {resolved.length > 0 && (
          <div className="mt-6">
            <h3 className="eyebrow text-muted-foreground">{t("resolvedTitle")}</h3>
            <ul className="mt-3 divide-y overflow-hidden rounded-3xl border">
              {resolved.map((a) => (
                <li key={a.id} className="flex flex-col gap-1 p-4 text-sm text-muted-foreground sm:flex-row sm:items-baseline sm:gap-4 sm:px-5">
                  <span className="flex items-center gap-2 font-medium">
                    <CircleCheck className="size-4 shrink-0 text-primary" aria-hidden />
                    {text.kind(a)}
                  </span>
                  <span className="min-w-0 flex-1">{text.message(a)}</span>
                  <span className="shrink-0 text-xs">{t("resolvedAt", { when: text.when(a.resolved_at!) })}</span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </section>
      <Jobs jobs={data.jobs} alerts={open} text={text} now={now} />
    </div>
  );
}

/** Alert and job wording rebuilt from codes, so it follows the visitor's language. */
function useMonitoringText(jobs: JobRun[], now: string) {
  const t = useTranslations("admin");
  const tcodes = useTranslations("codes");
  const fmt = useFormat();
  const when = (iso: string) => `${fmt.dayMonth(iso)}, ${fmt.time(iso)}`;
  const job = (code: string) => {
    const key = `monitoring.jobs.names.${code}` as Parameters<typeof t>[0];
    return t.has(key) ? t(key) : code;
  };
  const source = (code: string) => {
    const key = `source.${code}` as Parameters<typeof tcodes>[0];
    return tcodes.has(key) ? tcodes(key) : code;
  };
  const count = (n?: number) => formatSom(Math.round(n ?? 0));
  return {
    when,
    job,
    ago: (iso: string) => {
      const minutes = Math.floor((Date.parse(now) - Date.parse(iso)) / 60_000);
      if (minutes < 1) return t("ago.now");
      if (minutes < 60) return t("ago.minutes", { count: minutes });
      if (minutes < 24 * 60) return t("ago.hours", { count: Math.floor(minutes / 60) });
      return fmt.dayAgo(iso, new Date(now));
    },
    kind: (a: Alert) => {
      const key = `monitoring.kinds.${a.kind}` as Parameters<typeof t>[0];
      return t.has(key) ? t(key) : a.kind;
    },
    message: (a: Alert) => {
      const subject = a.subject ?? "";
      const started = jobs.find((j) => j.job === subject)?.started_at;
      const values = {
        source: source(subject),
        job: job(subject),
        value: count(a.value),
        usual: count(a.baseline),
        share: fmt.percent(a.value ?? 0, { digits: 0 }),
        count: Math.round(a.value ?? 0),
        when: started ? when(started) : "—",
      };
      const key = `monitoring.messages.${a.kind}` as Parameters<typeof t>[0];
      // A kind this page does not know yet keeps the API's own (Uzbek) sentence.
      if (!t.has(key)) return a.message;
      return a.kind === "review_share" ? t(key, { ...values, usual: fmt.percent(a.baseline ?? 0, { digits: 0 }) }) : t(key, values);
    },
  };
}

type MonitoringText = ReturnType<typeof useMonitoringText>;

const levelTone: Record<Alert["level"], string> = {
  critical: "bg-destructive/10 text-destructive",
  high: "bg-warm-soft text-warm-foreground",
  medium: "bg-muted text-foreground",
};

function AlertCard({ a, text }: { a: Alert; text: MonitoringText }) {
  const t = useTranslations("admin.monitoring");
  // A failed job's error is technical text: shown as is, under the sentence.
  const error = a.kind === "job_failed" ? a.message.split(": ").slice(1).join(": ") : "";
  return (
    <article className="rounded-3xl border bg-card p-4 sm:p-5">
      <div className="flex flex-wrap items-center gap-2">
        <span className={cn("inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium", levelTone[a.level])}>
          <AlertTriangle className="size-3.5 shrink-0" aria-hidden />
          {t(`levels.${a.level}`)}
        </span>
        <span className="font-semibold">{text.kind(a)}</span>
        {a.subject && <span className="rounded-full bg-muted px-2 py-0.5 font-mono text-xs text-muted-foreground">{a.subject}</span>}
      </div>
      <p className="mt-2 text-sm">{text.message(a)}</p>
      {error && <p className="mt-2 rounded-xl bg-muted px-3 py-2 font-mono text-xs break-words text-muted-foreground">{error}</p>}
      <p className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
        <span>{t("firstSeen", { when: text.when(a.created_at) })}</span>
        <span>{t("seenAt", { when: text.when(a.updated_at) })}</span>
        {a.notified_at && (
          <span className="flex items-center gap-1">
            <BellRing className="size-3.5" aria-hidden /> {t("notified")}
          </span>
        )}
      </p>
    </article>
  );
}

function jobStatus(j: JobRun, alerts: Alert[], now: string): JobStatus {
  const flagged = (kind: string) => alerts.some((a) => a.kind === kind && a.subject === j.job);
  if (!j.finished_at) return flagged("job_stuck") || Date.parse(now) - Date.parse(j.started_at) > STUCK_AFTER_MS ? "stuck" : "running";
  if (j.error) return "failed";
  // How long a job may pause lives with its schedule on the server; the hourly check reports it.
  return flagged("job_overdue") ? "overdue" : "ok";
}

const statusTone: Record<JobStatus, string> = {
  ok: "bg-primary/10 text-primary",
  running: "bg-muted text-foreground",
  stuck: "bg-destructive/10 text-destructive",
  failed: "bg-destructive/10 text-destructive",
  overdue: "bg-warm-soft text-warm-foreground",
};

function StatusPill({ status }: { status: JobStatus }) {
  const t = useTranslations("admin.monitoring.jobs.status");
  return (
    <span className={cn("inline-flex shrink-0 items-center rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap", statusTone[status])}>
      {t(status)}
    </span>
  );
}

function useDuration() {
  const t = useTranslations("admin.monitoring.jobs");
  const fmt = useFormat();
  return (j: JobRun) => {
    if (!j.finished_at) return "—";
    const s = (Date.parse(j.finished_at) - Date.parse(j.started_at)) / 1000;
    if (s < 1) return t("seconds", { value: "<1" });
    if (s < 60) return t("seconds", { value: fmt.decimal(s, s < 10 ? 1 : 0) });
    if (s < 3600) return t("minutes", { value: Math.round(s / 60) });
    return t("hours", { value: fmt.decimal(s / 3600, 1) });
  };
}

function Jobs({ jobs, alerts, text, now }: { jobs: JobRun[]; alerts: Alert[]; text: MonitoringText; now: string }) {
  const t = useTranslations("admin.monitoring.jobs");
  const duration = useDuration();
  return (
    <section aria-labelledby="monitoring-jobs">
      <h2 id="monitoring-jobs" className="text-lg font-semibold tracking-tight">
        {t("title")}
      </h2>
      <p className="mt-1 max-w-2xl text-sm text-muted-foreground">{t("hint")}</p>
      {jobs.length === 0 ? (
        <p className="mt-4 rounded-3xl border border-dashed px-6 py-10 text-center text-sm text-muted-foreground">{t("empty")}</p>
      ) : (
        <>
          <div className="mt-4 hidden overflow-hidden rounded-3xl border bg-card md:block">
            <Table>
              <TableHeader>
                <TableRow className="text-xs hover:bg-transparent [&>th]:text-muted-foreground">
                  <TableHead className="pl-5">{t("columns.job")}</TableHead>
                  <TableHead>{t("columns.started")}</TableHead>
                  <TableHead className="text-right">{t("columns.duration")}</TableHead>
                  <TableHead className="pr-5">{t("columns.status")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {jobs.map((j) => (
                  <TableRow key={j.job} className="align-top">
                    <TableCell className="py-3 pl-5">
                      <span className="font-medium">{text.job(j.job)}</span>
                      <span className="block font-mono text-xs text-muted-foreground">{j.job}</span>
                    </TableCell>
                    <TableCell className="py-3">
                      <span className="num">{text.when(j.started_at)}</span>
                      <span className="block text-xs text-muted-foreground">{text.ago(j.started_at)}</span>
                    </TableCell>
                    <TableCell className="num py-3 text-right text-muted-foreground">{duration(j)}</TableCell>
                    <TableCell className="max-w-sm py-3 pr-5 whitespace-normal">
                      <StatusPill status={jobStatus(j, alerts, now)} />
                      {j.error && <span className="mt-1 block font-mono text-xs break-words text-muted-foreground">{j.error}</span>}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <ul className="mt-4 divide-y overflow-hidden rounded-3xl border bg-card md:hidden">
            {jobs.map((j) => (
              <li key={j.job} className="p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="font-medium">{text.job(j.job)}</p>
                    <p className="font-mono text-xs text-muted-foreground">{j.job}</p>
                  </div>
                  <StatusPill status={jobStatus(j, alerts, now)} />
                </div>
                <p className="mt-2 text-xs text-muted-foreground">
                  <span className="num">{text.when(j.started_at)}</span> · {text.ago(j.started_at)} · <span className="num">{duration(j)}</span>
                </p>
                {j.error && <p className="mt-2 rounded-xl bg-muted px-3 py-2 font-mono text-xs break-words text-muted-foreground">{j.error}</p>}
              </li>
            ))}
          </ul>
        </>
      )}
    </section>
  );
}

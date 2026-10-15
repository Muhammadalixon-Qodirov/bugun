"use client";

import { useMutation } from "@tanstack/react-query";
import { AlertTriangle, ArrowDownRight, ArrowUpRight, Check, ClipboardCheck, MapPin, Package, UserCheck, X } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import { AdminAiUsage } from "@/components/admin/ai-usage";
import { AdminAssistant } from "@/components/admin/assistant";
import { AdminForecastModel } from "@/components/admin/forecast-model";
import { AdminAnomalies } from "@/components/admin/anomalies";
import { AdminDeals } from "@/components/admin/deals";
import { AdminEvents } from "@/components/admin/events";
import { AdminMonitoring } from "@/components/admin/monitoring";
import { AdminRatings } from "@/components/admin/ratings";
import { AdminSuspicious } from "@/components/admin/suspicious";
import { useCatalogNames } from "@/components/catalog-names";
import { categoryIcons } from "@/components/category-icon";
import { EmptyState } from "@/components/empty-state";
import { ease } from "@/components/motion";
import { Photo } from "@/components/photo";
import { Button } from "@/components/ui/button";
import { adminDecideListingMutation, adminDecideRoleMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { ReviewItem, RoleRequest } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";
import type { AdminBoardData } from "@/lib/server/admin";
import { cn } from "@/lib/utils";

const adminTabs = ["listings", "roles", "deals", "ratings", "events", "monitoring", "anomalies", "sellers", "assistant", "forecast", "ai"] as const;
type AdminTab = (typeof adminTabs)[number];

const isAdminTab = (v: string | undefined): v is AdminTab => adminTabs.some((k) => k === v);

/**
 * The admin's sections: the review and role queues, deals, held ratings, the
 * events journal, system health, suspicious price days and unusual sellers.
 */
export function AdminBoard({ data, initialTab }: { data: Omit<AdminBoardData, "summary">; initialTab?: string }) {
  const t = useTranslations("admin");
  // Without a tab in the address, open the first queue that has work in it.
  const [tab, setTab] = useState<AdminTab>(
    () => (isAdminTab(initialTab) ? initialTab : data.listings.length > 0 ? "listings" : data.requests.length > 0 ? "roles" : data.ratings.length > 0 ? "ratings" : "listings"),
  );
  const counts: Partial<Record<AdminTab, number>> = {
    listings: data.listings.length,
    roles: data.requests.length,
    deals: data.deals.length,
    ratings: data.ratings.length,
    events: data.events.length,
    monitoring: data.monitoring.alerts.filter((a) => !a.resolved_at).length,
    // Days still waiting for a decision; one day can carry two flags.
    anomalies: new Set(data.anomalies.filter((a) => a.status === "open").map((a) => `${a.product_id}|${a.trade_type}|${a.province_id}|${a.day}`)).size,
    sellers: data.suspicious.sellers.length,
  };
  // On phones the tab row scrolls sideways: keep the chosen tab in view.
  const list = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = list.current?.querySelector<HTMLElement>('[aria-selected="true"]');
    if (list.current && el) list.current.scrollLeft = el.offsetLeft - 16;
  }, [tab]);
  const select = (next: AdminTab) => {
    setTab(next);
    // Keeps the section across reloads and makes it linkable, without a server round trip.
    const url = new URL(window.location.href);
    url.searchParams.set("tab", next);
    window.history.replaceState(null, "", url);
  };

  return (
    <div>
      <div ref={list} className="relative -mx-4 flex gap-1 overflow-x-auto px-4 scrollbar-none sm:mx-0 sm:px-0" role="tablist" aria-label={t("queue")}>
        {adminTabs.map((key) => (
          <button
            key={key}
            type="button"
            role="tab"
            id={`admin-tab-${key}`}
            aria-selected={tab === key}
            aria-controls="admin-panel"
            onClick={() => select(key)}
            className={cn(
              "relative isolate flex h-10 shrink-0 items-center gap-1.5 rounded-full px-4 text-sm transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
              tab === key ? "font-medium text-background" : "text-muted-foreground hover:text-foreground",
            )}
          >
            {tab === key && (
              <m.span layoutId="admin-tab" className="absolute inset-0 -z-10 rounded-full bg-foreground" transition={{ type: "spring", stiffness: 500, damping: 40 }} />
            )}
            {t(`tabs.${key}`)}
            {counts[key] != null && <span className={cn("num text-xs", tab === key ? "text-background/70" : "text-muted-foreground/70")}>{counts[key]}</span>}
          </button>
        ))}
      </div>
      <div id="admin-panel" role="tabpanel" aria-labelledby={`admin-tab-${tab}`} className="mt-6">
        {tab === "listings" && <ReviewQueue items={data.listings} />}
        {tab === "roles" && <RoleQueue items={data.requests} />}
        {tab === "deals" && <AdminDeals deals={data.deals} />}
        {tab === "ratings" && <AdminRatings ratings={data.ratings} />}
        {tab === "events" && <AdminEvents events={data.events} catalog={data.catalog} />}
        {tab === "monitoring" && <AdminMonitoring data={data.monitoring} now={data.now} />}
        {tab === "anomalies" && <AdminAnomalies anomalies={data.anomalies} />}
        {tab === "sellers" && <AdminSuspicious data={data.suspicious} />}
        {tab === "assistant" && <AdminAssistant />}
        {tab === "forecast" && <AdminForecastModel />}
        {tab === "ai" && <AdminAiUsage />}
      </div>
    </div>
  );
}

const row = {
  initial: { opacity: 0 },
  animate: { opacity: 1 },
  exit: { opacity: 0, height: 0, marginTop: 0, transition: { duration: 0.25, ease } },
};

function ReviewQueue({ items }: { items: ReviewItem[] }) {
  const t = useTranslations("admin.review");
  if (items.length === 0) {
    return (
      <EmptyState
        icon={<ClipboardCheck strokeWidth={1.5} />}
        title={t("emptyTitle")}
        description={t("emptyDescription")}
      />
    );
  }
  return (
    <ul className="space-y-3">
      <AnimatePresence initial={false}>
        {items.map((it) => (
          <m.li key={it.id} {...row} className="overflow-hidden">
            <ReviewCard it={it} />
          </m.li>
        ))}
      </AnimatePresence>
    </ul>
  );
}

function ReviewCard({ it }: { it: ReviewItem }) {
  const router = useRouter();
  const t = useTranslations("admin");
  const tc = useTranslations("common");
  const tcodes = useTranslations("codes");
  const fmt = useFormat();
  const names = useCatalogNames();
  const errors = useApiErrors();
  const [rejecting, setRejecting] = useState(false);
  const [note, setNote] = useState("");
  const decide = useMutation({
    ...adminDecideListingMutation(),
    onSuccess: (_, v) => {
      toast.success(v.body.approve ? t("review.approved") : t("review.rejected"));
      router.refresh();
    },
    onError: (err) => errors.show(err),
  });
  const Icon = categoryIcons[it.category] ?? Package;
  const unit = fmt.unit(it.unit);
  const name = names.product(it.product_slug, it.product_name);
  const d = it.diff_percent;

  return (
    <article className="flex flex-col gap-5 rounded-3xl border bg-card p-4 sm:flex-row sm:p-5">
      <div className="grid size-24 shrink-0 place-items-center overflow-hidden rounded-2xl bg-muted">
        {it.cover_key ? <Photo thumb k={it.cover_key} alt={name} sizes="96px" className="size-full" /> : <Icon strokeWidth={1.25} className="size-8 text-muted-foreground/60" />}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <Link href={`/mahsulot/${it.product_slug}`} className="font-semibold underline-offset-4 hover:underline">
            {name}
            {it.variety && <span className="font-normal text-muted-foreground"> · {it.variety}</span>}
          </Link>
          <span className="text-xs text-muted-foreground capitalize">
            {tc(it.trade_type === "ulgurji" ? "wholesaleLower" : "retailLower")} · #{it.id} · {fmt.dayAgo(it.updated_at)}
          </span>
        </div>
        <div className="mt-2 flex flex-wrap items-baseline gap-x-4 gap-y-1">
          <p className="num text-2xl font-semibold">
            {formatSom(it.price_som)}
            <span className="ml-1 font-sans text-sm font-normal text-muted-foreground">{tc("perUnit", { unit })}</span>
          </p>
          {d != null && it.median ? (
            <p className={cn("flex items-center gap-1 text-sm font-medium", d < 0 ? "text-primary" : "text-warm-foreground")}>
              {d < 0 ? <ArrowDownRight className="size-4" /> : <ArrowUpRight className="size-4" />}
              {t(d < 0 ? "review.below" : "review.above", { percent: Math.abs(d) })}
              <span className="font-normal text-muted-foreground">
                {t.rich("review.median", { median: formatSom(it.median), count: it.offers, num: (c) => <span className="num">{c}</span> })}
              </span>
            </p>
          ) : (
            <p className="text-sm text-muted-foreground">
              {t(it.reason === "far_from_reference" ? "review.noCompareReference" : "review.noCompare")}
            </p>
          )}
        </div>
        {it.reason && (
          <div className="mt-2.5">
            <p
              title={tcodes(`reviewReason.${it.reason}.hint`)}
              className="inline-flex items-center gap-1.5 rounded-full bg-warm-soft px-2.5 py-1 text-xs font-medium text-warm-foreground"
            >
              <AlertTriangle className="size-3.5 shrink-0" aria-hidden />
              {tcodes(`reviewReason.${it.reason}.title`)}
            </p>
            {/* Phones have no hover, so the explanation sits under the badge. */}
            <p className="mt-1 text-xs text-muted-foreground sm:hidden">{tcodes(`reviewReason.${it.reason}.hint`)}</p>
          </div>
        )}
        <p className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted-foreground">
          <span className="font-medium text-foreground">{it.seller_name}</span>
          <span className="flex items-center gap-1">
            <MapPin className="size-3.5" /> {[it.region_name, it.province_name]
              .filter((n): n is string => Boolean(n))
              .map(names.region)
              .join(", ")}
          </span>
        </p>
        {it.description && <p className="mt-2 line-clamp-2 text-sm text-muted-foreground">{it.description}</p>}

        <AnimatePresence initial={false}>
          {rejecting && (
            <m.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: "auto" }}
              exit={{ opacity: 0, height: 0 }}
              transition={{ duration: 0.2, ease }}
              className="overflow-hidden"
            >
              <label className="mt-4 block text-sm font-medium" htmlFor={`note-${it.id}`}>
                {t("review.reason")} <span className="font-normal text-muted-foreground">{t("review.reasonHint")}</span>
              </label>
              <textarea
                id={`note-${it.id}`}
                value={note}
                onChange={(e) => setNote(e.target.value)}
                maxLength={300}
                rows={2}
                placeholder={t("review.reasonPlaceholder")}
                className="mt-2 w-full resize-none rounded-xl border bg-background px-3 py-2 text-sm outline-none focus:border-foreground/40"
              />
            </m.div>
          )}
        </AnimatePresence>
      </div>
      <div className="flex shrink-0 gap-2 sm:flex-col">
        {rejecting ? (
          <>
            <Button
              variant="destructive"
              size="pill"
              disabled={decide.isPending}
              onClick={() => decide.mutate({ path: { id: it.id }, body: { approve: false, note: note.trim() || undefined } })}
            >
              <X /> {t("reject")}
            </Button>
            <Button variant="ghost" size="pill" onClick={() => setRejecting(false)}>
              {tc("cancel")}
            </Button>
          </>
        ) : (
          <>
            <Button variant="solid" size="pill" disabled={decide.isPending} onClick={() => decide.mutate({ path: { id: it.id }, body: { approve: true } })}>
              <Check /> {t("approve")}
            </Button>
            <Button variant="outline" size="pill" disabled={decide.isPending} onClick={() => setRejecting(true)}>
              {t("rejectEllipsis")}
            </Button>
          </>
        )}
      </div>
    </article>
  );
}

function RoleQueue({ items }: { items: RoleRequest[] }) {
  const t = useTranslations("admin.roles");
  if (items.length === 0) {
    return (
      <EmptyState
        icon={<UserCheck strokeWidth={1.5} />}
        title={t("emptyTitle")}
        description={t("emptyDescription")}
      />
    );
  }
  return (
    <ul className="divide-y overflow-hidden rounded-3xl border bg-card">
      <AnimatePresence initial={false}>
        {items.map((r) => (
          <m.li key={r.id} {...row} className="overflow-hidden">
            <RoleRow r={r} />
          </m.li>
        ))}
      </AnimatePresence>
    </ul>
  );
}

function RoleRow({ r }: { r: RoleRequest }) {
  const router = useRouter();
  const t = useTranslations("admin");
  const fmt = useFormat();
  const names = useCatalogNames();
  const errors = useApiErrors();
  const decide = useMutation({
    ...adminDecideRoleMutation(),
    onSuccess: (_, v) => {
      toast.success(t(v.body.approve ? "roles.approved" : "roles.rejected", { name: r.display_name }));
      router.refresh();
    },
    onError: (err) => errors.show(err),
  });
  const initials = r.display_name
    .split(/\s+/)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() ?? "")
    .join("");
  return (
    <div className="flex flex-col gap-4 p-4 sm:flex-row sm:items-center sm:px-5">
      <span className="grid size-11 shrink-0 place-items-center rounded-full bg-muted text-sm font-semibold">{initials}</span>
      <div className="min-w-0 flex-1">
        <p className="font-medium">
          {r.display_name}
          {r.telegram_username && <span className="ml-2 text-sm font-normal text-muted-foreground">@{r.telegram_username}</span>}
        </p>
        <p className="mt-0.5 flex flex-wrap items-center gap-x-3 text-sm text-muted-foreground">
          <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-foreground">{fmt.role(r.role)}</span>
          {[r.market_name && names.market(r.market_name), r.region_name && names.region(r.region_name), r.province_name && names.region(r.province_name)]
            .filter(Boolean)
            .join(", ")}
          <span>· {fmt.date(r.requested_at)}</span>
        </p>
      </div>
      <div className="flex gap-2">
        <Button variant="solid" size="pill" disabled={decide.isPending} onClick={() => decide.mutate({ path: { id: r.id }, body: { approve: true } })}>
          <Check /> {t("approve")}
        </Button>
        <Button
          variant="outline"
          size="pill"
          disabled={decide.isPending}
          title={t("roles.rejectTitle")}
          onClick={() => decide.mutate({ path: { id: r.id }, body: { approve: false } })}
        >
          {t("reject")}
        </Button>
      </div>
    </div>
  );
}

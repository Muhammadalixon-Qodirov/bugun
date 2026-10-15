"use client";

import { BarChart3, ChevronRight, FileDown, MapPinned } from "lucide-react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { useCatalogNames } from "@/components/catalog-names";
import { buttonVariants } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { apiFileUrl } from "@/lib/api/download";

type Province = { id: number; name: string };

/**
 * The weekly PDF report of a province. Analysts and admins pick any province;
 * a market administration gets its own one only (`provinces` has one item).
 */
export function WeeklyReport({ provinces, initial }: { provinces: Province[]; initial?: number }) {
  const t = useTranslations("cabinet.report");
  const names = useCatalogNames();
  const [region, setRegion] = useState(String(initial ?? provinces[0]?.id ?? ""));
  const fixed = provinces.length === 1;
  const current = provinces.find((p) => String(p.id) === region);

  return (
    <section aria-labelledby="weekly-report" className="rounded-3xl border bg-card p-5 sm:p-6">
      <div className="flex items-start gap-4">
        <span className="grid size-12 shrink-0 place-items-center rounded-2xl bg-muted">
          <FileDown strokeWidth={1.5} className="size-5" />
        </span>
        <div className="min-w-0">
          <h2 id="weekly-report" className="font-semibold tracking-tight">
            {t("title")}
          </h2>
          <p className="mt-0.5 text-sm leading-relaxed text-muted-foreground">{t("text")}</p>
        </div>
      </div>

      <div className="mt-5 flex flex-col gap-3 sm:flex-row sm:items-end">
        <div className="min-w-0 flex-1">
          {fixed ? (
            <>
              <p className="text-sm font-medium">{t("province")}</p>
              <p className="mt-1.5 flex h-11 items-center rounded-xl border bg-muted/50 px-3 text-sm">{current ? names.region(current.name) : "—"}</p>
            </>
          ) : (
            <>
              <label htmlFor="report-province" className="text-sm font-medium">
                {t("province")}
              </label>
              <Select
                items={provinces.map((p) => ({ value: String(p.id), label: names.region(p.name) }))}
                value={region || null}
                onValueChange={(v) => setRegion(v ?? "")}
              >
                <SelectTrigger id="report-province" className="mt-1.5 h-11! w-full rounded-xl bg-card">
                  <SelectValue placeholder={t("choose")} />
                </SelectTrigger>
                <SelectContent>
                  {provinces.map((p) => (
                    <SelectItem key={p.id} value={String(p.id)}>
                      {names.region(p.name)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </>
          )}
        </div>
        {region ? (
          <a href={apiFileUrl("/v1/reports/weekly", { region })} download className={buttonVariants({ variant: "solid", size: "pill", className: "h-11 px-6" })}>
            <FileDown /> {t("download")}
          </a>
        ) : (
          <span aria-disabled className={buttonVariants({ variant: "solid", size: "pill", className: "pointer-events-none h-11 px-6 opacity-50" })}>
            <FileDown /> {t("download")}
          </span>
        )}
      </div>
      <p className="mt-3 text-xs text-muted-foreground">{t("note")}</p>

      <div className="mt-5 grid gap-2 border-t pt-5 sm:grid-cols-2">
        <Link href="/statistika" className="group flex min-h-12 items-center gap-3 rounded-2xl bg-muted/60 px-4 py-3 text-sm transition-colors hover:bg-muted">
          <BarChart3 className="size-4 shrink-0 text-muted-foreground" />
          <span className="flex-1 font-medium">{t("statistics")}</span>
          <ChevronRight className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
        </Link>
        {current && (
          <Link
            href={`/hudud/${current.id}`}
            className="group flex min-h-12 items-center gap-3 rounded-2xl bg-muted/60 px-4 py-3 text-sm transition-colors hover:bg-muted"
          >
            <MapPinned className="size-4 shrink-0 text-muted-foreground" />
            <span className="min-w-0 flex-1 truncate font-medium">{t("region", { name: names.region(current.name) })}</span>
            <ChevronRight className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
          </Link>
        )}
      </div>
    </section>
  );
}

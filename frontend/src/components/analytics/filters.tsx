"use client";

import { useTranslations } from "next-intl";

import { useCatalogNames } from "@/components/catalog-names";
import { useMarketNav, useMarketParams } from "@/components/market-nav";
import { Segmented } from "@/components/segmented";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { type Period, periods } from "@/lib/analytics";

/**
 * The one filter row above the analytics: province, trade and period. Each
 * change swaps the URL inside the market transition, so the controls flip at
 * once and the charts below stay on screen, dimmed, until new data arrives.
 */
export function AnalyticsFilters({
  basePath,
  provinces,
  province,
}: {
  basePath: string;
  provinces: { id: number; name: string }[];
  /** The province the data is for (a district in the URL resolves to its province). */
  province: number;
}) {
  const t = useTranslations("analytics");
  const tc = useTranslations("common");
  const tcodes = useTranslations("codes");
  const tm = useTranslations("market");
  const names = useCatalogNames();
  const { go } = useMarketNav();
  const params = useMarketParams();

  const trade = params.get("trade") === "chakana" ? "chakana" : "ulgurji";
  const period = (periods as readonly string[]).includes(params.get("period") ?? "") ? (params.get("period") as Period) : "1y";

  const set = (key: string, value: string, fallback: string) => {
    const next = new URLSearchParams(params);
    if (value === fallback) next.delete(key);
    else next.set(key, value);
    const qs = next.toString();
    go(qs ? `${basePath}?${qs}` : basePath);
  };

  const items = [{ value: "0", label: tc("wholeCountry") }, ...provinces.map((p) => ({ value: String(p.id), label: names.region(p.name) }))];
  const asked = params.get("region") ?? "0";
  const region = items.some((i) => i.value === asked) ? asked : String(province);

  return (
    <div role="group" aria-label={t("filters.label")} className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
      <Select items={items} value={region} onValueChange={(v) => set("region", v ?? "0", "0")}>
        <SelectTrigger aria-label={t("filters.region")} className="h-11! w-full rounded-full bg-card px-4 sm:w-64">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {items.map((i) => (
            <SelectItem key={i.value} value={i.value}>
              {i.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Segmented
        label={t("filters.period")}
        className="w-full sm:w-80"
        value={period}
        onChange={(v) => set("period", v, "1y")}
        options={periods.map((p) => ({ value: p, label: tcodes(`period.${p}`) }))}
      />
      <Segmented
        label={tm("tradeType")}
        className="w-full sm:w-56"
        value={trade}
        onChange={(v) => set("trade", v, "ulgurji")}
        options={[
          { value: "ulgurji", label: tc("wholesale") },
          { value: "chakana", label: tc("retail") },
        ]}
      />
    </div>
  );
}

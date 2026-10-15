import "server-only";

import { cache } from "react";

import { priceHeatmap, regionOverview, topDemand } from "@/lib/api/gen/sdk.gen";
import { withCookies } from "@/lib/server-api";
import type { Category, Trade } from "@/lib/stats";

/** Province overview (a district id gives its province), or null when the region does not exist. */
export const getRegionOverview = cache(async function getRegionOverview(id: number, trade: Trade) {
  const { data, response } = await regionOverview({ ...(await withCookies()), path: { id }, query: { trade } });
  if (response?.status === 404 || response?.status === 422) return null;
  if (!data) throw new Error("Could not load the region overview");
  return data;
});

/** Province × product medians. No category means every category (the API rejects an empty one). */
export async function getHeatmap(trade: Trade, category?: Category) {
  const { data } = await priceHeatmap({ ...(await withCookies()), query: category ? { trade, category } : { trade } });
  if (!data) throw new Error("Could not load the heatmap");
  return data;
}

/** Most requested and fastest-rising products, for the country (0) or a province. */
export async function getTopDemand(region = 0) {
  const { data } = await topDemand({ ...(await withCookies()), query: region ? { region } : {} });
  return data ?? { top: [], rising: [] };
}

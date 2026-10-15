import "server-only";

import { cache } from "react";

import { productAdviceText, productAnalytics, productPressure, productReferences } from "@/lib/api/gen/sdk.gen";
import type { ProductAnalyticsData } from "@/lib/api/gen/types.gen";
import { withCookies } from "@/lib/server-api";

export type AnalyticsQuery = NonNullable<ProductAnalyticsData["query"]>;

/** The product analytics page's data, or null for an unknown product. Deduplicated per request. */
export const getProductAnalytics = cache(async function getProductAnalytics(slug: string, query: AnalyticsQuery) {
  const { data, response } = await productAnalytics({ ...(await withCookies()), path: { slug }, query });
  if (response?.status === 404) return null;
  if (!data) throw new Error("Could not load the product analytics");
  return data;
});

/** The demand–supply pressure for a product page, or null when the API has nothing to say. */
export async function getProductPressure(slug: string, region: number, trade: "ulgurji" | "chakana") {
  const { data } = await productPressure({ ...(await withCookies()), path: { slug }, query: { region, trade } });
  return data ?? null;
}

/** Official reference prices (the commodity exchange) for a product; empty when there are none or the API has no such data. */
export async function getProductReferences(slug: string, region: number, trade?: "ulgurji" | "chakana") {
  const { data } = await productReferences({ ...(await withCookies()), path: { slug }, query: { region, trade } });
  return data?.references ?? [];
}

/**
 * The buy/sell advice as sentences in the visitor's language (fixed templates,
 * or reworded when the API has that turned on); null when the API has none,
 * e.g. an older API without the endpoint. The page then lists the reasons itself.
 */
export async function getProductAdviceText(slug: string, region: number, trade: "ulgurji" | "chakana", lang: "uz" | "ru" | "en") {
  const { data } = await productAdviceText({ ...(await withCookies()), path: { slug }, query: { region, trade, lang } }).catch(() => ({ data: undefined }));
  return data?.text ? data : null;
}

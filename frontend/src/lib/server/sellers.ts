import "server-only";

import { cache } from "react";

import { getSeller } from "@/lib/api/gen/sdk.gen";
import { withCookies } from "@/lib/server-api";

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** A seller's public page with live listings, or null when there is no such seller. */
export const getSellerPage = cache(async function getSellerPage(id: string) {
  if (!uuid.test(id)) return null;
  const { data, response } = await getSeller({ ...(await withCookies()), path: { id } });
  if (response?.status === 404) return null;
  if (!data) throw new Error("Could not load the seller");
  return data;
});

import "server-only";

import { getRfq, listMyRfqs, rfqInbox } from "@/lib/api/gen/sdk.gen";
import { withCookies } from "@/lib/server-api";

/** The buyer's own requests, newest first. */
export async function getMyRfqs() {
  const { data } = await listMyRfqs(await withCookies());
  return data?.rfqs ?? [];
}

/** One request with its offers, or null when it is missing or not the visitor's. */
export async function getRfqDetail(id: number) {
  const { data, response } = await getRfq({ ...(await withCookies()), path: { id } });
  if (response?.status === 404) return null;
  if (!data) throw new Error("Could not load the request");
  return data;
}

/** Requests sent to the seller, the ones the buyer chose first. */
export async function getRfqInbox() {
  const { data } = await rfqInbox(await withCookies());
  return data?.rfqs ?? [];
}

import "server-only";

import {
  adminDeals,
  adminHeldRatings,
  adminListAnomalies,
  adminListEvents,
  adminMonitoring,
  adminReviewQueue,
  adminRoleRequests,
  adminSummary,
  adminSuspiciousSellers,
} from "@/lib/api/gen/sdk.gen";
import { getCatalogData, withCookies } from "@/lib/server-api";

/**
 * Everything the admin page shows: the summary, the queues, system health,
 * price anomalies, unusual sellers and the catalog lists the event form picks
 * from. Null when any call fails.
 */
export async function getAdminBoard() {
  const opts = await withCookies();
  const [summary, review, roles, deals, ratings, events, monitoring, anomalies, suspicious, catalog] = await Promise.all([
    adminSummary(opts),
    adminReviewQueue(opts),
    adminRoleRequests(opts),
    adminDeals(opts),
    adminHeldRatings(opts),
    adminListEvents(opts),
    adminMonitoring(opts),
    adminListAnomalies(opts),
    adminSuspiciousSellers(opts),
    getCatalogData(),
  ]);
  if (
    !summary.data ||
    !review.data ||
    !roles.data ||
    !deals.data ||
    !ratings.data ||
    !events.data ||
    !monitoring.data ||
    !anomalies.data ||
    !suspicious.data
  )
    return null;
  return {
    summary: summary.data,
    listings: review.data.listings,
    requests: roles.data.requests,
    deals: deals.data.deals,
    ratings: ratings.data.ratings,
    events: events.data.events,
    monitoring: monitoring.data,
    anomalies: anomalies.data.anomalies,
    suspicious: suspicious.data,
    // Ages ("12 minutes ago") are measured from the server's clock so the
    // server and browser renders agree.
    now: new Date().toISOString(),
    catalog: {
      // Viloyatlar (plus Toshkent shahri and Qoraqalpog'iston): the top level of the region tree.
      provinces: catalog.regions.filter((r) => !r.parent_id).map(({ id, name }) => ({ id, name })),
      products: catalog.products.map(({ id, slug, name, category }) => ({ id, slug, name, category })),
      categories: catalog.categories,
    },
  };
}

export type AdminBoardData = NonNullable<Awaited<ReturnType<typeof getAdminBoard>>>;
export type AdminCatalog = AdminBoardData["catalog"];

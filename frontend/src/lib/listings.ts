import type { MyListing } from "@/lib/api/gen/types.gen";
import { formatSom } from "@/lib/labels";

/** Whole days until the listing expires (negative once it has). */
export function daysLeft(iso: string, now = Date.now()) {
  return Math.ceil((new Date(iso).getTime() - now) / 86_400_000);
}

export type ListingState = "active" | "ending" | "review" | "hidden" | "expired" | "rejected";

/** One state per listing, in the order a seller has to act on them. */
export function stateOf(l: MyListing): ListingState {
  if (l.status === "rejected") return "rejected";
  if (l.status === "review") return "review";
  if (l.status === "hidden") return "hidden";
  if (l.expired) return "expired";
  return daysLeft(l.valid_until) <= 2 ? "ending" : "active";
}

/** Status dot colour per state; labels live in the "mine.states" messages. */
export const stateDots: Record<ListingState, string> = {
  active: "bg-primary",
  ending: "bg-warm",
  review: "bg-warm",
  hidden: "bg-muted-foreground/50",
  expired: "bg-destructive",
  rejected: "bg-destructive",
};

/** Needs a "narx o'zgarmadi" tap to stay visible. */
export function needsRefresh(l: MyListing) {
  const s = stateOf(l);
  return s === "ending" || s === "expired";
}

export function listingStats(listings: MyListing[]) {
  const states = listings.map(stateOf);
  return {
    total: listings.length,
    live: states.filter((s) => s === "active" || s === "ending").length,
    attention: listings.filter(needsRefresh).length,
    review: states.filter((s) => s === "review").length,
    views: listings.reduce((sum, l) => sum + l.views, 0),
  };
}

/** When automatic pricing is on but paused after a price set by hand: the end of the pause. */
export function pausedUntil(l: MyListing, now = Date.now()) {
  const until = l.pricing.paused_until;
  return l.pricing.mode === "auto" && until && Date.parse(until) > now ? until : undefined;
}

/** A quantity in the listing's unit: "2 000", or "12,5" when it has a fraction. */
export function formatAmount(n: number, decimal: (n: number, digits: number) => string) {
  return Number.isInteger(n) ? formatSom(n) : decimal(n, 2);
}

/** Sections of the seller's listing page, as they appear in the address (?bolim=…). */
export const manageTabs = { details: "tahrir", stats: "statistika", pricing: "narx" } as const;
export type ManageTab = keyof typeof manageTabs;

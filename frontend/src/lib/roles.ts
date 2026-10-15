/**
 * Account roles. Users pick one of two: an entrepreneur both sells (listings,
 * prices, offers) and buys (requests); a market administration only watches
 * its market and needs an admin's approval. "admin" is a system role.
 */
export const SELF_ROLES = ["entrepreneur", "market_admin"] as const;
export type SelfRole = (typeof SELF_ROLES)[number];

// Accounts from before the two-role model may still carry these until the
// backend migrates them; treat them as entrepreneurs.
const LEGACY_ENTREPRENEUR = new Set(["seller", "buyer", "org_analyst"]);

export function isEntrepreneur(role: string | null | undefined): boolean {
  return role === "entrepreneur" || (!!role && LEGACY_ENTREPRENEUR.has(role));
}

export function isMarketAdmin(role: string | null | undefined): boolean {
  return role === "market_admin";
}

/** Normalises a stored role to one a user can pick in the profile form. */
export function selfRole(role: string | null | undefined): SelfRole | undefined {
  if (isEntrepreneur(role)) return "entrepreneur";
  if (isMarketAdmin(role)) return "market_admin";
  return undefined;
}
